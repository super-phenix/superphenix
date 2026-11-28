package project

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testProjectID = "6ba7b810-9dad-41d1-80b4-00c04fd430c8"

func newTestReconciler(t *testing.T, objects ...client.Object) *Reconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := argov1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	return &Reconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(&operatorv1alpha1.Project{}).Build(),
		Scheme: scheme,
		// Same namespace as newTestProject, as in production.
		OperatorNamespace: "spx",
	}
}

func newTestProject() *operatorv1alpha1.Project {
	return &operatorv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-project",
			Namespace: "spx",
			UID:       types.UID("project-uid"),
		},
		Spec: operatorv1alpha1.ProjectSpec{
			ProjectID: testProjectID,
		},
	}
}

func TestEnsureRepositorySecretAndCredentialRotation(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Spec.GitOps = &operatorv1alpha1.ProjectGitOpsSpec{
		ValuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{
			RepoURL: "https://example.test/values.git",
			Credentials: &operatorv1alpha1.ProjectGitCredentialsSpec{
				Username:           "git-user",
				Password:           "old-password",
				Insecure:           true,
				ForceHttpBasicAuth: true,
				EnableLfs:          true,
				SshPrivateKey:      "private-key",
			},
		},
	}
	r := newTestReconciler(t)
	r.OperatorNamespace = "superphenix-operator"

	if err := r.ensureRepositorySecret(ctx, project, project.Spec.GitOps.ValuesLocation); err != nil {
		t.Fatal(err)
	}
	// ArgoCD only reads repository Secrets from its own (the operator) namespace.
	key := types.NamespacedName{Name: "repository-" + project.Spec.ProjectID, Namespace: r.OperatorNamespace}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, key, secret); err != nil {
		t.Fatal(err)
	}
	if secret.Labels[repositorySecretTypeLabel] != "repository" {
		t.Fatalf("expected repository type label, got %q", secret.Labels[repositorySecretTypeLabel])
	}
	// Only the operator deletes it: no finalizer to get stuck on, no owner reference for GC to act on.
	if len(secret.Finalizers) != 0 || len(secret.OwnerReferences) != 0 {
		t.Fatalf("expected no finalizers nor owner references, got %v / %#v", secret.Finalizers, secret.OwnerReferences)
	}
	for key, want := range map[string]string{
		"url":                "https://example.test/values.git",
		"username":           "git-user",
		"password":           "old-password",
		"insecure":           "true",
		"forceHttpBasicAuth": "true",
		"enableLfs":          "true",
		"sshPrivateKey":      "private-key",
	} {
		if got := string(secret.Data[key]); got != want {
			t.Errorf("secret data[%q] = %q, want %q", key, got, want)
		}
	}

	project.Spec.GitOps.ValuesLocation.RepoURL = "https://example.test/new-values.git"
	project.Spec.GitOps.ValuesLocation.Credentials.Password = "new-password"
	if err := r.ensureRepositorySecret(ctx, project, project.Spec.GitOps.ValuesLocation); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(ctx, key, secret); err != nil {
		t.Fatal(err)
	}
	if got := string(secret.Data["password"]); got != "new-password" {
		t.Fatalf("rotated password = %q, want new-password", got)
	}
	if got := string(secret.Data["url"]); got != "https://example.test/new-values.git" {
		t.Fatalf("updated repository URL = %q, want https://example.test/new-values.git", got)
	}
}

func TestEnsureArgoApplicationSources(t *testing.T) {
	tests := []struct {
		name            string
		valuesLocation  *operatorv1alpha1.ProjectValuesLocationSpec
		wantMultiSource bool
		wantSourceCount int
		wantTargetRev   string
		wantValuesRoot  string
	}{
		{
			name: "with values repository",
			valuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{
				RepoURL:        "https://example.test/values.git",
				TargetRevision: "release-1",
			},
			wantMultiSource: true,
			wantSourceCount: 2,
			wantTargetRev:   "release-1",
			wantValuesRoot:  "$values",
		},
		{
			name: "with values repository at its root",
			valuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{
				RepoURL: "https://example.test/values.git",
			},
			wantMultiSource: true,
			wantSourceCount: 2,
			wantTargetRev:   "HEAD",
			wantValuesRoot:  "$values",
		},
		{
			name:            "without values repository",
			wantSourceCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			project := newTestProject()
			if tt.valuesLocation != nil {
				project.Spec.GitOps = &operatorv1alpha1.ProjectGitOpsSpec{ValuesLocation: tt.valuesLocation}
			}
			r := newTestReconciler(t)
			if err := r.ensureArgoApplication(ctx, project, "spx-project", "gitops-project", "https://example.test/chart.git", "./chart", "main", "projectID: sentinel"); err != nil {
				t.Fatal(err)
			}

			app := &argov1alpha1.Application{}
			key := types.NamespacedName{Name: "gitops-project", Namespace: "spx-project"}
			if err := r.Get(ctx, key, app); err != nil {
				t.Fatal(err)
			}
			if !containsFinalizer(app.Finalizers, argocdResourcesFinalizer) {
				t.Fatalf("expected Argo CD resource finalizer, got %v", app.Finalizers)
			}
			if tt.wantMultiSource {
				if app.Spec.Source != nil || len(app.Spec.Sources) != tt.wantSourceCount {
					t.Fatalf("expected %d sources and no singular source, got source=%#v sources=%#v", tt.wantSourceCount, app.Spec.Source, app.Spec.Sources)
				}
				helm := app.Spec.Sources[0].Helm
				if helm == nil || !helm.IgnoreMissingValueFiles || len(helm.ValueFiles) != 2 || helm.ValueFiles[0] != tt.wantValuesRoot+"/**/*.yaml" || helm.ValueFiles[1] != tt.wantValuesRoot+"/**/*.yml" {
					t.Fatalf("unexpected Helm values configuration: %#v", helm)
				}
				if app.Spec.Sources[1].Ref != "values" || app.Spec.Sources[1].RepoURL != tt.valuesLocation.RepoURL || app.Spec.Sources[1].TargetRevision != tt.wantTargetRev || app.Spec.Sources[1].Path != "" {
					t.Fatalf("unexpected values source: %#v", app.Spec.Sources[1])
				}
			} else if app.Spec.Source == nil || len(app.Spec.Sources) != 0 {
				t.Fatalf("expected a chart-only singular source, got source=%#v sources=%#v", app.Spec.Source, app.Spec.Sources)
			}
		})
	}
}

func TestProjectDeletionWaitsForGitOpsApplication(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		name := "existing"
		if deleting {
			name = "deleting"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			project := newTestProject()
			project.Finalizers = []string{FinalizerName}
			deletionTimestamp := metav1.NewTime(time.Now())
			project.DeletionTimestamp = &deletionTimestamp
			projectSPXID := spxId.ToSPXID(project.Spec.ProjectID)
			app := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "gitops-" + projectSPXID,
					Namespace:  projectSPXID,
					Finalizers: []string{argocdResourcesFinalizer},
				},
			}
			if deleting {
				app.DeletionTimestamp = &deletionTimestamp
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repositorySecretName(project),
					Namespace: project.Namespace,
				},
				Data: map[string][]byte{"url": []byte("https://example.test/values.git")},
			}
			r := newTestReconciler(t, project, secret, app)

			stop, result, err := r.reconcileFinalizers(ctx, project)
			if err != nil {
				t.Fatal(err)
			}
			if !stop || result.RequeueAfter == 0 {
				t.Fatalf("expected project deletion to wait for GitOps Application, got stop=%t result=%#v", stop, result)
			}
			stored := &corev1.Secret{}
			if err := r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, stored); err != nil {
				t.Fatal(err)
			}
			if !stored.DeletionTimestamp.IsZero() {
				t.Fatal("repository Secret was deleted while the GitOps Application still exists")
			}
		})
	}
}

func TestProjectDeletionDeletesSecretAfterGitOpsApplicationIsGone(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Finalizers = []string{FinalizerName}
	deletionTimestamp := metav1.NewTime(time.Now())
	project.DeletionTimestamp = &deletionTimestamp
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      repositorySecretName(project),
			Namespace: project.Namespace,
		},
		Data: map[string][]byte{"url": []byte("https://example.test/values.git")},
	}
	unrelatedApplication := &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "argocd"},
		Spec: argov1alpha1.ApplicationSpec{
			Sources: []argov1alpha1.ApplicationSource{{RepoURL: "https://example.test/values.git", Ref: "values"}},
		},
	}
	r := newTestReconciler(t, project, secret, unrelatedApplication)

	stop, result, err := r.reconcileFinalizers(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if !stop || result.RequeueAfter == 0 {
		t.Fatalf("expected project deletion to wait for Secret deletion, got stop=%t result=%#v", stop, result)
	}
	err = r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected repository Secret to be deleted after GitOps Application disappeared, got err=%v", err)
	}

	// Next pass confirms the Secret is gone and releases the project.
	stop, result, err = r.reconcileFinalizers(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if !stop || result.RequeueAfter != 0 {
		t.Fatalf("expected project deletion to complete, got stop=%t result=%#v", stop, result)
	}
}

func TestProjectDeletionRecreatesLostSecretWhileApplicationExists(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Finalizers = []string{FinalizerName}
	now := metav1.NewTime(time.Now())
	project.DeletionTimestamp = &now
	project.Spec.GitOps = &operatorv1alpha1.ProjectGitOpsSpec{
		ValuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{RepoURL: "https://example.test/values.git"},
	}
	projectSPXID := spxId.ToSPXID(project.Spec.ProjectID)
	app := &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "gitops-" + projectSPXID,
			Namespace:         projectSPXID,
			DeletionTimestamp: &now,
			Finalizers:        []string{argocdResourcesFinalizer},
		},
	}
	r := newTestReconciler(t, project, app)

	if _, _, err := r.reconcileFinalizers(ctx, project); err != nil {
		t.Fatal(err)
	}
	stored := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: repositorySecretName(project), Namespace: project.Namespace}, stored); err != nil {
		t.Fatalf("expected lost repository Secret to be recreated while the Application exists: %v", err)
	}
	if got := string(stored.Data["url"]); got != "https://example.test/values.git" {
		t.Fatalf("recreated Secret url = %q", got)
	}
}

func TestFindProjectsForSecret(t *testing.T) {
	project := newTestProject()
	other := newTestProject()
	other.Name = "other-project"
	other.Spec.ProjectID = "6ba7b812-9dad-41d1-80b4-00c04fd430c8"
	r := newTestReconciler(t, project, other)
	r.OperatorNamespace = project.Namespace

	secretMeta := func(name, namespace string) client.Object {
		return &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	}

	requests := r.findProjectsForSecret(context.Background(), secretMeta(repositorySecretName(project), project.Namespace))
	if len(requests) != 1 || requests[0].Name != project.Name || requests[0].Namespace != project.Namespace {
		t.Fatalf("expected a request for %s, got %v", project.Name, requests)
	}
	for _, obj := range []client.Object{
		secretMeta("unrelated", project.Namespace),
		secretMeta(repositorySecretName(project), "spx-elsewhere"),
		secretMeta("repository-unknown", project.Namespace),
	} {
		if requests := r.findProjectsForSecret(context.Background(), obj); len(requests) != 0 {
			t.Fatalf("expected no request for %s/%s, got %v", obj.GetNamespace(), obj.GetName(), requests)
		}
	}
}

func TestRepositorySecretRequiresOperatorNamespace(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	valuesLocation := &operatorv1alpha1.ProjectValuesLocationSpec{RepoURL: "https://example.test/values.git"}
	r := newTestReconciler(t, project)
	r.OperatorNamespace = ""

	if err := r.ensureRepositorySecret(ctx, project, valuesLocation); err == nil {
		t.Fatal("expected an error without an operator namespace")
	}
	secrets := &corev1.SecretList{}
	if err := r.List(ctx, secrets); err != nil {
		t.Fatal(err)
	}
	if len(secrets.Items) != 0 {
		t.Fatalf("expected no Secret to be created outside the operator namespace, got %d", len(secrets.Items))
	}
	// Nothing can have been created, so deletion must not block.
	if remaining, err := r.deleteRepositorySecret(ctx, project); remaining != nil || err != nil {
		t.Fatalf("expected deletion to be a no-op, got remaining=%v err=%v", remaining, err)
	}
}

func TestRepositorySecretNameIsStableForProject(t *testing.T) {
	project := newTestProject()
	if got, want := repositorySecretName(project), "repository-"+project.Spec.ProjectID; got != want {
		t.Fatalf("repository Secret name = %q, want %q", got, want)
	}
}

func TestProjectDeletionCompletesWhenRepositorySecretIsAbsent(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Finalizers = []string{FinalizerName}
	project.DeletionTimestamp = func() *metav1.Time { now := metav1.NewTime(time.Now()); return &now }()
	r := newTestReconciler(t, project)

	stop, result, err := r.reconcileFinalizers(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if !stop || result.RequeueAfter != 0 {
		t.Fatalf("expected project deletion to complete without a repository Secret, got stop=%t result=%#v", stop, result)
	}
	stored := &operatorv1alpha1.Project{}
	err = r.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, stored)
	if err == nil && containsFinalizer(stored.Finalizers, FinalizerName) {
		t.Fatalf("expected project finalizer to be removed, got %v", stored.Finalizers)
	}
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
}

func TestBuildHelmValuesDoesNotContainRepositoryCredentials(t *testing.T) {
	project := newTestProject()
	project.Spec.GitOps = &operatorv1alpha1.ProjectGitOpsSpec{
		ValuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{
			RepoURL: "https://example.test/values.git",
			Credentials: &operatorv1alpha1.ProjectGitCredentialsSpec{
				Username: "private-user",
				Password: "private-password",
			},
		},
	}
	org := &operatorv1alpha1.Organization{Spec: operatorv1alpha1.OrganizationSpec{OrganizationID: "org-id"}}
	values, err := (&Reconciler{}).buildHelmValues(project, org, "org", "project")
	if err != nil {
		t.Fatal(err)
	}
	var helmValues map[string]interface{}
	if err := yaml.Unmarshal([]byte(values), &helmValues); err != nil {
		t.Fatalf("unmarshal Helm values: %v", err)
	}
	projectValues, ok := helmValues["project"].(map[string]interface{})
	if !ok {
		t.Fatalf("project Helm values missing: %s", values)
	}
	gitops, ok := projectValues["gitops"].(map[string]interface{})
	if !ok {
		t.Fatalf("project GitOps values missing: %s", values)
	}
	valuesLocation, ok := gitops["valuesLocation"].(map[string]interface{})
	if !ok {
		t.Fatalf("values location missing: %s", values)
	}
	if got := valuesLocation["repoURL"]; got != "https://example.test/values.git" {
		t.Errorf("repoURL = %v, want values repository URL", got)
	}
	credentials, ok := valuesLocation["credentials"].(map[string]interface{})
	if !ok {
		t.Fatalf("credentials missing or malformed: %s", values)
	}
	if got := credentials["username"]; got != "private-user" {
		t.Errorf("credentials.username = %v, want private-user", got)
	}
	if got := credentials["password"]; got != "private-password" {
		t.Errorf("credentials.password = %v, want private-password", got)
	}
}

func containsFinalizer(finalizers []string, name string) bool {
	for _, finalizer := range finalizers {
		if finalizer == name {
			return true
		}
	}
	return false
}

func storedCondition(t *testing.T, r *Reconciler, project *operatorv1alpha1.Project, condType string) *metav1.Condition {
	t.Helper()
	stored := &operatorv1alpha1.Project{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, stored); err != nil {
		t.Fatal(err)
	}
	return meta.FindStatusCondition(stored.Status.Conditions, condType)
}

func TestProjectDeletionReportsApplicationDeletionError(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Finalizers = []string{FinalizerName}
	now := metav1.NewTime(time.Now())
	project.DeletionTimestamp = &now
	projectSPXID := spxId.ToSPXID(project.Spec.ProjectID)
	app := &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "gitops-" + projectSPXID,
			Namespace:         projectSPXID,
			DeletionTimestamp: &now,
			Finalizers:        []string{argocdResourcesFinalizer},
		},
		Status: argov1alpha1.ApplicationStatus{
			Conditions: []argov1alpha1.ApplicationCondition{
				{Type: argov1alpha1.ApplicationConditionDeletionError, Message: "forbidden: cannot delete pods"},
			},
		},
	}
	r := newTestReconciler(t, project, app)

	if _, _, err := r.reconcileFinalizers(ctx, project); err != nil {
		t.Fatal(err)
	}
	deleting := storedCondition(t, r, project, operatorv1alpha1.ConditionTypeDeleting)
	if deleting == nil || deleting.Status != metav1.ConditionTrue || deleting.Reason != operatorv1alpha1.ReasonWaitingForApplication {
		t.Fatalf("unexpected Deleting condition: %#v", deleting)
	}
	if !strings.Contains(deleting.Message, "forbidden: cannot delete pods") {
		t.Fatalf("expected Argo CD deletion error in message, got %q", deleting.Message)
	}
	gitops := storedCondition(t, r, project, operatorv1alpha1.ConditionTypeGitOpsReady)
	if gitops == nil || gitops.Status != metav1.ConditionFalse || gitops.Reason != operatorv1alpha1.ReasonGitOpsProjectDeleting {
		t.Fatalf("unexpected GitOpsReady condition: %#v", gitops)
	}
}

func TestProjectDeletionReportsBlockedRepositorySecret(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Finalizers = []string{FinalizerName}
	now := metav1.NewTime(time.Now())
	project.DeletionTimestamp = &now
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       repositorySecretName(project),
			Namespace:  project.Namespace,
			Finalizers: []string{"example.com/other"},
		},
	}
	r := newTestReconciler(t, project, secret)

	for range 2 {
		stop, result, err := r.reconcileFinalizers(ctx, project)
		if err != nil {
			t.Fatal(err)
		}
		if !stop || result.RequeueAfter == 0 {
			t.Fatalf("expected deletion to wait on the blocked Secret, got stop=%t result=%#v", stop, result)
		}
	}
	deleting := storedCondition(t, r, project, operatorv1alpha1.ConditionTypeDeleting)
	if deleting == nil || deleting.Reason != operatorv1alpha1.ReasonWaitingForRepositorySecret {
		t.Fatalf("unexpected Deleting condition: %#v", deleting)
	}
	if !strings.Contains(deleting.Message, "example.com/other") {
		t.Fatalf("expected the blocking finalizer in message, got %q", deleting.Message)
	}
}

func TestReconcileGitOpsSetsGitOpsReady(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Spec.OrganizationRef.Name = "org"
	project.Spec.GitOps = &operatorv1alpha1.ProjectGitOpsSpec{
		ValuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{RepoURL: "https://example.test/values.git"},
	}

	// Missing Organization: reason reported, error returned.
	r := newTestReconciler(t, project)
	if err := r.reconcileGitOps(ctx, project); err == nil {
		t.Fatal("expected an error without Organization")
	}
	cond := meta.FindStatusCondition(project.Status.Conditions, operatorv1alpha1.ConditionTypeGitOpsReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != operatorv1alpha1.ReasonOrganizationFetchFailed {
		t.Fatalf("unexpected GitOpsReady condition: %#v", cond)
	}

	// Organization present: Application and Secret created.
	org := &operatorv1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{Name: "org", Namespace: project.Namespace},
		Spec:       operatorv1alpha1.OrganizationSpec{OrganizationID: "6ba7b811-9dad-41d1-80b4-00c04fd430c8"},
	}
	r = newTestReconciler(t, project, org)
	if err := r.reconcileGitOps(ctx, project); err != nil {
		t.Fatal(err)
	}
	cond = meta.FindStatusCondition(project.Status.Conditions, operatorv1alpha1.ConditionTypeGitOpsReady)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != operatorv1alpha1.ReasonGitOpsReconciled {
		t.Fatalf("unexpected GitOpsReady condition: %#v", cond)
	}
	if !strings.Contains(cond.Message, repositorySecretName(project)) {
		t.Fatalf("expected repository Secret in message, got %q", cond.Message)
	}
}

func TestBuildHelmValuesIgnoresLegacyValuesPath(t *testing.T) {
	var proj operatorv1alpha1.Project
	if err := json.Unmarshal([]byte(`{
		"spec": {
			"projectID": "project-id",
			"gitops": {
				"valuesLocation": {
					"repoURL": "https://example.com/values.git",
					"path": "./untrusted",
					"targetRevision": "production",
					"credentials": {
						"username": "git-user",
						"password": "git-password"
					}
				}
			}
		}
	}`), &proj); err != nil {
		t.Fatalf("unmarshal project: %v", err)
	}

	valuesYAML, err := (&Reconciler{}).buildHelmValues(&proj, &operatorv1alpha1.Organization{
		Spec: operatorv1alpha1.OrganizationSpec{OrganizationID: "organization-id"},
	}, "organization", "project")
	if err != nil {
		t.Fatalf("build Helm values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYAML), &values); err != nil {
		t.Fatalf("unmarshal Helm values: %v", err)
	}
	projectValues, ok := values["project"].(map[string]interface{})
	if !ok {
		t.Fatalf("project Helm values missing: %s", valuesYAML)
	}
	if got := projectValues["projectID"]; got != "project-id" {
		t.Errorf("projectID = %v, want project-id", got)
	}
	if got := projectValues["name"]; got != "project" {
		t.Errorf("project name = %v, want project", got)
	}
	gitops, ok := projectValues["gitops"].(map[string]interface{})
	if !ok {
		t.Fatalf("project GitOps values missing: %s", valuesYAML)
	}
	valuesLocation, ok := gitops["valuesLocation"].(map[string]interface{})
	if !ok {
		t.Fatalf("values location missing: %s", valuesYAML)
	}
	if got := valuesLocation["repoURL"]; got != "https://example.com/values.git" {
		t.Errorf("repoURL = %v, want values repository URL", got)
	}
	if got := valuesLocation["targetRevision"]; got != "production" {
		t.Errorf("targetRevision = %v, want production", got)
	}
	if _, found := valuesLocation["path"]; found {
		t.Errorf("legacy values repository path must not be forwarded: %s", valuesYAML)
	}
	credentials, ok := valuesLocation["credentials"].(map[string]interface{})
	if !ok {
		t.Fatalf("credentials missing or malformed: %s", valuesYAML)
	}
	if got := credentials["username"]; got != "git-user" {
		t.Errorf("credentials.username = %v, want git-user", got)
	}
	if got := credentials["password"]; got != "git-password" {
		t.Errorf("credentials.password = %v, want git-password", got)
	}
	organization, ok := values["organization"].(map[string]interface{})
	if !ok {
		t.Fatalf("organization Helm values missing: %s", valuesYAML)
	}
	if got := organization["organizationID"]; got != "organization-id" {
		t.Errorf("organizationID = %v, want organization-id", got)
	}
	if got := organization["name"]; got != "organization" {
		t.Errorf("organization name = %v, want organization", got)
	}
}

func TestBuildHelmValuesHandlesAbsentValuesLocationFields(t *testing.T) {
	tests := []struct {
		name   string
		gitops *operatorv1alpha1.ProjectGitOpsSpec
	}{
		{name: "nil GitOps"},
		{name: "nil values location", gitops: &operatorv1alpha1.ProjectGitOpsSpec{}},
		{
			name: "empty URL and revision without credentials",
			gitops: &operatorv1alpha1.ProjectGitOpsSpec{
				ValuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := newTestProject()
			project.Spec.GitOps = tt.gitops
			valuesYAML, err := (&Reconciler{}).buildHelmValues(project, &operatorv1alpha1.Organization{
				Spec: operatorv1alpha1.OrganizationSpec{OrganizationID: "organization-id"},
			}, "organization", "project")
			if err != nil {
				t.Fatalf("build Helm values: %v", err)
			}

			var values map[string]interface{}
			if err := yaml.Unmarshal([]byte(valuesYAML), &values); err != nil {
				t.Fatalf("unmarshal Helm values: %v", err)
			}
			projectValues, ok := values["project"].(map[string]interface{})
			if !ok {
				t.Fatalf("project Helm values missing: %s", valuesYAML)
			}
			if got := projectValues["projectID"]; got != testProjectID {
				t.Errorf("projectID = %v, want %s", got, testProjectID)
			}
			if got := projectValues["name"]; got != "project" {
				t.Errorf("project name = %v, want project", got)
			}
			gitops, ok := projectValues["gitops"].(map[string]interface{})
			if !ok {
				t.Fatalf("project GitOps values missing: %s", valuesYAML)
			}
			valuesLocation, ok := gitops["valuesLocation"].(map[string]interface{})
			if !ok {
				t.Fatalf("values location missing: %s", valuesYAML)
			}
			if got := valuesLocation["repoURL"]; got != "" {
				t.Errorf("repoURL = %v, want empty", got)
			}
			if got := valuesLocation["targetRevision"]; got != "" {
				t.Errorf("targetRevision = %v, want empty", got)
			}
			if _, ok := valuesLocation["credentials"].(map[string]interface{}); !ok {
				t.Errorf("empty credentials missing or malformed: %#v", valuesLocation["credentials"])
			}
		})
	}
}

func TestSfsGitopsApplicationsDoNotSetValuesRepositoryPath(t *testing.T) {
	chart, err := loader.Load("../../../components/dependencies/sfs-gitops")
	if err != nil {
		t.Fatalf("load sfs-gitops chart: %v", err)
	}

	values := chartutil.Values{
		"global": map[string]interface{}{
			"organization": map[string]interface{}{
				"organizationID": "organization-id",
				"name":           "organization",
			},
			"project": map[string]interface{}{
				"projectID": "project-id",
				"name":      "project",
				"availabilityZones": []interface{}{map[string]interface{}{
					"name":    "fr01-agc01",
					"cluster": "cluster-1",
				}},
				"gitops": map[string]interface{}{
					"valuesLocation": map[string]interface{}{
						"repoURL":        "https://example.com/values.git",
						"path":           "./untrusted",
						"targetRevision": "production",
					},
				},
			},
		},
		"kubernetes": map[string]interface{}{
			"cluster-1": map[string]interface{}{
				"location":    "fr01-agc01",
				"name":        "cluster-1",
				"kubeVersion": "1.35.0",
			},
		},
		"backups": map[string]interface{}{
			"backup-1": map[string]interface{}{
				"location": "fr01-agc01",
				"name":     "backup-1",
			},
		},
	}
	renderValues, err := chartutil.ToRenderValues(chart, values, chartutil.ReleaseOptions{
		Name:      "values-path-test",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}, chartutil.DefaultCapabilities)
	if err != nil {
		t.Fatalf("prepare chart values: %v", err)
	}

	renderedTemplates, err := (engine.Engine{}).Render(chart, renderValues)
	if err != nil {
		t.Fatalf("render sfs-gitops chart: %v", err)
	}

	for _, templatePath := range []string{
		"sfs-gitops/templates/kubernetes/application.yaml",
		"sfs-gitops/templates/backups/application.yaml",
	} {
		template, found := renderedTemplates[templatePath]
		if !found {
			t.Fatalf("rendered chart is missing %s", templatePath)
		}
		assertValuesSourceHasNoPath(t, template)
	}
}

func assertValuesSourceHasNoPath(t *testing.T, rendered string) {
	t.Helper()

	decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(rendered), 4096)
	applicationCount := 0
	for {
		application := map[string]interface{}{}
		err := decoder.Decode(&application)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode rendered Application: %v", err)
		}
		if application["kind"] != "Application" {
			continue
		}
		applicationCount++

		sources, found, err := unstructured.NestedSlice(application, "spec", "sources")
		if err != nil || !found || len(sources) == 0 {
			t.Fatalf("Application sources = %v, found = %t, error = %v; want at least one source", sources, found, err)
		}
		for _, source := range sources {
			sourceMap, ok := source.(map[string]interface{})
			if !ok {
				t.Fatalf("source has unexpected type %T", source)
			}
			if sourceMap["ref"] == "values" || sourceMap["repoURL"] == "https://example.com/values.git" {
				t.Errorf("child Application must not render the values repository: %#v", sourceMap)
			}
		}
	}
	if applicationCount != 1 {
		t.Errorf("rendered %d Applications, want exactly one", applicationCount)
	}
}

package project

import (
	"context"
	"strings"
	"testing"
	"time"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
		Scheme: scheme,
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

	if err := r.ensureRepositorySecret(ctx, project, project.Spec.GitOps.ValuesLocation); err != nil {
		t.Fatal(err)
	}
	key := types.NamespacedName{Name: "repository-" + project.Spec.ProjectID, Namespace: project.Namespace}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, key, secret); err != nil {
		t.Fatal(err)
	}
	if secret.Labels[repositorySecretTypeLabel] != "repository" {
		t.Fatalf("expected repository type label, got %q", secret.Labels[repositorySecretTypeLabel])
	}
	if !containsFinalizer(secret.Finalizers, FinalizerName) {
		t.Fatalf("expected secret finalizer %q", FinalizerName)
	}
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].UID != project.UID || secret.OwnerReferences[0].Kind != "Project" {
		t.Fatalf("expected Project owner reference, got %#v", secret.OwnerReferences)
	}
	if secret.Namespace != project.Namespace {
		t.Fatalf("repository secret namespace = %q, want project namespace %q", secret.Namespace, project.Namespace)
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
	}{
		{
			name: "with values repository",
			valuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{
				RepoURL:        "https://example.test/values.git",
				Path:           "ignored/path",
				TargetRevision: "release-1",
			},
			wantMultiSource: true,
			wantSourceCount: 2,
			wantTargetRev:   "release-1",
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
				if helm == nil || !helm.IgnoreMissingValueFiles || len(helm.ValueFiles) != 2 || helm.ValueFiles[0] != "$values/**/*.yaml" || helm.ValueFiles[1] != "$values/**/*.yml" {
					t.Fatalf("unexpected Helm values configuration: %#v", helm)
				}
				if app.Spec.Sources[1].Ref != "values" || app.Spec.Sources[1].TargetRevision != tt.wantTargetRev {
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
					Name:       repositorySecretName(project),
					Namespace:  project.Namespace,
					Finalizers: []string{FinalizerName},
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
			Name:       repositorySecretName(project),
			Namespace:  project.Namespace,
			Finalizers: []string{FinalizerName},
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

func TestEnsureRepositorySecretReleasesTerminatingSecret(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Spec.GitOps = &operatorv1alpha1.ProjectGitOpsSpec{
		ValuesLocation: &operatorv1alpha1.ProjectValuesLocationSpec{RepoURL: "https://example.test/values.git"},
	}
	now := metav1.NewTime(time.Now())
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              repositorySecretName(project),
			Namespace:         project.Namespace,
			DeletionTimestamp: &now,
			Finalizers:        []string{FinalizerName},
		},
	}
	r := newTestReconciler(t, project, secret)

	if err := r.ensureRepositorySecret(ctx, project, project.Spec.GitOps.ValuesLocation); err == nil {
		t.Fatal("expected an error to requeue while the terminating Secret is released")
	}
	if err := r.ensureRepositorySecret(ctx, project, project.Spec.GitOps.ValuesLocation); err != nil {
		t.Fatalf("expected Secret to be recreated, got %v", err)
	}
	stored := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, stored); err != nil {
		t.Fatal(err)
	}
	if !stored.DeletionTimestamp.IsZero() || !containsFinalizer(stored.Finalizers, FinalizerName) {
		t.Fatalf("expected a fresh protected Secret, got %#v", stored.ObjectMeta)
	}
}

func TestCleanupRepositorySecretReleasesFinalizer(t *testing.T) {
	ctx := context.Background()
	project := newTestProject()
	project.Finalizers = []string{FinalizerName}
	project.DeletionTimestamp = func() *metav1.Time { now := metav1.NewTime(time.Now()); return &now }()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              repositorySecretName(project),
			Namespace:         project.Namespace,
			DeletionTimestamp: project.DeletionTimestamp,
			Finalizers:        []string{FinalizerName},
		},
	}
	r := newTestReconciler(t, project, secret)

	stop, result, err := r.reconcileFinalizers(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if !stop || result.RequeueAfter == 0 {
		t.Fatalf("expected project deletion to wait for Secret finalization, got stop=%t result=%#v", stop, result)
	}
	stored := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, stored)
	if err == nil && containsFinalizer(stored.Finalizers, FinalizerName) {
		t.Fatalf("expected repository Secret finalizer to be removed, got %v", stored.Finalizers)
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Fatal(err)
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
	for _, forbidden := range []string{"private-user", "private-password", "values.git", "credentials:"} {
		if strings.Contains(values, forbidden) {
			t.Errorf("Helm values leaked %q: %s", forbidden, values)
		}
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

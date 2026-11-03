package quota

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	orgId     = "11111111-1111-1111-1111-111111111111"
	projectId = "22222222-2222-2222-2222-222222222222"
	namespace = "spx-22222222-2222-2222-2222-222222222222"
	quotaName = "quota-22222222-2222-2222-2222-222222222222"
	spxPrefix = "spx"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		hard    map[string]string
		wantErr bool
	}{
		{"empty", map[string]string{}, false},
		{"valid", map[string]string{"requests.cpu": "4", "limits.memory": "8Gi", "count/virtualmachines.kubevirt.io": "0"}, false},
		{"invalid name", map[string]string{"bad name": "1"}, true},
		{"yaml injection", map[string]string{"pods\n  secrets": "1"}, true},
		{"invalid quantity", map[string]string{"pods": "abc"}, true},
		{"negative", map[string]string{"pods": "-1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.hard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyAndDelete(t *testing.T) {
	fakeClient := fake.NewClientset()
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = spxPrefix
	ctx := context.Background()

	hard, err := Validate(map[string]string{"requests.cpu": "4", "pods": "10"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, orgId, projectId, hard, 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := fakeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err != nil {
		t.Fatalf("namespace not created: %v", err)
	}

	// Replacing the quota drops the resources that are no longer limited.
	hard, _ = Validate(map[string]string{"pods": "0"})
	if err := Apply(ctx, orgId, projectId, hard, 0); err != nil {
		t.Fatalf("update: %v", err)
	}
	rq, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("quota not found: %v", err)
	}
	if len(rq.Spec.Hard) != 1 || rq.Spec.Hard.Pods().String() != "0" {
		t.Errorf("unexpected hard %v", rq.Spec.Hard)
	}
	if len(rq.Labels) == 0 {
		t.Errorf("expected spx labels on the quota")
	}

	if err := Delete(ctx, orgId, projectId, 0); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := Delete(ctx, orgId, projectId, 0); err != nil {
		t.Fatalf("delete should be idempotent: %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("quota should be kept: %v", err)
	}
	if len(rq.Spec.Hard) != 0 {
		t.Errorf("expected no limit, got %v", rq.Spec.Hard)
	}
}

// A quota rendered by the sfs-iaas chart is the same object: the controller replaces its limits
// and keeps its labels instead of creating a second ResourceQuota.
func TestApplyAdoptsChartQuota(t *testing.T) {
	fakeClient := fake.NewClientset(&v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "Helm"},
		},
		Spec: v1.ResourceQuotaSpec{Hard: v1.ResourceList{v1.ResourcePods: resource.MustParse("100")}},
	})
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = spxPrefix
	ctx := context.Background()

	hard, _ := Validate(map[string]string{"pods": "5"})
	if err := Apply(ctx, orgId, projectId, hard, 0); err != nil {
		t.Fatalf("apply: %v", err)
	}

	list, err := fakeClient.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected a single ResourceQuota, got %d", len(list.Items))
	}
	rq := list.Items[0]
	if rq.Spec.Hard.Pods().String() != "5" {
		t.Errorf("unexpected hard %v", rq.Spec.Hard)
	}
	if rq.Labels["app.kubernetes.io/managed-by"] != "Helm" {
		t.Errorf("chart labels should be kept, got %v", rq.Labels)
	}
	if rq.Labels["superphenix.net/organizationID"] != "spx-"+orgId || rq.Labels["superphenix.net/projectID"] != "spx-"+projectId {
		t.Errorf("spx labels should match the chart ones, got %v", rq.Labels)
	}
}

// A GitOps-managed quota is skipped, like the other GitOps resources: neither Apply nor Delete
// touches it.
func TestGitopsQuotaIsSkipped(t *testing.T) {
	fakeClient := fake.NewClientset(&v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: namespace,
			Labels:    map[string]string{"superphenix.net/gitops": "true"},
		},
		Spec: v1.ResourceQuotaSpec{Hard: v1.ResourceList{v1.ResourcePods: resource.MustParse("100")}},
	})
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = spxPrefix
	ctx := context.Background()

	hard, _ := Validate(map[string]string{"pods": "5"})
	if err := Apply(ctx, orgId, projectId, hard, 0); err != nil {
		t.Fatalf("apply: %v", err)
	}
	rq, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rq.Spec.Hard.Pods().String() != "100" {
		t.Errorf("gitops quota should not be updated, got %v", rq.Spec.Hard)
	}
	if len(rq.Labels) != 1 {
		t.Errorf("gitops quota labels should not be changed, got %v", rq.Labels)
	}

	if err := Delete(ctx, orgId, projectId, 0); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{}); err != nil {
		t.Errorf("gitops quota should not be deleted: %v", err)
	}
}

func TestGet(t *testing.T) {
	fakeClient := fake.NewClientset()
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = spxPrefix
	ctx := context.Background()

	hard, used, err := Get(ctx, projectId)
	if err != nil || len(hard) != 0 || len(used) != 0 {
		t.Fatalf("no quota: got hard=%v used=%v err=%v", hard, used, err)
	}

	_, err = fakeClient.CoreV1().ResourceQuotas(namespace).Create(ctx, &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
		Spec:       v1.ResourceQuotaSpec{Hard: v1.ResourceList{"requests.cpu": resource.MustParse("4")}},
		Status: v1.ResourceQuotaStatus{
			Hard: v1.ResourceList{"requests.cpu": resource.MustParse("4")},
			Used: v1.ResourceList{"requests.cpu": resource.MustParse("1500m")},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hard, used, err = Get(ctx, projectId)
	if err != nil {
		t.Fatal(err)
	}
	if q := hard["requests.cpu"]; q.String() != "4" {
		t.Fatalf("hard = %v", hard)
	}
	if q := used["requests.cpu"]; q.String() != "1500m" {
		t.Fatalf("used = %v", used)
	}
}

func TestApplyRefusesLoweringBelowUsage(t *testing.T) {
	fakeClient := fake.NewClientset(&v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
		Spec: v1.ResourceQuotaSpec{Hard: v1.ResourceList{
			"requests.cpu": resource.MustParse("10"), "requests.memory": resource.MustParse("8Gi"),
		}},
		Status: v1.ResourceQuotaStatus{Used: v1.ResourceList{
			"requests.cpu": resource.MustParse("6"), "requests.memory": resource.MustParse("10Gi"),
		}},
	})
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = spxPrefix
	ctx := context.Background()

	apply := func(cpu, memory string) error {
		hard, err := Validate(map[string]string{"requests.cpu": cpu, "requests.memory": memory, "pods": "0"})
		if err != nil {
			t.Fatal(err)
		}
		return Apply(ctx, orgId, projectId, hard, 0)
	}
	currentCPU := func() string {
		rq, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		q := rq.Spec.Hard["requests.cpu"]
		return q.String()
	}

	// Lowered below the 6 cpu used: refused, the quota is unchanged.
	if err := apply("5", "8Gi"); !errors.Is(err, ErrBelowUsage) {
		t.Fatalf("expected ErrBelowUsage, got %v", err)
	}
	if got := currentCPU(); got != "10" {
		t.Errorf("quota changed to %s", got)
	}
	// Memory is already above its quota (stored before the check): kept as is, it is accepted,
	// and cpu can be lowered down to exactly what is used. A new resource (pods) has no usage.
	if err := apply("6", "8Gi"); err != nil {
		t.Fatalf("lowering to the usage: %v", err)
	}
	if got := currentCPU(); got != "6" {
		t.Errorf("expected 6 cpu, got %s", got)
	}
	// Memory lowered further while above its usage: refused. Raising is always accepted.
	if err := apply("6", "7Gi"); !errors.Is(err, ErrBelowUsage) {
		t.Fatalf("expected ErrBelowUsage, got %v", err)
	}
	if err := apply("20", "16Gi"); err != nil {
		t.Fatalf("raise: %v", err)
	}
}

func TestApplyWithRevision(t *testing.T) {
	fakeClient := fake.NewClientset()
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = spxPrefix
	ctx := context.Background()

	hard, err := Validate(map[string]string{"requests.cpu": "4"})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Initial apply with revision 10 creates ResourceQuota with revision annotation 10.
	if err := Apply(ctx, orgId, projectId, hard, 10); err != nil {
		t.Fatalf("apply with rev 10: %v", err)
	}
	rq, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("quota not found: %v", err)
	}
	if rq.Annotations[spxId.SpxAnnotationQuotaRevision] != "10" {
		t.Fatalf("expected annotation 10, got %s", rq.Annotations[spxId.SpxAnnotationQuotaRevision])
	}
	q := rq.Spec.Hard[v1.ResourceName("requests.cpu")]
	if q.String() != "4" {
		t.Fatalf("expected 4 cpu, got %s", q.String())
	}

	// 2. Stale apply with revision 5 (older than 10) is refused.
	hardStale, _ := Validate(map[string]string{"requests.cpu": "100"})
	if err := Apply(ctx, orgId, projectId, hardStale, 5); !errors.Is(err, ErrStale) {
		t.Fatalf("stale apply: expected ErrStale, got %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q = rq.Spec.Hard[v1.ResourceName("requests.cpu")]
	if q.String() != "4" {
		t.Fatalf("stale apply should have been ignored, cpu is %s", q.String())
	}

	// 3. Stale delete with revision 8 (older than 10) is refused.
	if err := Delete(ctx, orgId, projectId, 8); !errors.Is(err, ErrStale) {
		t.Fatalf("stale delete: expected ErrStale, got %v", err)
	}
	if _, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{}); err != nil {
		t.Fatalf("stale delete removed the quota: %v", err)
	}

	// 4. Newer apply with revision 15 updates the quota and annotation.
	hardNew, _ := Validate(map[string]string{"requests.cpu": "8"})
	if err := Apply(ctx, orgId, projectId, hardNew, 15); err != nil {
		t.Fatalf("newer apply failed: %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rq.Annotations[spxId.SpxAnnotationQuotaRevision] != "15" {
		t.Fatalf("expected annotation 15, got %s", rq.Annotations[spxId.SpxAnnotationQuotaRevision])
	}
	q = rq.Spec.Hard[v1.ResourceName("requests.cpu")]
	if q.String() != "8" {
		t.Fatalf("expected 8 cpu, got %s", q.String())
	}

	// 5. Newer delete with revision 20 keeps the quota with no limit and annotation 20.
	if err := Delete(ctx, orgId, projectId, 20); err != nil {
		t.Fatalf("delete with rev 20 failed: %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("quota should have been kept: %v", err)
	}
	if len(rq.Spec.Hard) != 0 {
		t.Fatalf("expected no limit, got %v", rq.Spec.Hard)
	}
	if rq.Annotations[spxId.SpxAnnotationQuotaRevision] != "20" {
		t.Fatalf("expected annotation 20, got %s", rq.Annotations[spxId.SpxAnnotationQuotaRevision])
	}

	// 6. Stale apply with revision 15 after the delete is refused.
	if err := Apply(ctx, orgId, projectId, hardNew, 15); !errors.Is(err, ErrStale) {
		t.Fatalf("stale apply after delete: expected ErrStale, got %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rq.Spec.Hard) != 0 {
		t.Fatalf("stale apply after delete should have been ignored, hard is %v", rq.Spec.Hard)
	}

	// 7. Apply without revision is older than the annotation and refused.
	if err := Apply(ctx, orgId, projectId, hardNew, 0); !errors.Is(err, ErrStale) {
		t.Fatalf("apply without revision: expected ErrStale, got %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rq.Spec.Hard) != 0 {
		t.Fatalf("apply without revision should have been ignored, hard is %v", rq.Spec.Hard)
	}

	// 8. Delete without revision is older than the annotation and refused.
	if err := Delete(ctx, orgId, projectId, 0); !errors.Is(err, ErrStale) {
		t.Fatalf("delete without revision: expected ErrStale, got %v", err)
	}
	if _, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{}); err != nil {
		t.Fatalf("delete without revision removed the quota: %v", err)
	}

	// 9. Apply at the revision of the annotation is applied.
	if err := Apply(ctx, orgId, projectId, hardNew, 20); err != nil {
		t.Fatalf("apply at the same revision failed: %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q = rq.Spec.Hard[v1.ResourceName("requests.cpu")]
	if q.String() != "8" {
		t.Fatalf("apply at the same revision should have been applied, cpu is %s", q.String())
	}
}

// Without a ResourceQuota, a delete with a revision creates an empty one annotated with it in an
// active project namespace, so that an older push is refused later. Nothing is created without
// revision, nor without an active namespace: the namespace is never created.
func TestDeleteWithoutQuota(t *testing.T) {
	hard, err := Validate(map[string]string{"requests.cpu": "4"})
	if err != nil {
		t.Fatal(err)
	}
	active := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	terminating := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}, Status: v1.NamespaceStatus{Phase: v1.NamespaceTerminating}}
	tests := []struct {
		name      string
		namespace *v1.Namespace
		// createTerminating makes the first create of a ResourceQuota fail as in a terminating namespace.
		createTerminating bool
		revision          int64
		wantNamespace     bool
		wantQuota         bool
		// wantLaterApply is the error of an apply at revision 10 after the delete.
		wantLaterApply error
	}{
		{"revision and namespace", active, false, 20, true, true, ErrStale},
		{"revision without namespace", nil, false, 20, false, false, nil},
		{"revision, namespace terminating", terminating, false, 20, true, false, nil},
		{"revision, create refused in a terminating namespace", active, true, 20, true, false, nil},
		{"no revision", active, false, 0, true, false, nil},
		{"no revision nor namespace", nil, false, 0, false, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []runtime.Object
			if tt.namespace != nil {
				objects = append(objects, tt.namespace.DeepCopy())
			}
			fakeClient := fake.NewClientset(objects...)
			if tt.createTerminating {
				refused := false
				fakeClient.PrependReactor("create", "resourcequotas", func(k8stesting.Action) (bool, runtime.Object, error) {
					if refused {
						return false, nil, nil
					}
					refused = true
					err := apierrors.NewForbidden(v1.Resource("resourcequotas"), quotaName, errors.New("namespace is being terminated"))
					err.ErrStatus.Details.Causes = []metav1.StatusCause{{Type: v1.NamespaceTerminatingCause}}
					return true, nil, err
				})
			}
			config.K8sClient = fakeClient
			config.Global.SpxPrefix = spxPrefix
			ctx := context.Background()
			quotas := fakeClient.CoreV1().ResourceQuotas(namespace)

			if err := Delete(ctx, orgId, projectId, tt.revision); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if _, err := fakeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); (err == nil) != tt.wantNamespace {
				t.Errorf("expected namespace %v, got error %v", tt.wantNamespace, err)
			}
			if _, err := quotas.Get(ctx, quotaName, metav1.GetOptions{}); !tt.wantQuota && !apierrors.IsNotFound(err) {
				t.Errorf("expected no quota, got %v", err)
			}

			if err := Apply(ctx, orgId, projectId, hard, 10); !errors.Is(err, tt.wantLaterApply) {
				t.Fatalf("later apply: expected %v, got %v", tt.wantLaterApply, err)
			}
			if !tt.wantQuota {
				return
			}
			rq, err := quotas.Get(ctx, quotaName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("quota not created: %v", err)
			}
			wantRevision := strconv.FormatInt(tt.revision, 10)
			if len(rq.Spec.Hard) != 0 || rq.Annotations[spxId.SpxAnnotationQuotaRevision] != wantRevision {
				t.Errorf("expected no limit at revision %s, got %v at %q", wantRevision, rq.Spec.Hard,
					rq.Annotations[spxId.SpxAnnotationQuotaRevision])
			}
		})
	}
}

// A push whose Get misses a ResourceQuota created concurrently gets AlreadyExists on Create: it is
// retried and updates that ResourceQuota.
func TestCreateRace(t *testing.T) {
	hard, err := Validate(map[string]string{"requests.cpu": "4"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		push     func(ctx context.Context) error
		wantHard v1.ResourceList
	}{
		{"apply", func(ctx context.Context) error { return Apply(ctx, orgId, projectId, hard, 10) }, hard},
		{"delete", func(ctx context.Context) error { return Delete(ctx, orgId, projectId, 10) }, v1.ResourceList{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientset(
				&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
				&v1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
					Spec:       v1.ResourceQuotaSpec{Hard: v1.ResourceList{"requests.cpu": resource.MustParse("16")}},
				},
			)
			missed := false
			fakeClient.PrependReactor("get", "resourcequotas", func(k8stesting.Action) (bool, runtime.Object, error) {
				if missed {
					return false, nil, nil
				}
				missed = true
				return true, nil, apierrors.NewNotFound(v1.Resource("resourcequotas"), quotaName)
			})
			config.K8sClient = fakeClient
			config.Global.SpxPrefix = spxPrefix
			ctx := context.Background()

			if err := tt.push(ctx); err != nil {
				t.Fatalf("push: %v", err)
			}
			rq, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !equality.Semantic.DeepEqual(rq.Spec.Hard, tt.wantHard) {
				t.Errorf("expected hard %v, got %v", tt.wantHard, rq.Spec.Hard)
			}
			if rq.Annotations[spxId.SpxAnnotationQuotaRevision] != "10" {
				t.Errorf("expected annotation 10, got %s", rq.Annotations[spxId.SpxAnnotationQuotaRevision])
			}
		})
	}
}

func TestCheckRevision(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		revision   int64
		wantStale  *StaleError
	}{
		{"older push", "10", 5, &StaleError{Stored: 10, Pushed: 5}},
		{"push without revision", "10", 0, &StaleError{Stored: 10, Pushed: 0}},
		{"same revision", "10", 10, nil},
		{"newer push", "10", 11, nil},
		{"no annotation", "", 5, nil},
		{"invalid annotation", "abc", 5, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq := &v1.ResourceQuota{}
			if tt.annotation != "" {
				rq.Annotations = map[string]string{spxId.SpxAnnotationQuotaRevision: tt.annotation}
			}
			err := checkRevision(rq, tt.revision)
			if tt.wantStale == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			var stale *StaleError
			if !errors.As(err, &stale) || !errors.Is(err, ErrStale) {
				t.Fatalf("expected a StaleError matching ErrStale, got %v", err)
			}
			if *stale != *tt.wantStale {
				t.Errorf("expected %+v, got %+v", *tt.wantStale, *stale)
			}
		})
	}
}

// A push retries the conflicts of concurrent updates of the ResourceQuota, within conflictBackoff.
func TestApplyRetriesConflicts(t *testing.T) {
	previous := conflictBackoff
	conflictBackoff.Duration = time.Millisecond
	t.Cleanup(func() { conflictBackoff = previous })

	hard, err := Validate(map[string]string{"requests.cpu": "4"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		conflicts int
		wantErr   bool
	}{
		{"one conflict", 1, false},
		{"more conflicts than the default retry allows", 6, false},
		{"conflicts until the backoff is exhausted", conflictBackoff.Steps, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientset(
				&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
				&v1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace}},
			)
			conflicts := 0
			fakeClient.PrependReactor("update", "resourcequotas", func(k8stesting.Action) (bool, runtime.Object, error) {
				if conflicts >= tt.conflicts {
					return false, nil, nil
				}
				conflicts++
				return true, nil, apierrors.NewConflict(v1.Resource("resourcequotas"), quotaName, errors.New("modified"))
			})
			config.K8sClient = fakeClient
			config.Global.SpxPrefix = spxPrefix

			err := Apply(context.Background(), orgId, projectId, hard, 10)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if tt.wantErr && !apierrors.IsConflict(err) {
				t.Errorf("expected a Conflict, got %v", err)
			}
		})
	}
}

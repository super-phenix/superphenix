package quota

import (
	"context"
	"errors"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	orgId     = "11111111-1111-1111-1111-111111111111"
	projectId = "22222222-2222-2222-2222-222222222222"
	namespace = "spx-22222222-2222-2222-2222-222222222222"
	quotaName = "quota-22222222-2222-2222-2222-222222222222"
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
	config.Global.SpxPrefix = "spx"
	ctx := context.Background()

	hard, err := Validate(map[string]string{"requests.cpu": "4", "pods": "10"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, orgId, projectId, hard); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := fakeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err != nil {
		t.Fatalf("namespace not created: %v", err)
	}

	// Replacing the quota drops the resources that are no longer limited.
	hard, _ = Validate(map[string]string{"pods": "0"})
	if err := Apply(ctx, orgId, projectId, hard); err != nil {
		t.Fatalf("update: %v", err)
	}
	rq, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("quota not found: %v", err)
	}
	if len(rq.Spec.Hard) != 1 || rq.Spec.Hard.Pods().String() != "0" {
		t.Errorf("unexpected hard %v", rq.Spec.Hard)
	}
	if rq.Labels == nil || len(rq.Labels) == 0 {
		t.Errorf("expected spx labels on the quota")
	}

	if err := Delete(ctx, projectId); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := Delete(ctx, projectId); err != nil {
		t.Fatalf("delete should be idempotent: %v", err)
	}
	if _, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{}); err == nil {
		t.Errorf("quota should be deleted")
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
	config.Global.SpxPrefix = "spx"
	ctx := context.Background()

	hard, _ := Validate(map[string]string{"pods": "5"})
	if err := Apply(ctx, orgId, projectId, hard); err != nil {
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
	config.Global.SpxPrefix = "spx"
	ctx := context.Background()

	hard, _ := Validate(map[string]string{"pods": "5"})
	if err := Apply(ctx, orgId, projectId, hard); err != nil {
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

	if err := Delete(ctx, projectId); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{}); err != nil {
		t.Errorf("gitops quota should not be deleted: %v", err)
	}
}

func TestGet(t *testing.T) {
	fakeClient := fake.NewClientset()
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = "spx"
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
	config.Global.SpxPrefix = "spx"
	ctx := context.Background()

	apply := func(cpu, memory string) error {
		hard, err := Validate(map[string]string{"requests.cpu": cpu, "requests.memory": memory, "pods": "0"})
		if err != nil {
			t.Fatal(err)
		}
		return Apply(ctx, orgId, projectId, hard)
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
	config.Global.SpxPrefix = "spx"
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

	// 2. Stale apply with revision 5 (older than 10) is skipped.
	hardStale, _ := Validate(map[string]string{"requests.cpu": "100"})
	if err := Apply(ctx, orgId, projectId, hardStale, 5); err != nil {
		t.Fatalf("stale apply failed: %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q = rq.Spec.Hard[v1.ResourceName("requests.cpu")]
	if q.String() != "4" {
		t.Fatalf("stale apply should have been ignored, cpu is %s", q.String())
	}

	// 3. Stale delete with revision 8 (older than 10) is skipped.
	if err := Delete(ctx, projectId, 8); err != nil {
		t.Fatalf("stale delete failed: %v", err)
	}
	rq, err = fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
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

	// 5. Newer delete with revision 20 deletes the quota.
	if err := Delete(ctx, projectId, 20); err != nil {
		t.Fatalf("delete with rev 20 failed: %v", err)
	}
	if _, err := fakeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{}); err == nil {
		t.Fatalf("quota should have been deleted")
	}
}

package k8s

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateNamespaceIfNotExistsWithQuota(t *testing.T) {
	fakeClient := fake.NewClientset()
	config.K8sClient = fakeClient
	config.Global.SpxPrefix = "spx"
	config.Global.Quotas.DefaultAZQuotas.Pods = "50"
	config.Global.Quotas.DefaultAZQuotas.PersistentVolumeClaims = "50"
	config.Global.Quotas.DefaultAZQuotas.VirtualMachines = "20"

	orgId := "11111111-1111-1111-1111-111111111111"
	projectId := "22222222-2222-2222-2222-222222222222"
	nsName := "spx-22222222-2222-2222-2222-222222222222"
	quotaName := "quota-22222222-2222-2222-2222-222222222222"

	ctx := context.Background()
	err := CreateNamespaceIfNotExists(ctx, orgId, projectId)
	if err != nil {
		t.Fatalf("unexpected error creating namespace: %v", err)
	}

	ns, err := fakeClient.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("namespace not found: %v", err)
	}
	if ns.Name != nsName {
		t.Errorf("expected namespace %s, got %s", nsName, ns.Name)
	}

	rq, err := fakeClient.CoreV1().ResourceQuotas(nsName).Get(ctx, quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("resource quota not found: %v", err)
	}
	if rq.Name != quotaName {
		t.Errorf("expected quota %s, got %s", quotaName, rq.Name)
	}
	if rq.Spec.Hard.Pods().String() != "50" {
		t.Errorf("expected 50 pods, got %s", rq.Spec.Hard.Pods().String())
	}

	// Calling a second time should be idempotent
	err = CreateNamespaceIfNotExists(ctx, orgId, projectId)
	if err != nil {
		t.Fatalf("unexpected error on idempotent call: %v", err)
	}
}

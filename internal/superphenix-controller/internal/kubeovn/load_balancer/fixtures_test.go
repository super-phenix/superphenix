package loadBalancer

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

const (
	projectUUID  = "11111111-1111-1111-1111-111111111111"
	project      = "spx-" + projectUUID
	otherProject = "spx-22222222-2222-2222-2222-222222222222"

	vpcA = "spx-vpc-a"
	vpcB = "spx-vpc-b"

	subnetA       = "spx-subnet-a"
	subnetA2      = "spx-subnet-a2"
	subnetShared  = "spx-subnet-shared"
	subnetOther   = "spx-subnet-other"
	subnetCluster = "spx-subnet-cluster"

	testVIP = "198.18.0.10"
)

func subnetObj(name, projectID, vpc, cidr string, annotations map[string]string) *v1.Subnet {
	return &v1.Subnet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{spxId.SpxLabelProjectID: projectID},
			Annotations: annotations,
		},
		Spec: v1.SubnetSpec{Vpc: vpc, CIDRBlock: cidr},
	}
}

func ruleObj(name, namespace, vip string, annotations map[string]string) *v1.SwitchLBRule {
	return &v1.SwitchLBRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			UID:         types.UID("uid-" + name),
			Labels:      map[string]string{spxId.SpxLabelProjectID: namespace},
			Annotations: annotations,
		},
		Spec: v1.SwitchLBRuleSpec{Vip: vip, Namespace: namespace},
	}
}

func pinned(vpc, subnet string) map[string]string {
	return map[string]string{logicalRouterAnnotation: vpc, logicalSwitchAnnotation: subnet}
}

func ruleService(rule, namespace, vpc string) *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:        ruleServicePrefix + rule,
		Namespace:   namespace,
		Annotations: map[string]string{serviceVPCAnnotation: vpc},
	}}
}

type fixture struct {
	rules    []*v1.SwitchLBRule
	services []*corev1.Service
}

// setup seeds the fake clients and informers, and returns the kube-ovn client.
func setup(t *testing.T, f fixture) *testhelper.FakeKubeOvnClientset {
	t.Helper()
	ctx := context.Background()

	client := testhelper.NewFakeKubeOvnClientset()
	subnets := []*v1.Subnet{
		subnetObj(subnetA, project, vpcA, "10.10.0.0/16,fd00:10:10::/64", nil),
		subnetObj(subnetA2, project, vpcA, "10.11.0.0/16", nil),
		subnetObj(subnetShared, otherProject, vpcB, "10.20.0.0/16",
			map[string]string{spxId.SpxAnnotationAllowedProjects: project}),
		subnetObj(subnetOther, otherProject, vpcB, "10.30.0.0/16", nil),
		subnetObj(subnetCluster, project, defaultVPC, "10.40.0.0/16", nil),
	}
	for _, s := range subnets {
		if _, err := client.KubeovnV1().Subnets().Create(ctx, s, metav1.CreateOptions{}); err != nil {
			t.Fatalf("failed to seed subnet: %v", err)
		}
	}

	ruleItems := make([]any, 0, len(f.rules))
	for _, r := range f.rules {
		if _, err := client.KubeovnV1().SwitchLBRules().Create(ctx, r, metav1.CreateOptions{}); err != nil {
			t.Fatalf("failed to seed rule: %v", err)
		}
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
		if err != nil {
			t.Fatalf("failed to convert rule: %v", err)
		}
		ruleItems = append(ruleItems, &unstructured.Unstructured{Object: obj})
	}
	testhelper.SetupFakeWatcher(informers.SwitchLBRules, ruleItems...)

	k8sClient := k8sfake.NewClientset()
	for _, s := range f.services {
		if _, err := k8sClient.CoreV1().Services(s.Namespace).Create(ctx, s, metav1.CreateOptions{}); err != nil {
			t.Fatalf("failed to seed service: %v", err)
		}
	}

	config.KubeOvnClient = client
	config.K8sClient = k8sClient
	return client
}

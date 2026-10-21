package loadBalancer

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// vipInUse reports whether a rule other than selfName uses vip in vpc, or in an unknown VPC.
func vipInUse(ctx context.Context, vip, vpc, selfName string) (bool, error) {
	for _, item := range informers.WatcherSet[informers.SwitchLBRules].List() {
		rule, ok := item.(*unstructured.Unstructured)
		if !ok || rule.GetName() == selfName {
			continue
		}
		if ruleVip, _, _ := unstructured.NestedString(rule.Object, "spec", "vip"); ruleVip != vip {
			continue
		}

		namespace, _, _ := unstructured.NestedString(rule.Object, "spec", "namespace")
		// Rules without a pinned VPC take the one kube-ovn resolved on their service, "" when unknown
		other := rule.GetAnnotations()[logicalRouterAnnotation]
		if other == "" {
			svc, err := k8s.K8sClient.CoreV1().Services(namespace).Get(ctx, ruleServicePrefix+rule.GetName(), metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return false, err
			}
			if err == nil {
				other = svc.GetAnnotations()[serviceVPCAnnotation]
			}
		}
		if other == "" || other == vpc {
			return true, nil
		}
	}
	return false, nil
}

func checkVIPAvailable(ctx context.Context, vip, vpc, selfName string) error {
	inUse, err := vipInUse(ctx, vip, vpc, selfName)
	if err != nil {
		return err
	}
	if inUse {
		return apierrors.NewBadRequest(fmt.Sprintf("vip %q is already used in this VPC", vip))
	}
	return nil
}

// VIPAvailable reports whether vip is free in the VPC of the subnet.
func VIPAvailable(ctx context.Context, vip, subnetName, projectID string) (bool, error) {
	if err := checkVIP(vip); err != nil {
		return false, err
	}
	subnet, err := getRuleSubnet(ctx, subnetName, projectID)
	if err != nil {
		return false, err
	}
	inUse, err := vipInUse(ctx, vip, subnet.Spec.Vpc, "")
	return !inUse, err
}

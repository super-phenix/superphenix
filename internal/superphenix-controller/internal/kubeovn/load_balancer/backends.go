package loadBalancer

import (
	"fmt"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// checkBackendsInSubnet ensures the endpoints, and the pods currently matching the selector, are in the subnet.
func checkBackendsInSubnet(subnet *v1.Subnet, namespace string, selector map[string]string, endpoints []string) error {
	for _, endpoint := range endpoints {
		if ok, err := utils.IsIPInCIDR(subnet.Spec.CIDRBlock, endpoint); err != nil || !ok {
			return apierrors.NewBadRequest(fmt.Sprintf("endpoint %q is not in subnet %q", endpoint, subnet.Name))
		}
	}

	if len(selector) == 0 {
		return nil
	}
	podSelector := labels.SelectorFromSet(selector)
	for _, item := range informers.WatcherSet[informers.Pods].List() {
		pod, ok := item.(k8smetav1.Object)
		if !ok || pod.GetNamespace() != namespace || !podSelector.Matches(labels.Set(pod.GetLabels())) {
			continue
		}

		// A pod has one logical switch annotation per network, for its default network and its attachments
		inSubnet, hasSwitch := false, false
		for key, value := range pod.GetAnnotations() {
			if strings.HasSuffix(key, logicalSwitchAnnotation) {
				hasSwitch = true
				inSubnet = inSubnet || value == subnet.Name
			}
		}
		if hasSwitch && !inSubnet {
			return apierrors.NewBadRequest(fmt.Sprintf("selected pod %q is not in subnet %q", pod.GetName(), subnet.Name))
		}
	}
	return nil
}

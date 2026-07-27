package eip

import (
	"context"
	"fmt"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsEIPInSubnet(ctx context.Context, namespace, subnetName string) (bool, error) {
	projectLabel := fmt.Sprintf("%s=%s", spxId.SpxLabelProjectID, namespace)
	eipList, err := k8s.KubeOvnClient.KubeovnV1().IptablesEIPs().List(ctx, metav1.ListOptions{
		LabelSelector: projectLabel,
	})
	if err != nil {
		return false, err
	}

	for _, eip := range eipList.Items {
		if eip.Spec.NatGwDp == subnetName {
			return true, nil
		}
	}

	return false, nil
}

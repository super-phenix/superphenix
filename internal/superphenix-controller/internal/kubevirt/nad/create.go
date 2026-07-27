package nad

import (
	"context"
	"fmt"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateNADInfo struct {
	Namespace string
	SubnetEID string
	Labels    map[string]string
}

func CreateNAD(ctx context.Context, info CreateNADInfo) (*v1.NetworkAttachmentDefinition, error) {
	config := fmt.Sprintf(`{
	"cniVersion": "0.3.1",
	"name": "generic-veth",
	"plugins": [
		{
			"type": "kube-ovn",
			"server_socket": "/run/openvswitch/kube-ovn-daemon.sock",
			"provider": "%s.%s.ovn"
		}
	]
}`, info.SubnetEID, info.Namespace)

	return k8s.VirtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(info.Namespace).Create(ctx, &v1.NetworkAttachmentDefinition{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   info.SubnetEID,
			Labels: info.Labels,
		},
		Spec: v1.NetworkAttachmentDefinitionSpec{Config: config},
	}, k8smetav1.CreateOptions{})
}

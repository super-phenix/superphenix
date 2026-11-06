package ssh

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ErrSSHKeyInUse is returned when a VM references the key: KubeVirt cannot boot a VM whose key is gone.
var ErrSSHKeyInUse = errors.New("SSH key is in use and cannot be deleted")

// Usage returns the VMs referencing each SSH key of a namespace, stopped ones included.
// It reads the informer because the vm package imports this one.
func Usage(namespace string) (map[string][]view.SSHUser, error) {
	list, err := informers.WatcherSet[informers.VirtualMachine].ByIndex("namespace", namespace)
	if err != nil {
		return nil, err
	}

	usage := make(map[string][]view.SSHUser)
	for _, item := range list {
		var vm view.VirtualMachineView
		if err := utils.UnstructuredToStruct(item.(*unstructured.Unstructured), &vm); err != nil {
			return nil, err
		}
		if vm.Spec.Template == nil {
			continue
		}
		for _, credential := range vm.Spec.Template.Spec.AccessCredentials {
			if credential.SSHPublicKey != nil && credential.SSHPublicKey.Source.Secret != nil {
				key := credential.SSHPublicKey.Source.Secret.SecretName
				usage[key] = append(usage[key], view.SSHUser{EId: vm.Name, ProductName: vm.Labels[spxId.SpxLabelResourceName]})
			}
		}
	}

	for _, users := range usage {
		slices.SortFunc(users, func(a, b view.SSHUser) int { return cmp.Compare(a.EId, b.EId) })
	}
	return usage, nil
}

func DeleteSSHKey(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	sshKey, err := k8s.K8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("method", "DeleteSSHKey").Str("namespace", namespace).Str("name", name).Msg("Error getting ssh key")
		return err
	}

	if err := utils.CheckProjectLabel(sshKey, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("SSH key access denied")
		return err
	}

	if err := utils.IsEditAllowed(sshKey.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	usage, err := Usage(namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error listing ssh key usage")
		return err
	}
	if users := usage[name]; len(users) > 0 {
		eids := make([]string, 0, len(users))
		for _, user := range users {
			eids = append(eids, user.EId)
		}
		log.Warn().Str("namespace", namespace).Str("name", name).Strs("instances", eids).Msg("SSH key in use")
		return ErrSSHKeyInUse
	}

	err = k8s.K8sClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Err(err).Str("method", "DeleteSSHKey").Str("namespace", namespace).Str("name", name).Msg("Error deleting ssh key")
		return err
	}
	return nil
}

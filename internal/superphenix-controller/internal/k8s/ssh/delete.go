package ssh

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
)

// ErrSSHKeyInUse is returned when a VM references the key: KubeVirt cannot boot a VM whose key is gone.
var ErrSSHKeyInUse = errors.New("SSH key is in use and cannot be deleted")

// KeyNames returns the SSH keys referenced by the access credentials of a VM, in order.
func KeyNames(credentials []v1.AccessCredential) []string {
	keys := make([]string, 0, len(credentials))
	for _, credential := range credentials {
		if credential.SSHPublicKey != nil && credential.SSHPublicKey.Source.Secret != nil {
			keys = append(keys, credential.SSHPublicKey.Source.Secret.SecretName)
		}
	}
	return keys
}

// Usage returns the VMs referencing each SSH key of a namespace, stopped ones included.
// It reads the API server: the informer may lag or not be synced yet, and deleting a key still in use breaks the VM.
func Usage(ctx context.Context, namespace string) (map[string][]view.SSHUser, error) {
	vms, err := k8s.VirtClient.VirtualMachine(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	usage := make(map[string][]view.SSHUser)
	for _, vm := range vms.Items {
		if vm.Spec.Template == nil {
			continue
		}
		for _, key := range KeyNames(vm.Spec.Template.Spec.AccessCredentials) {
			usage[key] = append(usage[key], view.SSHUser{EId: vm.Name, ProductName: vm.Labels[spxId.SpxLabelResourceName]})
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

	usage, err := Usage(ctx, namespace)
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

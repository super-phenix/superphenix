package ssh

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

const testNamespace = "spx-project"

func credential(key string) v1.AccessCredential {
	return v1.AccessCredential{SSHPublicKey: &v1.SSHPublicKeyAccessCredential{
		Source: v1.SSHPublicKeyAccessCredentialSource{Secret: &v1.AccessCredentialSecretSource{SecretName: key}},
	}}
}

func testVM(name, displayName string, keys ...string) v1.VirtualMachine {
	vm := v1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec:       v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{}},
	}
	if displayName != "" {
		vm.Labels = map[string]string{spxId.SpxLabelResourceName: displayName}
	}
	for _, key := range keys {
		vm.Spec.Template.Spec.AccessCredentials = append(vm.Spec.Template.Spec.AccessCredentials, credential(key))
	}
	return vm
}

// mockVMs serves vms, or listErr, as the VMs of testNamespace. Any other namespace fails the test.
func mockVMs(t *testing.T, vms []v1.VirtualMachine, listErr error) {
	t.Helper()
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	vmIface := kubecli.NewMockVirtualMachineInterface(ctrl)
	client.EXPECT().VirtualMachine(testNamespace).Return(vmIface).AnyTimes()
	list := &v1.VirtualMachineList{Items: vms}
	if listErr != nil {
		list = nil
	}
	vmIface.EXPECT().List(gomock.Any(), gomock.Any()).Return(list, listErr).AnyTimes()

	orig := config.VirtClient
	t.Cleanup(func() { config.VirtClient = orig })
	config.VirtClient = client
}

func TestKeyNames(t *testing.T) {
	tests := []struct {
		name        string
		credentials []v1.AccessCredential
		want        []string
	}{
		{name: "no credential", want: []string{}},
		{name: "keys in order", credentials: []v1.AccessCredential{credential("key-2"), credential("key-1")}, want: []string{"key-2", "key-1"}},
		{
			name: "credentials without an ssh key secret",
			credentials: []v1.AccessCredential{
				{UserPassword: &v1.UserPasswordAccessCredential{}},
				{SSHPublicKey: &v1.SSHPublicKeyAccessCredential{}},
				credential("key-1"),
			},
			want: []string{"key-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KeyNames(tt.credentials); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KeyNames = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUsage(t *testing.T) {
	tests := []struct {
		name    string
		vms     []v1.VirtualMachine
		listErr error
		want    map[string][]view.SSHUser
		wantErr bool
	}{
		{name: "no vm", want: map[string][]view.SSHUser{}},
		{name: "vm without keys", vms: []v1.VirtualMachine{testVM("vm-1", "")}, want: map[string][]view.SSHUser{}},
		{
			name: "vm without template",
			vms:  []v1.VirtualMachine{{ObjectMeta: metav1.ObjectMeta{Name: "vm-1"}}},
			want: map[string][]view.SSHUser{},
		},
		{
			name: "several vms and keys, sorted by eid",
			vms:  []v1.VirtualMachine{testVM("vm-2", "gitops-vm", "key-1"), testVM("vm-1", "", "key-1", "key-2")},
			want: map[string][]view.SSHUser{
				"key-1": {{EId: "vm-1"}, {EId: "vm-2", ProductName: "gitops-vm"}},
				"key-2": {{EId: "vm-1"}},
			},
		},
		{name: "vm listing failure", listErr: errors.New("api server unavailable"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVMs(t, tt.vms, tt.listErr)

			got, err := Usage(context.Background(), testNamespace)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Usage error = %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Usage = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteSSHKey(t *testing.T) {
	const key = "key-1"

	tests := []struct {
		name        string
		vms         []v1.VirtualMachine
		listErr     error
		wantErr     func(error) bool
		wantDeleted bool
	}{
		{name: "unused key", wantDeleted: true},
		{name: "another key used", vms: []v1.VirtualMachine{testVM("vm-1", "", "key-2")}, wantDeleted: true},
		{
			name:    "used by a stopped vm",
			vms:     []v1.VirtualMachine{testVM("vm-1", "", key)},
			wantErr: func(err error) bool { return errors.Is(err, ErrSSHKeyInUse) },
		},
		{
			name:    "one of several keys of a vm",
			vms:     []v1.VirtualMachine{testVM("vm-1", "", "key-2", key)},
			wantErr: func(err error) bool { return errors.Is(err, ErrSSHKeyInUse) },
		},
		{
			name:    "vm listing failure keeps the key",
			listErr: errors.New("api server unavailable"),
			wantErr: func(err error) bool { return err != nil && !errors.Is(err, ErrSSHKeyInUse) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVMs(t, tt.vms, tt.listErr)

			orig := config.K8sClient
			t.Cleanup(func() { config.K8sClient = orig })
			config.K8sClient = fake.NewSimpleClientset(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      key,
				Namespace: testNamespace,
				Labels:    map[string]string{spxId.SpxLabelProjectID: testNamespace},
			}})

			err := DeleteSSHKey(context.Background(), testNamespace, key)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("DeleteSSHKey: %v", err)
			}
			if tt.wantErr != nil && !tt.wantErr(err) {
				t.Fatalf("unexpected error: %v", err)
			}

			_, getErr := config.K8sClient.CoreV1().Secrets(testNamespace).Get(context.Background(), key, metav1.GetOptions{})
			if deleted := apierrors.IsNotFound(getErr); deleted != tt.wantDeleted {
				t.Errorf("deleted = %v, want %v", deleted, tt.wantDeleted)
			}
		})
	}
}

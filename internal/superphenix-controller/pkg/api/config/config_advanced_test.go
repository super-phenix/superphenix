package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"github.com/go-chi/chi/v5"
	"go.uber.org/mock/gomock"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

func boolPtr(b bool) *bool { return &b }

// blockField returns the resolved field with fieldKey inside the block with
// blockKey (zero value if absent).
func blockField(o view.AdvancedOptions, blockKey, fieldKey string) view.ResolvedBool {
	for _, b := range o.Blocks {
		if b.Key != blockKey {
			continue
		}
		for _, f := range b.Fields {
			if f.Key == fieldKey {
				return f.ResolvedBool
			}
		}
	}
	return view.ResolvedBool{}
}

func reqWithParams(method, target string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestGetVMClusterPreferenceAdvancedOptionsHandler(t *testing.T) {
	origPrefix := config.Global.SpxPrefix
	config.Global.SpxPrefix = "spx"
	t.Cleanup(func() { config.Global.SpxPrefix = origPrefix })

	const projectId, pref = "proj", "windows.2k25.virtio"
	namespace := "spx-" + projectId

	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	expandIface := kubecli.NewMockExpandSpecInterface(ctrl)
	orig := config.VirtClient
	t.Cleanup(func() { config.VirtClient = orig })
	config.VirtClient = client

	client.EXPECT().ExpandSpec(namespace).Return(expandIface)
	expandIface.EXPECT().ForVirtualMachine(gomock.Any()).DoAndReturn(
		func(vm *v1.VirtualMachine) (*v1.VirtualMachine, error) {
			if vm.Spec.Preference == nil || vm.Spec.Preference.Name != pref {
				t.Fatalf("expected stub carrying preference %q, got %+v", pref, vm.Spec.Preference)
			}
			return &v1.VirtualMachine{
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Domain: v1.DomainSpec{
								Devices: v1.Devices{TPM: &v1.TPMDevice{Persistent: boolPtr(true)}},
							},
						},
					},
				},
			}, nil
		},
	)

	w := httptest.NewRecorder()
	r := reqWithParams(http.MethodGet, "/vm-type/"+pref+"/advanced-options", map[string]string{
		"orgId": "org", "projectId": projectId, "name": pref,
	})

	GetVMClusterPreferenceAdvancedOptions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var got view.AdvancedOptions
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	tpmPersistent := blockField(got, "tpm", "persistent")
	if tpmPersistent.Source != view.SourcePreference ||
		tpmPersistent.Value == nil || !*tpmPersistent.Value {
		t.Fatalf("expected tpm.persistent {true, preference}, got %+v", tpmPersistent)
	}
}

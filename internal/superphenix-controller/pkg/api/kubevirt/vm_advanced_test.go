package kubevirt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"github.com/go-chi/chi/v5"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func boolPtr(b bool) *bool { return &b }

// blockEnabled returns the enable toggle of the resolved block with the given key.
func blockEnabled(o view.AdvancedOptions, key string) view.ResolvedBool {
	for _, b := range o.Blocks {
		if b.Key == key {
			return b.Enabled
		}
	}
	return view.ResolvedBool{}
}

// reqWithParams builds a request carrying chi URL params, as the router would.
func reqWithParams(method, target string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// installMockVirtClient swaps config.VirtClient for a mock for the test.
func installMockVirtClient(t *testing.T) (*kubecli.MockKubevirtClient, *kubecli.MockVirtualMachineInterface, *kubecli.MockExpandSpecInterface, *kubecli.MockVirtualMachineInstanceInterface) {
	t.Helper()
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	vmIface := kubecli.NewMockVirtualMachineInterface(ctrl)
	expandIface := kubecli.NewMockExpandSpecInterface(ctrl)
	vmiIface := kubecli.NewMockVirtualMachineInstanceInterface(ctrl)

	orig := config.VirtClient
	t.Cleanup(func() { config.VirtClient = orig })
	config.VirtClient = client

	return client, vmIface, expandIface, vmiIface
}

func projectVM(namespace, name string, d v1.DomainSpec) *v1.VirtualMachine {
	return &v1.VirtualMachine{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{spxId.SpxLabelProjectID: namespace},
		},
		Spec: v1.VirtualMachineSpec{
			Template: &v1.VirtualMachineInstanceTemplateSpec{
				Spec: v1.VirtualMachineInstanceSpec{Domain: d},
			},
		},
	}
}

func TestGetInstanceAdvancedOptionsHandler(t *testing.T) {
	origPrefix := config.Global.SpxPrefix
	config.Global.SpxPrefix = "spx"
	t.Cleanup(func() { config.Global.SpxPrefix = origPrefix })

	const projectId, eid = "proj", "vm-1"
	namespace := "spx-" + projectId

	t.Run("returns resolved options", func(t *testing.T) {
		client, vmIface, _, vmiIface := installMockVirtClient(t)

		raw := projectVM(namespace, eid, v1.DomainSpec{
			Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(false)}},
		})
		client.EXPECT().VirtualMachine(namespace).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), eid, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), eid).Return(raw, nil)
		// VM is stopped: the VMI lookup returns NotFound (no live values).
		client.EXPECT().VirtualMachineInstance(namespace).Return(vmiIface)
		vmiIface.EXPECT().Get(gomock.Any(), eid, gomock.Any()).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{Resource: "virtualmachineinstances"}, eid))

		w := httptest.NewRecorder()
		r := reqWithParams(http.MethodGet, "/instance/"+eid+"/advanced-options", map[string]string{
			"orgId": "org", "projectId": projectId, "effectiveId": eid,
		})

		getInstanceAdvancedOptions(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		var got view.AdvancedOptions
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON body: %v", err)
		}
		tpm := blockEnabled(got, "tpm")
		if tpm.Source != view.SourceVM || tpm.Value == nil || *tpm.Value {
			t.Fatalf("expected tpm.enabled {false, vm}, got %+v", tpm)
		}
	})

	t.Run("not found maps to 404", func(t *testing.T) {
		client, vmIface, _, _ := installMockVirtClient(t)

		notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "vm"}, eid)
		client.EXPECT().VirtualMachine(namespace).Return(vmIface)
		vmIface.EXPECT().Get(gomock.Any(), eid, gomock.Any()).Return(nil, notFound)

		w := httptest.NewRecorder()
		r := reqWithParams(http.MethodGet, "/instance/"+eid+"/advanced-options", map[string]string{
			"orgId": "org", "projectId": projectId, "effectiveId": eid,
		})

		getInstanceAdvancedOptions(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("expand error maps to 500", func(t *testing.T) {
		client, vmIface, _, _ := installMockVirtClient(t)

		raw := projectVM(namespace, eid, v1.DomainSpec{})
		client.EXPECT().VirtualMachine(namespace).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), eid, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), eid).Return(nil, errors.New("expand failed"))

		w := httptest.NewRecorder()
		r := reqWithParams(http.MethodGet, "/instance/"+eid+"/advanced-options", map[string]string{
			"orgId": "org", "projectId": projectId, "effectiveId": eid,
		})

		getInstanceAdvancedOptions(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

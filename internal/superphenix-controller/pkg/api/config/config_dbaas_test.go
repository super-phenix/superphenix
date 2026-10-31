package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

func TestGetDBaaSConfigHandler(t *testing.T) {
	tests := []struct {
		name           string
		mapping        map[string]string
		externalSubnet string
		want           DBaaSConfig
	}{
		{
			name:           "configured classes and subnet",
			mapping:        map[string]string{"standard": "ceph-rbd", "fast": "ceph-rbd-ssd"},
			externalSubnet: "external",
			want: DBaaSConfig{
				StorageClasses: []StorageClass{
					{Shortname: "fast", Fullname: "ceph-rbd-ssd"},
					{Shortname: "standard", Fullname: "ceph-rbd"},
				},
				ExternalSubnet: "external",
			},
		},
		{
			name: "nothing configured",
			want: DBaaSConfig{StorageClasses: []StorageClass{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldMapping := config.Global.ProductsConfig.BlockStorage.StorageClassMapping
			oldEip := config.Global.ProductsConfig.EipDefault
			config.Global.ProductsConfig.BlockStorage.StorageClassMapping = tt.mapping
			config.Global.ProductsConfig.EipDefault.ExternalSubnet = tt.externalSubnet
			t.Cleanup(func() {
				config.Global.ProductsConfig.BlockStorage.StorageClassMapping = oldMapping
				config.Global.ProductsConfig.EipDefault = oldEip
			})

			w := httptest.NewRecorder()
			GetDBaaSConfig(w, httptest.NewRequest(http.MethodGet, "/org/proj/dbaas-config", nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			var got DBaaSConfig
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			slices.SortFunc(got.StorageClasses, func(a, b StorageClass) int {
				return strings.Compare(a.Shortname, b.Shortname)
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDBaaSConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

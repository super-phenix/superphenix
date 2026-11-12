package kaas

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

func TestQuantityToGiB(t *testing.T) {
	tests := []struct {
		name        string
		quantity    string
		want        int
		expectError bool
	}{
		{name: "gibibytes", quantity: "24Gi", want: 24},
		{name: "canonical tebibyte", quantity: "1Ti", want: 1024},
		{name: "non canonical gibibytes", quantity: "1536Gi", want: 1536},
		{name: "canonical tebibytes", quantity: "2Ti", want: 2048},
		{name: "largest accepted size", quantity: "999999Gi", want: 999999},
		{name: "unparseable", quantity: "abc", expectError: true},
		{name: "empty", quantity: "", expectError: true},
		{name: "unit with byte suffix", quantity: "4GiB", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := quantityToGiB(tt.quantity)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQuantityToBinary(t *testing.T) {
	tests := []struct {
		name        string
		quantity    string
		want        string
		expectError bool
	}{
		{name: "gibibytes", quantity: "4Gi", want: "4Gi"},
		{name: "fractional gibibytes", quantity: "1.5Gi", want: "1536Mi"},
		{name: "mebibytes", quantity: "2500Mi", want: "2500Mi"},
		{name: "mebibytes as gibibytes", quantity: "1024Mi", want: "1Gi"},
		{name: "unparseable", quantity: "abc", expectError: true},
		{name: "empty", quantity: "", expectError: true},
		{name: "unit with byte suffix", quantity: "4GiB", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := quantityToBinary(tt.quantity)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertAppToUpdateKaaSSpec_InvalidQuantity(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	spec := KaaSSpec{
		KubeVersion:   "1.35.0",
		CPNetPol:      "default",
		WorkersNetPol: "default",
		Groups: []Group{
			{Name: "group-1", Replicas: 3, Cpu: 2, Memory: "4Gi", BootDiskSize: 20, StorageClass: "sc1",
				Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
		},
		ControlPlane: ControlPlaneSpec{DataStore: DataStoreSpec{Dedicated: true, StorageClassName: "encrypted", Storage: 3}},
	}
	values, _, err := CreateKaaSAppValues(context.Background(), "test-cluster", "test-loc", spec, kaasConfig, nil)
	if err != nil {
		t.Fatalf("CreateKaaSAppValues() error = %v", err)
	}

	tests := []struct {
		name    string
		old     string
		new     string
		wantErr string
	}{
		{name: "unparseable memory", old: "memory: 4Gi", new: "memory: abc", wantErr: "memory of group"},
		{name: "unparseable boot disk storage", old: "storage: 20Gi", new: "storage: abc", wantErr: "boot disk storage"},
		{name: "unparseable datastore storage", old: "storage: 3Gi", new: "storage: abc", wantErr: "datastore storage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(values, tt.old) {
				t.Fatalf("rendered values do not contain %q", tt.old)
			}
			app := appViewWithHelmValues(strings.Replace(values, tt.old, tt.new, 1))

			_, err := ConvertAppToUpdateKaaSSpec(app)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

package kaas

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

func TestParseSubnetId(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr bool
	}{
		{name: "uuid", id: "3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f", want: "3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f"},
		{name: "uppercase uuid is normalized", id: "3F2B9A10-1B3C-4C5D-8E7F-0A1B2C3D4E5F", want: "3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f"},
		{name: "empty", id: "", wantErr: true},
		{name: "nil uuid", id: "00000000-0000-0000-0000-000000000000", wantErr: true},
		{name: "name instead of id", id: "subnet-1", wantErr: true},
		{name: "uuid with braces", id: "{3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f}", wantErr: true},
		{name: "uuid with urn prefix", id: "urn:uuid:3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f", wantErr: true},
		{name: "uuid followed by a new line", id: "3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f\nextra: value", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSubnetId(tt.id)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseSubnetId(%q) = %q, want an error", tt.id, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("parseSubnetId(%q) = %q, %v, want %q", tt.id, got, err, tt.want)
			}
		})
	}
}

func TestCreateKaaSAppValuesRejectsInvalidSubnet(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.24.0"}}
	kaasConfig := KaaSConfig{StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}}}

	spec := func(subnetId string) KaaSSpec {
		return KaaSSpec{
			KubeVersion:   "1.24.0",
			CPNetPol:      "default",
			WorkersNetPol: "default",
			Groups: []Group{{
				Name: "group-1", Replicas: 1, Cpu: 2, Memory: "4Gi", BootDiskSize: 20, StorageClass: "sc1",
				Subnets: []GroupSubnet{{Order: 1, Id: subnetId}},
			}},
		}
	}

	if _, _, err := CreateKaaSAppValues(context.Background(), "cluster", "loc", spec("subnet with spaces\nkey: value"), kaasConfig, nil); err == nil {
		t.Error("CreateKaaSAppValues() accepted an invalid subnet id")
	}

	values, _, err := CreateKaaSAppValues(context.Background(), "cluster", "loc", spec("3F2B9A10-1B3C-4C5D-8E7F-0A1B2C3D4E5F"), kaasConfig, nil)
	if err != nil {
		t.Fatalf("CreateKaaSAppValues() error = %v", err)
	}
	if !strings.Contains(values, "subnet: 3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f") {
		t.Errorf("subnet id not rendered in canonical form:\n%s", values)
	}
}

// The group version changes when the node group changes: the same subnet written in
// another case must keep it, another subnet must increment it.
func TestCreateKaaSAppValuesSubnetCaseKeepsGroupVersion(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.24.0"}}
	kaasConfig := KaaSConfig{StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}}}

	spec := func(subnetId string, version int) KaaSSpec {
		return KaaSSpec{
			KubeVersion:   "1.24.0",
			CPNetPol:      "default",
			WorkersNetPol: "default",
			Groups: []Group{{
				Name: "group-1", Replicas: 1, Cpu: 2, Memory: "4Gi", BootDiskSize: 20, StorageClass: "sc1", Version: version,
				Subnets: []GroupSubnet{{Order: 1, Id: subnetId}},
			}},
		}
	}

	const lower = "3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f"
	const upper = "3F2B9A10-1B3C-4C5D-8E7F-0A1B2C3D4E5F"
	tests := []struct {
		name    string
		newId   string
		oldId   string
		version string
	}{
		{name: "upper case request, lower case stored", newId: upper, oldId: lower, version: "version: 3"},
		{name: "lower case request, upper case stored", newId: lower, oldId: upper, version: "version: 3"},
		{name: "another subnet", newId: "00000000-0000-4000-8000-000000000002", oldId: lower, version: "version: 4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := spec(tt.oldId, 3)
			values, _, err := CreateKaaSAppValues(context.Background(), "cluster", "loc", spec(tt.newId, 0), kaasConfig, &old)
			if err != nil {
				t.Fatalf("CreateKaaSAppValues() error = %v", err)
			}
			if !strings.Contains(values, tt.version) {
				t.Errorf("want %q in values:\n%s", tt.version, values)
			}
		})
	}
}

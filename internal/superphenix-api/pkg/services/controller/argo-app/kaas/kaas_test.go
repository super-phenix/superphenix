package kaas

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"gopkg.in/yaml.v2"
)

func TestCreateKaaSAppValues_Comparison(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.24.0"}}
	ctx := context.Background()
	localId := "test-cluster"
	location := "test-loc"
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{
			{Shortname: "sc1", Fullname: "storage-class-1"},
		},
	}

	group1 := Group{
		Name:         "group-1",
		Replicas:     3,
		Cpu:          2,
		Memory:       4,
		BootDiskSize: 20,
		StorageClass: "sc1",
		Version:      1,
		Subnets: []GroupSubnet{
			{Order: 1, Id: "subnet-1"},
		},
	}

	spec := KaaSSpec{
		KubeVersion:   "1.24.0",
		CPNetPol:      "default",
		WorkersNetPol: "default",
		Groups:        []Group{group1},
	}

	tests := []struct {
		name            string
		spec            KaaSSpec
		oldSpec         *KaaSSpec
		wantErr         bool
		expectedStrings []string
	}{
		{
			name:    "Initial call without oldSpec",
			spec:    spec,
			oldSpec: nil,
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
			},
		},
		{
			name:    "Call with same spec as oldSpec",
			spec:    spec,
			oldSpec: &spec,
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
			},
		},
		{
			name: "Modify spec (CPU) and compare - should auto-increment version",
			spec: func() KaaSSpec {
				s := spec
				s.Groups = []Group{group1}
				s.Groups[0].Cpu = 4
				return s
			}(),
			oldSpec: &spec,
			wantErr: false,
			expectedStrings: []string{
				"cores: 4",
				"version: 2",
			},
		},
		{
			name: "Modify subnets and compare - should auto-increment version",
			spec: func() KaaSSpec {
				s := spec
				s.Groups = []Group{group1}
				s.Groups[0].Subnets = []GroupSubnet{{Order: 2, Id: "subnet-2"}}
				return s
			}(),
			oldSpec: &spec,
			wantErr: false,
			expectedStrings: []string{
				"version: 2",
			},
		},
		{
			name: "Modify boot disk size and compare - should auto-increment version",
			spec: func() KaaSSpec {
				s := spec
				s.Groups = []Group{group1}
				s.Groups[0].BootDiskSize = 40
				return s
			}(),
			oldSpec: &spec,
			wantErr: false,
			expectedStrings: []string{
				"version: 2",
			},
		},
		{
			name: "Modify replicas - version should NOT change",
			spec: func() KaaSSpec {
				s := spec
				s.Groups = []Group{group1}
				s.Groups[0].Replicas = 10
				return s
			}(),
			oldSpec: &spec,
			wantErr: false,
			expectedStrings: []string{
				"replicas: 10",
				"version: 1",
			},
		},
		{
			name: "Sort Groups by Name - should compare correctly even if unsorted in input",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-2", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
					{Name: "group-1", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
				},
			},
			oldSpec: &KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-1", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 1},
					{Name: "group-2", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 1},
				},
			},
			wantErr: false,
			expectedStrings: []string{
				"group-1",
				"group-2",
				"version: 1",
			},
		},
		{
			name: "Multiple identical groups in spec matching identical groups in oldSpec - should NOT reuse the same old group",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-0", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Replicas: 3, Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
					{Name: "group-1", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Replicas: 3, Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
				},
			},
			oldSpec: &KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-0", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Replicas: 3, Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 1},
					{Name: "group-1", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Replicas: 3, Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 2},
				},
			},
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
				"version: 2",
			},
		},
		{
			name: "Add a new group - should NOT panic and should use default version",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-0", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
					{Name: "group-1", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
				},
			},
			oldSpec: &KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-0", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 1},
				},
			},
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
			},
		},
		{
			name: "Remove a group - should NOT panic",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-0", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
				},
			},
			oldSpec: &KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-0", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 1},
					{Name: "group-1", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 1},
				},
			},
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
			},
		},
		{
			name: "Valid CPNetPol 'none' - should succeed and appear in output",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "none",
				WorkersNetPol: "default",
				Groups:        []Group{group1},
			},
			oldSpec: nil,
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
			},
		},
		{
			name: "Valid WorkersNetPol 'strict' - should succeed and appear in output",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "strict",
				Groups:        []Group{group1},
			},
			oldSpec: nil,
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
			},
		},
		{
			name: "Valid WorkersNetPol 'none' - should succeed",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "none",
				Groups:        []Group{group1},
			},
			oldSpec: nil,
			wantErr: false,
			expectedStrings: []string{
				"version: 1",
			},
		},
		{
			name: "Invalid CPNetPol - should return error",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "invalid",
				WorkersNetPol: "default",
				Groups:        []Group{group1},
			},
			oldSpec: nil,
			wantErr: true,
		},
		{
			name: "Invalid WorkersNetPol - should return error",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "invalid",
				Groups:        []Group{group1},
			},
			oldSpec: nil,
			wantErr: true,
		},
		{
			name: "CPNetPol 'strict' is not valid for CP - should return error",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "strict",
				WorkersNetPol: "default",
				Groups:        []Group{group1},
			},
			oldSpec: nil,
			wantErr: true,
		},
		{
			name: "Replicas below minimum - should return error",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-1", Replicas: 0, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
				},
			},
			oldSpec: nil,
			wantErr: true,
		},
		{
			name: "Replicas above maximum - should return error",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-1", Replicas: 11, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
				},
			},
			oldSpec: nil,
			wantErr: true,
		},
		{
			name: "Both NetPol invalid - should return error on CPNetPol first",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "invalid",
				WorkersNetPol: "invalid",
				Groups:        []Group{group1},
			},
			oldSpec: nil,
			wantErr: true,
		},
		{
			name: "2 groups in oldSpec, 3 in newSpec (1 same, 1 changed, 1 new)",
			spec: KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-1", Replicas: 1, Cpu: 1, Memory: 1, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}}, // Same as old[1] -> version 4
					{Name: "group-0", Replicas: 1, Cpu: 4, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}}, // Changed from old[0] -> version 3
					{Name: "group-2", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}}, // New -> version 1
				},
			},
			oldSpec: &KaaSSpec{
				KubeVersion:   "1.24.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups: []Group{
					{Name: "group-0", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 2},
					{Name: "group-1", Cpu: 1, Memory: 1, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}, Version: 4},
				},
			},
			wantErr: false,
			expectedStrings: []string{
				"group-0", "version: 3",
				"group-1", "version: 4",
				"group-2", "version: 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _, err := CreateKaaSAppValues(ctx, localId, location, tt.spec, kaasConfig, tt.oldSpec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateKaaSAppValues() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if values == "" {
				t.Error("values should not be empty")
			}
			for _, s := range tt.expectedStrings {
				if !strings.Contains(values, s) {
					t.Errorf("expected values to contain %q, but it didn't.\nValues:\n%s", s, values)
				}
			}
		})
	}
}

func TestCreateKaaSAppValues_DataStore(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.34.0"}, {Version: "v1.34.5"}, {Version: "1.35.0"}, {Version: "v1.35.5"}}
	ctx := context.Background()
	localId := "test-cluster"
	location := "test-loc"
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	baseGroup := Group{
		Name:         "group-1",
		Replicas:     3,
		Cpu:          2,
		Memory:       4,
		BootDiskSize: 20,
		StorageClass: "sc1",
		Subnets:      []GroupSubnet{{Order: 1, Id: "subnet-1"}},
	}
	specWith := func(version string, ds DataStoreSpec) KaaSSpec {
		return KaaSSpec{
			KubeVersion:   version,
			CPNetPol:      "default",
			WorkersNetPol: "default",
			Groups:        []Group{baseGroup},
			ControlPlane:  ControlPlaneSpec{DataStore: ds},
		}
	}
	ptr := func(s KaaSSpec) *KaaSSpec { return &s }

	tests := []struct {
		name    string
		spec    KaaSSpec
		oldSpec *KaaSSpec // nil means create (no prior datastore)
		wantErr bool
		want    *DataStore // nil means the dataStore block must be absent
	}{
		{
			name: "DR enabled with storage class and size",
			spec: specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "encrypted", Storage: 3}),
			want: &DataStore{Dedicated: true, StorageClassName: "encrypted", Storage: "3Gi"},
		},
		{
			name: "DR enabled with default class and size",
			spec: specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8}),
			want: &DataStore{Dedicated: true, StorageClassName: "default", Storage: "8Gi"},
		},
		{
			name: "DR enabled on v-prefixed supported version",
			spec: specWith("v1.35.5", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8}),
			want: &DataStore{Dedicated: true, StorageClassName: "default", Storage: "8Gi"},
		},
		{
			name: "DR disabled - no dataStore block",
			spec: specWith("1.35.0", DataStoreSpec{Dedicated: false}),
			want: nil,
		},
		{
			name: "DR disabled on v-prefixed supported version - no dataStore block",
			spec: specWith("v1.35.5", DataStoreSpec{Dedicated: false}),
			want: nil,
		},
		{
			name: "DR enabled with explicit class, no size - size defaults to 8Gi",
			spec: specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default"}),
			want: &DataStore{Dedicated: true, StorageClassName: "default", Storage: "8Gi"},
		},
		{
			name: "DR enabled with no class and no size - both default",
			spec: specWith("1.35.0", DataStoreSpec{Dedicated: true}),
			want: &DataStore{Dedicated: true, StorageClassName: "sc1", Storage: "8Gi"},
		},
		{
			name: "DR enabled with no class, explicit size - class defaults, size respected",
			spec: specWith("1.35.0", DataStoreSpec{Dedicated: true, Storage: 3}),
			want: &DataStore{Dedicated: true, StorageClassName: "sc1", Storage: "3Gi"},
		},
		{
			name:    "DR enabled on unsupported version returns error",
			spec:    specWith("1.34.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8}),
			wantErr: true,
		},
		{
			name:    "DR enabled on v-prefixed unsupported version returns error",
			spec:    specWith("v1.34.5", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8}),
			wantErr: true,
		},
		{
			name: "DR disabled on unsupported version is allowed",
			spec: specWith("1.34.0", DataStoreSpec{Dedicated: false}),
			want: nil,
		},
		{
			name: "DR disabled on v-prefixed unsupported version is allowed",
			spec: specWith("v1.34.5", DataStoreSpec{Dedicated: false}),
			want: nil,
		},
		{
			name:    "shrinking storage is rejected",
			spec:    specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8}),
			oldSpec: ptr(specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 16})),
			wantErr: true,
		},
		{
			name:    "growing storage is allowed",
			spec:    specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 16}),
			oldSpec: ptr(specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8})),
			want:    &DataStore{Dedicated: true, StorageClassName: "default", Storage: "16Gi"},
		},
		{
			name:    "keeping storage equal is allowed",
			spec:    specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8}),
			oldSpec: ptr(specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8})),
			want:    &DataStore{Dedicated: true, StorageClassName: "default", Storage: "8Gi"},
		},
		{
			name:    "enabling DR fresh (old not dedicated) is allowed",
			spec:    specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 8}),
			oldSpec: ptr(specWith("1.35.0", DataStoreSpec{Dedicated: false})),
			want:    &DataStore{Dedicated: true, StorageClassName: "default", Storage: "8Gi"},
		},
		{
			name:    "shrinking below default via unset size is rejected",
			spec:    specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default"}),
			oldSpec: ptr(specWith("1.35.0", DataStoreSpec{Dedicated: true, StorageClassName: "default", Storage: 16})),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _, err := CreateKaaSAppValues(ctx, localId, location, tt.spec, kaasConfig, tt.oldSpec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateKaaSAppValues() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			var parsed Values
			if err := yaml.Unmarshal([]byte(values), &parsed); err != nil {
				t.Fatalf("failed to unmarshal values: %v", err)
			}
			got := parsed.Clusters[localId].ControlPlane.DataStore
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected no dataStore block, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected dataStore %+v, got nil", tt.want)
			}
			if *got != *tt.want {
				t.Errorf("dataStore = %+v, want %+v", *got, *tt.want)
			}
		})
	}

	t.Run("DR enabled but no storage classes configured returns error", func(t *testing.T) {
		emptyConfig := KaaSConfig{}
		spec := specWith("1.35.0", DataStoreSpec{Dedicated: true})
		if _, _, err := CreateKaaSAppValues(ctx, localId, location, spec, emptyConfig, nil); err == nil {
			t.Fatal("expected error when no storage classes are configured, got nil")
		}
	})
}

// TestConvertAppToUpdateKaaSSpec_DataStore round-trips a DR-enabled cluster:
// build the helm values, wrap them in the argo app shape, then convert back.
// appViewWithHelmValues wraps rendered helm values in the argo app shape the
// converter reads, mirroring what GetApp returns for a plugin-sourced app.
func appViewWithHelmValues(values string) view.AppView {
	return view.AppView{
		Spec: view.ApplicationSpec{
			Source: &view.ApplicationSource{
				Plugin: &v1alpha1.ApplicationSourcePlugin{
					Env: v1alpha1.Env{{Name: "HELM_VALUES", Value: values}},
				},
			},
		},
	}
}

func TestConvertAppToUpdateKaaSSpec_DataStore(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	ctx := context.Background()
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	group := Group{
		Name:         "group-1",
		Replicas:     3,
		Cpu:          2,
		Memory:       4,
		BootDiskSize: 20,
		StorageClass: "sc1",
		Subnets:      []GroupSubnet{{Order: 1, Id: "subnet-1"}},
	}

	tests := []struct {
		name      string
		dataStore DataStoreSpec
		want      DataStoreSpec
	}{
		{
			name:      "round-trips dedicated datastore",
			dataStore: DataStoreSpec{Dedicated: true, StorageClassName: "encrypted", Storage: 3},
			want:      DataStoreSpec{Dedicated: true, StorageClassName: "encrypted", Storage: 3},
		},
		{
			name:      "no datastore yields zero value",
			dataStore: DataStoreSpec{Dedicated: false},
			want:      DataStoreSpec{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := KaaSSpec{
				KubeVersion:   "1.35.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups:        []Group{group},
				ControlPlane:  ControlPlaneSpec{DataStore: tt.dataStore},
			}
			values, _, err := CreateKaaSAppValues(ctx, "test-cluster", "test-loc", spec, kaasConfig, nil)
			if err != nil {
				t.Fatalf("CreateKaaSAppValues() error = %v", err)
			}

			app := appViewWithHelmValues(values)

			got, err := ConvertAppToUpdateKaaSSpec(app)
			if err != nil {
				t.Fatalf("ConvertAppToUpdateKaaSSpec() error = %v", err)
			}
			if got.ControlPlane.DataStore != tt.want {
				t.Errorf("DataStore = %+v, want %+v", got.ControlPlane.DataStore, tt.want)
			}
		})
	}
}

func TestCreateKaaSAppValues_AzDomains(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	t.Cleanup(func() { config.Global.ProductsConfig.ArgoApp.Kubernetes.AzDomains = nil })
	ctx := context.Background()
	localId := "test-cluster"
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	spec := KaaSSpec{
		KubeVersion:   "1.35.0",
		CPNetPol:      "default",
		WorkersNetPol: "default",
		Groups: []Group{
			{Name: "group-1", Replicas: 3, Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
		},
	}

	tests := []struct {
		name      string
		azDomains map[string]any
		wantKeys  []string
		rawWant   []string // empty means the azDomains key must be absent
	}{
		{
			name: "configured with two entries",
			azDomains: map[string]any{
				"az01": map[string]any{"internal": "azs.az01.example.org", "external": "%s.kaas.az01.example.org"},
				"az02": map[string]any{"internal": "azs.az02.example.net", "external": "%s.kaas.az02.example.net"},
			},
			wantKeys: []string{"az01", "az02"},
			rawWant:  []string{"internal: azs.az01.example.org", "external: '%s.kaas.az02.example.net'"},
		},
		{
			name: "mixed legacy string and object entries",
			azDomains: map[string]any{
				"aq01":        "example.org",
				"aq01-test01": map[string]any{"internal": "azs.aq01.example.org", "external": "%s.kaas.aq01-test01.example.org"},
			},
			wantKeys: []string{"aq01", "aq01-test01"},
			rawWant:  []string{"aq01: example.org", "internal: azs.aq01.example.org", "external: '%s.kaas.aq01-test01.example.org'"},
		},
		{
			name: "configured with one entry",
			azDomains: map[string]any{
				"az01": map[string]any{"internal": "azs.az01.example.org", "external": "%s.kaas.az01.example.org"},
			},
			wantKeys: []string{"az01"},
			rawWant:  []string{"internal: azs.az01.example.org", "external: '%s.kaas.az01.example.org'"},
		},
		{
			name:      "unconfigured omits the key",
			azDomains: nil,
		},
		{
			name:      "empty map omits the key",
			azDomains: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.ProductsConfig.ArgoApp.Kubernetes.AzDomains = tt.azDomains
			values, _, err := CreateKaaSAppValues(ctx, localId, "test-loc", spec, kaasConfig, nil)
			if err != nil {
				t.Fatalf("CreateKaaSAppValues() error = %v", err)
			}
			if len(tt.rawWant) == 0 {
				if strings.Contains(values, "azDomains") {
					t.Errorf("expected values to NOT contain azDomains.\nValues:\n%s", values)
				}
			}
			for _, want := range tt.rawWant {
				if !strings.Contains(values, want) {
					t.Errorf("expected values to contain %q.\nValues:\n%s", want, values)
				}
			}
			var parsed Values
			if err := yaml.Unmarshal([]byte(values), &parsed); err != nil {
				t.Fatalf("failed to unmarshal values: %v", err)
			}
			if len(parsed.AzDomains) != len(tt.wantKeys) {
				t.Fatalf("azDomains = %+v, want keys %v", parsed.AzDomains, tt.wantKeys)
			}
			for _, k := range tt.wantKeys {
				if _, ok := parsed.AzDomains[k]; !ok {
					t.Errorf("azDomains missing key %q in %+v", k, parsed.AzDomains)
				}
			}
		})
	}
}

// TestConvertAppToUpdateKaaSSpec_AzDomains checks the azDomains key in the
// helm values never breaks the update re-parse path.
func TestConvertAppToUpdateKaaSSpec_AzDomains(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	t.Cleanup(func() { config.Global.ProductsConfig.ArgoApp.Kubernetes.AzDomains = nil })
	ctx := context.Background()
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	group := Group{
		Name:         "group-1",
		Replicas:     3,
		Cpu:          2,
		Memory:       4,
		BootDiskSize: 20,
		StorageClass: "sc1",
		Subnets:      []GroupSubnet{{Order: 1, Id: "subnet-1"}},
	}

	tests := []struct {
		name      string
		azDomains map[string]any
	}{
		{name: "populated map", azDomains: map[string]any{
			"az01": map[string]any{"internal": "azs.az01.example.org", "external": "%s.kaas.az01.example.org"},
			"az02": map[string]any{"internal": "azs.az02.example.net", "external": "%s.kaas.az02.example.net"},
		}},
		{name: "mixed legacy string and object entries", azDomains: map[string]any{
			"aq01":        "example.org",
			"aq01-test01": map[string]any{"internal": "azs.aq01.example.org", "external": "%s.kaas.aq01-test01.example.org"},
		}},
		{name: "empty map", azDomains: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.ProductsConfig.ArgoApp.Kubernetes.AzDomains = tt.azDomains
			spec := KaaSSpec{
				KubeVersion:   "1.35.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups:        []Group{group},
			}
			values, _, err := CreateKaaSAppValues(ctx, "test-cluster", "test-loc", spec, kaasConfig, nil)
			if err != nil {
				t.Fatalf("CreateKaaSAppValues() error = %v", err)
			}

			app := appViewWithHelmValues(values)

			got, err := ConvertAppToUpdateKaaSSpec(app)
			if err != nil {
				t.Fatalf("ConvertAppToUpdateKaaSSpec() error = %v", err)
			}
			if got.KubeVersion != spec.KubeVersion {
				t.Errorf("KubeVersion = %q, want %q", got.KubeVersion, spec.KubeVersion)
			}
			if len(got.Groups) != 1 || got.Groups[0].Name != group.Name {
				t.Errorf("Groups = %+v, want one group named %q", got.Groups, group.Name)
			}
		})
	}
}

func TestCreateArgoApp_HelmParams(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	group := Group{
		Name: "group-1", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20,
		StorageClass: "default", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}},
	}
	az := config.AZConfig{Code: "az01", Destination: "dest1"}
	metadata := spxId.Metadata{OrgId: "org", ProjectId: "proj", ResourceEffectiveId: "cluster"}

	tests := []struct {
		name           string
		storageClasses []ClassMapping
		wantParams     []string
		unwantedParams []string
	}{
		{
			name:           "single class maps to flat storageClassMapping",
			storageClasses: []ClassMapping{{Shortname: "default", Fullname: "az01-storage01.default"}},
			wantParams:     []string{"--set storageClassMapping.default=az01-storage01.default"},
			unwantedParams: []string{"storageClassName", "snapshotClassName"},
		},
		{
			name: "multiple classes map to one entry each",
			storageClasses: []ClassMapping{
				{Shortname: "default", Fullname: "az01-storage01.default"},
				{Shortname: "fast", Fullname: "az01-storage01.fast"},
			},
			wantParams: []string{
				"--set storageClassMapping.default=az01-storage01.default",
				"--set storageClassMapping.fast=az01-storage01.fast",
			},
			unwantedParams: []string{"storageClassName", "snapshotClassName"},
		},
		{
			name:           "no classes keeps base params only",
			storageClasses: nil,
			wantParams:     []string{"--set location=az01", "--set organizationID=org", "--set projectID=proj"},
			unwantedParams: []string{"storageClassMapping"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := KaaSSpec{
				KubeVersion:   "1.35.0",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups:        []Group{group},
			}
			body, _, err := CreateArgoApp(context.Background(), "cluster", az, spec, metadata, KaaSConfig{StorageClasses: tt.storageClasses}, nil, config.RepoArgoAppConfig{})
			if err != nil {
				t.Fatalf("CreateArgoApp() error = %v", err)
			}
			var helmParams string
			for _, env := range body.Spec.Source.Plugin.Env {
				if env.Name == "HELM_PARAMS" {
					helmParams = env.Value
				}
			}
			if helmParams == "" {
				t.Fatal("HELM_PARAMS env not found or empty")
			}
			for _, want := range tt.wantParams {
				if !strings.Contains(helmParams, want) {
					t.Errorf("expected HELM_PARAMS to contain %q.\nHELM_PARAMS: %s", want, helmParams)
				}
			}
			for _, unwanted := range tt.unwantedParams {
				if strings.Contains(helmParams, unwanted) {
					t.Errorf("expected HELM_PARAMS to NOT contain %q.\nHELM_PARAMS: %s", unwanted, helmParams)
				}
			}
		})
	}
}

func TestCreateArgoApp_Repo(t *testing.T) {
	configured := config.RepoArgoAppConfig{
		RepoURL:        "ghcr.io/super-phenix/charts",
		TargetRevision: "0.7.0",
		Chart:          "sfs-kaas",
	}
	pinned := config.RepoArgoAppConfig{
		RepoURL:        "ghcr.io/super-phenix/charts",
		TargetRevision: "0.3.8",
		Chart:          "sfs-kaas",
	}
	gitPath := config.RepoArgoAppConfig{
		RepoURL:        "https://github.com/super-phenix/superphenix",
		TargetRevision: "main",
		Path:           "components/dependencies/sfs-kaas",
	}

	config.Global.ProductsConfig.ArgoApp.Kubernetes.Repo = configured
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "v1.35.5"}}

	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	group := Group{
		Name: "group-1", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20,
		StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}},
	}
	az := config.AZConfig{Code: "az1", Destination: "dest1"}
	metadata := spxId.Metadata{OrgId: "org", ProjectId: "proj", ResourceEffectiveId: "cluster"}

	tests := []struct {
		name  string
		chart config.RepoArgoAppConfig
	}{
		{name: "configured chart", chart: configured},
		{name: "pinned chart differs from config", chart: pinned},
		{name: "path based chart", chart: gitPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := KaaSSpec{
				KubeVersion:   "v1.35.5",
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups:        []Group{group},
			}
			body, _, err := CreateArgoApp(context.Background(), "cluster", az, spec, metadata, kaasConfig, nil, tt.chart)
			if err != nil {
				t.Fatalf("CreateArgoApp() error = %v", err)
			}
			src := body.Spec.Source
			got := config.RepoArgoAppConfig{RepoURL: src.RepoURL, TargetRevision: src.TargetRevision, Chart: src.Chart, Path: src.Path}
			if got != tt.chart {
				t.Errorf("source = %+v, want %+v", got, tt.chart)
			}
		})
	}
}

// TestCreateKaaSAppValues_KubeVersionSupport checks the kube version is only
// validated against config on create and on a version change.
func TestCreateKaaSAppValues_KubeVersionSupport(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "v1.36.3"}}
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	group := Group{
		Name: "group-1", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20,
		StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}},
	}
	specFor := func(version string) KaaSSpec {
		return KaaSSpec{KubeVersion: version, CPNetPol: "default", WorkersNetPol: "default", Groups: []Group{group}}
	}
	removed := specFor("v1.33.4")

	tests := []struct {
		name    string
		spec    KaaSSpec
		oldSpec *KaaSSpec
		wantErr bool
	}{
		{name: "create with supported version", spec: specFor("v1.36.3"), oldSpec: nil, wantErr: false},
		{name: "create with removed version", spec: specFor("v1.33.4"), oldSpec: nil, wantErr: true},
		{name: "update keeping removed version", spec: specFor("v1.33.4"), oldSpec: &removed, wantErr: false},
		{name: "update to supported version", spec: specFor("v1.36.3"), oldSpec: &removed, wantErr: false},
		{name: "update to unsupported version", spec: specFor("v1.34.1"), oldSpec: &removed, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := CreateKaaSAppValues(context.Background(), "test-cluster", "test-loc", tt.spec, kaasConfig, tt.oldSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateKaaSAppValues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// nodeRoleGroup is a valid group with a legacy-style mixed-case name.
func nodeRoleGroup(role string, version int) Group {
	return Group{
		Name:         "aB3xYz9Q",
		NodeRole:     role,
		Replicas:     3,
		Cpu:          2,
		Memory:       4,
		BootDiskSize: 20,
		StorageClass: "sc1",
		Subnets:      []GroupSubnet{{Order: 1, Id: "subnet-1"}},
		Version:      version,
	}
}

func nodeRoleSpec(g Group) KaaSSpec {
	return KaaSSpec{
		KubeVersion:   "1.35.0",
		CPNetPol:      "default",
		WorkersNetPol: "default",
		Groups:        []Group{g},
	}
}

func TestCreateKaaSAppValues_NodeRole(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	localId := "test-cluster"
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	updateFrom := func(role string) *KaaSSpec {
		s := nodeRoleSpec(nodeRoleGroup(role, 2))
		return &s
	}

	tests := []struct {
		name        string
		role        string
		oldSpec     *KaaSSpec
		wantErr     bool
		wantRole    string
		wantVersion int
	}{
		{name: "role is rendered", role: "gpu", wantRole: "gpu", wantVersion: 1},
		{name: "empty role is omitted", role: "", wantRole: "", wantVersion: 1},
		{name: "uppercase role is rendered", role: "GPU", wantRole: "GPU", wantVersion: 1},
		{name: "role with dot is rendered", role: "gpu.large", wantRole: "gpu.large", wantVersion: 1},
		{name: "role with underscore is rendered", role: "gpu_large", wantRole: "gpu_large", wantVersion: 1},
		{name: "control-plane is rendered", role: "control-plane", wantRole: "control-plane", wantVersion: 1},
		{name: "63 chars is rendered", role: strings.Repeat("a", 63), wantRole: strings.Repeat("a", 63), wantVersion: 1},
		{name: "leading dash is rejected", role: "-gpu", wantErr: true},
		{name: "trailing dot is rejected", role: "gpu.", wantErr: true},
		{name: "slash is rejected", role: "gpu/large", wantErr: true},
		{name: "space is rejected", role: "gpu large", wantErr: true},
		{name: "role over 63 chars is rejected", role: strings.Repeat("a", 64), wantErr: true},
		{name: "role change keeps version", role: "compute", oldSpec: updateFrom("gpu"), wantRole: "compute", wantVersion: 2},
		{name: "legacy group gets a role and keeps version", role: "worker", oldSpec: updateFrom(""), wantRole: "worker", wantVersion: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _, err := CreateKaaSAppValues(context.Background(), localId, "test-loc", nodeRoleSpec(nodeRoleGroup(tt.role, 0)), kaasConfig, tt.oldSpec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateKaaSAppValues() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var roleErr *NodeRoleError
				if !errors.As(err, &roleErr) {
					t.Errorf("error = %v, want a *NodeRoleError", err)
				}
				return
			}
			if tt.wantRole == "" && strings.Contains(values, "nodeRole") {
				t.Errorf("expected values to NOT contain nodeRole.\nValues:\n%s", values)
			}
			var parsed Values
			if err := yaml.Unmarshal([]byte(values), &parsed); err != nil {
				t.Fatalf("failed to unmarshal values: %v", err)
			}
			instance := parsed.Clusters[localId].Workers.Instances["aB3xYz9Q"]
			if instance.Deployment.NodeRole != tt.wantRole {
				t.Errorf("nodeRole = %q, want %q", instance.Deployment.NodeRole, tt.wantRole)
			}
			if instance.Template.Version != tt.wantVersion {
				t.Errorf("version = %d, want %d", instance.Template.Version, tt.wantVersion)
			}
		})
	}
}

func TestConvertAppToUpdateKaaSSpec_NodeRole(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}

	tests := []struct {
		name string
		role string
	}{
		{name: "round-trips a set role", role: "gpu"},
		{name: "round-trips a mixed-case dotted role", role: "GPU.large"},
		{name: "legacy group keeps an empty role", role: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := nodeRoleSpec(nodeRoleGroup(tt.role, 0))
			values, _, err := CreateKaaSAppValues(context.Background(), "test-cluster", "test-loc", spec, kaasConfig, nil)
			if err != nil {
				t.Fatalf("CreateKaaSAppValues() error = %v", err)
			}

			got, err := ConvertAppToUpdateKaaSSpec(appViewWithHelmValues(values))
			if err != nil {
				t.Fatalf("ConvertAppToUpdateKaaSSpec() error = %v", err)
			}
			if len(got.Groups) != 1 || got.Groups[0].NodeRole != tt.role {
				t.Errorf("Groups = %+v, want one group with nodeRole %q", got.Groups, tt.role)
			}
		})
	}
}

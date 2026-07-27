package kaas

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"gopkg.in/yaml.v2"
)

func TestCreateKaaSAppValues_Comparison(t *testing.T) {
	config.Global.ArgoController.App.KaaS.KubeVersions = []config.KubeVersionConfig{{Version: "1.24.0"}}
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
					{Name: "group-2", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
					{Name: "group-1", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
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
					{Name: "group-0", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
					{Name: "group-1", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
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
					{Name: "group-0", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}},
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
					{Name: "group-1", Cpu: 1, Memory: 1, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}}, // Same as old[1] -> version 4
					{Name: "group-0", Cpu: 4, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}}, // Changed from old[0] -> version 3
					{Name: "group-2", Cpu: 2, Memory: 4, BootDiskSize: 20, StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}}}, // New -> version 1
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
	config.Global.ArgoController.App.KaaS.KubeVersions = []config.KubeVersionConfig{{Version: "1.34.0"}, {Version: "v1.34.5"}, {Version: "1.35.0"}, {Version: "v1.35.5"}}
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
func TestConvertAppToUpdateKaaSSpec_DataStore(t *testing.T) {
	config.Global.ArgoController.App.KaaS.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
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

			app := map[string]interface{}{
				"app": map[string]interface{}{
					"spec": map[string]interface{}{
						"source": map[string]interface{}{
							"plugin": map[string]interface{}{
								"env": []interface{}{
									map[string]interface{}{"name": "HELM_VALUES", "value": values},
								},
							},
						},
					},
				},
			}

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

func TestCreateArgoApp_Repo(t *testing.T) {
	defaultRepo := config.RepoArgoAppConfig{
		RepoURL:        "ghcr.io/super-phenix/charts",
		TargetRevision: "0.1.0",
		Chart:          "sfs-kaas",
	}
	overrideRepo := config.RepoArgoAppConfig{
		RepoURL:        "ghcr.io/super-phenix/edge",
		TargetRevision: "0.2.0",
		Chart:          "sfs-kaas-edge",
	}

	config.Global.ArgoController.App.KaaS.Repo = defaultRepo
	config.Global.ArgoController.App.KaaS.KubeVersions = []config.KubeVersionConfig{
		{Version: "v1.35.5", Repo: &overrideRepo},
		{Version: "v1.34.8"},
	}

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
		name        string
		kubeVersion string
		want        config.RepoArgoAppConfig
	}{
		{name: "version with override uses override repo", kubeVersion: "v1.35.5", want: overrideRepo},
		{name: "version without override uses default repo", kubeVersion: "v1.34.8", want: defaultRepo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := KaaSSpec{
				KubeVersion:   tt.kubeVersion,
				CPNetPol:      "default",
				WorkersNetPol: "default",
				Groups:        []Group{group},
			}
			body, _, err := CreateArgoApp(context.Background(), "cluster", az, spec, metadata, kaasConfig, nil)
			if err != nil {
				t.Fatalf("CreateArgoApp() error = %v", err)
			}
			src := body.Spec.Source
			if src.RepoURL != tt.want.RepoURL || src.TargetRevision != tt.want.TargetRevision || src.Chart != tt.want.Chart {
				t.Errorf("source repo = {URL:%q Rev:%q Chart:%q}, want {URL:%q Rev:%q Chart:%q}",
					src.RepoURL, src.TargetRevision, src.Chart, tt.want.RepoURL, tt.want.TargetRevision, tt.want.Chart)
			}
		})
	}
}

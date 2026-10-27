package dbaas

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestConvertAppToDBaaSSpec(t *testing.T) {
	setPostgresVersions(t, "17.6")
	subnet := Subnet{LocalID: "subnet-local", CIDR: "10.10.0.0/24,fd00::/64"}
	subnetEId, err := effectiveID(testProjectID, subnet.LocalID)
	if err != nil {
		t.Fatalf("effectiveID() error: %v", err)
	}
	spec := validSpec()
	spec.SubnetEId = subnetEId
	spec.ExternalAccess = true
	values, err := CreateAppValues(context.Background(), "db-1", "az1", spec, subnet, nil)
	if err != nil {
		t.Fatalf("CreateAppValues() error: %v", err)
	}

	tests := []struct {
		name       string
		env        v1alpha1.Env
		wantSpec   DBaaSSpec
		wantSubnet Subnet
		wantErr    bool
	}{
		{
			name:       "round trip",
			env:        v1alpha1.Env{{Name: "HELM_VALUES", Value: values}},
			wantSpec:   spec,
			wantSubnet: subnet,
		},
		{name: "no database", env: v1alpha1.Env{{Name: "HELM_VALUES", Value: "databases: {}\n"}}, wantErr: true},
		{name: "invalid memory", env: v1alpha1.Env{{Name: "HELM_VALUES", Value: "databases:\n  db-1:\n    cpu: \"1\"\n    memory: lots\n"}}, wantErr: true},
		{name: "invalid yaml", env: v1alpha1.Env{{Name: "HELM_VALUES", Value: "databases: ["}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpec, gotSubnet, err := ConvertAppToDBaaSSpec(appView(tt.env), testProjectID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ConvertAppToDBaaSSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if gotSpec != tt.wantSpec {
				t.Errorf("spec = %+v, want %+v", gotSpec, tt.wantSpec)
			}
			if gotSubnet != tt.wantSubnet {
				t.Errorf("subnet = %+v, want %+v", gotSubnet, tt.wantSubnet)
			}
		})
	}
}

package superphenixId

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

const (
	validUUID   = "cd6c42c4-74ec-44b1-a90d-eb3786a6ec11"
	invalidUUID = "ThisIsNotAUUID"

	validOrgId        = "0f1e5be5-4c92-4147-b46c-1b018c80f722"
	validProjectID    = "e98222a9-187a-4b1b-8f37-89eb1d3bc23c"
	validLocalIdPlain = "my-resource"
	validLocalIdUuid  = "37b65e61-d66b-49c2-a6b7-4378463033f9"

	computedEffectiveIdFromPlain = "47124ad6-748a-544b-9ed8-ce6f193408d9"
	computedEffectiveIdFromUUID  = "e33fa933-3595-5ba6-adf9-7b311b5a261f"

	invalidOrgId     = "MyOrganization"
	invalidProjectID = "MyProject"
	invalidLocalId   = "MyResource"

	longString = "thisisasuperveryverylongwordtofailthelengthcheckfortestingpurpose"
)

var (
	nilUUID = uuid.Nil.String()
)

func TestMetadata_ConvertToSpxMetadata(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	type args struct {
		info Metadata
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if err := m.ConvertToSpxMetadata(tt.args.info); (err != nil) != tt.wantErr {
				t.Errorf("ConvertToSpxMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetadata_GenerateFromProject(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	type args struct {
		project Metadata
		localID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if err := m.GenerateFromProject(tt.args.project, tt.args.localID); (err != nil) != tt.wantErr {
				t.Errorf("GenerateFromProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetadata_GenerateMetadata(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	type args struct {
		projectId string
		orgId     string
		localID   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if err := m.GenerateMetadata(tt.args.projectId, tt.args.orgId, tt.args.localID); (err != nil) != tt.wantErr {
				t.Errorf("GenerateMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetadata_GetLabels(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if got := m.GetLabels(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadata_GetOrgID(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if got := m.GetOrgID(); got != tt.want {
				t.Errorf("GetOrgID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadata_GetProjectID(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if got := m.GetProjectID(); got != tt.want {
				t.Errorf("GetProjectID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadata_GetResourceEffectiveID(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if got := m.GetResourceEffectiveID(); got != tt.want {
				t.Errorf("GetResourceEffectiveID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadata_GetResourceLocalID(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if got := m.GetResourceLocalID(); got != tt.want {
				t.Errorf("GetResourceLocalID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadata_check(t *testing.T) {
	type fields struct {
		OrgId               string
		ProjectId           string
		ResourceEffectiveId string
		ResourceLocalId     string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:               tt.fields.OrgId,
				ProjectId:           tt.fields.ProjectId,
				ResourceEffectiveId: tt.fields.ResourceEffectiveId,
				ResourceLocalId:     tt.fields.ResourceLocalId,
			}
			if err := m.check(); (err != nil) != tt.wantErr {
				t.Errorf("check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetadata_checkUUIDs(t *testing.T) {
	type fields struct {
		OrgId           string
		ProjectId       string
		ResourceLocalId string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{
			name:    "Missing Org Id",
			fields:  fields{OrgId: "", ProjectId: validProjectID, ResourceLocalId: validLocalIdPlain},
			wantErr: InvalidOrgID,
		},
		{
			name:    "Invalid Org Id",
			fields:  fields{OrgId: invalidOrgId, ProjectId: validProjectID, ResourceLocalId: validLocalIdPlain},
			wantErr: InvalidOrgID,
		},
		{
			name:    "Missing Project Id",
			fields:  fields{OrgId: validOrgId, ProjectId: "", ResourceLocalId: validLocalIdPlain},
			wantErr: nil,
		},
		{
			name:    "Invalid Project Id",
			fields:  fields{OrgId: validOrgId, ProjectId: invalidProjectID, ResourceLocalId: validLocalIdPlain},
			wantErr: InvalidProjectID,
		},
		{
			name:    "Missing Local Id",
			fields:  fields{OrgId: validOrgId, ProjectId: validProjectID, ResourceLocalId: ""},
			wantErr: nil,
		},
		{
			name:    "Invalid Local Id",
			fields:  fields{OrgId: validOrgId, ProjectId: validProjectID, ResourceLocalId: invalidLocalId},
			wantErr: InvalidResourceLocalID,
		},
		{
			name:    "Too long Local Id",
			fields:  fields{OrgId: validOrgId, ProjectId: validProjectID, ResourceLocalId: longString},
			wantErr: InvalidResourceLocalID,
		},
		{
			name:    "Valid fields - Local ID as Plain",
			fields:  fields{OrgId: validOrgId, ProjectId: validProjectID, ResourceLocalId: validLocalIdPlain},
			wantErr: nil,
		},
		{
			name:    "Valid fields - Local ID as UUID",
			fields:  fields{OrgId: validOrgId, ProjectId: validProjectID, ResourceLocalId: validLocalIdUuid},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				OrgId:           tt.fields.OrgId,
				ProjectId:       tt.fields.ProjectId,
				ResourceLocalId: tt.fields.ResourceLocalId,
			}
			if err := m.checkUUIDs(); !errors.Is(err, tt.wantErr) {
				t.Errorf("checkUUIDs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetadata_computeEffectiveID(t *testing.T) {
	type fields struct {
		ProjectId       string
		ResourceLocalId string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr error
		wantEID string
	}{
		{
			name:    "Missing Project ID",
			fields:  fields{ProjectId: "", ResourceLocalId: validLocalIdPlain},
			wantErr: InvalidProjectOrResourceLocalID},
		{
			name:    "Missing Local ID",
			fields:  fields{ProjectId: validProjectID, ResourceLocalId: ""},
			wantErr: InvalidProjectOrResourceLocalID},
		{
			name:    "Invalid Project ID",
			fields:  fields{ProjectId: invalidProjectID, ResourceLocalId: validLocalIdPlain},
			wantErr: InvalidProjectID},
		{
			name:    "Valid fields - Local as Plain",
			fields:  fields{ProjectId: validProjectID, ResourceLocalId: validLocalIdPlain},
			wantErr: nil,
			wantEID: fmt.Sprintf("%s-%s", FrameworkPrefix(), computedEffectiveIdFromPlain)},
		{
			name:    "Valid fields - Local as UUID",
			fields:  fields{ProjectId: validProjectID, ResourceLocalId: validLocalIdUuid},
			wantErr: nil,
			wantEID: fmt.Sprintf("%s-%s", FrameworkPrefix(), computedEffectiveIdFromUUID)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metadata{
				ProjectId:       tt.fields.ProjectId,
				ResourceLocalId: tt.fields.ResourceLocalId,
			}
			err := m.computeEffectiveID()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("computeEffectiveID() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				got := m.GetResourceEffectiveID()
				if got != tt.wantEID {
					t.Errorf("computeEffectiveID() EffectiveId = %v, want %v", got, tt.wantEID)
				}
			}
		})
	}
}

func TestToSPXID(t *testing.T) {
	type args struct {
		id string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "Empty value", args: struct{ id string }{id: ""}, want: ""},
		{name: "Valid value", args: struct{ id string }{id: validUUID}, want: fmt.Sprintf("%s-%s", FrameworkPrefix(), validUUID)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToSPXID(tt.args.id); got != tt.want {
				t.Errorf("ToSPXID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isValidName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "Invalid Resource Name", args: struct{ name string }{name: invalidLocalId}, want: false},
		{name: "Empty Resource Name", args: struct{ name string }{name: ""}, want: false},
		{name: "Long Resource Name", args: struct{ name string }{name: longString}, want: false},
		{name: "Valid Resource Name - as Plain", args: struct{ name string }{name: validLocalIdPlain}, want: true},
		{name: "Valid Resource Name - as UUID", args: struct{ name string }{name: validLocalIdUuid}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidName(tt.args.name); got != tt.want {
				t.Errorf("isValidName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isValidUUID(t *testing.T) {
	type args struct {
		uuidString string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "Invalid UUID", args: struct{ uuidString string }{uuidString: invalidUUID}, want: false},
		{name: "Nil UUID", args: struct{ uuidString string }{uuidString: nilUUID}, want: true},
		{name: "Valid UUID", args: struct{ uuidString string }{uuidString: validUUID}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidUUID(tt.args.uuidString); got != tt.want {
				t.Errorf("isValidUUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

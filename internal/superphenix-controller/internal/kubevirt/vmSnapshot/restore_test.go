package vmSnapshot

import (
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// TestCheckSourceLocalId ensures a restore is only accepted with the local ID
// of the snapshot source VM, so the API cannot be made to register the
// restored VM under another product ID.
func TestCheckSourceLocalId(t *testing.T) {
	const (
		orgId        = "33333333-3333-3333-3333-333333333333"
		projectId    = "11111111-1111-1111-1111-111111111111"
		otherProject = "22222222-2222-2222-2222-222222222222"
		vmLocalId    = "44444444-4444-4444-4444-444444444444"
		otherLocalId = "55555555-5555-5555-5555-555555555555"
	)

	source := spxId.Metadata{}
	if err := source.GenerateMetadata(projectId, orgId, vmLocalId); err != nil {
		t.Fatal(err)
	}
	sourceName := source.GetResourceEffectiveID()

	tests := []struct {
		name      string
		projectId string
		localId   string
		wantErr   bool
	}{
		{name: "local ID of the source VM is accepted", projectId: projectId, localId: vmLocalId},
		{name: "local ID of another product is rejected", projectId: projectId, localId: otherLocalId, wantErr: true},
		{name: "source local ID under another project is rejected", projectId: otherProject, localId: vmLocalId, wantErr: true},
		{name: "empty local ID is rejected", projectId: projectId, localId: "", wantErr: true},
		{name: "invalid local ID is rejected", projectId: projectId, localId: "not-a-uuid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSourceLocalId(orgId, tt.projectId, tt.localId, sourceName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkSourceLocalId() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !apierrors.IsBadRequest(err) {
				t.Errorf("expected a BadRequest error, got %v", err)
			}
		})
	}
}

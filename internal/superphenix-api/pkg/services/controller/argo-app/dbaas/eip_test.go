package dbaas

import "testing"

func TestEipEffectiveID(t *testing.T) {
	tests := []struct {
		name      string
		projectId string
		localId   string
		want      string
		wantErr   bool
	}{
		// uuidv5(6f1c2c1e-3b8a-4b8e-9d4a-2f1e5c7b9a10, "eip.db-1"), same as the uuidv5 Argo plugin
		{name: "valid project", projectId: testProjectID, localId: "db-1", want: "spx-622eb9ec-9e05-5ef2-a49b-609f5c3bb779"},
		{name: "invalid project", projectId: "not-a-uuid", localId: "db-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EipEffectiveID(tt.projectId, tt.localId)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EipEffectiveID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("EipEffectiveID() = %s, want %s", got, tt.want)
			}
		})
	}
}

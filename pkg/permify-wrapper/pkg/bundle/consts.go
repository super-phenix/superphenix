package bundle

import permifyPayload "buf.build/gen/go/permifyco/permify/protocolbuffers/go/base/v1"

var (
	// CreateProjectBundle initialize relation between organisation member and the new project
	CreateProjectBundle = &DataBundle{
		Name:      "create_project",
		Arguments: []string{"projectId", "orgaId"},
		Operations: []*permifyPayload.Operation{
			{
				RelationshipsWrite: []string{
					"project:{{.projectId}}#parent@organization:{{.orgaId}}",
				},
			},
		},
	}
)

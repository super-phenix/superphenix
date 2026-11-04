package ssh

import (
	"reflect"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/google/uuid"
)

func TestFillUserNames(t *testing.T) {
	instances := []model.Product{{EffectiveID: "vm-1", ProductName: "db name"}}

	tests := []struct {
		name      string
		users     []SSHUser
		instances []model.Product
		want      []SSHUser
	}{
		{name: "no user", users: []SSHUser{}, instances: instances, want: []SSHUser{}},
		{name: "name from the database", users: []SSHUser{{EId: "vm-1"}}, instances: instances, want: []SSHUser{{EId: "vm-1", ProductName: "db name"}}},
		{name: "gitops name kept", users: []SSHUser{{EId: "vm-1", ProductName: "gitops"}}, instances: instances, want: []SSHUser{{EId: "vm-1", ProductName: "gitops"}}},
		{name: "unknown instance", users: []SSHUser{{EId: "vm-2"}}, instances: instances, want: []SSHUser{{EId: "vm-2"}}},
		{name: "database failure", users: []SSHUser{{EId: "vm-1"}}, want: []SSHUser{{EId: "vm-1"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fillUserNames(tt.users, tt.instances)
			if !reflect.DeepEqual(tt.users, tt.want) {
				t.Errorf("users = %+v, want %+v", tt.users, tt.want)
			}
		})
	}
}

func TestCombineListResultIsUsed(t *testing.T) {
	inDb := uuid.New()
	missing := uuid.New()

	tests := []struct {
		name   string
		result map[string]any
		eid    string
		want   bool
	}{
		{
			name:   "key in database",
			result: map[string]any{"id": inDb.String(), "eid": "key-1", "gitops": "false", "ssh": nil, "isUsed": true},
			eid:    "key-1",
			want:   true,
		},
		{
			name:   "gitops only key",
			result: map[string]any{"id": uuid.NewString(), "eid": "key-2", "productName": "gitops", "gitops": "true", "ssh": nil, "isUsed": true},
			eid:    "key-2",
			want:   true,
		},
		{
			name:   "unused key",
			result: map[string]any{"id": inDb.String(), "eid": "key-1", "gitops": "false", "ssh": nil, "isUsed": false},
			eid:    "key-1",
		},
		{
			name:   "field absent",
			result: map[string]any{"id": inDb.String(), "eid": "key-1", "gitops": "false", "ssh": nil},
			eid:    "key-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := []model.Product{{Model: model.Model{ID: inDb}, EffectiveID: "key-1"}, {Model: model.Model{ID: missing}, EffectiveID: "key-3"}}
			check := map[uuid.UUID]bool{inDb: false, missing: false}

			results := combineListResult(map[string][]any{"az1": {tt.result}}, resources, check)

			found := false
			for _, result := range results {
				switch result.EId {
				case tt.eid:
					found = true
					if result.IsUsed != tt.want {
						t.Errorf("isUsed = %v, want %v", result.IsUsed, tt.want)
					}
				case "key-3":
					if result.IsUsed {
						t.Error("key missing from the AZ is marked as used")
					}
				}
			}
			if !found {
				t.Errorf("key %s not in results %+v", tt.eid, results)
			}
		})
	}
}

func TestRoutePermissions(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		// Added to the group permission, ProjectSSHRead
		want int
	}{
		{name: "key details only need ssh read", pattern: "/{az}/{projectId}/ssh/{effectiveId}"},
		{name: "usage also needs instance read", pattern: "/{az}/{projectId}/ssh/{effectiveId}/usage", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, route := range Module(&config.Config{}, New(&config.Config{})).Groups[0].Routes {
				if route.Pattern == tt.pattern {
					found = true
					if len(route.Middlewares) != tt.want {
						t.Errorf("route has %d permissions, want %d", len(route.Middlewares), tt.want)
					}
				}
			}
			if !found {
				t.Errorf("route %s not declared", tt.pattern)
			}
		})
	}
}

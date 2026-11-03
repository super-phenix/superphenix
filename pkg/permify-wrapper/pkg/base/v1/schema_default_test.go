package v1

import (
	"testing"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permissionSet"

	"github.com/stretchr/testify/assert"
)

func TestPredefinedGroupsCatalog(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantName     string
		wantContains []string
	}{
		{
			name:         "owner",
			key:          PredefinedGroupOwner,
			wantName:     DefaultGroupOwnerName,
			wantContains: []string{permissionSet.SpxOwner},
		},
		{
			name:         "admin merges organization and project sets",
			key:          PredefinedGroupAdmin,
			wantName:     DefaultGroupAdminName,
			wantContains: []string{permissionSet.IAMFullAccess, permissionSet.ProjectBucketFullAccess},
		},
		{
			name:         "billing",
			key:          PredefinedGroupBilling,
			wantName:     DefaultGroupBillingName,
			wantContains: []string{permissionSet.SpxMember, permissionSet.BillingFullAccess},
		},
		{
			name:         "developer",
			key:          PredefinedGroupDeveloper,
			wantName:     DefaultGroupDeveloperName,
			wantContains: []string{permissionSet.ProjectManagement, permissionSet.ProjectInstanceFullAccess, permissionSet.ProjectQuotaReadOnly, permissionSet.ProjectAZQuotaReadOnly},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, ok := FindPredefinedGroup(tt.key)
			assert.True(t, ok)
			assert.Equal(t, tt.wantName, group.Name)
			for _, pSet := range tt.wantContains {
				assert.Contains(t, group.PermissionSets, pSet)
			}
		})
	}

	t.Run("unknown key is not found", func(t *testing.T) {
		_, ok := FindPredefinedGroup("does-not-exist")
		assert.False(t, ok)
	})

	t.Run("keys are unique", func(t *testing.T) {
		seen := make(map[string]struct{})
		for _, group := range PredefinedGroups {
			_, duplicate := seen[group.Key]
			assert.False(t, duplicate, "duplicate key %s", group.Key)
			seen[group.Key] = struct{}{}
		}
	})

	// An aliased slice would let one caller corrupt the catalog for every other.
	t.Run("Sets returns a copy", func(t *testing.T) {
		group, ok := FindPredefinedGroup(PredefinedGroupBilling)
		assert.True(t, ok)

		sets := group.Sets()
		assert.Equal(t, group.PermissionSets, sets)

		sets[0] = "Injected"
		assert.NotContains(t, group.PermissionSets, "Injected")

		// The entries must not alias the DefaultOrganizationX slices either.
		assert.NotContains(t, DefaultOrganizationBilling, "Injected")
	})
}

func TestIsPredefinedGroupName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "owner", input: DefaultGroupOwnerName, want: true},
		{name: "developer", input: DefaultGroupDeveloperName, want: true},
		{name: "custom name", input: "Data Engineers", want: false},
		{name: "case sensitive", input: "owner", want: false},
		{name: "empty", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsPredefinedGroupName(tt.input))
		})
	}
}

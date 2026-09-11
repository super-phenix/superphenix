package summary

import (
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"

	"github.com/stretchr/testify/assert"
)

func TestFilterReadable(t *testing.T) {
	allPermissions := map[string]bool{}
	for _, permission := range model.ProductTypeReadPermission {
		allPermissions[permission] = true
	}

	tests := []struct {
		name    string
		granted map[string]bool
		want    []string
	}{
		{
			name:    "no permission",
			granted: map[string]bool{},
			want:    []string{},
		},
		{
			name:    "snapshot read gates both snapshot types",
			granted: map[string]bool{pwPermission.ProjectSnapshotRead: true},
			want:    []string{model.ProductTypeSnapshot, model.ProductTypeVmSnapshot},
		},
		{
			name:    "two unrelated products",
			granted: map[string]bool{pwPermission.ProjectKaaSRead: true, pwPermission.ProjectBucketRead: true},
			want:    []string{model.ProductTypeBucket, model.ProductTypeKaaS},
		},
		{
			name:    "permission that gates no product type",
			granted: map[string]bool{pwPermission.ProjectArgoCdRead: true, pwPermission.ProjectRead: true},
			want:    []string{},
		},
		{
			name:    "every permission",
			granted: allPermissions,
			want:    slices.Sorted(maps.Keys(model.ProductTypeReadPermission)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, filterReadable(tt.granted))
		})
	}
}

func TestModuleRoute(t *testing.T) {
	module := Module(&config.Global, New(&config.Global))

	assert.Equal(t, moduleName, module.Name)
	assert.Equal(t, "/{orgaId}"+config.ApiPrefix, module.Mount)
	assert.Empty(t, module.Routes, "summary declares its route inside a permission-scoped group")

	assert.Len(t, module.Groups, 1)
	assert.Len(t, module.Groups[0].Routes, 1)

	route := module.Groups[0].Routes[0]
	assert.Equal(t, http.MethodGet, route.Method)
	assert.Equal(t, "/{projectId}/summary", route.Pattern)
}

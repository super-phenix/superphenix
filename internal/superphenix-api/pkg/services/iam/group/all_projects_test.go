package group

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// With allProjects the project list is ignored, so it must not be checked against the
// organization: an invalid or foreign id in it cannot change the group's permissions.
func TestUpdateGroupAllProjectsIgnoresProjectIds(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	old := findIdsInOrga
	findIdsInOrga = func([]uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
		t.Error("project ids must not be resolved when allProjects is true")
		return nil, nil
	}
	defer func() { findIdsInOrga = old }()

	orgaId, groupId := uuid.New(), uuid.New()
	mock.ExpectQuery(`SELECT \* FROM "groups"`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "orga_id", "all_projects", "project_ids", "permission_sets", "predefined_key"}).
			AddRow(groupId, "custom", orgaId, false, `[]`, `[]`, nil))
	// Stop at the database write: reaching it means the request was not refused.
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "groups"`).WillReturnError(errors.New("stop"))
	mock.ExpectRollback()

	body, _ := json.Marshal(map[string]any{
		"id":          groupId,
		"name":        "custom",
		"allProjects": true,
		"projectIds":  []string{"not-a-uuid", uuid.NewString()},
	})
	cfg := &config.Config{}
	cfg.PublicHTTP.MaxBodySize = 1 << 20

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgaId", orgaId.String())
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	New(cfg).CreateOrUpdateOrganizationGroup(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

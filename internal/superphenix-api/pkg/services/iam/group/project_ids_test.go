package group

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// stubProjects makes findIdsInOrga answer from a fixed organization content.
func stubProjects(t *testing.T, orgaId uuid.UUID, inOrga ...uuid.UUID) {
	t.Helper()
	old := findIdsInOrga
	findIdsInOrga = func(ids []uuid.UUID, orga uuid.UUID) ([]uuid.UUID, error) {
		found := []uuid.UUID{}
		if orga != orgaId {
			return found, nil
		}
		for _, id := range ids {
			if slices.Contains(inOrga, id) {
				found = append(found, id)
			}
		}
		return found, nil
	}
	t.Cleanup(func() { findIdsInOrga = old })
}

func TestResolveGroupProjectIds(t *testing.T) {
	orgaId := uuid.New()
	own1, own2 := uuid.New(), uuid.New()
	foreign := uuid.New()
	stubProjects(t, orgaId, own1, own2)

	t.Run("projects of the organization", func(t *testing.T) {
		got, err := resolveGroupProjectIds(orgaId, []string{own1.String(), own2.String(), own1.String()}, nil)
		assert.NoError(t, err)
		assert.Equal(t, []string{own1.String(), own2.String()}, got)
	})

	t.Run("new foreign project is refused", func(t *testing.T) {
		_, err := resolveGroupProjectIds(orgaId, []string{own1.String(), foreign.String()}, nil)
		assert.ErrorIs(t, err, errProjectsNotInOrga)
		assert.Contains(t, err.Error(), foreign.String())
	})

	t.Run("invalid and nil ids are refused", func(t *testing.T) {
		_, err := resolveGroupProjectIds(orgaId, []string{"not-a-uuid"}, nil)
		assert.ErrorIs(t, err, errProjectsNotInOrga)
		_, err = resolveGroupProjectIds(orgaId, []string{uuid.Nil.String()}, nil)
		assert.ErrorIs(t, err, errProjectsNotInOrga)
	})

	t.Run("project of the organization under another organization", func(t *testing.T) {
		_, err := resolveGroupProjectIds(uuid.New(), []string{own1.String()}, nil)
		assert.ErrorIs(t, err, errProjectsNotInOrga)
	})

	t.Run("stale ids already held are dropped", func(t *testing.T) {
		current := []string{own1.String(), foreign.String(), "not-a-uuid"}
		got, err := resolveGroupProjectIds(orgaId, current, current)
		assert.NoError(t, err)
		assert.Equal(t, []string{own1.String()}, got)
	})

	t.Run("lookup error", func(t *testing.T) {
		old := findIdsInOrga
		findIdsInOrga = func([]uuid.UUID, uuid.UUID) ([]uuid.UUID, error) { return nil, errors.New("db down") }
		defer func() { findIdsInOrga = old }()

		_, err := resolveGroupProjectIds(orgaId, []string{own1.String()}, nil)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, errProjectsNotInOrga)
	})
}

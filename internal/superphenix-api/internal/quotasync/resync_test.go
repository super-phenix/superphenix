package quotasync

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
)

func TestResync(t *testing.T) {
	orgID := uuid.New()
	projects := []model.Project{{OrgaId: orgID}, {OrgaId: orgID}, {OrgaId: orgID}}
	for i := range projects {
		projects[i].ID = uuid.New()
	}
	newResyncer := func(acquired bool, pushed *[]uuid.UUID, failing uuid.UUID) *Resyncer {
		return &Resyncer{
			Lock: func(context.Context) (func(), bool, error) { return func() {}, acquired, nil },
			List: func(context.Context) ([]model.Project, error) { return projects, nil },
			Push: func(_ context.Context, o, p uuid.UUID) error {
				assert.Equal(t, orgID, o)
				*pushed = append(*pushed, p)
				if p == failing {
					return ErrNotApplied
				}
				return nil
			},
		}
	}

	t.Run("pushes every project, a failure does not stop the others", func(t *testing.T) {
		var pushed []uuid.UUID
		report, err := newResyncer(true, &pushed, projects[1].ID).Resync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, ResyncReport{Projects: 3, Failed: 1}, report)
		assert.Equal(t, []uuid.UUID{projects[0].ID, projects[1].ID, projects[2].ID}, pushed)
	})

	t.Run("skipped when another replica holds the lock", func(t *testing.T) {
		var pushed []uuid.UUID
		report, err := newResyncer(false, &pushed, uuid.Nil).Resync(context.Background())
		require.NoError(t, err)
		assert.Zero(t, report)
		assert.Empty(t, pushed)
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		var pushed []uuid.UUID
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := newResyncer(true, &pushed, uuid.Nil).Resync(ctx)
		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, pushed)
	})
}

func TestPushAZBelowUsage(t *testing.T) {
	setupController(t, http.StatusConflict)
	err := crudQuota.PushAZ(context.Background(), uuid.New(), uuid.New(), "az1", map[string]string{"requests.cpu": "1"})
	require.ErrorIs(t, err, crudQuota.ErrBelowUsage)

	setupController(t, http.StatusInternalServerError)
	err = crudQuota.PushAZ(context.Background(), uuid.New(), uuid.New(), "az1", map[string]string{"requests.cpu": "1"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, crudQuota.ErrBelowUsage)
}

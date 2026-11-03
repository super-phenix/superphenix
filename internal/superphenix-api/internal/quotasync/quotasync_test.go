package quotasync

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

func TestTargets(t *testing.T) {
	orgId, otherOrg := uuid.New(), uuid.New()
	old := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{
		"az2": {Code: "az2"},
		"az1": {Code: "az1"},
		"az3": {Code: "az3", Whitelist: []string{otherOrg.String()}},
	}
	t.Cleanup(func() { config.Global.AZs = old })

	codes := func(azs []config.AZConfig) []string {
		var res []string
		for _, a := range azs {
			res = append(res, a.Code)
		}
		return res
	}

	assert.Equal(t, []string{"az1", "az2"}, codes(targets(orgId, nil)))
	assert.Equal(t, []string{"az2"}, codes(targets(orgId, []string{"az2"})))
	// An AZ the organization cannot use is never targeted.
	assert.Empty(t, targets(orgId, []string{"az3"}))
	assert.Equal(t, []string{"az1", "az2", "az3"}, codes(targets(otherOrg, nil)))
}

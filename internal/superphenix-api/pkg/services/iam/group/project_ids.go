package group

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"

	"github.com/google/uuid"
)

// errProjectsNotInOrga is returned when a group references projects outside
// its organization.
var errProjectsNotInOrga = errors.New("projects not found in the organization")

// findIdsInOrga is the lookup used to resolve project ids, replaced in tests.
var findIdsInOrga = project.FindIdsInOrga

// resolveGroupProjectIds returns the requested project ids that belong to the
// organization. Permify relations are written in the tenant of the group's
// organization, so a foreign project id would grant access to a project of
// another organization.
//
// A new id outside the organization is refused. An id the group already held
// (current) that is no longer valid, such as a deleted project, is dropped so
// that editing the group keeps working.
func resolveGroupProjectIds(orgaId uuid.UUID, requested, current []string) ([]string, error) {
	// Held ids are compared as UUIDs when they parse, so another textual form of the same
	// UUID (URN, braces, upper case) still counts as held. Malformed ones are compared raw.
	heldIds := make([]uuid.UUID, 0, len(current))
	for _, id := range current {
		if parsed, err := uuid.Parse(id); err == nil {
			heldIds = append(heldIds, parsed)
		}
	}

	candidates := make([]uuid.UUID, 0, len(requested))
	var refused []string
	for _, id := range requested {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil {
			if !slices.Contains(current, id) {
				refused = append(refused, id)
			}
			continue
		}
		candidates = append(candidates, parsed)
	}

	found, err := findIdsInOrga(candidates, orgaId)
	if err != nil {
		return nil, err
	}

	resolved := make([]string, 0, len(found))
	for _, id := range candidates {
		switch {
		case slices.Contains(found, id):
			if !slices.Contains(resolved, id.String()) {
				resolved = append(resolved, id.String())
			}
		case !slices.Contains(heldIds, id):
			refused = append(refused, id.String())
		}
	}

	if len(refused) > 0 {
		return nil, fmt.Errorf("%w: %s", errProjectsNotInOrga, strings.Join(refused, ", "))
	}
	return resolved, nil
}

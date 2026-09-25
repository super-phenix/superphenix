package auditlog

import (
	"cmp"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
)

// OtherEventType is the reserved eventType filter value matching every event whose type is no
// longer declared.
const OtherEventType = "other"

// EventType is one declared audit event type.
type EventType struct {
	EventType     string `json:"eventType"`     // e.g. `instance.stop-force`
	ResourceType  string `json:"resourceType"`  // e.g. `instance`
	ResourceLabel string `json:"resourceLabel"` // e.g. `Instance`
	Action        string `json:"action"`        // e.g. `stop-force`
}

type EventTypesResponse struct {
	Items []EventType `json:"items"`
}

// catalogue is the declared event types of one audit log.
type catalogue struct {
	items []EventType
	seen  map[string]struct{}
}

func newCatalogue() *catalogue {
	return &catalogue{items: []EventType{}, seen: map[string]struct{}{}}
}

func (c *catalogue) add(audit router.Audit) {
	eventType := audit.EventType()
	if _, seen := c.seen[eventType]; seen {
		return
	}
	c.items = append(c.items, EventType{
		EventType:     eventType,
		ResourceType:  audit.Resource.Name,
		ResourceLabel: audit.Resource.Label,
		Action:        audit.Action,
	})
	c.seen[eventType] = struct{}{}
}

// resolveOther strips OtherEventType from eventTypes and, when it was present, returns the
// declared types an "other" event must not match.
func (c *catalogue) resolveOther(eventTypes []string) (kept, otherThan []string) {
	kept = slices.DeleteFunc(slices.Clone(eventTypes), func(t string) bool { return t == OtherEventType })
	if len(kept) == len(eventTypes) {
		return kept, nil
	}
	otherThan = make([]string, 0, len(c.items))
	for _, item := range c.items {
		otherThan = append(otherThan, item.EventType)
	}
	return kept, otherThan
}

// buildCatalogues splits the declared audited routes between the organization and the user log,
// sorted by resource label then action.
func buildCatalogues(declared []router.RouteInfo) (organization, user *catalogue) {
	organization, user = newCatalogue(), newCatalogue()
	for _, info := range declared {
		if info.Audit == nil || info.Audit.Skip {
			continue
		}
		if info.OrganizationScoped() {
			organization.add(*info.Audit)
		} else {
			user.add(*info.Audit)
		}
	}
	byResource := func(a, b EventType) int {
		return cmp.Or(cmp.Compare(a.ResourceLabel, b.ResourceLabel), cmp.Compare(a.Action, b.Action))
	}
	slices.SortFunc(organization.items, byResource)
	slices.SortFunc(user.items, byResource)
	return organization, user
}

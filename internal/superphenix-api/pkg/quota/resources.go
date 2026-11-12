// Package quota holds the quota resource model of the Superphenix API, which
// stores and validates quotas, then translates them into the Kubernetes
// ResourceQuota `spec.hard` entries sent to the controller of each AZ.
//
// A quota is a flat map of resource name to quantity. Only the abstract
// resources cpu, memory and disk are allowed for now; each one is translated
// into one or several Kubernetes quota keys.
//
// Semantics: a resource absent from the map is not limited by this level, a
// resource present with "0" is limited to zero.
package quota

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Abstract resources, translated by ToKubernetesQuota.
const (
	CPU    = "cpu"
	Memory = "memory"
	Disk   = "disk"
)

var ErrInvalidResources = errors.New("invalid quota resources")

// translations maps each allowed abstract resource to the Kubernetes quota keys it produces.
// A resource missing from this map is rejected.
var translations = map[string][]string{
	CPU:    {"requests.cpu", "limits.cpu"},
	Memory: {"requests.memory", "limits.memory"},
	Disk:   {"requests.storage"},
}

// Resources is the abstract quota of an organization, a project or a project AZ.
type Resources map[string]string

// Quantities is the canonical numeric form of Resources: CPU in millicores,
// every other resource in its base unit (rounded up).
type Quantities map[string]int64

// Parse validates and converts Resources into Quantities.
func (r Resources) Parse() (Quantities, error) {
	q := make(Quantities, len(r))
	for name, value := range r {
		if err := validateName(name); err != nil {
			return nil, err
		}
		v, err := parseQuantity(name, value)
		if err != nil {
			return nil, err
		}
		q[name] = v
	}
	return q, nil
}

func validateName(name string) error {
	if _, ok := translations[name]; !ok {
		return fmt.Errorf("%w: unsupported resource %q, allowed resources are: %s", ErrInvalidResources, name, strings.Join(allowedNames(), ", "))
	}
	return nil
}

func allowedNames() []string {
	names := make([]string, 0, len(translations))
	for name := range translations {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func parseQuantity(name, value string) (int64, error) {
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid %s %q: %v", ErrInvalidResources, name, value, err)
	}
	if q.Sign() < 0 {
		return 0, fmt.Errorf("%w: %s cannot be negative", ErrInvalidResources, name)
	}
	limit := resource.NewQuantity(math.MaxInt64, resource.DecimalSI)
	if name == CPU {
		limit = resource.NewMilliQuantity(math.MaxInt64, resource.DecimalSI)
	}
	// ParseQuantity clamps huge values to the limit, so reaching it means too large.
	if q.Cmp(*limit) >= 0 {
		return 0, fmt.Errorf("%w: %s %q is too large", ErrInvalidResources, name, value)
	}
	if name == CPU {
		return q.MilliValue(), nil
	}
	return q.Value(), nil
}

// ToKubernetesQuota translates the abstract resources into ResourceQuota `spec.hard` keys.
func (r Resources) ToKubernetesQuota() map[string]string {
	hard := make(map[string]string, len(r))
	for name, value := range r {
		for _, k := range translations[name] {
			hard[k] = value
		}
	}
	return hard
}

// FromKubernetesUsage translates the `status.used` entries of a ResourceQuota back into abstract
// quantities. When a resource produces several Kubernetes keys (cpu: requests.cpu and
// limits.cpu), its usage is the highest of them, since that is the one closest to its limit.
// A resource is only reported when at least one of its keys is tracked (Kubernetes only tracks
// the limited ones); unknown keys and invalid quantities are ignored.
func FromKubernetesUsage(used map[string]string) Quantities {
	res := Quantities{}
	for name, keys := range translations {
		for _, k := range keys {
			value, ok := used[k]
			if !ok {
				continue
			}
			v, err := parseQuantity(name, value)
			if err != nil {
				continue
			}
			if current, seen := res[name]; !seen || v > current {
				res[name] = v
			}
		}
	}
	return res
}

// Resources formats Quantities back into Resources.
func (q Quantities) Resources() Resources {
	r := make(Resources, len(q))
	for name, v := range q {
		switch name {
		case CPU:
			if v%1000 == 0 {
				r[name] = fmt.Sprintf("%d", v/1000)
			} else {
				r[name] = fmt.Sprintf("%dm", v)
			}
		default:
			r[name] = resource.NewQuantity(v, resource.BinarySI).String()
		}
	}
	return r
}

// Add returns the per-resource sum of q and other (union of resources, saturating on overflow).
func (q Quantities) Add(other Quantities) Quantities {
	res := make(Quantities, len(q)+len(other))
	for name, v := range q {
		res[name] = v
	}
	for name, v := range other {
		res[name] = saturatingAdd(res[name], v)
	}
	return res
}

// Max returns the per-resource highest of q and other (union of resources).
func (q Quantities) Max(other Quantities) Quantities {
	res := make(Quantities, len(q)+len(other))
	for name, v := range q {
		res[name] = v
	}
	for name, v := range other {
		if current, ok := res[name]; !ok || v > current {
			res[name] = v
		}
	}
	return res
}

// WithZeros returns q with 0 for every resource of names that q does not have.
func (q Quantities) WithZeros(names Quantities) Quantities {
	res := make(Quantities, len(q)+len(names))
	for name := range names {
		res[name] = 0
	}
	for name, v := range q {
		res[name] = v
	}
	return res
}

// Only returns the resources of q that keep also has.
func (q Quantities) Only(keep Quantities) Quantities {
	res := make(Quantities, len(keep))
	for name, v := range q {
		if _, ok := keep[name]; ok {
			res[name] = v
		}
	}
	return res
}

// Sub returns q minus other for the resources of q, clamped to 0.
func (q Quantities) Sub(other Quantities) Quantities {
	res := make(Quantities, len(q))
	for name, v := range q {
		res[name] = max(v-other[name], 0)
	}
	return res
}

// SplitEqually divides every resource into count parts (floor division, so the
// sum of the parts never exceeds q). A resource is kept even when its part is 0,
// so that a limited resource never becomes unlimited after a split.
func (q Quantities) SplitEqually(count int) Quantities {
	if count <= 0 {
		return Quantities{}
	}
	res := make(Quantities, len(q))
	for name, v := range q {
		res[name] = v / int64(count)
	}
	return res
}

// Exceeds reports the first resource limited by limit for which q is greater.
// A resource missing from q counts as 0.
func (q Quantities) Exceeds(limit Quantities) (string, bool) {
	for _, name := range sortedNames(limit) {
		if q[name] > limit[name] {
			return fmt.Sprintf("%s quota exceeded: requested %s exceeds limit %s",
				name, q.format(name), limit.format(name)), true
		}
	}
	return "", false
}

func (q Quantities) format(name string) string {
	return Quantities{name: q[name]}.Resources()[name]
}

func sortedNames(q Quantities) []string {
	names := make([]string, 0, len(q))
	for name := range q {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func saturatingAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// EffectiveAZQuota computes the resources to enforce in one AZ of a project,
// given the quota of each level (nil when the level has no quota).
//
// The AZ quota is enforced as is, even without any ancestor quota, and even
// when it exceeds an ancestor (overcommitment is tolerated, see the crud
// layer: overcommitted quotas can only be lowered). It fails closed: when an
// ancestor (organization or project) limits a resource the AZ has no
// allocation for (e.g. a project created, or an AZ added, after the quotas
// were distributed), that resource is enforced at 0 in the AZ.
func EffectiveAZQuota(org, project, az Resources) Resources {
	return Effective(az, org, project)
}

// Effective returns the limit that actually applies to a level, given its own quota and the
// quotas of its ancestors (nil when a level has no quota): its own quota, plus 0 for every
// resource an ancestor limits but its own quota does not. A resource absent from the result
// is not limited at all.
func Effective(own Resources, ancestors ...Resources) Resources {
	res := make(Resources, len(own))
	for _, parent := range ancestors {
		for name := range parent {
			res[name] = "0"
		}
	}
	for name, v := range own {
		res[name] = v
	}
	return res
}

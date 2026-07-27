package vm

import (
	"fmt"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

// errInvalidPrefix maps resolution failures to a 400 in the HTTP handlers.
const errInvalidPrefix = "invalid container disk"

func supportsOS(vmPreference string, supportedOS []string) bool {
	if vmPreference == "" {
		return false
	}
	for _, os := range supportedOS {
		if os == "" {
			continue
		}
		if strings.Contains(vmPreference, os) {
			return true
		}
	}
	return false
}

func catalogToSpec(c config.ContainerDiskCatalogEntry) ContainerDiskSpec {
	return ContainerDiskSpec{
		ID:    c.ID,
		Image: c.Image,
		Bus:   c.Bus,
	}
}

// CatalogList returns this AZ's catalog, normalising nil to an empty slice.
func CatalogList() []config.ContainerDiskCatalogEntry {
	if config.Global.ContainerDiskCatalog == nil {
		return []config.ContainerDiskCatalogEntry{}
	}
	return config.Global.ContainerDiskCatalog
}

// RecommendedFor returns the recommended specs matching the VM preference,
// used to default the mount set on create when no list is given.
func RecommendedFor(vmPreference string) []ContainerDiskSpec {
	out := make([]ContainerDiskSpec, 0)
	for _, c := range config.Global.ContainerDiskCatalog {
		if c.Recommended && supportsOS(vmPreference, c.SupportedOS) {
			out = append(out, catalogToSpec(c))
		}
	}
	return out
}

// ResolveByIDs resolves each ID against the catalog (existence only), in order.
// Used by mount/unmount, where the OS preference does not apply.
func ResolveByIDs(ids []string) ([]ContainerDiskSpec, error) {
	index := catalogIndex()
	out := make([]ContainerDiskSpec, 0, len(ids))
	for _, id := range ids {
		entry, ok := index[id]
		if !ok {
			return nil, fmt.Errorf("%s: unknown id %q", errInvalidPrefix, id)
		}
		out = append(out, catalogToSpec(entry))
	}
	return out, nil
}

// Resolve resolves each ID and checks it supports the VM preference.
// Used by create/update.
func Resolve(ids []string, vmPreference string) ([]ContainerDiskSpec, error) {
	index := catalogIndex()
	out := make([]ContainerDiskSpec, 0, len(ids))
	for _, id := range ids {
		entry, ok := index[id]
		if !ok {
			return nil, fmt.Errorf("%s: unknown id %q", errInvalidPrefix, id)
		}
		if !supportsOS(vmPreference, entry.SupportedOS) {
			return nil, fmt.Errorf("%s: %q is not supported for preference %q", errInvalidPrefix, id, vmPreference)
		}
		out = append(out, catalogToSpec(entry))
	}
	return out, nil
}

func catalogIndex() map[string]config.ContainerDiskCatalogEntry {
	catalog := config.Global.ContainerDiskCatalog
	index := make(map[string]config.ContainerDiskCatalogEntry, len(catalog))
	for _, c := range catalog {
		index[c.ID] = c
	}
	return index
}

package utils

import (
	"fmt"
	"strings"
)

// NetPolForAnnotation scopes a NetworkPolicy to a comma separated list of NAD references (<namespace>/<subnetEID>).
const NetPolForAnnotation = "ovn.kubernetes.io/network_policy_for"

// BuildSubnetScope formats subnet effective IDs for NetPolForAnnotation, dropping duplicates and keeping order.
func BuildSubnetScope(namespace string, subnetEIds []string) string {
	refs := make([]string, 0, len(subnetEIds))
	seen := make(map[string]bool, len(subnetEIds))
	for _, eid := range subnetEIds {
		if eid == "" || seen[eid] {
			continue
		}
		seen[eid] = true
		refs = append(refs, fmt.Sprintf("%s/%s", namespace, eid))
	}

	return strings.Join(refs, ",")
}

// ParseSubnetScope returns the subnet effective IDs stored in NetPolForAnnotation.
func ParseSubnetScope(annotations map[string]string) []string {
	value, ok := annotations[NetPolForAnnotation]
	if !ok || value == "" {
		return nil
	}

	refs := strings.Split(value, ",")
	subnetEIds := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		// multus defaults a reference without a namespace to the object namespace.
		if _, eid, found := strings.Cut(ref, "/"); found {
			ref = eid
		}
		if ref != "" {
			subnetEIds = append(subnetEIds, ref)
		}
	}

	if len(subnetEIds) == 0 {
		return nil
	}

	return subnetEIds
}

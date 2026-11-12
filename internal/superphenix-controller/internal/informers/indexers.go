package informers

import (
	"maps"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

const (
	// SwitchLBRuleVPCVIPIndex indexes load balancer rules by VPC and VIP.
	SwitchLBRuleVPCVIPIndex = "vpc-vip"
	// UnpinnedVPC is the VPC key of the rules without a pinned VPC.
	UnpinnedVPC = "*"
)

var extraIndexers = map[string]cache.Indexers{
	SwitchLBRules: {SwitchLBRuleVPCVIPIndex: switchLBRuleVPCVIP},
}

// SwitchLBRuleVPCVIPKey returns the SwitchLBRuleVPCVIPIndex key of a VPC and a VIP.
func SwitchLBRuleVPCVIPKey(vpc, vip string) string {
	return vpc + "|" + vip
}

func switchLBRuleVPCVIP(obj any) ([]string, error) {
	rule, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, nil
	}
	vip, _, _ := unstructured.NestedString(rule.Object, "spec", "vip")
	vpc := rule.GetAnnotations()[utils.LogicalRouterAnnotation]
	if vpc == "" {
		vpc = UnpinnedVPC
	}
	return []string{SwitchLBRuleVPCVIPKey(vpc, vip)}, nil
}

// ResourceIndexers returns every indexer of a resource informer, the dynamic informer factory registers the namespace one.
func ResourceIndexers(resource string) cache.Indexers {
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	maps.Copy(indexers, extraIndexers[resource])
	return indexers
}

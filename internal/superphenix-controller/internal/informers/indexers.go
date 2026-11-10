package informers

import (
	"maps"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

const (
	// SwitchLBRuleVPCVIPIndex indexes load balancer rules by VPC and VIP.
	SwitchLBRuleVPCVIPIndex = "vpc-vip"
	// UnpinnedVPC is the VPC key of the rules without a pinned VPC.
	UnpinnedVPC = "*"
	// ProjectLocalIDIndex indexes objects by their project and local ID labels.
	ProjectLocalIDIndex = "project-local-id"
)

var extraIndexers = map[string]cache.Indexers{
	SwitchLBRules: {SwitchLBRuleVPCVIPIndex: switchLBRuleVPCVIP, ProjectLocalIDIndex: projectLocalID},
	EIP:           {ProjectLocalIDIndex: projectLocalID},
}

// ProjectLocalIDKey returns the ProjectLocalIDIndex key of a project and a local ID.
func ProjectLocalIDKey(project, localID string) string {
	return project + "|" + localID
}

func projectLocalID(obj any) ([]string, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, nil
	}
	labels := u.GetLabels()
	project, localID := labels[spxId.SpxLabelProjectID], labels[spxId.SpxLabelResourceLocalID]
	if project == "" || localID == "" {
		return nil, nil
	}
	return []string{ProjectLocalIDKey(project, localID)}, nil
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

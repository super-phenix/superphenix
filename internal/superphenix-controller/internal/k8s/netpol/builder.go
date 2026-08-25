package netpol

import (
	"context"
	"fmt"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func withPodSelection(ctx context.Context, np *v1.NetworkPolicy, podSelector LabelSelector, projectSpxId string) error {
	log := logger.GetLogger(ctx)

	// Add labels selectors
	matchLabels := map[string]string{
		spxId.SpxLabelProjectID: projectSpxId,
	}
	if len(podSelector.MatchLabels) > 0 {
		for _, label := range podSelector.MatchLabels {
			if err := utils.CheckSelector(label.Key, label.Value); err != nil {
				log.Err(err).Any("podSelector", podSelector).Any("label", label).Msg("Failed to validate selector")
				return err
			}
			matchLabels[label.Key] = label.Value
		}
	}
	np.Spec.PodSelector.MatchLabels = matchLabels

	// Add expressions selectors
	matchExpressions := make([]metav1.LabelSelectorRequirement, 0)
	// Do not apply network policies to ignored resources
	matchExpressions = append(matchExpressions, metav1.LabelSelectorRequirement{
		Key:      fmt.Sprintf("%signoreNetworkPolicies", spxId.SpxLabelPrefix),
		Operator: metav1.LabelSelectorOpNotIn,
		Values:   []string{"true"},
	})

	if len(podSelector.MatchExpressions) > 0 {
		for _, exp := range podSelector.MatchExpressions {
			if err := utils.CheckExpressions(exp.Key, exp.Operator, exp.Values); err != nil {
				log.Err(err).Any("podSelector", podSelector).Any("exp", exp).Msg("Failed to validate expression")
				return err
			}

			newExp := metav1.LabelSelectorRequirement{
				Key:      exp.Key,
				Operator: metav1.LabelSelectorOperator(exp.Operator),
				Values:   exp.Values,
			}
			// Add expression if not already exists
			if !ExpressionExistsInList(matchExpressions, newExp) {
				matchExpressions = append(matchExpressions, newExp)
			}
		}
	}

	np.Spec.PodSelector.MatchExpressions = matchExpressions

	return nil
}

func withIngress(ctx context.Context, np *v1.NetworkPolicy, rules []IngressRule, projectSpxId string) error {
	log := logger.GetLogger(ctx)

	ingressRules := make([]v1.NetworkPolicyIngressRule, 0)
	denyAll := false
	for _, rule := range rules {
		if rule.AllowAll {
			ingressRules = append(ingressRules, v1.NetworkPolicyIngressRule{
				Ports: make([]v1.NetworkPolicyPort, 0),
				From:  make([]v1.NetworkPolicyPeer, 0),
			})
		} else if rule.DenyAll {
			denyAll = true
		} else {
			ports := make([]v1.NetworkPolicyPort, 0)
			fromList := make([]v1.NetworkPolicyPeer, 0)
			for _, port := range rule.Ports {
				protocol := corev1.Protocol(strings.ToUpper(port.Protocol))
				p := intstr.FromInt32(port.Port)

				ports = append(ports, v1.NetworkPolicyPort{
					Protocol: &protocol,
					Port:     &p,
					EndPort:  &port.EndPort,
				})
			}

			for _, fromRule := range rule.From {
				netPolPeer := v1.NetworkPolicyPeer{}

				if fromRule.IPBlock.CIDR != "" {
					netPolPeer.IPBlock = &v1.IPBlock{
						CIDR:   fromRule.IPBlock.CIDR,
						Except: fromRule.IPBlock.Except,
					}
				} else {
					matchLabels := map[string]string{
						spxId.SpxLabelProjectID: projectSpxId,
					}
					if len(fromRule.PodSelector.MatchLabels) > 0 {
						for _, label := range fromRule.PodSelector.MatchLabels {
							if err := utils.CheckSelector(label.Key, label.Value); err != nil {
								log.Err(err).Any("podSelector", fromRule.PodSelector).Any("label", label).Msg("Failed to validate selector")
								return err
							}
							matchLabels[label.Key] = label.Value
						}
					}

					matchExpressions := make([]metav1.LabelSelectorRequirement, 0)
					if len(fromRule.PodSelector.MatchExpressions) > 0 {
						for _, exp := range fromRule.PodSelector.MatchExpressions {
							if err := utils.CheckExpressions(exp.Key, exp.Operator, exp.Values); err != nil {
								log.Err(err).Any("podSelector", fromRule.PodSelector).Any("exp", exp).Msg("Failed to validate expression")
								return err
							}
							matchExpressions = append(matchExpressions, metav1.LabelSelectorRequirement{
								Key:      exp.Key,
								Operator: metav1.LabelSelectorOperator(exp.Operator),
								Values:   exp.Values,
							})
						}
					}

					netPolPeer.PodSelector = &metav1.LabelSelector{
						MatchLabels:      matchLabels,
						MatchExpressions: matchExpressions,
					}
				}

				fromList = append(fromList, netPolPeer)
			}

			ingressRules = append(ingressRules, v1.NetworkPolicyIngressRule{
				Ports: ports,
				From:  fromList,
			})
		}

	}

	if len(ingressRules) > 0 || denyAll {
		np.Spec.PolicyTypes = append(np.Spec.PolicyTypes, v1.PolicyTypeIngress)
	}
	np.Spec.Ingress = ingressRules

	return nil
}

func withEgress(ctx context.Context, np *v1.NetworkPolicy, rules []EgressRule, projectSpxId string) error {
	log := logger.GetLogger(ctx)

	egressRules := make([]v1.NetworkPolicyEgressRule, 0)
	denyAll := false
	for _, rule := range rules {
		if rule.AllowAll {
			egressRules = append(egressRules, v1.NetworkPolicyEgressRule{
				Ports: make([]v1.NetworkPolicyPort, 0),
				To:    make([]v1.NetworkPolicyPeer, 0),
			})
		} else if rule.DenyAll {
			denyAll = true
		} else {
			ports := make([]v1.NetworkPolicyPort, 0)
			toList := make([]v1.NetworkPolicyPeer, 0)

			for _, port := range rule.Ports {
				protocol := corev1.Protocol(strings.ToUpper(port.Protocol))
				p := intstr.FromInt32(port.Port)

				ports = append(ports, v1.NetworkPolicyPort{
					Protocol: &protocol,
					Port:     &p,
					EndPort:  &port.EndPort,
				})
			}

			for _, toRule := range rule.To {
				netPolPeer := v1.NetworkPolicyPeer{}

				if toRule.IPBlock.CIDR != "" {
					netPolPeer.IPBlock = &v1.IPBlock{
						CIDR:   toRule.IPBlock.CIDR,
						Except: toRule.IPBlock.Except,
					}
				} else {
					matchLabels := map[string]string{
						spxId.SpxLabelProjectID: projectSpxId,
					}
					if len(toRule.PodSelector.MatchLabels) > 0 {
						for _, label := range toRule.PodSelector.MatchLabels {
							if err := utils.CheckSelector(label.Key, label.Value); err != nil {
								log.Err(err).Any("podSelector", toRule.PodSelector).Any("label", label).Msg("Failed to validate selector")
								return err
							}
							matchLabels[label.Key] = label.Value
						}
					}

					matchExpressions := make([]metav1.LabelSelectorRequirement, 0)
					if len(toRule.PodSelector.MatchExpressions) > 0 {
						for _, exp := range toRule.PodSelector.MatchExpressions {
							if err := utils.CheckExpressions(exp.Key, exp.Operator, exp.Values); err != nil {
								log.Err(err).Any("podSelector", toRule.PodSelector).Any("exp", exp).Msg("Failed to validate expression")
								return err
							}
							matchExpressions = append(matchExpressions, metav1.LabelSelectorRequirement{
								Key:      exp.Key,
								Operator: metav1.LabelSelectorOperator(exp.Operator),
								Values:   exp.Values,
							})
						}
					}

					netPolPeer.PodSelector = &metav1.LabelSelector{
						MatchLabels:      matchLabels,
						MatchExpressions: matchExpressions,
					}
				}

				toList = append(toList, netPolPeer)
			}

			egressRules = append(egressRules, v1.NetworkPolicyEgressRule{
				Ports: ports,
				To:    toList,
			})
		}
	}

	if len(egressRules) > 0 || denyAll {
		np.Spec.PolicyTypes = append(np.Spec.PolicyTypes, v1.PolicyTypeEgress)
	}
	np.Spec.Egress = egressRules

	return nil
}

func ExpressionExistsInList(expList []metav1.LabelSelectorRequirement, newExp metav1.LabelSelectorRequirement) bool {
	for _, exp := range expList {
		if exp.Key == newExp.Key && exp.Operator == newExp.Operator {
			less := func(a, b string) bool { return a < b }
			equalIgnoreOrder := cmp.Diff(exp.Values, newExp.Values, cmpopts.SortSlices(less)) == ""

			if equalIgnoreOrder {
				return true
			}
		}
	}
	return false
}

// withSubnetScope scopes the network policy to the given subnets, an empty list clearing any
// existing scope. Every effective ID must be available to the project.
func withSubnetScope(np *v1.NetworkPolicy, namespace string, subnetEIds []string, available []view.SubnetView) error {
	if len(subnetEIds) == 0 {
		delete(np.Annotations, utils.NetPolForAnnotation)
		return nil
	}

	if len(subnetEIds) > MaxSubnets {
		return ErrTooManySubnets
	}

	known := make(map[string]bool, len(available))
	for _, subnet := range available {
		known[subnet.Name] = true
	}

	for _, eid := range subnetEIds {
		if !known[eid] {
			return fmt.Errorf("%w: %s", ErrUnknownSubnet, eid)
		}
	}

	if np.Annotations == nil {
		np.Annotations = make(map[string]string)
	}
	np.Annotations[utils.NetPolForAnnotation] = utils.BuildSubnetScope(namespace, subnetEIds)

	return nil
}

package kaas

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"
)

var numericRegex = regexp.MustCompile(`[^\p{N}]+`)

// quantityToGiB returns a quantity in whole GiB, whatever unit it is written in.
func quantityToGiB(quantity string) (int, error) {
	q, err := resource.ParseQuantity(quantity)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity %q: %w", quantity, err)
	}
	return int(q.Value() >> 30), nil
}

// quantityToBinary returns a quantity in canonical binary form ("1536Mi", "4Gi").
func quantityToBinary(quantity string) (string, error) {
	q, err := resource.ParseQuantity(quantity)
	if err != nil {
		return "", fmt.Errorf("invalid quantity %q: %w", quantity, err)
	}
	return resource.NewQuantity(q.Value(), resource.BinarySI).String(), nil
}

// ConvertAppToUpdateKaaSSpec convert kaas helm chart values to KaaSSPec used for updating KaaS product
func ConvertAppToUpdateKaaSSpec(app view.AppView) (KaaSSpec, error) {
	if app.Spec.Source == nil || app.Spec.Source.Plugin == nil {
		return KaaSSpec{}, fmt.Errorf("failed to convert app to KaasSpec")
	}

	var helmValues Values
	for _, entry := range app.Spec.Source.Plugin.Env {
		if entry == nil || entry.Name != "HELM_VALUES" {
			continue
		}
		if err := yaml.Unmarshal([]byte(entry.Value), &helmValues); err != nil {
			return KaaSSpec{}, err
		}
	}

	var result KaaSSpec
	for _, cluster := range helmValues.Clusters {
		var corednsStr string
		if cluster.KaaSEssentials.Values.Coredns != nil {
			corednsBytes, _ := yaml.Marshal(cluster.KaaSEssentials.Values.Coredns)
			corednsStr = string(corednsBytes)
		}

		var ciliumStr string
		if cluster.KaaSEssentials.Values.Cilium != nil {
			ciliumBytes, _ := yaml.Marshal(cluster.KaaSEssentials.Values.Cilium)
			ciliumStr = string(ciliumBytes)
		}

		var metricsServerStr string
		if cluster.KaaSEssentials.Values.MetricsServer != nil {
			metricsServerBytes, _ := yaml.Marshal(cluster.KaaSEssentials.Values.MetricsServer)
			metricsServerStr = string(metricsServerBytes)
		}

		var picValuesStr string
		if cluster.PostInstallChart.Values != nil {
			picValuesBytes, _ := yaml.Marshal(cluster.PostInstallChart.Values)
			picValuesStr = string(picValuesBytes)
		}

		pic := PostInstallChartSpec{
			ChartName:    cluster.PostInstallChart.ChartName,
			ChartVersion: cluster.PostInstallChart.ChartVersion,
			Namespace:    cluster.PostInstallChart.Namespace,
			RepoUrl:      cluster.PostInstallChart.RepoUrl,
			Revision:     cluster.PostInstallChart.Revision,
			Values:       picValuesStr,
		}

		result = KaaSSpec{
			KubeVersion:   cluster.KubeVersion,
			CPNetPol:      cluster.ControlPlane.Network.DefaultPolicies,
			WorkersNetPol: cluster.Workers.Network.DefaultPolicies,
			Groups:        make([]Group, 0),
			KaasEssentials: EssentialsSpec{
				Revision:            cluster.KaaSEssentials.Revision,
				CorednsValues:       corednsStr,
				CiliumValues:        ciliumStr,
				MetricsServerValues: metricsServerStr,
				ClusterFriendlyName: cluster.KaaSEssentials.ClusterFriendlyName,
			},
			PostInstallChart: pic,
		}

		if cluster.ControlPlane.DataStore != nil {
			storage := 0
			if cluster.ControlPlane.DataStore.Storage != "" {
				var err error
				storage, err = quantityToGiB(cluster.ControlPlane.DataStore.Storage)
				if err != nil {
					return KaaSSpec{}, fmt.Errorf("datastore storage: %w", err)
				}
			}
			result.ControlPlane = ControlPlaneSpec{DataStore: DataStoreSpec{
				Dedicated:        cluster.ControlPlane.DataStore.Dedicated,
				StorageClassName: cluster.ControlPlane.DataStore.StorageClassName,
				Storage:          storage,
			}}
		}

		// We sort instances name to be sure that the order remain the same every time
		keys := make([]string, 0, len(cluster.Workers.Instances))
		for k := range cluster.Workers.Instances {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b string) int {
			idxA, _ := strconv.Atoi(numericRegex.ReplaceAllString(a, ""))
			idxB, _ := strconv.Atoi(numericRegex.ReplaceAllString(b, ""))
			return idxA - idxB
		})

		for _, key := range keys {
			instance := cluster.Workers.Instances[key]
			mem, err := quantityToBinary(instance.Template.Memory)
			if err != nil {
				return KaaSSpec{}, fmt.Errorf("memory of group '%s': %w", key, err)
			}
			storage, err := quantityToGiB(instance.Template.BootDisk.Storage)
			if err != nil {
				return KaaSSpec{}, fmt.Errorf("boot disk storage of group '%s': %w", key, err)
			}

			subnets := make([]GroupSubnet, 0)
			for i, it := range instance.Template.Interfaces {
				sub := struct {
					Order int    `json:"order"`
					Id    string `json:"id"`
				}{
					Order: i,
					Id:    it.Subnet,
				}
				subnets = append(subnets, sub)
			}

			result.Groups = append(result.Groups, Group{
				Name:         key,
				NodeRole:     instance.Deployment.NodeRole,
				Replicas:     instance.Deployment.Replicas,
				Cpu:          instance.Template.Cores,
				Memory:       mem,
				BootDiskSize: storage,
				StorageClass: instance.Template.BootDisk.StorageClassName,
				Subnets:      subnets,
				Version:      instance.Template.Version,
			})
		}

		break // We should only have 1 cluster describe in the value
	}

	return result, nil
}

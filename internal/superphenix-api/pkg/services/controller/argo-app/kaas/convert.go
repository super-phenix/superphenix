package kaas

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var numericRegex = regexp.MustCompile(`[^\p{N}]+`)

// ConvertAppToUpdateKaaSSpec convert kaas helm chart values to KaaSSPec used for updating KaaS product
func ConvertAppToUpdateKaaSSpec(app map[string]interface{}) (KaaSSpec, error) {
	slice, b, err := unstructured.NestedSlice(app, "app", "spec", "source", "plugin", "env")
	if err != nil || !b {
		return KaaSSpec{}, fmt.Errorf("failed to convert app to KaasSpec")
	}

	var helmValues Values
	for _, item := range slice {
		name, _, _ := unstructured.NestedString(item.(map[string]interface{}), "name")
		if name == "HELM_VALUES" {
			value, _, _ := unstructured.NestedString(item.(map[string]interface{}), "value")
			err := yaml.Unmarshal([]byte(value), &helmValues)
			if err != nil {
				return KaaSSpec{}, err
			}
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
			},
			PostInstallChart: pic,
		}

		if cluster.ControlPlane.DataStore != nil {
			storage := 0
			if cluster.ControlPlane.DataStore.Storage != "" {
				q := resource.MustParse(cluster.ControlPlane.DataStore.Storage)
				storage, _ = strconv.Atoi(numericRegex.ReplaceAllString(q.String(), ""))
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
			memQ := resource.MustParse(instance.Template.Memory)
			storageQ := resource.MustParse(instance.Template.BootDisk.Storage)

			memStr := numericRegex.ReplaceAllString(memQ.String(), "")
			storageStr := numericRegex.ReplaceAllString(storageQ.String(), "")

			mem, _ := strconv.Atoi(memStr)
			storage, _ := strconv.Atoi(storageStr)

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

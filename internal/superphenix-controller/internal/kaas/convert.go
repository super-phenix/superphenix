package kaas

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func unstructuredToCluster(item unstructured.Unstructured) v1beta2.Cluster {
	labels := utils.FilterLabels(item.GetLabels())

	cluster := v1beta2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Labels:    labels,
		},
	}
	return cluster
}

func unstructuredToMachineDeployment(item unstructured.Unstructured) v1beta2.MachineDeployment {
	specClusterName, _, _ := unstructured.NestedString(item.Object, "spec", "clusterName")
	specInfrastructureRef, _, _ := unstructured.NestedString(item.Object, "spec", "template", "spec", "infrastructureRef", "name")
	specKubeVersion, _, _ := unstructured.NestedString(item.Object, "spec", "template", "spec", "version")
	specReplicasInt64, _, _ := unstructured.NestedInt64(item.Object, "spec", "replicas")
	specReplicas := int32(specReplicasInt64)

	statusReplicasInt64, _, _ := unstructured.NestedInt64(item.Object, "status", "replicas")
	statusReplicas := int32(statusReplicasInt64)
	statusReadyReplicasInt64, _, _ := unstructured.NestedInt64(item.Object, "status", "readyReplicas")
	statusReadyReplicas := int32(statusReadyReplicasInt64)
	statusAvailableReplicasInt64, _, _ := unstructured.NestedInt64(item.Object, "status", "availableReplicas")
	statusAvailableReplicas := int32(statusAvailableReplicasInt64)
	statusUpToDateReplicasInt64, _, _ := unstructured.NestedInt64(item.Object, "status", "upToDateReplicas")
	statusUpToDateReplicas := int32(statusUpToDateReplicasInt64)

	labels := utils.FilterLabels(item.GetLabels())

	md := v1beta2.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Labels:    labels,
		},
		Spec: v1beta2.MachineDeploymentSpec{
			ClusterName: specClusterName,
			Replicas:    &specReplicas,
			Template: v1beta2.MachineTemplateSpec{
				Spec: v1beta2.MachineSpec{
					InfrastructureRef: v1beta2.ContractVersionedObjectReference{
						Name: specInfrastructureRef,
					},
					Version: specKubeVersion,
				},
			},
		},
		Status: v1beta2.MachineDeploymentStatus{
			Replicas:          &statusReplicas,
			ReadyReplicas:     &statusReadyReplicas,
			AvailableReplicas: &statusAvailableReplicas,
			UpToDateReplicas:  &statusUpToDateReplicas,
		},
	}

	return md
}

func unstructuredToKubevirtMachineTemplate(item unstructured.Unstructured) view.KubevirtMachineTemplateSimplified {
	vmTemplateSpec, _, _ := unstructured.NestedMap(item.Object, "spec", "template", "spec", "virtualMachineTemplate", "spec")

	dataVolumeTemplates, _, _ := unstructured.NestedSlice(vmTemplateSpec, "dataVolumeTemplates")

	dvtList := make([]view.DataVolumeTemplate, 0)
	for _, v := range dataVolumeTemplates {
		dvt := v.(map[string]interface{})
		dvRegistryUrl, _, _ := unstructured.NestedString(dvt, "spec", "source", "registry", "url")
		storageSize, _, _ := unstructured.NestedString(dvt, "spec", "storage", "resources", "requests", "storage")
		volumeMode, _, _ := unstructured.NestedString(dvt, "spec", "storage", "volumeMode")

		dvtList = append(dvtList, view.DataVolumeTemplate{
			RegistryUrl: dvRegistryUrl,
			StorageSize: storageSize,
			VolumeMode:  volumeMode,
		})
	}

	preferenceName, _, _ := unstructured.NestedString(vmTemplateSpec, "preference", "name")

	cpuCoresInt64, _, _ := unstructured.NestedInt64(vmTemplateSpec, "template", "spec", "domain", "cpu", "cores")
	cpuCores := int(cpuCoresInt64)
	cpuSocketsInt64, _, _ := unstructured.NestedInt64(vmTemplateSpec, "template", "spec", "domain", "cpu", "sockets")
	cpuSockets := int(cpuSocketsInt64)
	cpuThreadsInt64, _, _ := unstructured.NestedInt64(vmTemplateSpec, "template", "spec", "domain", "cpu", "threads")
	cpuThreads := int(cpuThreadsInt64)

	memoryGuest, _, _ := unstructured.NestedString(vmTemplateSpec, "template", "spec", "domain", "memory", "guest")

	labels := utils.FilterLabels(item.GetLabels())

	kmt := view.KubevirtMachineTemplateSimplified{
		ObjectMeta: view.ObjectMeta{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Labels:    labels,
		},
		DataVolumeTemplates: dvtList,
		MachineTemplate: view.MachineTemplate{
			CPU: view.MachineTemplateCPU{
				Cores:   cpuCores,
				Sockets: cpuSockets,
				Threads: cpuThreads,
			},
			Memory: view.MachineTemplateMemory{
				Guest: memoryGuest,
			},
			Preference: view.MachineTemplatePreference{
				Name: preferenceName,
			},
		},
	}

	return kmt
}

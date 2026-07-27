package kaas

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCluster(ctx context.Context, namespace, eid string) (view.Cluster, error) {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return view.Cluster{}, fmt.Errorf("no namespace provided")
	}

	clusterResources := k8s.DynamicClientSet.Resource(clustersGVR).Namespace(namespace)
	unstructuredCluster, err := clusterResources.Get(ctx, eid, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return view.Cluster{}, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error getting cluster list")
		return view.Cluster{}, err
	}

	if err := utils.CheckProjectLabel(unstructuredCluster, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("eid", eid).Msg("Cluster access denied")
		return view.Cluster{}, err
	}

	clusterObject := unstructuredToCluster(*unstructuredCluster)

	mdR := k8s.DynamicClientSet.Resource(machineDeploymentsGVR).Namespace(namespace)

	unstructuredMDList, err := mdR.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", ClusterLabelKey, clusterObject.GetName()),
	})

	if apierrors.IsNotFound(err) {
		return view.Cluster{}, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", eid).Msg("Error getting machine deployment for cluster")
		return view.Cluster{}, err
	}

	mdList := make([]view.MachineDeployment, 0)
	for _, item := range unstructuredMDList.Items {
		md := unstructuredToMachineDeployment(item)
		kvt, err := getKubevirtMachineTemplate(ctx, namespace, md.Spec.Template.Spec.InfrastructureRef.Name)

		if err != nil {
			log.Err(err).Str("effectiveID", eid).Msg("Error getting machine template")
		} else {
			mdList = append(mdList, view.MachineDeployment{
				MachineDeployment: md,
				MachineTemplate:   kvt,
			})
		}
	}
	cluster := view.Cluster{
		Cluster:            clusterObject,
		MachineDeployments: mdList,
	}

	return cluster, nil
}

func getKubevirtMachineTemplate(ctx context.Context, namespace, eid string) (view.KubevirtMachineTemplateSimplified, error) {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return view.KubevirtMachineTemplateSimplified{}, fmt.Errorf("no namespace provided")
	}
	kmtR := k8s.DynamicClientSet.Resource(kubevirtMachineTemplatesGVR).Namespace(namespace)

	unstructuredItem, err := kmtR.Get(ctx, eid, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		return view.KubevirtMachineTemplateSimplified{}, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", eid).Msg("Error getting kubevirt machine template")
		return view.KubevirtMachineTemplateSimplified{}, err
	}

	kvt := unstructuredToKubevirtMachineTemplate(*unstructuredItem)

	return kvt, nil
}

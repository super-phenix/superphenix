package kaas

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetClusterMachines(ctx context.Context, namespace, clusterEid string) ([]view.Instance, error) {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.Instance, 0), fmt.Errorf("no namespace provided")
	}

	listVM, err := informers.WatcherSet[informers.VirtualMachine].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error ListVM")
		return make([]view.Instance, 0), fmt.Errorf("error ListVM")
	}

	vms := make([]view.VirtualMachineView, 0)
	for _, item := range listVM {
		vm := item.(*unstructured.Unstructured)
		if vm.GetLabels()[ClusterLabelKey] == clusterEid {
			vmView := view.UnstructuredVMToView(vm)
			vms = append(vms, vmView)
		}
	}

	listVMI, err := informers.WatcherSet[informers.VirtualMachineInstance].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error ListVMI")
		return make([]view.Instance, 0), fmt.Errorf("error ListVMI")
	}

	vmis := make([]view.VirtualMachineInstanceView, 0)
	for _, item := range listVMI {
		vmi := item.(*unstructured.Unstructured)
		if vmi.GetLabels()[ClusterLabelKey] == clusterEid {
			vmiView := view.UnstructuredVMIToView(vmi)
			vmis = append(vmis, vmiView)
		}
	}

	instances := view.VMsToResources(vms, vmis)
	return instances, nil
}

func GetNetPols(ctx context.Context, namespace, clusterEid string) ([]view.Firewall, error) {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.Firewall, 0), fmt.Errorf("no namespace provided")
	}

	listNetPol := informers.WatcherSet[informers.NetworkPolicy].List()
	firewalls := make([]view.Firewall, 0)
	for _, item := range listNetPol {
		netpol := item.(*unstructured.Unstructured)
		// Filtering netpol on label referring to cluster name
		if netpol.GetLabels()[spxId.SpxLabelProjectID] == namespace && netpol.GetLabels()[ClusterAppNameLabelKey] == fmt.Sprintf("%s%s", KaaSPrefix, clusterEid) {
			netPolView := view.UnstructuredNetPolToView(netpol)
			firewalls = append(firewalls, netPolView.ToResource())
		}
	}

	return firewalls, nil
}

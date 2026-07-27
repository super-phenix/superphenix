package vm

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListVM(ctx context.Context, namespace string) []view.VirtualMachineView {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.VirtualMachineView, 0)
	}

	list, err := informers.WatcherSet[informers.VirtualMachine].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("error listing vms")
		return make([]view.VirtualMachineView, 0)
	}

	vms := make([]view.VirtualMachineView, 0)
	for _, item := range list {
		vm := item.(*unstructured.Unstructured)
		vmView := view.UnstructuredVMToView(vm)
		vms = append(vms, vmView)
	}

	return vms
}

func ListVMI(ctx context.Context, namespace string) []view.VirtualMachineInstanceView {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.VirtualMachineInstanceView, 0)
	}

	list, err := informers.WatcherSet[informers.VirtualMachineInstance].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("error listing vmis")
		return make([]view.VirtualMachineInstanceView, 0)
	}

	vmis := make([]view.VirtualMachineInstanceView, 0)
	for _, item := range list {
		vmi := item.(*unstructured.Unstructured)
		vmiView := view.UnstructuredVMIToView(vmi)
		vmis = append(vmis, vmiView)
	}
	return vmis
}

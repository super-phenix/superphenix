package vm

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetInfoVM(ctx context.Context, namespace, name string) (view.VirtualMachineView, error) {
	log := logger.GetLogger(ctx)
	vm, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error GetInfoVM")
		return view.VirtualMachineView{}, err
	}
	if err := utils.CheckProjectLabel(vm, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("VM access denied")
		return view.VirtualMachineView{}, err
	}

	return view.VMToView(*vm), nil
}

func GetInfoVMI(ctx context.Context, namespace, name string) (view.VirtualMachineInstanceView, error) {
	log := logger.GetLogger(ctx)
	vmi, err := config.VirtClient.VirtualMachineInstance(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error GetInfoVMI")
		return view.VirtualMachineInstanceView{}, err
	}
	if err := utils.CheckProjectLabel(vmi, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("VMI access denied")
		return view.VirtualMachineInstanceView{}, err
	}
	return view.VMIToView(*vmi), nil
}

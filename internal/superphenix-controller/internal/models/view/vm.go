package view

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog/log"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "kubevirt.io/api/core/v1"
)

type VirtualMachineView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`
	// Spec contains the specification of VirtualMachineInstance created
	Spec VirtualMachineSpec `json:"spec" valid:"required"`
	// Status holds the current state of the controller and brief information
	// about its associated VirtualMachineInstance
	Status VirtualMachineStatus `json:"status,omitempty"`
}

type VirtualMachineInstanceView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`
	// VirtualMachineInstance Spec contains the VirtualMachineInstance specification.
	Spec VirtualMachineInstanceSpec `json:"spec" valid:"required"`
	// Status is the high level overview of how the VirtualMachineInstance is doing. It contains information available to controllers and users.
	Status VirtualMachineInstanceStatus `json:"status,omitempty"`
}

// VirtualMachineSpec describes how the proper VirtualMachine
// should look like
type VirtualMachineSpec struct {
	// Running controls whether the associated VirtualMachineInstance is created or not
	// Mutually exclusive with RunStrategy
	Running *bool `json:"running,omitempty" optional:"true"`

	// Running state indicates the requested running state of the VirtualMachineInstance
	// mutually exclusive with Running
	RunStrategy *v1.VirtualMachineRunStrategy `json:"runStrategy,omitempty" optional:"true"`

	// PreferenceMatcher references a set of preference that is used to fill fields in Template
	Preference *PreferenceMatcher `json:"preference,omitempty" optional:"true"`

	// Template is the direct specification of VirtualMachineInstance
	Template *v1.VirtualMachineInstanceTemplateSpec `json:"template"`
}

// VirtualMachineStatus represents the status returned by the
// controller to describe how the VirtualMachine is doing
type VirtualMachineStatus struct {
	// RestoreInProgress *string `json:"restoreInProgress,omitempty"`
	// Created indicates if the virtual machine is created in the cluster
	Created bool `json:"created,omitempty"`
	// Ready indicates if the virtual machine is running and ready
	Ready bool `json:"ready,omitempty"`
	// PrintableStatus is a human-readable, high-level representation of the status of the virtual machine
	// +kubebuilder:default=Stopped
	PrintableStatus v1.VirtualMachinePrintableStatus `json:"printableStatus,omitempty"`
}

// VirtualMachineInstanceSpec is a description of a VirtualMachineInstance.
type VirtualMachineInstanceSpec struct {
	// Specification of the desired behavior of the VirtualMachineInstance on the host.
	Domain DomainSpec `json:"domain"`
	// List of volumes that can be mounted by disks belonging to the vmi.
	Volumes []v1.Volume `json:"volumes,omitempty"`
	//AccessCredentials []AccessCredential `json:"accessCredentials,omitempty"`
	// Specifies the architecture of the vm guest you are attempting to run. Defaults to the compiled architecture of the KubeVirt components
	Architecture string `json:"architecture,omitempty"`
	// Specifies the hostname of the vmi
	// If not specified, the hostname will be set to the name of the vmi, if dhcp or cloud-init is configured properly.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// List of networks that can be attached to a vm's virtual interface.
	// +kubebuilder:validation:MaxItems:=256
	Networks []v1.Network `json:"networks,omitempty"`
	// Specifies a set of public keys to inject into the vm guest
	// +listType=atomic
	// +optional
	// +kubebuilder:validation:MaxItems:=256
	AccessCredentials []AccessCredential `json:"accessCredentials,omitempty"`
}

// PreferenceMatcher references a set of preference that is used to fill fields in the VMI template.
type PreferenceMatcher struct {
	// Name is the name of the VirtualMachinePreference or VirtualMachineClusterPreference
	//
	// +optional
	Name string `json:"name,omitempty"`
}

// VirtualMachineInstanceStatus represents information about the status of a VirtualMachineInstance. Status may trail the actual
// state of a system.
type VirtualMachineInstanceStatus struct {
	// NodeName is the name where the VirtualMachineInstance is currently running.
	NodeName string `json:"nodeName,omitempty"`

	// Phase is the status of the VirtualMachineInstance in kubernetes world. It is not the VirtualMachineInstance status, but partially correlates to it.
	Phase v1.VirtualMachineInstancePhase `json:"phase,omitempty"`
	// PhaseTransitionTimestamp is the timestamp of when the last phase change occurred
	// +listType=atomic
	// +optional
	PhaseTransitionTimestamps []v1.VirtualMachineInstancePhaseTransitionTimestamp `json:"phaseTransitionTimestamps,omitempty"`

	// Conditions are specific points in VirtualMachineInstance's pod runtime.
	Conditions []v1.VirtualMachineInstanceCondition `json:"conditions,omitempty"`

	// VolumeStatus contains the statuses of all the volumes
	// +optional
	// +listType=atomic
	VolumeStatus []v1.VolumeStatus `json:"volumeStatus,omitempty"`

	// CurrentCPUTopology specifies the current CPU topology used by the VM workload.
	// Current topology may differ from the desired topology in the spec while CPU hotplug
	// takes place.
	CurrentCPUTopology *v1.CPUTopology `json:"currentCPUTopology,omitempty"`

	// Memory shows various information about the VirtualMachine memory.
	// +optional
	Memory *v1.MemoryStatus `json:"memory,omitempty"`

	// Interfaces represent the details of available network interfaces.
	Interfaces []VirtualMachineInstanceNetworkInterface `json:"interfaces,omitempty"`
}

type VirtualMachineInstanceNetworkInterface struct {
	// Hardware address of a Virtual Machine interface
	MAC string `json:"mac,omitempty"`
	// Name of the interface, corresponds to name of the network assigned to the interface
	Name string `json:"name,omitempty"`
	// List of all IP addresses of a Virtual Machine interface
	IPs []string `json:"ipAddresses,omitempty"`
}

type DomainSpec struct {
	// Resources describes the Compute Resources required by this vmi.
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// CPU allow specified the detailed CPU topology inside the vmi.
	// +optional
	CPU *v1.CPU `json:"cpu,omitempty"`
	// Memory allow specifying the VMI memory features.
	// +optional
	Memory *v1.Memory `json:"memory,omitempty"`
	// Machine type.
	// +optional
	Machine *v1.Machine `json:"machine,omitempty"`
	// Devices allows adding disks, network interfaces, and others
	Devices v1.Devices `json:"devices"`
}

// AccessCredential represents a credential source that can be used to
// authorize remote access to the vm guest
// Only one of its members may be specified.
type AccessCredential struct {
	// SSHPublicKey represents the source and method of applying a ssh public
	// key into a guest virtual machine.
	// +optional
	SSHPublicKey *v1.SSHPublicKeyAccessCredential `json:"sshPublicKey,omitempty"`
}

func UnstructuredVMToView(vm *unstructured.Unstructured) VirtualMachineView {
	var view VirtualMachineView
	err := utils.UnstructuredToStruct(vm, &view)
	if err != nil {
		log.Error().AnErr("error converting vm to view", err).Any("vm", vm).Send()
		return VirtualMachineView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	return view
}

func VMToView(vm v1.VirtualMachine) VirtualMachineView {
	vm.Labels = utils.FilterLabels(vm.GetLabels())
	vm.Annotations = utils.FilterAnnotations(vm.GetAnnotations())

	vmStr, err := json.Marshal(vm)
	if err != nil {
		log.Error().AnErr("error converting vm to view", err).Any("vm", vm).Send()
		return VirtualMachineView{}
	}
	var view VirtualMachineView
	err = json.Unmarshal(vmStr, &view)
	if err != nil {
		log.Error().AnErr("error converting vm to view", err).Any("vm", vm).Send()
		return VirtualMachineView{}
	}
	return view
}

func UnstructuredVMIToView(vmi *unstructured.Unstructured) VirtualMachineInstanceView {
	var view VirtualMachineInstanceView
	err := utils.UnstructuredToStruct(vmi, &view)
	if err != nil {
		log.Error().AnErr("error converting vmi to view", err).Any("vmi", vmi).Send()
		return VirtualMachineInstanceView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	return view
}

func VMIToView(vmi v1.VirtualMachineInstance) VirtualMachineInstanceView {
	vmi.Labels = utils.FilterLabels(vmi.GetLabels())
	vmi.Annotations = utils.FilterAnnotations(vmi.GetAnnotations())

	vmiStr, err := json.Marshal(vmi)
	if err != nil {
		log.Error().AnErr("error converting vm to view", err).Any("vm", vmi).Send()
	}
	var view VirtualMachineInstanceView
	err = json.Unmarshal(vmiStr, &view)
	if err != nil {
		log.Error().AnErr("error converting vm to view", err).Any("vm", vmi).Send()
	}
	return view
}

func VMsToResources(vmViews []VirtualMachineView, vmiViews []VirtualMachineInstanceView) []Instance {
	var instances []Instance
	for _, vm := range vmViews {
		instance := Instance{
			Resource: Resource{
				ID:          vm.Labels[spxId.SpxLabelResourceLocalID],
				EId:         vm.Name,
				ProductName: vm.Labels[spxId.SpxLabelResourceName],
				Gitops:      vm.Labels[spxId.SpxLabelGitops],
			},
			Vm: vm,
		}
		for _, vmi := range vmiViews {
			if vmi.Name == vm.Name && vmi.Namespace == vm.Namespace {
				instance.Vmi = vmi
				break
			}
		}

		instances = append(instances, instance)
	}
	return instances
}

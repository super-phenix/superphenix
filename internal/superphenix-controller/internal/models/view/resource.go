package view

import "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

type Resource struct {
	ID             string `json:"id"`          // local ID
	EId            string `json:"eid"`         // effective ID
	ProductName    string `json:"productName"` // human-readable name
	CodeAZ         string `json:"codeAZ"`
	ResourceTypeId string `json:"resourceTypeId"`
	Gitops         string `json:"gitops"`
}

type Instance struct {
	Resource  `json:",inline"`
	Vm        VirtualMachineView         `json:"vm"`
	Vmi       VirtualMachineInstanceView `json:"vmi,omitempty"`
	CloudInit string                     `json:"cloudInit,omitempty"`
	Gpus      []config.GpuClassEntry     `json:"gpus,omitempty"`
}

type Subnet struct {
	Resource   `json:",inline"`
	Subnet     SubnetView `json:"subnet"`
	NatGateway *NatGwView `json:"natGateway,omitempty"`
}

type VPC struct {
	Resource `json:",inline"`
	VPC      VPCView `json:"vpc"`
}

type EIP struct {
	Resource `json:",inline"`
	EIP      EIPView    `json:"eip"`
	FIP      *FIPView   `json:"fip,omitempty"`
	SNAT     []SNATView `json:"snat,omitempty"`
	DNAT     []DNATView `json:"dnat,omitempty"`
}

type Disk struct {
	Resource    `json:",inline"`
	Disk        DiskView         `json:"disk,omitempty"`
	PVC         PVCView          `json:"pvc"`
	MountStatus MountStatus      `json:"mountStatus"`
	Replication *ReplicationView `json:"replication,omitempty"`
}

type Snapshot struct {
	Resource         `json:",inline"`
	Snapshot         *SnapshotView         `json:"snapshot,omitempty"`
	SnapshotSchedule *SnapshotScheduleView `json:"snapshotSchedule,omitempty"`
}

type VmSnapshot struct {
	Resource          `json:",inline"`
	VmSnapshot        VmSnapshotView        `json:"vmSnapshot"`
	VmSnapshotContent VmSnapshotContentView `json:"vmSnapshotContent"`
}

type SSH struct {
	Resource `json:",inline"`
	SSH      SSHView `json:"ssh"`
}

type LoadBalancer struct {
	Resource     `json:",inline"`
	LoadBalancer LBView `json:"loadBalancer"`
}

type SecurityGroup struct {
	Resource      `json:",inline"`
	SecurityGroup NetPolView `json:"securityGroup"`
}

type KaaS struct {
	Resource    `json:",inline"`
	Chart       string  `json:"chart,omitempty"`
	KubeVersion string  `json:"kubeVersion,omitempty"`
	Cluster     Cluster `json:"cluster"`
}

type BaaS struct {
	Resource `json:",inline"`
	Backup   BaasObject `json:"backup"`
}

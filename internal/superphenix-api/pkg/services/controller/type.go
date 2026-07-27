package controller

import (
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

// CreateVPCSpxControllerBody is the body send to superphenix-controller to create a VPC.
// It stays in the controller kit because defaultInit (default project resources) builds it.
type CreateVPCSpxControllerBody struct {
	spxId.Metadata
}

type CreateDiskBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
		Storage     string `json:"storage"` // Storage Size
		Source      struct {
			Type     string `json:"type"`
			URL      string `json:"url,omitempty"`
			Clone    string `json:"clone,omitempty"`
			Snapshot string `json:"snapshot,omitempty"`
		} `json:"source"`
		StorageClass string   `json:"storageClass"`
		Labels       []string `json:"labels,omitempty"`
	} `json:"general"`
}

// CreateDiskSpxControllerBody is the body send to superphenix-controller to create a Disk
type CreateDiskSpxControllerBody struct {
	spxId.Metadata
	General struct {
		Storage string `json:"storage"` // Storage Size
		Source  struct {
			Type     string `json:"type"`
			URL      string `json:"url,omitempty"`
			Clone    string `json:"clone,omitempty"`
			Snapshot string `json:"snapshot,omitempty"`
		} `json:"source"`
		StorageClass string   `json:"storageClass"`
		Labels       []string `json:"labels,omitempty"`
	} `json:"general"`
}

// CreateSubnetSpxControllerBody is the body send to superphenix-controller to create a Subnet.
// It stays in the controller kit because defaultInit (default project resources) builds it.
type CreateSubnetSpxControllerBody struct {
	spxId.Metadata
	General struct {
		VpcEId string `json:"vpcEId"` // VpcEID = VPC Effective ID
	} `json:"general"`
	Network struct {
		Private  bool   `json:"private"`
		Protocol string `json:"protocol"`
		IPv4     string `json:"ipv4,omitempty"`
		IPv6     string `json:"ipv6,omitempty"`
		DnsV4    string `json:"dnsV4,omitempty"`
		DnsV6    string `json:"dnsV6,omitempty"`
	} `json:"network"`
	NatGateway struct {
		Enable bool `json:"enable"`
	} `json:"natGateway"`
}

// ProductResponse contains basic info shared between database and AZ
type ProductResponse struct {
	ID            string `json:"id"`          // local ID
	EId           string `json:"eid"`         // effective ID
	ProductName   string `json:"productName"` // human-readable name
	CodeAZ        string `json:"codeAZ"`
	ProductTypeId string `json:"productTypeId"`
	Gitops        string `json:"gitops"`
}

// ContainerDiskSpecResponse documents the catalog entry shape returned to
// clients. The set of available container disks may vary by availability zone.
type ContainerDiskSpecResponse struct {
	Id          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Image       string   `json:"image"`
	Bus         string   `json:"bus"`
	SupportedOS []string `json:"supportedOS"`
	Recommended bool     `json:"recommended"`
}

// BatchContainerDiskBody is the request body for batched mount/unmount.
// One call applies all disks as a single update on the instance.
type BatchContainerDiskBody struct {
	Ids []string `json:"ids"`
}

type KaaSFullResponse struct {
	ProductResponse `json:",inline"`
	Cluster         interface{} `json:"cluster"`
}

type BaaSFullResponse struct {
	ProductResponse `json:",inline"`
	Backup          interface{} `json:"backup"`
}

type AppSpecFullResponse struct {
	ProductResponse `json:",inline"`
	Spec            interface{} `json:"spec"`
}

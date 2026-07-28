package cluster

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kjson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/klauspost/compress/zstd"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

// PXE setup related variables:

// Known motherboard manufacturers:
var ManufacturerNames = map[string]string{"DELL": "Dell Inc.", "LENOVO": "Lenovo"}

// Manufacturers supported for PXE setup:
var PXESetupSupportedManufacturers = []string{ManufacturerNames["DELL"], ManufacturerNames["LENOVO"]}

const (
	// TalosManagerApp is the ArgoCD Application name for the talos-manager chart.
	TalosManagerApp = "talos-manager"

	// LinkAliasTemplate is the template of a Talos LinkAlias.
	LinkAliasTemplate = "apiVersion: v1alpha1\nkind: LinkAliasConfig\nname: %s\nselector:\n  match: mac(link.permanent_addr) == \"%s\""

	// State of a TalosMachine resource when the machine is booting.
	TalosMachineStateBooting = "Booting"
)

// TalosManagerConfigurationSpec represents the values of the talos-manager chart.
type TalosManagerConfigurationSpec struct {
	PxeEnabled                   bool               `json:"pxeEnabled"`
	DhcpInterface                string             `json:"dhcpInterface"`
	PxeIpAddr                    string             `json:"pxeIpAddr"`
	ClusterName                  string             `json:"clusterName"`
	ClusterType                  string             `json:"clusterType"`
	ReconciliationMode           string             `json:"reconciliationMode"`
	ClusterNetwork               ClusterNetworkSpec `json:"clusterNetwork"`
	PublicNetwork                NetworkSpec        `json:"publicNetwork"`
	StorageNetwork               NetworkSpec        `json:"storageNetwork"`
	PodSubnets                   SubnetSpec         `json:"podSubnets"`
	ServiceSubnets               SubnetSpec         `json:"serviceSubnets"`
	ControlplaneIpv4             string             `json:"controlplaneIpv4"`
	ControlplaneIpv6             string             `json:"controlplaneIpv6"`
	ControlplanePort             int                `json:"controlplanePort"`
	ArgocdToken                  string             `json:"argocdToken"`
	TalosVersion                 string             `json:"talosVersion"`
	K8sVersion                   string             `json:"k8sVersion"`
	MachineGlobalOverrides       []any              `json:"machineGlobalOverrides"`
	MachineOverridesControlPlane []any              `json:"machineOverridesControlPlane"`
	MachineOverridesWorker       []any              `json:"machineOverridesWorker"`
	Nodes                        []NodeSpec         `json:"nodes"`
	Image                        ImageSpec          `json:"image"`
}

type ClusterNetworkSpec struct {
	Ipv4        string `json:"ipv4"`
	Ipv6        string `json:"ipv6"`
	CidrIpv4    int    `json:"cidrIpv4"`
	CidrIpv6    int    `json:"cidrIpv6"`
	VlanId      int    `json:"vlanId"`
	GatewayIpv4 string `json:"gatewayIpv4"`
	GatewayIpv6 string `json:"gatewayIpv6"`
	LinkMtu     int    `json:"linkMtu"`
}

type NetworkSpec struct {
	Ipv4     string `json:"ipv4"`
	Ipv6     string `json:"ipv6"`
	CidrIpv4 int    `json:"cidrIpv4"`
	CidrIpv6 int    `json:"cidrIpv6"`
	VlanId   int    `json:"vlanId"`
	LinkMtu  int    `json:"linkMtu"`
}

type SubnetSpec struct {
	Ipv4     string `json:"ipv4"`
	Ipv6     string `json:"ipv6"`
	CidrIpv4 int    `json:"cidrIpv4"`
	CidrIpv6 int    `json:"cidrIpv6"`
}

type NodeSpec struct {
	Hostname          string            `json:"hostname"`
	Type              string            `json:"type"`
	CpuArchitecture   string            `json:"cpuArchitecture"`
	PxeMacAddress     string            `json:"pxeMacAddress"`
	KernelCmdlineArgs string            `json:"kernelCmdlineArgs"`
	PxeSetup          bool              `json:"pxeSetup"`
	IpmiIpv4          string            `json:"ipmiIpv4"`
	IpmiUser          string            `json:"ipmiUser"`
	IpmiPassword      string            `json:"ipmiPassword"`
	PxeInterfaceName  string            `json:"pxeInterfaceName"`
	Interface         NodeInterfaceSpec `json:"interface"`
	InstallDisk       InstallDiskSpec   `json:"installDisk"`
	MachineOverrides  []any             `json:"machineOverrides"`
}

type NodeInterfaceSpec struct {
	Name               string             `json:"name"`
	ClusterNetwork     NodeNetworkAddress `json:"clusterNetwork"`
	PublicNetwork      NodeNetworkAddress `json:"publicNetwork"`
	StorageNetwork     NodeNetworkAddress `json:"storageNetwork"`
	UseVlan            bool               `json:"useVlan"`
	LacpBond           LacpBondSpec       `json:"lacpBond"`
	PhysicalMacAddress string             `json:"physicalMacAddress"`
}

type NodeNetworkAddress struct {
	Ipv4 string `json:"ipv4"`
	Ipv6 string `json:"ipv6"`
}

type LacpBondSpec struct {
	Enabled            bool                    `json:"enabled"`
	LacpRate           string                  `json:"lacpRate"`
	XmitHashPolicy     string                  `json:"xmitHashPolicy"`
	Miimon             int                     `json:"miimon"`
	Updelay            int                     `json:"updelay"`
	Downdelay          int                     `json:"downdelay"`
	LinkMtu            int                     `json:"linkMtu"`
	PhysicalInterfaces []LacpBondInterfaceSpec `json:"physicalInterfaces"`
}

type LacpBondInterfaceSpec struct {
	Name       string `json:"name"`
	MacAddress string `json:"macAddress"`
}

type InstallDiskSpec struct {
	Auto     bool   `json:"auto"`
	Selector string `json:"selector"`
}

type ImageSpec struct {
	PullPolicy string `json:"pullPolicy"`
}

// reconcileTalosManager creates or updates the ArgoCD Application for the talos-manager chart,
// which sets up PXE on nodes and generates a TalosCluster resource for talos-operator to manage the cluster.
func (r *Reconciler) reconcileTalosManager(ctx context.Context, cluster *operatorv1alpha1.Cluster) error {
	log := logf.FromContext(ctx)

	if cluster.Spec.TalosManagerConfiguration != nil {
		var values map[string]any
		var config TalosManagerConfigurationSpec
		config.PxeEnabled = true
		if cluster.Spec.TalosManagementMode == operatorv1alpha1.TalosManagementImport {
			// "Import" mode disables PXE from talos-manager
			config.PxeEnabled = false
		}

		// Use k8s JSON unmarshaler (PreserveInts) so integer values like port numbers
		// decode as int64 — matching what the API server returns when reading the spec back.
		// Standard encoding/json decodes all numbers as float64, which causes a type mismatch
		// in CreateOrUpdate's DeepEqual check and triggers an infinite reconcile loop.
		if err := kjson.Unmarshal(cluster.Spec.TalosManagerConfiguration.Raw, &config); err != nil {
			return err
		}

		// If we use PXE boot, we need to tag network interfaces
		// with a deterministic name (using Talos LinkAliases),
		// because it will be used for the initial network configuration
		// injected via kernel command line arguments by talos-manager:
		if cluster.Spec.TalosManagementMode == operatorv1alpha1.TalosManagementFull {
			if err := r.injectLinkAliases(&config); err != nil {
				return err
			}

			// Set up PXE boot on nodes and reboot to PXE:
			if err := r.pxeSetup(ctx, config, cluster); err != nil {
				return err
			}
		}

		// Converting config to a map by marshaling then unmarshaling:
		jconfig, err := kjson.Marshal(&config)
		if err != nil {
			return err
		}
		if err := kjson.Unmarshal(jconfig, &values); err != nil {
			return err
		}

		app := r.initTalosManagerApp(fmt.Sprintf("%s-%s", TalosManagerApp, cluster.Name))

		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, app, func() error {
			// Ensure labels are up to date
			r.setTalosManagerAppLabels(app)

			// Set ownership and finalizers
			if err := r.setApplicationOwnership(cluster, app); err != nil {
				return err
			}

			// Handle finalizers based on CleanupOnDeletion
			// This finalizer propagates the deletion of the app to the resources it manages
			finalizer := "resources-finalizer.argocd.argoproj.io"
			if cluster.Spec.CleanupOnDeletion {
				controllerutil.AddFinalizer(app, finalizer)
			} else {
				controllerutil.RemoveFinalizer(app, finalizer)
			}

			// Define and set Application Spec
			spec := r.buildTalosManagerAppSpec(values)
			return unstructured.SetNestedMap(app.Object, spec, "spec")
		}); err != nil {
			log.Error(err, "Failed to reconcile talos-manager Application")
			return err
		}

		log.Info("Successfully reconciled talos-manager Application")

		return nil
	} else {
		return fmt.Errorf("Failed to reconcile talos-manager Application: missing TalosManagerConfiguration")
	}
}

// initTalosManagerApp creates the template of the talos-manager Application.
func (r *Reconciler) initTalosManagerApp(name string) *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetName(name)
	app.SetNamespace(r.OperatorNamespace)
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})

	return app
}

// setTalosManagerAppLabels sets the required labels on the talos-manager Application.
func (r *Reconciler) setTalosManagerAppLabels(app *unstructured.Unstructured) {
	labels := app.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[version.ManagedLabel] = "true"
	app.SetLabels(labels)
}

// buildTalosManagerAppSpec creates the specs of the talos-manager application
func (r *Reconciler) buildTalosManagerAppSpec(vals map[string]any) map[string]any {
	return map[string]any{
		"project": "default",
		"source": map[string]any{
			"repoURL":        r.TalosManagerChartURL,
			"targetRevision": r.TalosManagerChartVersion,
			"chart":          "talos-manager",
			"helm": map[string]any{
				"valuesObject": vals,
			},
		},
		"destination": map[string]any{
			"name":      "in-cluster",
			"namespace": r.OperatorNamespace,
		},
		"syncPolicy": map[string]any{
			"automated": map[string]any{
				"prune":    true,
				"selfHeal": true,
			},
		},
	}
}

// injectLinkAliases generates the Talos LinkAliases for all the nodes and injects them in the kernel command line arguments
func (r *Reconciler) injectLinkAliases(config *TalosManagerConfigurationSpec) error {
	for i, node := range config.Nodes {
		// Generate LinkAliases:
		linkAliases := ""
		if node.Interface.LacpBond.Enabled {
			for _, iface := range node.Interface.LacpBond.PhysicalInterfaces {
				linkAliases = fmt.Sprintf("%s\n---\n%s", linkAliases, fmt.Sprintf(LinkAliasTemplate, iface.Name, strings.ToLower(iface.MacAddress)))
			}
		} else {
			linkAliases = fmt.Sprintf(LinkAliasTemplate, node.Interface.Name, strings.ToLower(node.Interface.PhysicalMacAddress))
		}

		// Prepare LinkAliases for injection using the 'talos.config.inline' option.
		// This option takes a Talos configuration string compressed with zstd and encoded in base64.
		// zstd compression:
		var linkAliasesZstd bytes.Buffer
		writer, err := zstd.NewWriter(&linkAliasesZstd)
		if err != nil {
			return err
		}
		_, err = writer.Write([]byte(linkAliases))
		if err != nil {
			return err
		}
		if err = writer.Close(); err != nil {
			return err
		}
		// base64 encoding:
		linkAliasesFinal := fmt.Sprintf("talos.config.inline=%s", base64.StdEncoding.EncodeToString(linkAliasesZstd.Bytes()))

		// Inject into kernel command line arguments:
		if node.KernelCmdlineArgs != "" {
			// Add LinkAliases to existing arguments:
			config.Nodes[i].KernelCmdlineArgs = fmt.Sprintf("%s %s", linkAliasesFinal, node.KernelCmdlineArgs)
		} else {
			// No other arguments than LinkAliases:
			config.Nodes[i].KernelCmdlineArgs = linkAliasesFinal
		}
	}
	return nil
}

// pxeSetup configures IPMI of servers for PXE boot and reboots them to PXE
func (r *Reconciler) pxeSetup(ctx context.Context, config TalosManagerConfigurationSpec, cluster *operatorv1alpha1.Cluster) error {
	log := logf.FromContext(ctx)

	// Lists nodes that have already been set up:
	var doneList []string

	// Specifies whether there has been an update to the "done" list this time (if at least one node has been added or removed from the list):
	doneListUpdate := false

	// Only adding nodes that still exist to the up-to-date "done" list
	// in case some have been removed:
	if cluster.Status.PXESetupDone != nil {
		for _, done := range cluster.Status.PXESetupDone {
			found := false
			for _, node := range config.Nodes {
				if node.Hostname == done {
					found = true
				}
			}
			if found {
				doneList = append(doneList, done)
			} else {
				doneListUpdate = true
			}
		}
	} else {
		// "done" list is missing from status, so we fall back on TalosMachine resources
		// to check if nodes are already set up.
		// If a TalosMachine's status is not "Booting", then talos-operator is already handling it
		// so PXE is necessarily set up on this machine.
		talosMachines := &unstructured.UnstructuredList{}
		talosMachines.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "talos.alperen.cloud",
			Version: "v1alpha1",
			Kind:    "TalosMachine",
		})
		// We can ignore "IsNoMatchError" because it just means that talos-operator's CRDs are not installed
		// so machines are not handled by talos-operator yet:
		if err := r.List(ctx, talosMachines, client.InNamespace(cluster.Namespace)); err != nil && !meta.IsNoMatchError(err) {
			return err
		}
		for _, m := range talosMachines.Items {
			if clusterName, _, _ := unstructured.NestedString(m.Object, "spec", "controlPlaneRef", "name"); clusterName == cluster.Name {
				if state, _, _ := unstructured.NestedString(m.Object, "status", "state"); state != TalosMachineStateBooting {
					// TalosMachine's resource name isn't the same as the hostname
					// so we rely on the endpoint IP of this TalosMachine resource to find its hostname:
					var hostname string
					endpoint, _, _ := unstructured.NestedString(m.Object, "spec", "endpoint")
					for _, node := range config.Nodes {
						if node.Interface.ClusterNetwork.Ipv4 == endpoint {
							hostname = node.Hostname
						}
					}
					doneList = append(doneList, hostname)
					doneListUpdate = true
				}
			}
		}
	}

	// Setting up nodes:
	for _, node := range config.Nodes {
		// Skip node if PXE setup is not requested or if it is already set up:
		if node.PxeSetup && !slices.Contains(doneList, node.Hostname) {
			// Connect to Redfish endpoint:
			c, err := gofish.Connect(gofish.ClientConfig{
				Endpoint: fmt.Sprintf("https://%s", node.IpmiIpv4),
				Username: node.IpmiUser,
				Password: node.IpmiPassword,
				Insecure: true,
			})
			if err != nil {
				return err
			}
			defer c.Logout()

			// Get the System and BIOS objects and manufacturer name:
			systems, err := c.Service.Systems()
			if err != nil {
				return err
			}
			system := systems[0]
			bios, err := system.Bios()
			if err != nil {
				return err
			}
			manufacturer := system.Manufacturer

			// BIOS configuration for PXE:
			if slices.Contains(PXESetupSupportedManufacturers, manufacturer) {
				// Prepare patch for BIOS configuration:
				var pxePatch schemas.SettingsAttributes
				switch manufacturer {
				case ManufacturerNames["DELL"]:
					pxePatch = schemas.SettingsAttributes{
						"BootMode":         "Uefi",
						"PxeDev1EnDis":     "Enabled",
						"PxeDev1Protocol":  "IPv4",
						"PxeDev1Interface": node.PxeInterfaceName,
					}
				case ManufacturerNames["LENOVO"]:
					pxePatch = schemas.SettingsAttributes{
						"BootModes_SystemBootMode":            "UEFIMode",
						"NetworkStackSettings_NetworkStack":   "Enable",
						"NetworkStackSettings_IPv4PXESupport": "Enable",
					}
				}

				// Adding VLAN setup to patch if required:
				if node.Interface.UseVlan {
					if manufacturer == ManufacturerNames["DELL"] {
						pxePatch["PxeDev1VlanEnDis"] = "Enabled"
						pxePatch["PxeDev1VlanId"] = config.ClusterNetwork.VlanId
					} else {
						log.Info(fmt.Sprintf("WARNING: Setting up PXE VLAN automatically is not supported on '%s' machines yet, it might need to be set up manually.", manufacturer))
					}
				}

				// Apply patch:
				if err := bios.UpdateBiosAttributesApplyAt(pxePatch, schemas.OnResetSettingsApplyTime); err != nil {
					return err
				}
			} else {
				log.Info(fmt.Sprintf("WARNING: Setting up PXE automatically is not supported on '%s' machines yet, it might need to be set up manually. Only next boot option will be set.", manufacturer))
			}

			// Set next boot option to PXE:
			system.Boot.BootSourceOverrideEnabled = schemas.OnceBootSourceOverrideEnabled
			system.Boot.BootSourceOverrideTarget = schemas.PxeBootSource
			if err := system.SetBoot(&system.Boot); err != nil {
				return err
			}

			// Reboot if already on, else power on:
			resetType := schemas.ForceRestartResetType
			if system.PowerState == schemas.OffPowerState {
				resetType = schemas.OnResetType
			}
			if _, err := system.Reset(resetType); err != nil {
				return err
			}

			// Add this node to the "done" list:
			doneList = append(doneList, node.Hostname)
			doneListUpdate = true
		}
	}

	// Updating Cluster's status to add nodes that have been set up (skip if no node has been added or removed from the list):
	if doneListUpdate {
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.PXESetupDone = doneList
		if err := r.Status().Patch(ctx, cluster, patch); err != nil {
			return err
		}
	}

	return nil
}

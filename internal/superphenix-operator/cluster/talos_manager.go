package cluster

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kjson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/klauspost/compress/zstd"
	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

const (
	// TalosManagerApp is the ArgoCD Application name for the talos-manager chart.
	TalosManagerApp = "talos-manager"

	// LinkAliasTemplate is the template of a Talos LinkAlias.
	LinkAliasTemplate = "apiVersion: v1alpha1\nkind: LinkAliasConfig\nname: %s\nselector:\n  match: mac(link.permanent_addr) == \"%s\""
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
			r.setTalosManagerAppLabels(app, cluster)

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
func (r *Reconciler) setTalosManagerAppLabels(app *unstructured.Unstructured, cluster *operatorv1alpha1.Cluster) {
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

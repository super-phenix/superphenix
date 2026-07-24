package cluster

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"maps"
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
)

// reconcileTalosManager creates or updates the ArgoCD Application for the talos-manager chart,
// which sets up PXE on nodes and generates a TalosCluster resource for talos-operator to manage the cluster.
func (r *Reconciler) reconcileTalosManager(ctx context.Context, cluster *operatorv1alpha1.Cluster) error {
	log := logf.FromContext(ctx)

	values := map[string]any{
		"pxeEnabled": true,
	}
	if cluster.Spec.TalosManagementMode == operatorv1alpha1.TalosManagementImport {
		// "Import" mode disables PXE from talos-manager
		values = map[string]any{
			"pxeEnabled": false,
		}
	}
	if cluster.Spec.TalosManagerConfiguration != nil {
		var talosManagerConfig map[string]any
		// Use k8s JSON unmarshaler (PreserveInts) so integer values like port numbers
		// decode as int64 — matching what the API server returns when reading the spec back.
		// Standard encoding/json decodes all numbers as float64, which causes a type mismatch
		// in CreateOrUpdate's DeepEqual check and triggers an infinite reconcile loop.
		if err := kjson.Unmarshal(cluster.Spec.TalosManagerConfiguration.Raw, &talosManagerConfig); err == nil {
			maps.Copy(values, talosManagerConfig)
		} else {
			return err
		}

		// If we use PXE boot, we need to tag network interfaces
		// with a deterministic name (using Talos LinkAliases),
		// because it will be used for the initial network configuration
		// injected via kernel command line arguments by talos-manager:
		if cluster.Spec.TalosManagementMode == operatorv1alpha1.TalosManagementFull {
			if err := r.injectLinkAliases(ctx, &values); err != nil {
				return err
			}
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
func (r *Reconciler) injectLinkAliases(ctx context.Context, vals *map[string]any) error {
	nodes, ok := (*vals)["nodes"].([]any)
	if !ok {
		return fmt.Errorf("Incorrect type for 'nodes' in 'talosManagerConfiguration' ('%T' instead of '[]any')", (*vals)["nodes"])
	}
	for _, item := range nodes {
		node, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("Incorrect type for node in 'nodes' array ('%T' instead of 'map[string]any')", item)
		}
		// Generate LinkAliases:
		linkAliases := ""
		interfaceSpec, ok := node["interface"].(map[string]any)
		if !ok {
			return fmt.Errorf("Incorrect type for 'interface' in node ('%T' instead of 'map[string]any')", node["interface"])
		}
		lacpBond, ok := interfaceSpec["lacpBond"].(map[string]any)
		if !ok {
			return fmt.Errorf("Incorrect type for 'interface.lacpBond' in node ('%T' instead of 'map[string]any')", interfaceSpec["lacpBond"])
		}
		lacpEnabled, ok := lacpBond["enabled"].(bool)
		if !ok {
			return fmt.Errorf("Incorrect type for 'interface.lacpBond.enabled' in node ('%T' instead of 'bool')", lacpBond["enabled"])
		}
		if lacpEnabled {
			interfaces, ok := lacpBond["physicalInterfaces"].([]any)
			if !ok {
				return fmt.Errorf("Incorrect type for 'interface.lacpBond.physicalInterfaces' in node ('%T' instead of '[]any')", lacpBond["physicalInterfaces"])
			}
			for _, item := range interfaces {
				iface, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("Incorrect type for interface in 'interface.lacpBond.physicalInterfaces' array in node ('%T' instead of 'map[string]any')", item)
				}
				name, ok := iface["name"].(string)
				if !ok {
					return fmt.Errorf("Incorrect type for 'interface.lacpBond.physicalInterfaces[].name' in node ('%T' instead of 'string')", iface["name"])
				}
				macAddress, ok := iface["macAddress"].(string)
				if !ok {
					return fmt.Errorf("Incorrect type for 'interface.lacpBond.physicalInterfaces[].macAddress' in node ('%T' instead of 'string')", iface["macAddress"])
				}
				linkAliases = fmt.Sprintf("%s\n---\napiVersion: v1alpha1\nkind: LinkAliasConfig\nname: %s\nselector:\n  match: mac(link.permanent_addr) == \"%s\"", linkAliases, name, strings.ToLower(macAddress))
			}
		} else {
			name, ok := interfaceSpec["name"].(string)
			if !ok {
				return fmt.Errorf("Incorrect type for 'interface.name' in node ('%T' instead of 'string')", interfaceSpec["name"])
			}
			macAddress, ok := interfaceSpec["physicalMacAddress"].(string)
			if !ok {
				return fmt.Errorf("Incorrect type for 'interface.physicalMacAddress' in node ('%T' instead of 'string')", interfaceSpec["physicalMacAddress"])
			}
			linkAliases = fmt.Sprintf("apiVersion: v1alpha1\nkind: LinkAliasConfig\nname: %s\nselector:\n  match: mac(link.permanent_addr) == \"%s\"", name, strings.ToLower(macAddress))
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
		_, ok = node["kernelCmdlineArgs"].(string)
		if ok {
			// Add LinkAliases to existing arguments:
			node["kernelCmdlineArgs"] = fmt.Sprintf("%s %s", linkAliasesFinal, node["kernelCmdlineArgs"])
		} else {
			// No other arguments than LinkAliases:
			node["kernelCmdlineArgs"] = linkAliasesFinal
		}
	}
	return nil
}

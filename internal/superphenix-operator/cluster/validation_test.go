package cluster

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

var _ = Describe("Cluster Validation", func() {
	var oldMinClusterVersion string

	BeforeEach(func() {
		oldMinClusterVersion = version.MinClusterVersion
		version.MinClusterVersion = "1.0.0"
	})

	AfterEach(func() {
		version.MinClusterVersion = oldMinClusterVersion
	})

	const (
		operatorNamespace = "default"
		clusterName       = "validation-cluster"
		clusterNamespace  = "default"
	)

	ctx := context.Background()

	Context("validateUpgradePath", func() {
		BeforeEach(func() {
			// Clean up cluster app if it exists
			app := &unstructured.Unstructured{}
			app.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: operatorNamespace}, app)
			if app.GetResourceVersion() != "" {
				_ = k8sClient.Delete(ctx, app)
			}
		})

		It("should fail if upgrade path is not supported", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			By("Creating an ArgoCD Application for the cluster with version 0.9.0")
			app := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "Application",
					"metadata": map[string]interface{}{
						"name":      clusterName,
						"namespace": operatorNamespace,
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/charts",
							"targetRevision": "0.9.0",
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": operatorNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					Version: "1.1.0",
				},
			}

			err := r.validateUpgradePath(ctx, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("upgrade from 0.9.0 to 1.1.0 is not supported"))
		})

		It("should succeed if upgrade path is supported", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			By("Creating an ArgoCD Application for the cluster with version 1.0.0")
			app := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "Application",
					"metadata": map[string]interface{}{
						"name":      clusterName,
						"namespace": operatorNamespace,
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/charts",
							"targetRevision": "1.0.0",
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": operatorNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					Version: "1.1.0",
				},
			}

			err := r.validateUpgradePath(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if it is a new cluster (no app found)", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-cluster",
					Namespace: clusterNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					Version: "1.1.0",
				},
			}

			err := r.validateUpgradePath(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should use IsOperatorUpgradeSupported for management clusters", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			By("Setting MinOperatorVersion to 1.0.0")
			oldMinOperatorVersion := version.MinOperatorVersion
			version.MinOperatorVersion = "1.0.0"
			defer func() { version.MinOperatorVersion = oldMinOperatorVersion }()

			By("Creating an ArgoCD Application for the management cluster with version 0.9.0")
			app := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "Application",
					"metadata": map[string]interface{}{
						"name":      clusterName,
						"namespace": operatorNamespace,
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/charts",
							"targetRevision": "0.9.0",
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": operatorNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			clusterType := operatorv1alpha1.ClusterTypeManagement
			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					Type:    &clusterType,
					Version: "1.1.0",
				},
			}

			err := r.validateUpgradePath(ctx, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("management upgrade from 0.9.0 to 1.1.0 is not supported"))
		})
	})

	Context("validateManagementCompatibility", func() {
		BeforeEach(func() {
			// Clean up management app if it exists
			app := &unstructured.Unstructured{}
			app.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: version.ManagementAppName, Namespace: operatorNamespace}, app)
			if app.GetResourceVersion() != "" {
				_ = k8sClient.Delete(ctx, app)
			}
		})

		It("should fail if cluster version is not supported by management", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			By("Creating an ArgoCD Application for management with version 1.1.0")
			app := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "Application",
					"metadata": map[string]interface{}{
						"name":      version.ManagementAppName,
						"namespace": operatorNamespace,
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/charts",
							"targetRevision": "1.1.0", // Supports cluster >= 1.0.0
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": operatorNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			cluster := &operatorv1alpha1.Cluster{
				Spec: operatorv1alpha1.ClusterSpec{
					Version: "0.9.0",
				},
			}

			err := r.validateManagementCompatibility(ctx, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cluster version 0.9.0 is not supported by management version 1.1.0"))
		})

		It("should succeed if cluster version is supported by management", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			By("Creating an ArgoCD Application for management with version 1.1.0")
			app := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "Application",
					"metadata": map[string]interface{}{
						"name":      version.ManagementAppName,
						"namespace": operatorNamespace,
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/charts",
							"targetRevision": "1.1.0",
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": operatorNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			cluster := &operatorv1alpha1.Cluster{
				Spec: operatorv1alpha1.ClusterSpec{
					Version: "1.0.0",
				},
			}

			err := r.validateManagementCompatibility(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if cluster version is 0.0.0 (skipped check)", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			By("Creating an ArgoCD Application for management with version 1.1.0")
			app := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "Application",
					"metadata": map[string]interface{}{
						"name":      version.ManagementAppName,
						"namespace": operatorNamespace,
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/charts",
							"targetRevision": "1.1.0",
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": operatorNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			cluster := &operatorv1alpha1.Cluster{
				Spec: operatorv1alpha1.ClusterSpec{
					Version: "0.0.0",
				},
			}

			err := r.validateManagementCompatibility(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should skip check if cluster type is Management", func() {
			r := &Reconciler{
				Client:            k8sClient,
				OperatorNamespace: operatorNamespace,
			}

			By("Creating an ArgoCD Application for management with version 1.1.0")
			app := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "Application",
					"metadata": map[string]interface{}{
						"name":      version.ManagementAppName,
						"namespace": operatorNamespace,
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/charts",
							"targetRevision": "1.1.0",
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": operatorNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			clusterType := operatorv1alpha1.ClusterTypeManagement
			cluster := &operatorv1alpha1.Cluster{
				Spec: operatorv1alpha1.ClusterSpec{
					Type:    &clusterType,
					Version: "0.9.0", // Would normally fail if not skipped
				},
			}

			err := r.validateManagementCompatibility(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("DisableVersionValidation", func() {
		It("should skip version validation when disabled", func() {
			r := &Reconciler{
				Client:                   k8sClient,
				OperatorNamespace:        operatorNamespace,
				DisableVersionValidation: true,
			}

			// This would normally fail because management app is missing/mismatched
			cluster := &operatorv1alpha1.Cluster{
				Spec: operatorv1alpha1.ClusterSpec{
					Version:            "0.9.0",
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
				},
			}

			err := r.validate(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

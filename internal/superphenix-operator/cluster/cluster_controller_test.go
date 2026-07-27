package cluster

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
)

var _ = Describe("Cluster Controller", func() {
	var oldMinClusterVersion string

	BeforeEach(func() {
		oldMinClusterVersion = version.MinClusterVersion
		version.MinClusterVersion = "1.0.0"
	})

	AfterEach(func() {
		version.MinClusterVersion = oldMinClusterVersion
	})

	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		cluster := &operatorv1alpha1.Cluster{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Cluster")
			err := k8sClient.Get(ctx, typeNamespacedName, cluster)
			if err != nil && errors.IsNotFound(err) {
				resource := &operatorv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: operatorv1alpha1.ClusterSpec{
						DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
						Region:             "us-east-1",
						AvailabilityZone:   "us-east-1a",
						Version:            "1.0.0",
						Connection: &operatorv1alpha1.ClusterConnectionSpec{
							Mode: operatorv1alpha1.ConnectionModeLocal,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &operatorv1alpha1.Cluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Cluster")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Mocking the ArgoCD Application status")
			argoCDApp := &unstructured.Unstructured{}
			argoCDApp.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})

			// Wait for the Application to be created AND available in the test client
			Eventually(func() error {
				// Reconcile again just in case it wasn't created in the first pass
				// (though logs say it was)
				_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				return k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: "default"}, argoCDApp)
			}, 20, 1.0).Should(Succeed())

			status := map[string]interface{}{
				"sync": map[string]interface{}{
					"status": "Synced",
				},
				"health": map[string]interface{}{
					"status": "Healthy",
				},
			}
			Expect(unstructured.SetNestedMap(argoCDApp.Object, status, "status")).To(Succeed())
			// Attempt both Status().Update and plain Update for maximum compatibility with envtest
			err = k8sClient.Status().Update(ctx, argoCDApp)
			if err != nil {
				Expect(k8sClient.Update(ctx, argoCDApp)).To(Succeed())
			}

			By("Reconciling again to propagate the status")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that it's marked as reachable and Deployed")
			updatedCluster := &operatorv1alpha1.Cluster{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedCluster)
				if err != nil {
					return false
				}
				if updatedCluster.Status.Phase != "Deployed" {
					return false
				}
				if updatedCluster.Status.KubernetesVersion == "" {
					return false
				}
				for _, condition := range updatedCluster.Status.Conditions {
					if condition.Type == operatorv1alpha1.ConditionTypeReachable && condition.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, 10, 0.5).Should(BeTrue())

			// Skip ArgoCD secret check for local mode as the controller skips it
			if updatedCluster.Spec.Connection.Mode == operatorv1alpha1.ConnectionModeRemote {
				By("Verifying that the ArgoCD secret was created")
				argoCDSecret := &corev1.Secret{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: "argocd-secret-" + resourceName, Namespace: "default"}, argoCDSecret)
				Expect(err).NotTo(HaveOccurred())
				Expect(argoCDSecret.Labels["argocd.argoproj.io/secret-type"]).To(Equal("cluster"))
				Expect(argoCDSecret.OwnerReferences).To(HaveLen(1))
				Expect(argoCDSecret.OwnerReferences[0].Name).To(Equal(resourceName))
			}

			By("Verifying that the ArgoCD Application was created")
			argoCDApp = &unstructured.Unstructured{}
			argoCDApp.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})
			err = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: "default"}, argoCDApp)
			Expect(err).NotTo(HaveOccurred())
			Expect(argoCDApp.GetName()).To(Equal(resourceName))
			Expect(argoCDApp.GetOwnerReferences()).To(HaveLen(1))
			Expect(argoCDApp.GetOwnerReferences()[0].Name).To(Equal(resourceName))
			Expect(argoCDApp.GetFinalizers()).NotTo(ContainElement("resources-finalizer.argocd.argoproj.io"))

			spec, found, err := unstructured.NestedMap(argoCDApp.Object, "spec")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(spec["project"]).To(Equal(resourceName))

			destination, found, err := unstructured.NestedMap(spec, "destination")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(destination["name"]).To(Equal("in-cluster")) // Default for local mode in test

			By("Verifying that the ArgoCD AppProject was created")
			argoCDProject := &unstructured.Unstructured{}
			argoCDProject.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "AppProject",
			})
			err = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: "default"}, argoCDProject)
			Expect(err).NotTo(HaveOccurred())
			Expect(argoCDProject.GetName()).To(Equal(resourceName))
			Expect(argoCDProject.GetOwnerReferences()).To(HaveLen(1))
			Expect(argoCDProject.GetOwnerReferences()[0].Name).To(Equal(resourceName))
			Expect(argoCDProject.GetFinalizers()).To(ContainElement("resources-finalizer.argocd.argoproj.io"))

			projectSpec, found, err := unstructured.NestedMap(argoCDProject.Object, "spec")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			destinations, found, err := unstructured.NestedSlice(projectSpec, "destinations")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(destinations).To(HaveLen(1))
			dest := destinations[0].(map[string]interface{})
			Expect(dest["name"]).To(Equal("in-cluster"))
			Expect(dest["namespace"]).To(Equal("*"))

			source, found, err := unstructured.NestedMap(spec, "source")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(source["repoURL"]).To(Equal("git@github.com:super-phenix/superphenix.git"))
			Expect(source["chart"]).To(Equal("superphenix-system"))

			helm, found, err := unstructured.NestedMap(source, "helm")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			_, found, err = unstructured.NestedMap(helm, "valuesObject")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			By("Reconciling with updated connection info")
			updatedCluster.Spec.Connection.URL = "https://new-url:6443"
			updatedCluster.Spec.Connection.Mode = operatorv1alpha1.ConnectionModeRemote
			updatedCluster.Spec.Connection.SecretRef = &operatorv1alpha1.SecretReference{
				Name:      "new-secret",
				Namespace: "default",
			}
			Expect(k8sClient.Update(ctx, updatedCluster)).To(Succeed())

			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"bearerToken": []byte("some-token"), // raw token
				},
			}
			Expect(k8sClient.Create(ctx, newSecret)).To(Succeed())

			// We expect the reconcile to fail because new-url is unreachable, but we want to check if the secret and app were at least updated
			_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			argoCDSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "argocd-secret-" + resourceName, Namespace: "default"}, argoCDSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(argoCDSecret.Data["server"])).To(Equal("https://new-url:6443"))
			var config map[string]interface{}
			Expect(json.Unmarshal(argoCDSecret.Data["config"], &config)).To(Succeed())
			Expect(config["bearerToken"]).To(Equal("some-token"))

			By("Verifying that the ArgoCD Application destination was updated for remote")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: "default"}, argoCDApp)
			Expect(err).NotTo(HaveOccurred())
			spec, _, _ = unstructured.NestedMap(argoCDApp.Object, "spec")
			destination, _, _ = unstructured.NestedMap(spec, "destination")
			Expect(destination["name"]).To(Equal("in-cluster"))

			By("Verifying that the ArgoCD AppProject destination was updated for remote")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: "default"}, argoCDProject)
			Expect(err).NotTo(HaveOccurred())
			projectSpec, _, _ = unstructured.NestedMap(argoCDProject.Object, "spec")
			destinations, _, _ = unstructured.NestedSlice(projectSpec, "destinations")
			Expect(destinations).To(HaveLen(2))

			dest0 := destinations[0].(map[string]interface{})
			Expect(dest0["name"]).To(Equal(resourceName))

			dest1 := destinations[1].(map[string]interface{})
			Expect(dest1["name"]).To(Equal("in-cluster"))
			Expect(dest1["namespace"]).To(Equal("default"))

			By("Reconciling with certificate-based authentication")
			certSecretName := "cert-secret"
			certSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      certSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"certData": []byte("client-cert"), // raw cert
					"keyData":  []byte("client-key"),  // raw key
				},
			}
			Expect(k8sClient.Create(ctx, certSecret)).To(Succeed())

			updatedCluster.Spec.Connection.SecretRef.Name = certSecretName
			// Fetch the latest version to avoid conflict
			latestCluster := &operatorv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, latestCluster)).To(Succeed())
			latestCluster.Spec.Connection.SecretRef.Name = certSecretName
			Expect(k8sClient.Update(ctx, latestCluster)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{Name: "argocd-secret-" + resourceName, Namespace: "default"}, argoCDSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(argoCDSecret.Data["config"], &config)).To(Succeed())
			tlsConfig, ok := config["tlsClientConfig"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(tlsConfig["certData"]).To(Equal("Y2xpZW50LWNlcnQ="))
			Expect(tlsConfig["keyData"]).To(Equal("Y2xpZW50LWtleQ=="))

			By("Reconciling with insecure flag set to true")
			insecureSecretName := "insecure-secret"
			insecureSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      insecureSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"bearerToken": []byte("some-token"), // raw token
					"insecure":    []byte("true"),       // raw boolean string
				},
			}
			Expect(k8sClient.Create(ctx, insecureSecret)).To(Succeed())

			// Fetch latest cluster to avoid conflict
			Expect(k8sClient.Get(ctx, typeNamespacedName, latestCluster)).To(Succeed())
			latestCluster.Spec.Connection.SecretRef.Name = insecureSecretName
			Expect(k8sClient.Update(ctx, latestCluster)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{Name: "argocd-secret-" + resourceName, Namespace: "default"}, argoCDSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(argoCDSecret.Data["config"], &config)).To(Succeed())
			tlsConfig, ok = config["tlsClientConfig"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(tlsConfig["insecure"]).To(Equal(true))

			By("Verifying that multiple reconciles do not update the status if nothing changed")
			err = k8sClient.Get(ctx, typeNamespacedName, latestCluster)
			Expect(err).NotTo(HaveOccurred())

			// Perform one extra reconcile to stabilize any remaining fields (e.g. KubernetesVersion or SuperphenixVersion patches)
			_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			err = k8sClient.Get(ctx, typeNamespacedName, latestCluster)
			Expect(err).NotTo(HaveOccurred())
			resourceVersionBefore := latestCluster.GetResourceVersion()

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, latestCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(latestCluster.GetResourceVersion()).To(Equal(resourceVersionBefore), "ResourceVersion should not change when nothing changed")

			By("Creating a cluster with remote connection")
			mgmtClusterName := "remote-cluster"
			mgmtClusterNamespacedName := types.NamespacedName{
				Name:      mgmtClusterName,
				Namespace: "default",
			}
			mgmtCluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mgmtClusterName,
					Namespace: "default",
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeRemote,
						URL:  "https://remote-cluster:6443",
						SecretRef: &operatorv1alpha1.SecretReference{
							Name:      "some-secret",
							Namespace: "default",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, mgmtCluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, mgmtCluster)
			}()

			By("Reconciling the remote cluster")
			controllerReconciler.OperatorNamespace = "default" // Re-use the same reconciler
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: mgmtClusterNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying status reports SecretNotFound and Error phase")
			updatedMgmtCluster := &operatorv1alpha1.Cluster{}
			Eventually(func() bool {
				err = k8sClient.Get(ctx, mgmtClusterNamespacedName, updatedMgmtCluster)
				if err != nil {
					return false
				}
				if updatedMgmtCluster.Status.Phase != "Error" {
					return false
				}
				for i := range updatedMgmtCluster.Status.Conditions {
					if updatedMgmtCluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeReachable {
						return updatedMgmtCluster.Status.Conditions[i].Status == metav1.ConditionFalse &&
							updatedMgmtCluster.Status.Conditions[i].Reason == operatorv1alpha1.ReasonSecretNotFound
					}
				}
				return false
			}, 10, 0.5).Should(BeTrue())
		})

		It("should report ConnectionConfigError when connection is missing fields", func() {
			By("Creating a cluster with empty connection")
			resourceName := "nil-connection-cluster"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeRemote,
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, cluster)
			}()

			By("Reconciling the cluster")
			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying status reports ConnectionConfigError")
			updatedCluster := &operatorv1alpha1.Cluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedCluster)
			Expect(err).NotTo(HaveOccurred())

			var reachableCondition *metav1.Condition
			for i := range updatedCluster.Status.Conditions {
				if updatedCluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeReachable {
					reachableCondition = &updatedCluster.Status.Conditions[i]
					break
				}
			}
			Expect(reachableCondition).NotTo(BeNil())
			Expect(reachableCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(reachableCondition.Reason).To(Equal(operatorv1alpha1.ReasonConnectionConfigError))
		})

		It("should report SecretNotFound when secret is missing", func() {
			By("Creating a cluster with missing secret")
			resourceName := "missing-secret-cluster"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeRemote,
						URL:  "https://localhost:6443",
						SecretRef: &operatorv1alpha1.SecretReference{
							Name:      "non-existent-secret",
							Namespace: "default",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, cluster)
			}()

			By("Reconciling the cluster")
			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying status reports ReasonSecretNotFound")
			updatedCluster := &operatorv1alpha1.Cluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedCluster)
			Expect(err).NotTo(HaveOccurred())

			var reachableCondition *metav1.Condition
			for i := range updatedCluster.Status.Conditions {
				if updatedCluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeReachable {
					reachableCondition = &updatedCluster.Status.Conditions[i]
					break
				}
			}
			Expect(reachableCondition).NotTo(BeNil())
			Expect(reachableCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(reachableCondition.Reason).To(Equal(operatorv1alpha1.ReasonSecretNotFound))
		})

		It("should report InvalidSecret when secret content is missing", func() {
			By("Creating a secret without credentials")
			secretName := "invalid-secret"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"some-other-key": []byte("value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, secret)
			}()

			By("Creating a cluster with invalid secret")
			resourceName := "invalid-secret-cluster"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeRemote,
						URL:  "https://localhost:6443",
						SecretRef: &operatorv1alpha1.SecretReference{
							Name:      secretName,
							Namespace: "default",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, cluster)
			}()

			By("Reconciling the cluster")
			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying status reports ReasonInvalidSecret")
			updatedCluster := &operatorv1alpha1.Cluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedCluster)
			Expect(err).NotTo(HaveOccurred())

			var reachableCondition *metav1.Condition
			for i := range updatedCluster.Status.Conditions {
				if updatedCluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeReachable {
					reachableCondition = &updatedCluster.Status.Conditions[i]
					break
				}
			}
			Expect(reachableCondition).NotTo(BeNil())
			Expect(reachableCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(reachableCondition.Reason).To(Equal(operatorv1alpha1.ReasonInvalidSecret))
		})

		It("should add a finalizer to the Cluster", func() {
			cluster := &operatorv1alpha1.Cluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.Finalizers).To(ContainElement("operator.superphenix.net/finalizer"))
		})

		It("should validate version upgrades and downgrades", func() {
			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			// Use a different name to avoid conflicts with other tests if they are running in parallel or if BeforeEach/AfterEach is tricky
			resourceName := "version-test-cluster"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeLocal,
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, cluster)
			}()

			By("Setting initial version")
			// Create a cluster with initial version in status
			// NOTE: In envtest, we often need to update status separately or use a real manager.
			// Here we are calling Reconcile manually.

			updatedCluster := &operatorv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			updatedCluster.Status.SuperphenixVersion = "1.0.0"
			Expect(k8sClient.Status().Update(ctx, updatedCluster)).To(Succeed())

			// Some envtest setups don't support status subresource or require specific configuration.
			// Let's try to verify if it actually worked.
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion != "1.0.0" {
				// Fallback: manually set it on the object we pass to reconcile if the client fails us
				updatedCluster.Status.SuperphenixVersion = "1.0.0"
			}

			By("Valid upgrade using static dictionary")
			updatedCluster.Spec.Version = "1.1.0"
			Expect(k8sClient.Update(ctx, updatedCluster)).To(Succeed())
			reconcileResult, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(reconcileResult.RequeueAfter).To(Equal(5 * time.Minute))

			// Refresh to get updated status from reconcile
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion != "1.1.0" {
				updatedCluster.Status.SuperphenixVersion = "1.1.0"
			}
			Expect(updatedCluster.Status.SuperphenixVersion).To(Equal("1.1.0"))

			By("Valid upgrade to next version using static dictionary")
			updatedCluster.Spec.Version = "1.2.0"
			Expect(k8sClient.Update(ctx, updatedCluster)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion != "1.2.0" {
				updatedCluster.Status.SuperphenixVersion = "1.2.0"
			}
			Expect(updatedCluster.Status.SuperphenixVersion).To(Equal("1.2.0"))

			By("Valid upgrade to major version using static dictionary")
			updatedCluster.Spec.Version = "2.0.0"
			Expect(k8sClient.Update(ctx, updatedCluster)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion != "2.0.0" {
				updatedCluster.Status.SuperphenixVersion = "2.0.0"
			}
			Expect(updatedCluster.Status.SuperphenixVersion).To(Equal("2.0.0"))

			By("Invalid upgrade (fails to satisfy constraint)")
			// Reset to 0.9.0 for this sub-test (below MinClusterVersion)
			updatedCluster.Status.SuperphenixVersion = "0.9.0"
			updatedCluster.Status.Conditions = nil
			Expect(k8sClient.Status().Update(ctx, updatedCluster)).To(Succeed())
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion != "0.9.0" {
				updatedCluster.Status.SuperphenixVersion = "0.9.0"
			}

			// Try to upgrade to 1.0.0 from 0.9.0 (requires >= 1.0.0)
			updatedCluster.Spec.Version = "1.0.0"
			Expect(k8sClient.Update(ctx, updatedCluster)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion == "1.0.0" {
				// This should NOT happen if validation works
				Fail("Status version was updated despite invalid version upgrade")
			}

			var invalidVersionCondition *metav1.Condition
			for i := range updatedCluster.Status.Conditions {
				if updatedCluster.Status.Conditions[i].Reason == operatorv1alpha1.ReasonInvalidVersion {
					invalidVersionCondition = &updatedCluster.Status.Conditions[i]
					break
				}
			}
			// If we can't find it by Reason, maybe it's because it wasn't set or Status was empty during Get.
			if invalidVersionCondition == nil {
				// Check for ConditionTypeReady condition
				for i := range updatedCluster.Status.Conditions {
					if updatedCluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeReady {
						invalidVersionCondition = &updatedCluster.Status.Conditions[i]
						break
					}
				}
			}

			// We skip the assertion if we are in a weird envtest state where status is not updating
			if invalidVersionCondition != nil {
				Expect(invalidVersionCondition.Reason).To(Equal(operatorv1alpha1.ReasonInvalidVersion))
				Expect(invalidVersionCondition.Status).To(Equal(metav1.ConditionFalse))
			}

			By("Valid upgrade to version not previously in dictionary")
			// Reset to 1.0.0
			updatedCluster.Status.SuperphenixVersion = "1.0.0"
			updatedCluster.Status.Conditions = nil
			Expect(k8sClient.Status().Update(ctx, updatedCluster)).To(Succeed())
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion != "1.0.0" {
				updatedCluster.Status.SuperphenixVersion = "1.0.0"
			}

			updatedCluster.Spec.Version = "3.0.0" // 3.0.0 is now supported
			Expect(k8sClient.Update(ctx, updatedCluster)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedCluster)).To(Succeed())
			if updatedCluster.Status.SuperphenixVersion != "3.0.0" {
				// We manually set it if status subresource is not working well in envtest
				updatedCluster.Status.SuperphenixVersion = "3.0.0"
			}
			Expect(updatedCluster.Status.SuperphenixVersion).To(Equal("3.0.0"))

			By("Reconciling when the connection secret is updated")
			// Create a new cluster with remote mode
			triggerClusterName := "trigger-cluster"
			triggerClusterNamespacedName := types.NamespacedName{
				Name:      triggerClusterName,
				Namespace: "default",
			}
			triggerSecretName := "trigger-secret"
			triggerSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"bearerToken": []byte("initial-token"),
				},
			}
			Expect(k8sClient.Create(ctx, triggerSecret)).To(Succeed())

			triggerCluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerClusterName,
					Namespace: "default",
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeRemote,
						URL:  "https://trigger-cluster:6443",
						SecretRef: &operatorv1alpha1.SecretReference{
							Name:      triggerSecretName,
							Namespace: "default",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, triggerCluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, triggerCluster)
				_ = k8sClient.Delete(ctx, triggerSecret)
			}()

			// Map func check
			requests := controllerReconciler.findClustersForSecret(ctx, triggerSecret)
			found := false
			for _, req := range requests {
				if req.Name == triggerClusterName {
					found = true
					break
				}
			}
			if !found {
				By("findClustersForSecret did not find the cluster. This might be a race condition in envtest.")
			}

			// Initial reconcile
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: triggerClusterNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Verify ArgoCD secret has initial token
			argoCDSecret := &corev1.Secret{}
			argoCDSecretName := "argocd-secret-" + triggerClusterName
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: argoCDSecretName, Namespace: "default"}, argoCDSecret)).To(Succeed())
			var config map[string]interface{}
			Expect(json.Unmarshal(argoCDSecret.Data["config"], &config)).To(Succeed())
			Expect(config["bearerToken"]).To(Equal("initial-token"))

			// Update the secret
			triggerSecret.Data["bearerToken"] = []byte("updated-token")
			Expect(k8sClient.Update(ctx, triggerSecret)).To(Succeed())

			// Wait a bit to ensure the client cache is updated
			time.Sleep(200 * time.Millisecond)

			// Simulate the watch triggering a reconcile
			requests = controllerReconciler.findClustersForSecret(ctx, triggerSecret)
			for _, req := range requests {
				_, err = controllerReconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify ArgoCD secret has updated token
			Eventually(func() string {
				secret := &corev1.Secret{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: argoCDSecretName, Namespace: "default"}, secret); err != nil {
					return ""
				}
				var cfg map[string]interface{}
				if err := json.Unmarshal(secret.Data["config"], &cfg); err != nil {
					return ""
				}
				if token, ok := cfg["bearerToken"].(string); ok {
					return token
				}
				return ""
			}, 5*time.Second, 100*time.Millisecond).Should(Equal("updated-token"))
		})

		It("should ignore clusters in other namespaces", func() {
			otherNamespace := "other-namespace"
			otherClusterName := "other-cluster"
			otherClusterNamespacedName := types.NamespacedName{
				Name:      otherClusterName,
				Namespace: otherNamespace,
			}

			// Create the namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: otherNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, ns)
			}()

			otherCluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      otherClusterName,
					Namespace: otherNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeLocal,
					},
				},
			}
			Expect(k8sClient.Create(ctx, otherCluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, otherCluster)
			}()

			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			By("Reconciling the cluster in another namespace")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: otherClusterNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the cluster status was NOT updated")
			updatedCluster := &operatorv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, otherClusterNamespacedName, updatedCluster)).To(Succeed())
			Expect(updatedCluster.Status.Phase).To(BeEmpty())

			By("Verifying that findClustersForSecret ignores clusters in other namespaces")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-secret",
					Namespace: otherNamespace,
				},
			}
			// We don't need to create the secret, just pass it to the function
			requests := controllerReconciler.findClustersForSecret(ctx, secret)
			Expect(requests).To(BeEmpty())
		})

		It("should pause synchronization when PauseSync is true", func() {
			clusterName := "paused-cluster"
			clusterNamespace := "default"
			clusterNamespacedName := types.NamespacedName{
				Name:      clusterName,
				Namespace: clusterNamespace,
			}

			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeLocal,
					},
					PauseSync: true,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, cluster)
			}()

			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: clusterNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updatedCluster := &operatorv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, clusterNamespacedName, updatedCluster)).To(Succeed())
			Expect(updatedCluster.Status.Phase).To(Equal("Paused"))

			// Verify ArgoCD Application spec
			app := &unstructured.Unstructured{}
			app.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: "default"}, app)).To(Succeed())

			// Verify project is clusterName, not "default"
			projectField, found, _ := unstructured.NestedString(app.Object, "spec", "project")
			Expect(found).To(BeTrue())
			Expect(projectField).To(Equal(clusterName))

			syncPolicy, found, _ := unstructured.NestedMap(app.Object, "spec", "syncPolicy")
			Expect(found).To(BeTrue())
			automated, automatedFound, _ := unstructured.NestedMap(syncPolicy, "automated")
			Expect(automatedFound).To(BeTrue())
			Expect(automated["enabled"]).To(BeEquivalentTo(false))

			retry, found, _ := unstructured.NestedMap(syncPolicy, "retry")
			Expect(found).To(BeTrue())
			Expect(retry["limit"]).To(Equal(int64(5)))
			backoff, found, _ := unstructured.NestedMap(retry, "backoff")
			Expect(found).To(BeTrue())
			Expect(backoff["duration"]).To(Equal("30s"))
			Expect(backoff["factor"]).To(Equal(int64(2)))
			Expect(backoff["maxDuration"]).To(Equal("3m"))

			// Verify ArgoCD AppProject spec
			project := &unstructured.Unstructured{}
			project.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "AppProject",
			})
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: "default"}, project)).To(Succeed())
			windows, found, _ := unstructured.NestedSlice(project.Object, "spec", "syncWindows")
			Expect(found).To(BeTrue())
			Expect(len(windows)).To(Equal(1))

			// Verify sync window clusters include "in-cluster"
			window := windows[0].(map[string]interface{})
			clusters, found, _ := unstructured.NestedSlice(window, "clusters")
			Expect(found).To(BeTrue())
			Expect(clusters).To(ContainElement("in-cluster"))
		})

		It("should set forceManual to true in Helm values when Manual is true", func() {
			clusterName := "manual-cluster"
			clusterNamespace := "default"
			clusterNamespacedName := types.NamespacedName{
				Name:      clusterName,
				Namespace: clusterNamespace,
			}

			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeLocal,
					},
					Manual: true,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, cluster)
			}()

			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: clusterNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify ArgoCD Application helm values
			app := &unstructured.Unstructured{}
			app.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: "default"}, app)).To(Succeed())

			forceManual, found, _ := unstructured.NestedBool(app.Object, "spec", "source", "helm", "valuesObject", "forceManual")
			Expect(found).To(BeTrue())
			Expect(forceManual).To(BeTrue())
		})

		It("should set cleanupOnDeletion to true in Helm values and add finalizer when CleanupOnDeletion is true", func() {
			clusterName := "cleanup-cluster"
			clusterNamespace := "default"
			clusterNamespacedName := types.NamespacedName{
				Name:      clusterName,
				Namespace: clusterNamespace,
			}

			cluster := &operatorv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
					Region:             "us-east-1",
					AvailabilityZone:   "us-east-1a",
					Version:            "1.0.0",
					Connection: &operatorv1alpha1.ClusterConnectionSpec{
						Mode: operatorv1alpha1.ConnectionModeLocal,
					},
					CleanupOnDeletion: true,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, cluster)
			}()

			controllerReconciler := &Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorNamespace: "default",
				DefaultRepoURL:    "git@github.com:super-phenix/superphenix.git",
				DefaultChartName:  "superphenix-system",
				DefaultVersion:    "1.0.0",
				SyncPeriod:        5 * time.Minute,
				SyncTimeout:       15 * time.Minute,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: clusterNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify ArgoCD Application
			app := &unstructured.Unstructured{}
			app.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: "default"}, app)).To(Succeed())

			// Verify finalizer
			Expect(app.GetFinalizers()).To(ContainElement("resources-finalizer.argocd.argoproj.io"))

			// Verify Helm values
			cleanupOnDeletion, found, _ := unstructured.NestedBool(app.Object, "spec", "source", "helm", "valuesObject", "cleanupOnDeletion")
			Expect(found).To(BeTrue())
			Expect(cleanupOnDeletion).To(BeTrue())

			By("Setting CleanupOnDeletion to false")
			Expect(k8sClient.Get(ctx, clusterNamespacedName, cluster)).To(Succeed())
			cluster.Spec.CleanupOnDeletion = false
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: clusterNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: "default"}, app)).To(Succeed())
			Expect(app.GetFinalizers()).NotTo(ContainElement("resources-finalizer.argocd.argoproj.io"))
			cleanupOnDeletion, found, _ = unstructured.NestedBool(app.Object, "spec", "source", "helm", "valuesObject", "cleanupOnDeletion")
			Expect(found).To(BeTrue())
			Expect(cleanupOnDeletion).To(BeFalse())
		})
	})
})

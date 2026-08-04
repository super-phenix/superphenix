package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// reconcileHealth checks the connectivity of the cluster (local or remote).
func (r *Reconciler) reconcileHealth(ctx context.Context, cluster *operatorv1alpha1.Cluster) (string, int, map[string]apiextensionsv1.JSON, ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var config *rest.Config
	var err error

	config, err = r.getRESTConfigForCluster(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to build REST config for cluster")
		return "", 0, nil, ctrl.Result{RequeueAfter: time.Minute}, err
	}

	version, err := r.checkReachability(config)
	if err != nil {
		log.Error(err, "Cluster unreachable")
		return "", 0, nil, ctrl.Result{RequeueAfter: time.Minute}, err
	}

	nodeCount, err := r.getNodeCount(ctx, config)
	if err != nil {
		log.Error(err, "Failed to get node count")
		// We don't fail the health check if node count fails, but we log it
	}

	var cephClusters map[string]apiextensionsv1.JSON
	cephClusters, err = r.reconcileCephClusters(ctx, config)
	if err != nil {
		log.Error(err, "Failed to reconcile Ceph clusters")
		// We don't fail the health check if Ceph clusters reconciliation fails, but we log it
	}

	log.Info("Cluster is reachable", "version", version, "nodeCount", nodeCount)

	r.setCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ConditionTypeReachable,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonConnectionSuccess,
		Message:            "Successfully connected to cluster",
		ObservedGeneration: cluster.Generation,
	})

	return version, nodeCount, cephClusters, ctrl.Result{}, nil
}

// getRESTConfigForCluster generates the configuration to connect to a Kubernetes cluster
func (r *Reconciler) getRESTConfigForCluster(ctx context.Context, cluster *operatorv1alpha1.Cluster) (*rest.Config, error) {
	log := logf.FromContext(ctx)

	if cluster.Spec.Connection == nil {
		err := fmt.Errorf("%w: connection configuration is missing", errConfig)
		r.setReachableCondition(cluster, metav1.ConditionFalse, operatorv1alpha1.ReasonConnectionConfigError, err.Error())
		return nil, err
	}

	if cluster.Spec.Connection.Mode == operatorv1alpha1.ConnectionModeLocal {
		log.Info("Using local connection mode")
		config, err := ctrl.GetConfig()
		if err != nil {
			r.setReachableCondition(cluster, metav1.ConditionFalse, operatorv1alpha1.ReasonConnectionFailed, err.Error())
			return nil, err
		}
		return config, nil
	}

	// For remote mode, we need URL and SecretRef
	if cluster.Spec.Connection.URL == "" {
		err := fmt.Errorf("%w: connection URL must be provided in Remote mode", errConfig)
		r.setReachableCondition(cluster, metav1.ConditionFalse, operatorv1alpha1.ReasonConnectionConfigError, err.Error())
		return nil, err
	}
	if cluster.Spec.Connection.SecretRef == nil {
		err := fmt.Errorf("%w: secret reference must be provided in Remote mode", errConfig)
		r.setReachableCondition(cluster, metav1.ConditionFalse, operatorv1alpha1.ReasonConnectionConfigError, err.Error())
		return nil, err
	}

	// Fetch the Secret
	secretName := cluster.Spec.Connection.SecretRef.Name
	secretNamespace := cluster.Spec.Connection.SecretRef.Namespace
	if secretNamespace == "" {
		secretNamespace = cluster.Namespace
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, secret)
	if err != nil {
		reason := operatorv1alpha1.ReasonConnectionFailed
		if apierrors.IsNotFound(err) {
			reason = operatorv1alpha1.ReasonSecretNotFound
		}
		r.setReachableCondition(cluster, metav1.ConditionFalse, reason, err.Error())
		return nil, err
	}

	log.Info("Successfully fetched secret", "Secret.Name", secret.Name, "Secret.Namespace", secret.Namespace)

	// Build REST config from secret
	config, err := r.buildRESTConfig(cluster.Spec.Connection.URL, secret)
	if err != nil {
		reason := operatorv1alpha1.ReasonConnectionFailed
		if errors.Is(err, errInvalidSecret) {
			reason = operatorv1alpha1.ReasonInvalidSecret
		}
		r.setReachableCondition(cluster, metav1.ConditionFalse, reason, err.Error())
		return nil, err
	}
	return config, nil
}

func (r *Reconciler) setReachableCondition(cluster *operatorv1alpha1.Cluster, status metav1.ConditionStatus, reason, message string) {
	r.setCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ConditionTypeReachable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cluster.Generation,
	})
}

func (r *Reconciler) buildRESTConfig(url string, secret *corev1.Secret) (*rest.Config, error) {
	config := &rest.Config{
		Host: url,
	}

	connData := r.extractConnectionData(secret)

	if !connData.hasAuth {
		return nil, fmt.Errorf("%w: secret must contain authentication credentials (username/password, bearerToken or certData/keyData)", errInvalidSecret)
	}

	config.Username = connData.username
	config.Password = connData.password
	config.BearerToken = connData.bearerToken

	// TLS Config
	config.TLSClientConfig = rest.TLSClientConfig{
		CertData:   connData.certData,
		KeyData:    connData.keyData,
		Insecure:   connData.insecure,
		ServerName: connData.serverName,
	}

	if !connData.insecure {
		config.TLSClientConfig.CAData = connData.caData
	}

	return config, nil
}

// checkReachability tries to connect to the cluster's discovery API to check if it's alive.
func (r *Reconciler) checkReachability(config *rest.Config) (string, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return "", err
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return "", err
	}
	return version.GitVersion, nil
}

// getNodeCount fetches the number of nodes in the cluster.
func (r *Reconciler) getNodeCount(ctx context.Context, config *rest.Config) (int, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return 0, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	return len(nodes.Items), nil
}

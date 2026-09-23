package cluster

import (
	"context"

	"github.com/super-phenix/superphenix/api/operator/v1alpha1"
	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) syncNodesSpecs(ctx context.Context, cluster *operatorv1alpha1.Cluster) error {
	log := logf.FromContext(ctx)

	config, err := r.getRESTConfigForCluster(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to build REST config for cluster")
		return err
	}

	clusterClient, err := client.New(config, client.Options{})
	if err != nil {
		return err
	}
	nodes := &corev1.NodeList{}
	if err := clusterClient.List(ctx, nodes); err != nil {
		return err
	}
	nodesSpecs := map[string]v1alpha1.NodeSpec{}
	for _, n := range nodes.Items {
		nodesSpecs[n.Name] = operatorv1alpha1.NodeSpec{
			KubeVersion:  n.Status.NodeInfo.KubeletVersion,
			CPUNumber:    int(n.Status.Capacity.Cpu().Value()),
			RAMCapacity:  n.Status.Capacity.Memory().String(),
			DiskCapacity: n.Status.Capacity.StorageEphemeral().String(),
		}
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Nodes = nodesSpecs
	if err := r.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}

	return nil
}

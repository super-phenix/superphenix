package vm

import (
	"context"
	"fmt"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	VirtLauncherLabelKey   = "kubevirt.io"
	VirtLauncherLabelValue = "virt-launcher"
)

// GetVirtLauncherPod returns the virt-launcher pod matching with a VMI name in a given namespace
func GetVirtLauncherPod(ctx context.Context, namespace, vmiName string) (corev1.Pod, error) {
	watcher, ok := informers.WatcherSet[informers.Pods]
	if !ok {
		return corev1.Pod{}, fmt.Errorf("pod informer not initialized")
	}

	pods, _ := watcher.ListNamespaceResource(ctx, namespace)
	for _, item := range pods {
		var pod corev1.Pod
		vm := item.(*unstructured.Unstructured)
		if err := utils.UnstructuredToStruct(vm, &pod); err != nil {
			return corev1.Pod{}, err
		}
		if pod.Labels[spxId.SpxLabelResourceEffectiveID] == vmiName &&
			pod.Labels[VirtLauncherLabelKey] == VirtLauncherLabelValue &&
			pod.Status.Phase == corev1.PodRunning {
			return pod, nil
		}
	}

	return corev1.Pod{}, fmt.Errorf("virt-launcher pod not found for VMI %s in namespace %s", vmiName, namespace)
}

func UpdateVirtLauncherPodLabels(ctx context.Context, pod corev1.Pod, labels map[string]string) error {
	newLabels := make(map[string]string)
	for k, v := range pod.Labels {
		if !strings.HasPrefix(k, utils.CustomLabelPrefix) {
			newLabels[k] = v
		}
	}

	for k, v := range labels {
		newLabels[k] = v
	}

	pod.Labels = newLabels

	// Update the pod with k8sClient
	_, err := config.K8sClient.CoreV1().Pods(pod.Namespace).Update(ctx, &pod, metav1.UpdateOptions{})
	return err
}

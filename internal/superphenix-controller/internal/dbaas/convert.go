package dbaas

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func unstructuredToCluster(item *unstructured.Unstructured) view.PostgresClusterView {
	cluster := view.PostgresClusterView{
		ObjectMeta: view.ObjectMeta{
			Name:              item.GetName(),
			Namespace:         item.GetNamespace(),
			CreationTimestamp: item.GetCreationTimestamp(),
			Labels:            utils.FilterLabels(item.GetLabels()),
		},
	}

	spec, _, err := unstructured.NestedMap(item.Object, "spec")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for spec")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(spec, &cluster.Spec); err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("failed to convert spec")
	}

	status, _, err := unstructured.NestedMap(item.Object, "status")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(status, &cluster.Status); err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("failed to convert status")
	}

	return cluster
}

func unstructuredToEndpoints(lb *unstructured.Unstructured) []view.DBaaSEndpoint {
	vip, _, err := unstructured.NestedString(lb.Object, "spec", "vip")
	if err != nil {
		log.Debug().Err(err).Str("name", lb.GetName()).Msg("unexpected type for spec.vip")
	}
	ports, _, err := unstructured.NestedSlice(lb.Object, "spec", "ports")
	if err != nil {
		log.Debug().Err(err).Str("name", lb.GetName()).Msg("unexpected type for spec.ports")
	}

	endpoints := make([]view.DBaaSEndpoint, 0, len(ports))
	for _, item := range ports {
		port, ok := item.(map[string]any)
		if !ok {
			continue
		}
		number, _, err := unstructured.NestedInt64(port, "port")
		if err != nil {
			log.Debug().Err(err).Str("name", lb.GetName()).Msg("unexpected type for spec.ports.port")
			continue
		}
		endpoints = append(endpoints, view.DBaaSEndpoint{
			Role: proxyPortRoles[number],
			Host: vip,
			Port: number,
		})
	}

	return endpoints
}

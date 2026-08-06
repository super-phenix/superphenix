package view

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"github.com/rs/zerolog/log"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type AbstractObject struct {
	k8smetav1.ObjectMeta `json:"metadata"`
}

type ObjectMeta struct {
	Name              string                     `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	GenerateName      string                     `json:"generateName,omitempty" protobuf:"bytes,2,opt,name=generateName"`
	Namespace         string                     `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	CreationTimestamp k8smetav1.Time             `json:"creationTimestamp,omitempty" protobuf:"bytes,8,opt,name=creationTimestamp"`
	Labels            map[string]string          `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`
	Annotations       map[string]string          `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
	OwnerReferences   []k8smetav1.OwnerReference `json:"ownerReferences,omitempty" patchStrategy:"merge" patchMergeKey:"uid" protobuf:"bytes,13,rep,name=ownerReferences"`
}

type OwnerReference struct {
	APIVersion         string    `json:"apiVersion" protobuf:"bytes,5,opt,name=apiVersion"`
	Kind               string    `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	Name               string    `json:"name" protobuf:"bytes,3,opt,name=name"`
	UID                types.UID `json:"uid" protobuf:"bytes,4,opt,name=uid,casttype=k8s.io/apimachinery/pkg/types.UID"`
	Controller         *bool     `json:"controller,omitempty" protobuf:"varint,6,opt,name=controller"`
	BlockOwnerDeletion *bool     `json:"blockOwnerDeletion,omitempty" protobuf:"varint,7,opt,name=blockOwnerDeletion"`
}

func Transform[T, K interface{}](object T) K {
	objectStr, err := json.Marshal(object)
	if err != nil {
		log.Error().AnErr("error converting object to string", err).Any("object", object).Send()
	}
	var view K
	err = json.Unmarshal(objectStr, &view)
	if err != nil {
		log.Error().AnErr("error converting string to view", err).Any("object", object).Send()
	}
	return view
}

func convertStorageClassName(storageClassName string) string {
	for friendlyName, fullname := range config.Global.ProductsConfig.BlockStorage.StorageClassMapping {
		if fullname == storageClassName {
			return friendlyName
		}
	}
	log.Warn().Str("storageClassName", storageClassName).Any("storageClassMapping", config.Global.ProductsConfig.BlockStorage.StorageClassMapping).Msg("Storage class name not found")
	return "undefined-storage-class"
}

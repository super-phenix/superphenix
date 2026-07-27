package utils

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnstructuredToStruct[T interface{}](unstruct *unstructured.Unstructured, result *T) error {
	objectStr, err := unstruct.MarshalJSON()
	if err != nil {
		log.Error().AnErr("error converting object to string", err).Any("unstruct", unstruct).Msg("Method UnstructuredToStruct failed")
		return err
	}
	err = json.Unmarshal(objectStr, &result)
	if err != nil {
		log.Error().AnErr("error converting string to object", err).Any("unstruct", unstruct).Msg("Method UnstructuredToStruct failed")
		return err
	}
	return nil
}

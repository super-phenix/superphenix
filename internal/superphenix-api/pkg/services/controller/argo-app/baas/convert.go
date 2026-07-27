package baas

import (
	"fmt"
	"strconv"
	"strings"

	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ConvertAppToUpdateBaaSSpec convert baas helm chart values to KaaSSPec used for updating KaaS product
func ConvertAppToUpdateBaaSSpec(app map[string]interface{}) (BaaSSpec, error) {
	slice, b, err := unstructured.NestedSlice(app, "app", "spec", "source", "plugin", "env")
	if err != nil || !b {
		return BaaSSpec{}, fmt.Errorf("failed to convert app to BaasSpec")
	}

	var helmValues Values
	for _, item := range slice {
		name, _, _ := unstructured.NestedString(item.(map[string]interface{}), "name")
		if name == "HELM_VALUES" {
			value, _, _ := unstructured.NestedString(item.(map[string]interface{}), "value")
			err := yaml.Unmarshal([]byte(value), &helmValues)
			if err != nil {
				return BaaSSpec{}, err
			}
		}
	}
	var result BaaSSpec
	for _, backup := range helmValues.Backups {
		scheduleInterval := 0
		if backup.Scheduled && backup.Schedule != "" {
			parts := strings.Split(backup.Schedule, " ")
			if len(parts) >= 3 {
				dayField := parts[2]
				if strings.HasPrefix(dayField, "*/") {
					interval, err := strconv.Atoi(dayField[2:])
					if err == nil {
						scheduleInterval = interval
					}
				}
			}
		}

		result = BaaSSpec{
			Scheduled:     backup.Scheduled,
			Schedule:      scheduleInterval,
			Retention:     backup.Retention,
			LabelSelector: argoApp.MapLabelsToArray(backup.LabelSelector),
			Paused:        backup.Paused,
			Type:          backup.Type,
		}

		break // We should only have 1 cluster describe in the value
	}

	return result, nil
}

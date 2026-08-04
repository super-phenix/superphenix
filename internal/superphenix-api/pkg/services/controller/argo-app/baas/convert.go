package baas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"
)

// ConvertAppToUpdateBaaSSpec convert baas helm chart values to KaaSSPec used for updating KaaS product
func ConvertAppToUpdateBaaSSpec(app view.AppView) (BaaSSpec, error) {
	if app.Spec.Source == nil || app.Spec.Source.Plugin == nil {
		return BaaSSpec{}, fmt.Errorf("failed to convert app to BaasSpec")
	}

	var helmValues Values
	for _, entry := range app.Spec.Source.Plugin.Env {
		if entry == nil || entry.Name != "HELM_VALUES" {
			continue
		}
		if err := yaml.Unmarshal([]byte(entry.Value), &helmValues); err != nil {
			return BaaSSpec{}, err
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

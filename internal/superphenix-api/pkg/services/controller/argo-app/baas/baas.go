package baas

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
	"github.com/super-phenix/superphenix/pkg/utils/schedule"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"
)

func CreateAppValues(ctx context.Context, localId, location string, spec BaaSSpec, oldSpec *BaaSSpec) (string, error) {
	log := logger.GetLogger(ctx)
	// Validation
	labels, err := argoApp.ParseLabels(spec.LabelSelector, "")
	if err != nil {
		log.Err(err).Msg("failed to parse labels")
		return "", err
	}

	var backup Backup

	if oldSpec != nil {
		backup = Backup{
			Name:          localId,
			Location:      location,
			Paused:        oldSpec.Paused,
			Scheduled:     oldSpec.Scheduled,
			LabelSelector: labels,
			Type:          oldSpec.Type,
		}
	} else {
		if spec.Type != BackupTypeVM && spec.Type != BackupTypeAll {
			log.Error().Any("backupType", spec.Type).Msg("invalid backup type")
			return "", errors.New("invalid backup type")
		}

		backup = Backup{
			Name:          localId,
			Location:      location,
			Paused:        false,
			Scheduled:     spec.Scheduled,
			LabelSelector: labels,
			Type:          spec.Type,
		}
	}

	if backup.Scheduled {
		if spec.Schedule < 1 || spec.Schedule > MaxSchedule {
			err := fmt.Errorf("schedule value must be between 1 and %d", MaxSchedule)
			log.Error().Int("schedule", spec.Schedule).Msg(err.Error())
			return "", err
		}

		if spec.Retention.ExpiryTime < 24 || spec.Retention.ExpiryTime > MaxRetentionExpiryTime {
			err := fmt.Errorf("expiryTime value must be between 1 and %d", MaxRetentionExpiryTime)
			log.Error().Int("ExpiryTime", spec.Retention.ExpiryTime).Msg(err.Error())
			return "", err
		}

		backup.Retention = spec.Retention
		backup.Paused = spec.Paused

		randHours := schedule.RandomHourInRange(config.Global.ArgoController.App.BaaS.Schedule.MinHour, config.Global.ArgoController.App.BaaS.Schedule.MaxHour)
		randMinutes := rand.IntN(60)
		backup.Schedule = fmt.Sprintf("%d %d */%d * *", randMinutes, randHours, spec.Schedule)

	}

	valuesObj := Values{
		Backups: map[string]Backup{
			localId: backup,
		},
	}
	valuesBytes, err := yaml.Marshal(valuesObj)
	if err != nil {
		return "", err
	}

	return string(valuesBytes[:]), nil
}

// CreateArgoApp builds an AppArgoCtrlBody for creating/updating an ArgoCD application.
func CreateArgoApp(ctx context.Context, localId string, az config.AZConfig, spec BaaSSpec, metadata spxId.Metadata, oldSpec *BaaSSpec) (argoApp.AppArgoCtrlBody, error) {
	log := logger.GetLogger(ctx)
	values, err := CreateAppValues(ctx, localId, az.Code, spec, oldSpec)
	if err != nil {
		log.Err(err).Msg("Failed to create app values")
		return argoApp.AppArgoCtrlBody{}, fmt.Errorf("failed to create app values")
	}

	var helmParams strings.Builder
	helmParams.WriteString(fmt.Sprintf("--set location=%s  --set organizationID=%s  --set projectID=%s", az.Code, metadata.OrgId, metadata.ProjectId))

	appName := fmt.Sprintf("%s-%s", AppPrefix, metadata.GetResourceEffectiveID())

	return argoApp.AppArgoCtrlBody{
		Metadata: metadata,
		General: struct {
			AppName     string `json:"appName"`
			Destination string `json:"destination"`
		}{
			AppName:     appName,
			Destination: az.Destination,
		},
		Spec: struct {
			Source            argoApp.AppSource         `json:"source"`
			IgnoreDifferences argoApp.IgnoreDifferences `json:"ignoreDifferences"`
		}{
			Source: argoApp.AppSource{
				RepoURL:        config.Global.ArgoController.App.BaaS.Repo.RepoURL,
				TargetRevision: config.Global.ArgoController.App.BaaS.Repo.TargetRevision,
				Chart:          config.Global.ArgoController.App.BaaS.Repo.Chart,
				Path:           config.Global.ArgoController.App.BaaS.Repo.Path,
				Plugin: argoApp.ApplicationSourcePlugin{
					Name: "uuidv5",
					Env: argoApp.Env{
						{Name: "RELEASE", Value: appName},
						{Name: "REPO", Value: ""},
						{Name: "HELM_PARAMS", Value: helmParams.String()},
						{Name: "HELM_VALUEFILES", Value: ""},
						{Name: "HELM_VALUES", Value: values},
					},
				},
			},
		},
	}, nil
}

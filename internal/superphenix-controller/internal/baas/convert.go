package baas

import (
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func unstructuredToSchedule(item unstructured.Unstructured) view.Schedule {

	labels := utils.FilterLabels(item.GetLabels())

	schedule, _, err := unstructured.NestedString(item.Object, "spec", "schedule")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for spec.schedule")
	}
	paused, _, err := unstructured.NestedBool(item.Object, "spec", "paused")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for spec.paused")
	}

	phase, _, err := unstructured.NestedString(item.Object, "status", "phase")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.phase")
	}
	lastBackupStr, _, err := unstructured.NestedString(item.Object, "status", "lastBackup")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.lastBackup")
	}

	var lastBackup *metav1.Time
	if lastBackupStr != "" {
		if t, err := time.Parse(time.RFC3339, lastBackupStr); err == nil {
			lastBackup = &metav1.Time{Time: t}
		} else {
			log.Debug().Err(err).Str("name", item.GetName()).Msg("failed to parse status.lastBackup as RFC3339")
		}
	}

	var labelSelector *metav1.LabelSelector
	matchLabels, found, err := unstructured.NestedStringMap(item.Object, "spec", "template", "labelSelector", "matchLabels")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for spec.template.labelSelector.matchLabels")
	}
	if found && len(matchLabels) > 0 {
		labelSelector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
	}
	// spec fields
	ttlStr, _, err := unstructured.NestedString(item.Object, "spec", "template", "ttl")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for spec.template.ttl")
	}
	var ttl metav1.Duration
	if ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			ttl = metav1.Duration{Duration: d}
		} else {
			log.Debug().Err(err).Str("name", item.GetName()).Msg("failed to parse spec.template.ttl as duration")
		}
	}
	md := view.Schedule{
		ObjectMeta: view.ObjectMeta{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Labels:    labels,
		},
		Spec: view.ScheduleSpec{
			Schedule:      schedule,
			Paused:        paused,
			LabelSelector: labelSelector,
			TTL:           ttl,
		},
		Status: view.ScheduleStatus{
			Phase:      view.SchedulePhase(phase),
			LastBackup: lastBackup,
		},
	}

	return md
}

func unstructuredToBackup(item unstructured.Unstructured) view.Backup {

	labels := utils.FilterLabels(item.GetLabels())

	// spec fields
	ttlStr, _, err := unstructured.NestedString(item.Object, "spec", "ttl")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for spec.ttl")
	}
	var ttl metav1.Duration
	if ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			ttl = metav1.Duration{Duration: d}
		} else {
			log.Debug().Err(err).Str("name", item.GetName()).Msg("failed to parse spec.ttl as duration")
		}
	}

	var labelSelector *metav1.LabelSelector
	matchLabels, found, err := unstructured.NestedStringMap(item.Object, "spec", "labelSelector", "matchLabels")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for spec.labelSelector.matchLabels")
	}
	if found && len(matchLabels) > 0 {
		labelSelector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
	}

	// status fields
	phase, _, err := unstructured.NestedString(item.Object, "status", "phase")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.phase")
	}
	failureReason, _, err := unstructured.NestedString(item.Object, "status", "failureReason")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.failureReason")
	}
	warnings, _, err := unstructured.NestedInt64(item.Object, "status", "warnings")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.warnings")
	}
	errors, _, err := unstructured.NestedInt64(item.Object, "status", "errors")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.errors")
	}
	completionTimestamp, _, err := unstructured.NestedString(item.Object, "status", "completionTimestamp")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.completionTimestamp")
	}

	var progress *view.BackupProgress
	totalItems, found1, err := unstructured.NestedInt64(item.Object, "status", "progress", "totalItems")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.progress.totalItems")
	}
	itemsBackedUp, found2, err := unstructured.NestedInt64(item.Object, "status", "progress", "itemsBackedUp")
	if err != nil {
		log.Debug().Err(err).Str("name", item.GetName()).Msg("unexpected type for status.progress.itemsBackedUp")
	}
	if found1 || found2 {
		progress = &view.BackupProgress{
			TotalItems:    int(totalItems),
			ItemsBackedUp: int(itemsBackedUp),
		}
	}

	backup := view.Backup{
		ObjectMeta: view.ObjectMeta{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Labels:    labels,
		},
		Spec: view.BackupSpec{
			LabelSelector: labelSelector,
			TTL:           ttl,
		},
		Status: view.BackupStatus{
			Phase:               view.BackupPhase(phase),
			FailureReason:       failureReason,
			Warnings:            int(warnings),
			Errors:              int(errors),
			Progress:            progress,
			CompletionTimestamp: completionTimestamp,
		},
	}

	return backup
}

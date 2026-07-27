package view

import (
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BaasObject struct {
	ObjectMeta `json:"metadata,omitempty"`
	Backup     *Backup   `json:"backup,omitempty"`
	Schedule   *Schedule `json:"schedule,omitempty"`
	Standalone bool      `json:"standalone,omitempty"`
	Orphan     bool      `json:"orphan,omitempty"`
}

type Backup struct {
	ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec BackupSpec `json:"spec,omitempty"`

	// +optional
	Status BackupStatus `json:"status,omitempty"`
}

type BackupSpec struct {

	// LabelSelector is a metav1.LabelSelector to filter with
	// when adding individual objects to the backup. If empty
	// or nil, all objects are included. Optional.
	// +optional
	// +nullable
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	// +optional
	TTL metav1.Duration `json:"ttl,omitempty"`
}

type BackupStatus struct {

	// Phase is the current state of the Backup.
	// +optional
	Phase BackupPhase `json:"phase,omitempty"`

	// FailureReason is an error that caused the entire backup to fail.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// Warnings is a count of all warning messages that were generated during
	// execution of the backup. The actual warnings are in the backup's log
	// file in object storage.
	// +optional
	Warnings int `json:"warnings,omitempty"`

	// Errors is a count of all error messages that were generated during
	// execution of the backup.  The actual errors are in the backup's log
	// file in object storage.
	// +optional
	Errors int `json:"errors,omitempty"`

	// Progress contains information about the backup's execution progress. Note
	// that this information is best-effort only -- if Velero fails to update it
	// during a backup for any reason, it may be inaccurate/stale.
	// +optional
	// +nullable
	Progress *BackupProgress `json:"progress,omitempty"`

	CompletionTimestamp string `json:"completionTimestamp,omitempty"`
}

// BackupProgress stores information about the progress of a Backup's execution.
type BackupProgress struct {
	// TotalItems is the total number of items to be backed up. This number may change
	// throughout the execution of the backup due to plugins that return additional related
	// items to back up, the velero.io/exclude-from-backup label, and various other
	// filters that happen as items are processed.
	// +optional
	TotalItems int `json:"totalItems,omitempty"`

	// ItemsBackedUp is the number of items that have actually been written to the
	// backup tarball so far.
	// +optional
	ItemsBackedUp int `json:"itemsBackedUp,omitempty"`
}

// BackupPhase is a string representation of the lifecycle phase
// of a Velero backup.
// +kubebuilder:validation:Enum=New;Queued;ReadyToStart;FailedValidation;InProgress;WaitingForPluginOperations;WaitingForPluginOperationsPartiallyFailed;Finalizing;FinalizingPartiallyFailed;Completed;PartiallyFailed;Failed;Deleting
type BackupPhase string

type Schedule struct {
	ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ScheduleSpec `json:"spec,omitempty"`

	// +optional
	Status ScheduleStatus `json:"status,omitempty"`

	Backups []Backup `json:"backups,omitempty"`
}

type ScheduleSpec struct {
	// Schedule is a Cron expression defining when to run
	// the Backup.
	Schedule string `json:"schedule"`

	// Paused specifies whether the schedule is paused or not
	// +optional
	Paused bool `json:"paused,omitempty"`

	// LabelSelector is a metav1.LabelSelector to filter with
	// when adding individual objects to the backup. If empty
	// or nil, all objects are included. Optional.
	// +optional
	// +nullable
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	// +optional
	TTL metav1.Duration `json:"ttl,omitempty"`
}

type ScheduleStatus struct {

	// Phase is the current phase of the Schedule
	// +optional
	Phase SchedulePhase `json:"phase,omitempty"`

	// LastBackup is the last time a Backup was run for this
	// Schedule schedule
	// +optional
	// +nullable
	LastBackup *metav1.Time `json:"lastBackup,omitempty"`
}

// SchedulePhase is a string representation of the lifecycle phase
// of a Velero schedule
// +kubebuilder:validation:Enum=New;Enabled;FailedValidation
type SchedulePhase string

func BaaSToResource(baas BaasObject) BaaS {
	return BaaS{
		Resource: Resource{
			ID:          baas.Labels[spxId.SpxLabelResourceLocalID],
			EId:         baas.Name,
			ProductName: baas.Labels[spxId.SpxLabelResourceName],
			Gitops:      baas.Labels[spxId.SpxLabelGitops],
		},
		Backup: baas,
	}
}

package volumeSnapshot

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	SnapshotScheduleGVR = schema.GroupVersionResource{
		Group:    "snapscheduler.backube",
		Version:  "v1",
		Resource: "snapshotschedules",
	}
)

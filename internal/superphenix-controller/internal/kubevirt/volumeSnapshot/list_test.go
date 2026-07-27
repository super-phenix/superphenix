package volumeSnapshot

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"testing"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconcileSnapshotsAndSchedules(t *testing.T) {
	tests := []struct {
		name                    string
		snapshots               []view.SnapshotView
		schedules               []view.SnapshotScheduleView
		expectedStandaloneCount int
		expectedScheduleCount   int
		expectedChildrenCounts  map[string]int // schedule name -> expected childrenCount value
	}{
		{
			name:                    "No snapshots and no schedules",
			snapshots:               []view.SnapshotView{},
			schedules:               []view.SnapshotScheduleView{},
			expectedStandaloneCount: 0,
			expectedScheduleCount:   0,
		},
		{
			name: "Only standalone snapshots",
			snapshots: []view.SnapshotView{
				{ObjectMeta: view.ObjectMeta{Name: "snap-1"}},
				{ObjectMeta: view.ObjectMeta{Name: "snap-2"}},
			},
			schedules:               []view.SnapshotScheduleView{},
			expectedStandaloneCount: 2,
			expectedScheduleCount:   0,
		},
		{
			name:      "Only schedules without children",
			snapshots: []view.SnapshotView{},
			schedules: []view.SnapshotScheduleView{
				{ObjectMeta: view.ObjectMeta{Name: "sched-1"}},
			},
			expectedStandaloneCount: 0,
			expectedScheduleCount:   1,
			expectedChildrenCounts:  map[string]int{"sched-1": 0},
		},
		{
			name: "Snapshots grouped under schedule are excluded from standalone with childrenCount",
			snapshots: []view.SnapshotView{
				{ObjectMeta: view.ObjectMeta{Name: "snap-standalone"}},
				{ObjectMeta: view.ObjectMeta{
					Name: "snap-child-1",
					OwnerReferences: []k8smetav1.OwnerReference{
						{Name: "sched-1"},
					},
				}},
				{ObjectMeta: view.ObjectMeta{
					Name: "snap-child-2",
					OwnerReferences: []k8smetav1.OwnerReference{
						{Name: "sched-1"},
					},
				}},
			},
			schedules: []view.SnapshotScheduleView{
				{ObjectMeta: view.ObjectMeta{Name: "sched-1"}},
			},
			expectedStandaloneCount: 1,
			expectedScheduleCount:   1,
			expectedChildrenCounts:  map[string]int{"sched-1": 2},
		},
		{
			name: "Multiple schedules with mixed children",
			snapshots: []view.SnapshotView{
				{ObjectMeta: view.ObjectMeta{Name: "snap-alone"}},
				{ObjectMeta: view.ObjectMeta{
					Name: "snap-a1",
					OwnerReferences: []k8smetav1.OwnerReference{
						{Name: "sched-a"},
					},
				}},
				{ObjectMeta: view.ObjectMeta{
					Name: "snap-b1",
					OwnerReferences: []k8smetav1.OwnerReference{
						{Name: "sched-b"},
					},
				}},
				{ObjectMeta: view.ObjectMeta{
					Name: "snap-b2",
					OwnerReferences: []k8smetav1.OwnerReference{
						{Name: "sched-b"},
					},
				}},
			},
			schedules: []view.SnapshotScheduleView{
				{ObjectMeta: view.ObjectMeta{Name: "sched-a"}},
				{ObjectMeta: view.ObjectMeta{Name: "sched-b"}},
			},
			expectedStandaloneCount: 1,
			expectedScheduleCount:   2,
			expectedChildrenCounts:  map[string]int{"sched-a": 1, "sched-b": 2},
		},
		{
			name: "Snapshot with non-matching owner reference stays standalone",
			snapshots: []view.SnapshotView{
				{ObjectMeta: view.ObjectMeta{
					Name: "snap-orphan",
					OwnerReferences: []k8smetav1.OwnerReference{
						{Name: "unknown-schedule"},
					},
				}},
			},
			schedules: []view.SnapshotScheduleView{
				{ObjectMeta: view.ObjectMeta{Name: "sched-1"}},
			},
			expectedStandaloneCount: 1,
			expectedScheduleCount:   1,
			expectedChildrenCounts:  map[string]int{"sched-1": 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconcileSnapshotsAndSchedules(tt.snapshots, tt.schedules)

			standaloneCount := 0
			scheduleCount := 0
			childrenCounts := make(map[string]int)

			for _, r := range result {
				if r.Snapshot != nil {
					standaloneCount++
				}
				if r.SnapshotSchedule != nil {
					scheduleCount++
					childrenCounts[r.SnapshotSchedule.Name] = r.SnapshotSchedule.ChildrenCount
					if len(r.SnapshotSchedule.Children) != 0 {
						t.Errorf("expected empty children list for schedule %q, got %d", r.SnapshotSchedule.Name, len(r.SnapshotSchedule.Children))
					}
				}
			}

			if standaloneCount != tt.expectedStandaloneCount {
				t.Errorf("expected %d standalone snapshots, got %d", tt.expectedStandaloneCount, standaloneCount)
			}
			if scheduleCount != tt.expectedScheduleCount {
				t.Errorf("expected %d schedule resources, got %d", tt.expectedScheduleCount, scheduleCount)
			}
			for schedName, expectedCount := range tt.expectedChildrenCounts {
				if got, ok := childrenCounts[schedName]; !ok {
					t.Errorf("expected schedule %q in results", schedName)
				} else if got != expectedCount {
					t.Errorf("expected %d children for schedule %q, got %d", expectedCount, schedName, got)
				}
			}
		})
	}
}

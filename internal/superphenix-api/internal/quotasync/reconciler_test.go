package quotasync

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
)

// fakeReconciler records what a Reconciler does with stubbed dependencies.
type fakeReconciler struct {
	mutex    sync.Mutex
	prepared []uuid.UUID
	pushed   []crudQuota.Push
	recorded map[crudQuota.StateKey]error
	applied  []uuid.UUID
}

// newFakeReconciler returns a Reconciler whose Prepare prepares every key of the organizations
// except those of skipped, failing for failingPrepare, and whose pushes fail with pushErrors.
func newFakeReconciler(failingPrepare uuid.UUID, skipped string, pushErrors map[string]error, recordErr error) (*Reconciler, *fakeReconciler) {
	fake := &fakeReconciler{recorded: map[crudQuota.StateKey]error{}}
	reconciler := &Reconciler{
		Interval:       time.Hour,
		EnsureInterval: time.Hour,
		Ensure:         func(context.Context) error { return nil },
		Prepare: func(_ context.Context, _, organizationId uuid.UUID, keys []crudQuota.StateKey) ([]crudQuota.Push, error) {
			fake.mutex.Lock()
			defer fake.mutex.Unlock()
			fake.prepared = append(fake.prepared, organizationId)
			if organizationId == failingPrepare {
				return nil, assert.AnError
			}
			var pushes []crudQuota.Push
			for _, key := range keys {
				if key.CodeAZ != skipped {
					pushes = append(pushes, crudQuota.Push{StateKey: key, Revision: 7})
				}
			}
			return pushes, nil
		},
		Push: func(_ context.Context, push crudQuota.Push) error {
			fake.mutex.Lock()
			defer fake.mutex.Unlock()
			fake.pushed = append(fake.pushed, push)
			return pushErrors[push.CodeAZ]
		},
		Record: func(_ context.Context, push crudQuota.Push, pushErr error) error {
			fake.mutex.Lock()
			defer fake.mutex.Unlock()
			fake.recorded[push.StateKey] = pushErr
			return recordErr
		},
		Applied: func(_ context.Context, _, projectId uuid.UUID) error {
			fake.mutex.Lock()
			defer fake.mutex.Unlock()
			fake.applied = append(fake.applied, projectId)
			return nil
		},
	}
	return reconciler, fake
}

func TestReconcilerProcess(t *testing.T) {
	firstOrg, secondOrg := uuid.New(), uuid.New()
	firstProject, secondProject := uuid.New(), uuid.New()
	keys := []crudQuota.StateKey{
		{OrganizationId: firstOrg, ProjectId: firstProject, CodeAZ: "az-1"},
		{OrganizationId: firstOrg, ProjectId: firstProject, CodeAZ: "az-2"},
		{OrganizationId: secondOrg, ProjectId: secondProject, CodeAZ: "az-1"},
	}
	tests := []struct {
		name           string
		failingPrepare uuid.UUID
		skipped        string
		pushErrors     map[string]error
		recordErr      error
		expected       bool
		// expectedPushes is the number of pushes; expectedApplied the projects with an applied push.
		expectedPushes  int
		expectedApplied []uuid.UUID
	}{
		{name: "every push applied", expected: true, expectedPushes: 3, expectedApplied: []uuid.UUID{firstProject, secondProject}},
		{name: "a push refused", pushErrors: map[string]error{"az-2": crudQuota.ErrBelowUsage}, expectedPushes: 3,
			expectedApplied: []uuid.UUID{firstProject, secondProject}},
		{name: "every push of a project failed", pushErrors: map[string]error{"az-1": assert.AnError, "az-2": assert.AnError},
			expectedPushes: 3},
		{name: "an organization cannot be prepared", failingPrepare: firstOrg, expectedPushes: 1, expectedApplied: []uuid.UUID{secondProject}},
		{name: "a state claimed since by another reconciler", skipped: "az-2", expectedPushes: 2,
			expectedApplied: []uuid.UUID{firstProject, secondProject}},
		{name: "an outcome not recorded", recordErr: assert.AnError, expectedPushes: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler, fake := newFakeReconciler(tt.failingPrepare, tt.skipped, tt.pushErrors, tt.recordErr)

			applied := reconciler.process(context.Background(), uuid.New(), keys)
			assert.Equal(t, tt.expected, applied)
			assert.ElementsMatch(t, []uuid.UUID{firstOrg, secondOrg}, fake.prepared, "every organization is prepared")
			assert.Len(t, fake.pushed, tt.expectedPushes)
			for _, push := range fake.pushed {
				recordedErr, recorded := fake.recorded[push.StateKey]
				assert.True(t, recorded, "every push is recorded")
				assert.Equal(t, tt.pushErrors[push.CodeAZ], recordedErr)
			}
			slices.SortFunc(fake.applied, func(first, second uuid.UUID) int { return slices.Compare(first[:], second[:]) })
			slices.SortFunc(tt.expectedApplied, func(first, second uuid.UUID) int { return slices.Compare(first[:], second[:]) })
			assert.Equal(t, tt.expectedApplied, fake.applied)
		})
	}
}

func TestReconcileDue(t *testing.T) {
	key := crudQuota.StateKey{OrganizationId: uuid.New(), ProjectId: uuid.New(), CodeAZ: "az-1"}
	fullBatch := slices.Repeat([]crudQuota.StateKey{key}, claimBatch)
	tests := []struct {
		name string
		// batches are the claims answered in order.
		batches        [][]crudQuota.StateKey
		claimErr       error
		expectedClaims int
		expectedPushes int
	}{
		{"nothing due", [][]crudQuota.StateKey{nil}, nil, 1, 0},
		{"a partial batch is the last one", [][]crudQuota.StateKey{{key}}, nil, 1, 1},
		{"full batches are followed by another claim", [][]crudQuota.StateKey{fullBatch, {key}}, nil, 2, claimBatch + 1},
		{"a failed claim stops the round", nil, assert.AnError, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler, fake := newFakeReconciler(uuid.Nil, "", nil, nil)
			claims := 0
			reconciler.Claim = func(context.Context, uuid.UUID, int) ([]crudQuota.StateKey, error) {
				claims++
				if tt.claimErr != nil {
					return nil, tt.claimErr
				}
				return tt.batches[claims-1], nil
			}

			reconciler.ReconcileDue(context.Background())
			assert.Equal(t, tt.expectedClaims, claims)
			assert.Len(t, fake.pushed, tt.expectedPushes)
		})
	}
}

func TestReconcilerRun(t *testing.T) {
	reconciler, _ := newFakeReconciler(uuid.Nil, "", nil, nil)
	reconciler.EnsureInterval = 10 * time.Millisecond
	ensured, claimed := make(chan struct{}, 10), make(chan struct{}, 1)
	reconciler.Ensure = func(context.Context) error {
		select {
		case ensured <- struct{}{}:
		default:
		}
		return assert.AnError
	}
	reconciler.Claim = func(context.Context, uuid.UUID, int) ([]crudQuota.StateKey, error) {
		select {
		case claimed <- struct{}{}:
		default:
		}
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		reconciler.Run(ctx)
		close(done)
	}()

	// Run calls Ensure at start, even when it fails, then claims the quotas due, then calls Ensure
	// again on its own timer.
	<-ensured
	<-claimed
	<-ensured
	cancel()
	<-done
}

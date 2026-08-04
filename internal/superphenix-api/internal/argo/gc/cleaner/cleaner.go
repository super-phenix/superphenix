// Package cleaner implements the mark-and-sweep primitives the Argo garbage
// collector runs over Applications and AppProjects.
package cleaner

import (
	"context"
	"fmt"
	"time"
)

// TimestampFormat is the layout of the deletion timestamp stored in the mark label.
const TimestampFormat = "2006-01-02T15-04-05Z0700"

// Options configures a Cleaner. Previously read from package globals.
type Options struct {
	// LabelMarkKey is the label carrying the deletion timestamp.
	LabelMarkKey string
	// Debug turns Clean into a dry run: candidates are logged, not deleted.
	Debug bool
	// AppProjectNamespace is the namespace AppProjects live in.
	AppProjectNamespace string
}

// Cleaner marks resources for deletion and sweeps the ones whose grace period expired.
type Cleaner interface {
	// Mark for deletion each resource inside the given namespace
	Mark(ctx context.Context, namespace string, timestamp string) error
	// Clean all marked resources inside the cluster
	Clean(ctx context.Context) error
	// ErrorMessage returns the message to display when Clean failed
	ErrorMessage() string
}

// Resource is the minimum a marked object must expose.
type Resource interface {
	GetName() string
	GetLabels() map[string]string
}

// ParseTimestamp reads the deletion timestamp from labelMarkKey and reports
// whether it is in the past.
func ParseTimestamp(r Resource, labelMarkKey string) (bool, error) {
	deletionTimestamp := r.GetLabels()[labelMarkKey]
	if deletionTimestamp == "" {
		return false, fmt.Errorf("no timestamp found")
	}

	parse, err := time.Parse(TimestampFormat, deletionTimestamp)
	if err != nil {
		return false, err
	}

	return parse.Before(time.Now()), nil
}

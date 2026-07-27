package utils

import (
	"context"
)

type Cleaner interface {
	// Mark for deletion each resources inside the given namespace
	Mark(ctx context.Context, namespace string, timestamp string) error
	// Clean all marked resources inside the cluster
	Clean(ctx context.Context) error
	// ErrorMessage return the message to display when the clean method failed
	ErrorMessage() string
}

type Resource interface {
	GetName() string
	GetLabels() map[string]string
}

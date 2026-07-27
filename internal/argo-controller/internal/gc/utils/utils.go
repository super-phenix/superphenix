package utils

import (
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
)

// ParseTimestamp fetch deletion timestamp in labels and check if it's before time.Now()
func ParseTimestamp(r Resource) (bool, error) {
	deletionTimestamp := r.GetLabels()[config.Global.GarbageCollection.LabelMarkKey]
	if deletionTimestamp != "" {
		parse, err := time.Parse(TimestampFormat, deletionTimestamp)
		if err != nil {
			return false, err
		}

		return parse.Before(time.Now()), nil
	}
	return false, fmt.Errorf("no timestamp found")
}

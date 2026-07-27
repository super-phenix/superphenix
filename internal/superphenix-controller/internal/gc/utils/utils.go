package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
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

func Transform[T, K interface{}](object T) (K, error) {
	var view K
	objectStr, err := json.Marshal(object)
	if err != nil {
		return view, fmt.Errorf("failed to convert object to string")
	}
	err = json.Unmarshal(objectStr, &view)
	if err != nil {
		return view, fmt.Errorf("failed to convert string to object")
	}
	return view, nil
}

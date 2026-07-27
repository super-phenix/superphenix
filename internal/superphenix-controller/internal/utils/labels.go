package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	MaxCustomLabelNumber = 10

	labelMaxLength = 253
	labelPattern   = "^([a-z0-9A-Z](?:[a-z0-9A-Z-._]*[a-z0-9A-Z]{1})?\\/)?([a-z0-9A-Z](?:[a-z0-9A-Z-._]{0,61}[a-z0-9A-Z]{1})?):([a-z0-9A-Z]{1}(?:[a-z0-9A-Z-._]{0,61}[a-z0-9A-Z]{1})?)?$"
)

var (
	labelPatternRegex = regexp.MustCompile(labelPattern)
)

// CheckProjectLabel returns a not-found-shaped error when the object's project
// label does not match the request's namespace. Resources are stamped at
// creation by spxId.Metadata.GetLabels(); a mismatch means the caller is
// reaching across tenancy boundaries.
func CheckProjectLabel(obj metav1.Object, namespace string) error {
	if obj.GetLabels()[spxId.SpxLabelProjectID] != namespace {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "resource"}, obj.GetName())
	}
	return nil
}

func IsEditAllowed(labels map[string]string) error {
	if labels[spxId.SpxLabelGitops] == "true" {
		return fmt.Errorf("cannot update/delete gitops resources")
	}

	if labels["superphenix.net/generated"] == "true" {
		return fmt.Errorf("cannot update/delete generated resources")
	}

	for key, value := range config.Global.DisableEditionForResourcesByLabels {
		if labels[key] == value {
			return fmt.Errorf("cannot update/delete this resource")
		}
	}

	return nil
}

func ParseLabel(label string) (string, string, error) {
	if len(label) > labelMaxLength {
		return "", "", fmt.Errorf("label too long")
	}
	if !labelPatternRegex.MatchString(label) {
		return "", "", fmt.Errorf("invalid label format")
	}

	res := strings.Split(label, ":")
	if len(res) != 2 {
		return "", "", fmt.Errorf("invalid label format")
	}

	return res[0], res[1], nil
}

func ParseLabels(labels []string, prefix string) (map[string]string, error) {
	labelMap := map[string]string{}
	for _, label := range labels {
		key, val, err := ParseLabel(label)
		if err != nil {
			return map[string]string{}, err
		}

		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return map[string]string{}, fmt.Errorf("%s has invalid label prefix", label)
		}
		labelMap[key] = val

	}
	return labelMap, nil
}

package utils

import (
	"fmt"
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Selector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

const (
	selectorValueMaxLength = 63
	selectorValuePattern   = "^[a-z0-9A-Z][a-z0-9A-Z-._]*[a-z0-9A-Z]?$"
	selectorKeyMaxLength   = 253 - selectorValueMaxLength
	selectorKeyPattern     = "^[a-z0-9A-Z][a-z0-9A-Z-\\/._]*[a-z0-9A-Z]?$"
)

var (
	selectorValueRegex = regexp.MustCompile(selectorValuePattern)
	selectorKeyRegex   = regexp.MustCompile(selectorKeyPattern)
)

func CheckSelector(key, value string) error {
	if len(key) > selectorKeyMaxLength {
		return fmt.Errorf("selector key too long")
	}
	if len(value) > selectorValueMaxLength {
		return fmt.Errorf("selector value too long")
	}
	if len(key) <= 0 {
		return fmt.Errorf("selector key is empty")
	}
	if !selectorKeyRegex.MatchString(key) {
		return fmt.Errorf("invalid selector key")
	}
	if value != "" && !selectorValueRegex.MatchString(value) {
		return fmt.Errorf("invalid selector value")
	}
	return nil
}

func CheckExpressions(key, operator string, values []string) error {
	if len(key) > selectorKeyMaxLength {
		return fmt.Errorf("selector key too long")
	}
	if len(key) <= 0 {
		return fmt.Errorf("selector key is empty")
	}
	if !selectorKeyRegex.MatchString(key) {
		return fmt.Errorf("invalid selector key")
	}

	if operator != string(metav1.LabelSelectorOpIn) &&
		operator != string(metav1.LabelSelectorOpNotIn) &&
		operator != string(metav1.LabelSelectorOpExists) &&
		operator != string(metav1.LabelSelectorOpDoesNotExist) {
		return fmt.Errorf("invalid operator")
	}

	for _, value := range values {
		if len(value) > selectorValueMaxLength {
			return fmt.Errorf("selector value too long")
		}
		if value != "" && !selectorValueRegex.MatchString(value) {
			return fmt.Errorf("invalid selector value")
		}
	}
	return nil
}

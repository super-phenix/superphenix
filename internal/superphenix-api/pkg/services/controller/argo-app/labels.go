package argoApp

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	labelMaxLength = 253
	labelPattern   = "^([a-z0-9A-Z](?:[a-z0-9A-Z-._]*[a-z0-9A-Z]{1})?\\/)?([a-z0-9A-Z](?:[a-z0-9A-Z-._]{0,61}[a-z0-9A-Z]{1})?):([a-z0-9A-Z]{1}(?:[a-z0-9A-Z-._]{0,61}[a-z0-9A-Z]{1})?)?$"
	labelSeparator = ":"
)

var (
	labelPatternRegex = regexp.MustCompile(labelPattern)
)

func ParseLabel(label string) (string, string, error) {
	if len(label) > labelMaxLength {
		return "", "", fmt.Errorf("label too long")
	}
	if !labelPatternRegex.MatchString(label) {
		return "", "", fmt.Errorf("invalid label format")
	}

	res := strings.Split(label, labelSeparator)
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

func MapLabelsToArray(labelMap map[string]string) []string {
	var result []string
	for k, v := range labelMap {
		result = append(result, fmt.Sprintf("%s%s%s", k, labelSeparator, v))
	}
	return result
}

package vm

import (
	"context"
	"fmt"
	"strings"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

const cloudInitNameFormat = "cloudinit-%s"
const cloudInitDataKey = "userdata"

func GetCloudInit(ctx context.Context, namespace, effectiveId string) (string, error) {
	log := logger.GetLogger(ctx)

	secretName := fmt.Sprintf(cloudInitNameFormat, effectiveId)
	cloudInit, err := config.K8sClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("method", "GetCloudInit").Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Error getting cloudInit")
		return "", err
	}

	valBytes := cloudInit.Data[cloudInitDataKey]
	// Make sure \n are real line break and not just standard string characters
	value := strings.ReplaceAll(string(valBytes), `\n`, "\n")

	if value == "" {
		log.Err(err).Str("method", "GetCloudInit").Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("No value found in secret")
		return "", fmt.Errorf("no value found in secret")
	}

	return value, nil
}

// CreateOrUpdateCloudInit creates or updates a cloud-init secret and returns its effective ID.
func CreateOrUpdateCloudInit(ctx context.Context, namespace, eid, userData string, labels map[string]string) (string, error) {
	name := fmt.Sprintf(cloudInitNameFormat, eid)

	secrets := config.K8sClient.CoreV1().Secrets(namespace)

	// Try to get existing secret
	existing, err := secrets.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		if existing.StringData == nil {
			existing.StringData = make(map[string]string)
		}
		existing.StringData[cloudInitDataKey] = userData
		_, err = secrets.Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return "", err
		}
		return name, nil
	}

	// Create new secret if not found
	sec := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      name,
			Namespace: namespace,
		},
		Type:       v1.SecretTypeOpaque,
		StringData: map[string]string{cloudInitDataKey: userData},
	}
	_, err = secrets.Create(ctx, sec, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return name, nil
}

package dbaas

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Credentials is the connection information of the application user.
type Credentials struct {
	Username  string               `json:"username"`
	Password  string               `json:"password"`
	DBName    string               `json:"dbname"`
	Endpoints []view.DBaaSEndpoint `json:"endpoints"`
}

// GetCredentials reads the application user Secret CloudNative-PG creates for the Cluster.
// The Secret carries no SPX labels, so ownership is checked on the Cluster first.
// Its host field is the in-cluster service, the endpoints are the load balancer addresses instead.
func GetCredentials(ctx context.Context, namespace, effectiveId string) (Credentials, error) {
	log := logger.GetLogger(ctx)

	cluster, err := getCluster(ctx, namespace, effectiveId)
	if err != nil {
		return Credentials{}, err
	}

	name := effectiveId + AppSecretSuffix
	secret, err := config.K8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get database Secret")
		return Credentials{}, err
	}

	return Credentials{
		Username:  string(secret.Data["username"]),
		Password:  string(secret.Data["password"]),
		DBName:    string(secret.Data["dbname"]),
		Endpoints: listEndpoints(ctx, namespace, cluster.GetLabels()[spxId.SpxLabelResourceLocalID]),
	}, nil
}

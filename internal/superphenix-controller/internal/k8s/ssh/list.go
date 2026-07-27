package ssh

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListSSHKey(ctx context.Context, namespace string) ([]view.SSHView, error) {
	log := logger.GetLogger(ctx)

	list, err := informers.WatcherSet[informers.SSH].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing ssh")
		return make([]view.SSHView, 0), err
	}

	sshList := make([]view.SSHView, 0)
	for _, item := range list {
		ssh := item.(*unstructured.Unstructured)
		sshView := view.UnstructuredSSHToView(ssh)
		if sshView.Data[sshDataKey] != nil {
			sshList = append(sshList, sshView)
		}
	}
	return sshList, nil
}

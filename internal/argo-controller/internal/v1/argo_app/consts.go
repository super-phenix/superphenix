package argoApp

import "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

var (
	DefaultSyncPolicy = v1alpha1.SyncPolicy{
		Automated: &v1alpha1.SyncPolicyAutomated{
			SelfHeal: true,
			Prune:    true,
		},
		SyncOptions: []string{
			"CreateNamespace=false",
			"ApplyOutOfSyncOnly=true",
			"RespectIgnoreDifferences=true",
		},
	}
)

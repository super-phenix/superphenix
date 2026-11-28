package dbaas

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetCredentials(t *testing.T) {
	tests := []struct {
		name         string
		noCluster    bool
		otherProject bool
		noSecret     bool
		wantErr      bool
		wantNotFound bool
	}{
		{
			name: "credentials and endpoints",
		},
		{
			name:         "cluster missing",
			noCluster:    true,
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:         "cluster labelled for another project",
			otherProject: true,
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:     "secret missing",
			noSecret: true,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := testNamespace(t)
			eid := testEffectiveId(t)

			var objs []runtime.Object
			if !tt.noCluster {
				projectId := testProjectId
				if tt.otherProject {
					projectId = testOtherProject
				}
				objs = append(objs, newCluster(t, projectId))
			}
			setFakeDynamicClient(t, objs...)
			setFakeWatcher(t, informers.SwitchLBRules, newLBRule(namespace, testLocalId, 5432, 5434))

			fakeClient := fake.NewClientset()
			if !tt.noSecret {
				_, _ = fakeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: eid + AppSecretSuffix, Namespace: namespace},
					Data: map[string][]byte{
						"username": []byte("app"),
						"password": []byte("secret"),
						"dbname":   []byte("app"),
						"host":     []byte(eid + "-rw"),
					},
				}, metav1.CreateOptions{})
			}
			oldClient := config.K8sClient
			config.K8sClient = fakeClient
			t.Cleanup(func() { config.K8sClient = oldClient })

			got, err := GetCredentials(context.Background(), namespace, eid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.wantNotFound && !apierrors.IsNotFound(err) {
					t.Errorf("GetCredentials() error = %v, want NotFound", err)
				}
				return
			}
			if got.Username != "app" || got.Password != "secret" || got.DBName != "app" {
				t.Errorf("GetCredentials() = %+v, want app/secret/app", got)
			}
			if len(got.Endpoints) != 2 || got.Endpoints[0].Host != testVIP {
				t.Errorf("Endpoints = %+v, want 2 endpoints on %s", got.Endpoints, testVIP)
			}
		})
	}
}

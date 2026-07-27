package pvc

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUpdatePVC_CustomLabels(t *testing.T) {
	const (
		namespace = "prj-test"
		name      = "disk-1"
		storage   = "10"
	)

	baseLabels := map[string]string{
		spxId.SpxLabelProjectID: namespace,
		"some-other-label":      "keep-me",
	}

	newPVC := func(labels map[string]string) *corev1.PersistentVolumeClaim {
		merged := make(map[string]string)
		for k, v := range baseLabels {
			merged[k] = v
		}
		for k, v := range labels {
			merged[k] = v
		}
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    merged,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		}
	}

	tests := []struct {
		name           string
		existingLabels map[string]string
		inputLabels    []string
		wantErr        bool
		wantLabels     map[string]string
	}{
		{
			name:           "add custom labels to PVC with none",
			existingLabels: map[string]string{},
			inputLabels:    []string{utils.CustomLabelPrefix + "env:prod"},
			wantErr:        false,
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:         namespace,
				"some-other-label":              "keep-me",
				utils.CustomLabelPrefix + "env": "prod",
			},
		},
		{
			name: "replace old custom labels with new ones",
			existingLabels: map[string]string{
				utils.CustomLabelPrefix + "old-label": "old-value",
			},
			inputLabels: []string{utils.CustomLabelPrefix + "new-label:new-value"},
			wantErr:     false,
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:               namespace,
				"some-other-label":                    "keep-me",
				utils.CustomLabelPrefix + "new-label": "new-value",
			},
		},
		{
			name: "remove all custom labels when input is empty",
			existingLabels: map[string]string{
				utils.CustomLabelPrefix + "env":  "prod",
				utils.CustomLabelPrefix + "team": "backend",
			},
			inputLabels: []string{},
			wantErr:     false,
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID: namespace,
				"some-other-label":      "keep-me",
			},
		},
		{
			name:           "non-custom labels are preserved",
			existingLabels: map[string]string{},
			inputLabels:    []string{utils.CustomLabelPrefix + "app:web"},
			wantErr:        false,
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:         namespace,
				"some-other-label":              "keep-me",
				utils.CustomLabelPrefix + "app": "web",
			},
		},
		{
			name:           "invalid label format returns error",
			existingLabels: map[string]string{},
			inputLabels:    []string{"!!!invalid!!!"},
			wantErr:        true,
		},
		{
			name:           "wrong prefix returns error",
			existingLabels: map[string]string{},
			inputLabels:    []string{"wrong.prefix/key:value"},
			wantErr:        true,
		},
		{
			name:           "too many custom labels returns error",
			existingLabels: map[string]string{},
			inputLabels: []string{
				utils.CustomLabelPrefix + "l1:v1",
				utils.CustomLabelPrefix + "l2:v2",
				utils.CustomLabelPrefix + "l3:v3",
				utils.CustomLabelPrefix + "l4:v4",
				utils.CustomLabelPrefix + "l5:v5",
				utils.CustomLabelPrefix + "l6:v6",
				utils.CustomLabelPrefix + "l7:v7",
				utils.CustomLabelPrefix + "l8:v8",
				utils.CustomLabelPrefix + "l9:v9",
				utils.CustomLabelPrefix + "l10:v10",
				utils.CustomLabelPrefix + "l11:v11",
			},
			wantErr: true,
		},
		{
			name: "multiple custom labels added correctly",
			existingLabels: map[string]string{
				utils.CustomLabelPrefix + "old-label": "willgo",
			},
			inputLabels: []string{
				utils.CustomLabelPrefix + "env:staging",
				utils.CustomLabelPrefix + "team:platform",
			},
			wantErr: false,
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:          namespace,
				"some-other-label":               "keep-me",
				utils.CustomLabelPrefix + "env":  "staging",
				utils.CustomLabelPrefix + "team": "platform",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc := newPVC(tt.existingLabels)
			fakeClient := fake.NewClientset(pvc)
			config.K8sClient = fakeClient

			info := &UpdateDiskInfo{}
			info.General.Storage = storage
			info.General.Labels = tt.inputLabels

			err := info.UpdatePVC(context.Background(), namespace, name)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected an error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}

			updated, err := fakeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get updated PVC: %v", err)
			}

			gotLabels := updated.GetLabels()
			if len(gotLabels) != len(tt.wantLabels) {
				t.Errorf("label count mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(gotLabels), len(tt.wantLabels), gotLabels, tt.wantLabels)
				return
			}
			for k, wantV := range tt.wantLabels {
				if gotV, ok := gotLabels[k]; !ok {
					t.Errorf("missing expected label %q", k)
				} else if gotV != wantV {
					t.Errorf("label %q = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

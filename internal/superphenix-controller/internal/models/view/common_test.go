package view

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestPVCToView_AutomaticConversion(t *testing.T) {
	// Backup and restore Global config
	originalMapping := config.Global.StorageClassMapping
	defer func() {
		config.Global.StorageClassMapping = originalMapping
	}()

	config.Global.StorageClassMapping = map[string]string{
		"SSD Storage": "fast",
	}

	fastSC := "fast"
	unknownSC := "unknown"

	tests := []struct {
		name string
		pvc  corev1.PersistentVolumeClaim
		want string
	}{
		{
			name: "existing storage class conversion",
			pvc: corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &fastSC,
				},
			},
			want: "SSD Storage",
		},
		{
			name: "unknown storage class conversion",
			pvc: corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &unknownSC,
				},
			},
			want: "undefined-storage-class",
		},
		{
			name: "nil storage class no conversion",
			pvc: corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: nil,
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := PVCToView(tt.pvc)
			got := ""
			if view.Spec.StorageClassName != nil {
				got = *view.Spec.StorageClassName
			}
			if got != tt.want {
				t.Errorf("PVCToView() storage class = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertStorageClassName(t *testing.T) {
	// Backup and restore Global config
	originalMapping := config.Global.StorageClassMapping
	defer func() {
		config.Global.StorageClassMapping = originalMapping
	}()

	config.Global.StorageClassMapping = map[string]string{
		"SSD Storage": "fast",
		"HDD Storage": "slow",
	}

	tests := []struct {
		name             string
		storageClassName string
		want             string
	}{
		{
			name:             "existing storage class",
			storageClassName: "fast",
			want:             "SSD Storage",
		},
		{
			name:             "another existing storage class",
			storageClassName: "slow",
			want:             "HDD Storage",
		},
		{
			name:             "undefined storage class",
			storageClassName: "unknown",
			want:             "undefined-storage-class",
		},
		{
			name:             "empty storage class",
			storageClassName: "",
			want:             "undefined-storage-class",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertStorageClassName(tt.storageClassName); got != tt.want {
				t.Errorf("convertStorageClassName() = %v, want %v", got, tt.want)
			}
		})
	}
}

package eip

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newEIP(name string, labels map[string]string) *v1.IptablesEIP {
	return &v1.IptablesEIP{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func newFIPRule(name string, labels map[string]string) *v1.IptablesFIPRule {
	return &v1.IptablesFIPRule{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func newSnatRule(name string, labels map[string]string) *v1.IptablesSnatRule {
	return &v1.IptablesSnatRule{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func newDnatRule(name string, labels map[string]string) *v1.IptablesDnatRule {
	return &v1.IptablesDnatRule{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name          string
		eips          []*v1.IptablesEIP
		dnats         []*v1.IptablesDnatRule
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked EIPs",
			eips:          []*v1.IptablesEIP{},
			dnats:         []*v1.IptablesDnatRule{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired EIP",
			eips: []*v1.IptablesEIP{
				newEIP("eip-expired", map[string]string{labelMarkKey: pastTimestamp}),
			},
			dnats:         []*v1.IptablesDnatRule{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future EIP",
			eips: []*v1.IptablesEIP{
				newEIP("eip-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			dnats:         []*v1.IptablesDnatRule{},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			eips: []*v1.IptablesEIP{
				newEIP("eip-debug", map[string]string{labelMarkKey: pastTimestamp}),
			},
			dnats:         []*v1.IptablesDnatRule{},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			eips: []*v1.IptablesEIP{
				newEIP("eip-expired", map[string]string{labelMarkKey: pastTimestamp}),
				newEIP("eip-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			dnats:         []*v1.IptablesDnatRule{},
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			fakeClient := testhelper.NewFakeKubeOvnClientset()
			for _, eip := range tt.eips {
				_, err := fakeClient.KubeovnV1().IptablesEIPs().Create(context.Background(), eip, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create EIP: %v", err)
				}
			}
			for _, dnat := range tt.dnats {
				_, err := fakeClient.KubeovnV1().IptablesDnatRules().Create(context.Background(), dnat, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create DNAT: %v", err)
				}
			}
			config.KubeOvnClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.eips))
			for i, eip := range tt.eips {
				watcherObjs[i] = eip
			}
			testhelper.SetupFakeWatcher(informers.EIP, watcherObjs...)

			dnatObjs := make([]interface{}, len(tt.dnats))
			for i, dnat := range tt.dnats {
				dnatObjs[i] = dnat
			}
			testhelper.SetupFakeWatcher(informers.DNAT, dnatObjs...)

			c := &Cleaner{
				ResourceType: "EIP",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeClient.KubeovnV1().IptablesEIPs().List(context.Background(), k8smetav1.ListOptions{})
			if len(remaining.Items) != tt.wantRemaining {
				t.Errorf("Clean() remaining = %d, want %d", len(remaining.Items), tt.wantRemaining)
			}
		})
	}
}

func TestMark(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	timestamp := time.Now().Add(48 * time.Hour).Format(utils.TimestampFormat)
	projectNs := "project-1"

	tests := []struct {
		name      string
		eips      []*v1.IptablesEIP
		fips      []*v1.IptablesFIPRule
		snats     []*v1.IptablesSnatRule
		dnats     []*v1.IptablesDnatRule
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching EIPs and related resources",
			eips: []*v1.IptablesEIP{
				newEIP("eip-1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			fips:      []*v1.IptablesFIPRule{},
			snats:     []*v1.IptablesSnatRule{},
			dnats:     []*v1.IptablesDnatRule{},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no EIPs in namespace",
			eips:      []*v1.IptablesEIP{},
			fips:      []*v1.IptablesFIPRule{},
			snats:     []*v1.IptablesSnatRule{},
			dnats:     []*v1.IptablesDnatRule{},
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "mark EIPs with related FIP, SNAT and DNAT resources",
			eips: []*v1.IptablesEIP{
				newEIP("eip-full", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			fips: []*v1.IptablesFIPRule{
				newFIPRule("fip-1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			snats: []*v1.IptablesSnatRule{
				newSnatRule("snat-1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			dnats: []*v1.IptablesDnatRule{
				newDnatRule("dnat-1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			fakeClient := testhelper.NewFakeKubeOvnClientset()
			for _, eip := range tt.eips {
				_, err := fakeClient.KubeovnV1().IptablesEIPs().Create(context.Background(), eip, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create EIP: %v", err)
				}
			}
			for _, fip := range tt.fips {
				_, err := fakeClient.KubeovnV1().IptablesFIPRules().Create(context.Background(), fip, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create FIP: %v", err)
				}
			}
			for _, snat := range tt.snats {
				_, err := fakeClient.KubeovnV1().IptablesSnatRules().Create(context.Background(), snat, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create SNAT: %v", err)
				}
			}
			for _, dnat := range tt.dnats {
				_, err := fakeClient.KubeovnV1().IptablesDnatRules().Create(context.Background(), dnat, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create DNAT: %v", err)
				}
			}
			config.KubeOvnClient = fakeClient

			eipObjs := make([]interface{}, len(tt.eips))
			for i, eip := range tt.eips {
				eipObjs[i] = eip
			}
			testhelper.SetupFakeWatcher(informers.EIP, eipObjs...)

			fipObjs := make([]interface{}, len(tt.fips))
			for i, fip := range tt.fips {
				fipObjs[i] = fip
			}
			testhelper.SetupFakeWatcher(informers.FIP, fipObjs...)

			snatObjs := make([]interface{}, len(tt.snats))
			for i, snat := range tt.snats {
				snatObjs[i] = snat
			}
			testhelper.SetupFakeWatcher(informers.SNAT, snatObjs...)

			dnatObjs := make([]interface{}, len(tt.dnats))
			for i, dnat := range tt.dnats {
				dnatObjs[i] = dnat
			}
			testhelper.SetupFakeWatcher(informers.DNAT, dnatObjs...)

			c := &Cleaner{
				ResourceType: "EIP",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				eip, _ := fakeClient.KubeovnV1().IptablesEIPs().Get(context.Background(), tt.eips[0].Name, k8smetav1.GetOptions{})
				if eip.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() EIP label = %v, want %v", eip.Labels[labelMarkKey], timestamp)
				}
			}

			for _, fip := range tt.fips {
				got, _ := fakeClient.KubeovnV1().IptablesFIPRules().Get(context.Background(), fip.Name, k8smetav1.GetOptions{})
				if got.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() FIP %s label = %v, want %v", fip.Name, got.Labels[labelMarkKey], timestamp)
				}
			}

			for _, snat := range tt.snats {
				got, _ := fakeClient.KubeovnV1().IptablesSnatRules().Get(context.Background(), snat.Name, k8smetav1.GetOptions{})
				if got.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() SNAT %s label = %v, want %v", snat.Name, got.Labels[labelMarkKey], timestamp)
				}
			}

			for _, dnat := range tt.dnats {
				got, _ := fakeClient.KubeovnV1().IptablesDnatRules().Get(context.Background(), dnat.Name, k8smetav1.GetOptions{})
				if got.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() DNAT %s label = %v, want %v", dnat.Name, got.Labels[labelMarkKey], timestamp)
				}
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		{
			name:         "returns correct error message",
			resourceType: "EIP",
			want:         "Something went wrong during EIP cleaning.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cleaner{ResourceType: tt.resourceType}
			if got := c.ErrorMessage(); got != tt.want {
				t.Errorf("ErrorMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

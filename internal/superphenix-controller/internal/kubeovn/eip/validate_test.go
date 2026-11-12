package eip

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/dnat"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ownProjectUUID = "11111111-1111-1111-1111-111111111111"
	ownProject     = "spx-" + ownProjectUUID
	otherProject   = "spx-22222222-2222-2222-2222-222222222222"
	ownSubnet      = "spx-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	otherSubnet    = "spx-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	sharedSubnet   = "spx-cccccccc-cccc-cccc-cccc-cccccccccccc"
	subnetCIDR     = "10.10.0.0/24,fd00:10:10::/64"
)

func subnetObj(name, projectID string, annotations map[string]string) *v1.Subnet {
	return &v1.Subnet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{spxId.SpxLabelProjectID: projectID},
			Annotations: annotations,
		},
		Spec: v1.SubnetSpec{CIDRBlock: subnetCIDR},
	}
}

func fakeSubnets(extra ...*v1.Subnet) {
	objs := []*v1.Subnet{
		subnetObj(ownSubnet, ownProject, nil),
		subnetObj(otherSubnet, otherProject, nil),
		subnetObj(sharedSubnet, otherProject, map[string]string{spxId.SpxAnnotationAllowedProjects: "spx-x, " + ownProject}),
	}
	objs = append(objs, extra...)
	fake := testhelper.NewFakeKubeOvnClientset()
	for _, o := range objs {
		if _, err := fake.KubeovnV1().Subnets().Create(context.Background(), o, metav1.CreateOptions{}); err != nil {
			panic(err)
		}
	}
	config.KubeOvnClient = fake
}

func TestGetAccessibleSubnet(t *testing.T) {
	fakeSubnets()

	tests := []struct {
		name    string
		subnet  string
		wantErr bool
	}{
		{name: "own subnet is accepted", subnet: ownSubnet},
		{name: "subnet shared with the project is accepted", subnet: sharedSubnet},
		{name: "subnet of another project is rejected", subnet: otherSubnet, wantErr: true},
		{name: "empty subnet is rejected", subnet: "", wantErr: true},
		{name: "unknown subnet is rejected", subnet: "spx-dddddddd-dddd-dddd-dddd-dddddddddddd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getAccessibleSubnet(context.Background(), tt.subnet, ownProject)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getAccessibleSubnet() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !apierrors.IsNotFound(err) {
				t.Errorf("expected a NotFound error, got %v", err)
			}
		})
	}
}

func TestValidateInternalTargets(t *testing.T) {
	tests := []struct {
		name       string
		internalIP string
		snat       []string
		dnat       []dnat.InfoDNAT
		wantErr    bool
	}{
		{name: "FIP inside subnet", internalIP: "10.10.0.5"},
		{name: "FIP with spaces inside subnet", internalIP: " 10.10.0.5 "},
		{name: "IPv6 FIP inside subnet", internalIP: "fd00:10:10::5"},
		{name: "SNAT on the whole subnet", snat: []string{"10.10.0.0/24"}},
		{name: "SNAT on a smaller range", snat: []string{"10.10.0.128/25"}},
		{name: "SNAT on a single IP", snat: []string{"10.10.0.7"}},
		{name: "DNAT inside subnet", snat: []string{"10.10.0.0/24"}, dnat: []dnat.InfoDNAT{{InternalIP: "10.10.0.9"}}},
		{name: "FIP to a load balancer VIP", internalIP: "198.18.0.1"},
		{name: "DNAT to a load balancer VIP", snat: []string{"10.10.0.0/24"}, dnat: []dnat.InfoDNAT{{InternalIP: "198.18.0.1"}}},

		{name: "FIP to another tenant IP", internalIP: "10.20.0.5", wantErr: true},
		{name: "FIP to a node or service IP", internalIP: "192.168.1.10", wantErr: true},
		{name: "FIP next to the load balancer range", internalIP: "198.19.0.1", wantErr: true},
		{name: "FIP not an IP", internalIP: "not-an-ip", wantErr: true},
		{name: "SNAT on a load balancer VIP", snat: []string{"198.18.0.1"}, wantErr: true},
		{name: "SNAT on the load balancer range", snat: []string{"198.18.0.0/16"}, wantErr: true},
		{name: "SNAT on everything", snat: []string{"0.0.0.0/0"}, wantErr: true},
		{name: "SNAT on a larger range", snat: []string{"10.10.0.0/16"}, wantErr: true},
		{name: "SNAT on another range", snat: []string{"10.20.0.0/24"}, wantErr: true},
		{name: "SNAT invalid CIDR", snat: []string{"10.10.0.0/99"}, wantErr: true},
		{name: "DNAT to another tenant IP", snat: []string{"10.10.0.0/24"}, dnat: []dnat.InfoDNAT{{InternalIP: "10.20.0.9"}}, wantErr: true},
		{name: "one bad DNAT among good ones", snat: []string{"10.10.0.0/24"}, dnat: []dnat.InfoDNAT{{InternalIP: "10.10.0.9"}, {InternalIP: "8.8.8.8"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInternalTargets(subnetCIDR, tt.internalIP, tt.snat, tt.dnat)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateInternalTargets() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !apierrors.IsBadRequest(err) {
				t.Errorf("expected a BadRequest error, got %v", err)
			}
		})
	}
}

func newCreateInfo(subnet, internalIP string) *CreateEIPInfo {
	info := &CreateEIPInfo{}
	info.OrgId = "33333333-3333-3333-3333-333333333333"
	info.ProjectId = ownProjectUUID
	info.ResourceLocalId = "44444444-4444-4444-4444-444444444444"
	info.General.SubnetEId = subnet
	info.Spec.InternalIP = internalIP
	return info
}

// TestCreateEip_Refused checks that no EIP or FIP is created when the subnet
// is not accessible or the internal IP is outside of it.
func TestCreateEip_Refused(t *testing.T) {
	tests := []struct {
		name         string
		subnet       string
		internalIP   string
		wantNotFound bool
	}{
		{name: "NAT gateway of another project", subnet: otherSubnet, internalIP: "10.10.0.5", wantNotFound: true},
		{name: "internal IP outside own subnet", subnet: ownSubnet, internalIP: "10.20.0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeSubnets()

			err := newCreateInfo(tt.subnet, tt.internalIP).CreateEip(context.Background())
			if tt.wantNotFound && !apierrors.IsNotFound(err) {
				t.Fatalf("expected a NotFound error, got %v", err)
			}
			if !tt.wantNotFound && !apierrors.IsBadRequest(err) {
				t.Fatalf("expected a BadRequest error, got %v", err)
			}

			eips, _ := config.KubeOvnClient.KubeovnV1().IptablesEIPs().List(context.Background(), metav1.ListOptions{})
			fips, _ := config.KubeOvnClient.KubeovnV1().IptablesFIPRules().List(context.Background(), metav1.ListOptions{})
			if len(eips.Items) != 0 || len(fips.Items) != 0 {
				t.Errorf("nothing must be created, found %d EIP and %d FIP", len(eips.Items), len(fips.Items))
			}
		})
	}
}

// TestUpdateEip_InternalIPOutsideSubnet checks that an existing EIP cannot be
// repointed to an address outside its subnet.
func TestUpdateEip_InternalIPOutsideSubnet(t *testing.T) {
	const eipName = "spx-eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	fakeSubnets()

	existing := &v1.IptablesEIP{
		ObjectMeta: metav1.ObjectMeta{Name: eipName, Labels: map[string]string{spxId.SpxLabelProjectID: ownProject}},
		Spec:       v1.IptablesEIPSpec{NatGwDp: ownSubnet},
	}
	if _, err := config.KubeOvnClient.KubeovnV1().IptablesEIPs().Create(context.Background(), existing, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	info := &UpdateEIPInfo{}
	info.OrgId = "33333333-3333-3333-3333-333333333333"
	info.ProjectId = ownProjectUUID
	info.ResourceLocalId = "55555555-5555-5555-5555-555555555555"
	info.Spec.InternalIP = "10.20.0.5"

	err := info.UpdateEip(context.Background(), ownProject, eipName)
	if !apierrors.IsBadRequest(err) {
		t.Fatalf("expected a BadRequest error, got %v", err)
	}

	fips, _ := config.KubeOvnClient.KubeovnV1().IptablesFIPRules().List(context.Background(), metav1.ListOptions{})
	if len(fips.Items) != 0 {
		t.Errorf("no FIP must be created, found %d", len(fips.Items))
	}
}

// TestCreateEip_ValidatesOnlyTheAppliedMode checks that only the targets of
// the applied mode are validated: SNAT/DNAT are ignored in FIP mode, but a bad
// DNAT is still refused in SNAT mode.
func TestCreateEip_ValidatesOnlyTheAppliedMode(t *testing.T) {
	badDNAT := struct {
		ExternalPort string `json:"externalPort"`
		InternalIP   string `json:"internalIP"`
		InternalPort string `json:"internalPort"`
		Protocol     string `json:"protocol"`
	}{ExternalPort: "22", InternalIP: "10.20.0.9", InternalPort: "22", Protocol: "tcp"}

	t.Run("FIP mode ignores out-of-subnet SNAT and DNAT", func(t *testing.T) {
		fakeSubnets()

		info := newCreateInfo(ownSubnet, "10.10.0.5")
		info.Spec.SNAT = []string{"0.0.0.0/0"}
		info.Spec.DNAT = append(info.Spec.DNAT, badDNAT)

		if err := info.CreateEip(context.Background()); err != nil {
			t.Fatalf("CreateEip() error = %v", err)
		}
		fips, _ := config.KubeOvnClient.KubeovnV1().IptablesFIPRules().List(context.Background(), metav1.ListOptions{})
		dnats, _ := config.KubeOvnClient.KubeovnV1().IptablesDnatRules().List(context.Background(), metav1.ListOptions{})
		if len(fips.Items) != 1 || len(dnats.Items) != 0 {
			t.Errorf("expected 1 FIP and no DNAT, found %d FIP and %d DNAT", len(fips.Items), len(dnats.Items))
		}
	})

	t.Run("SNAT mode refuses an out-of-subnet DNAT before creating anything", func(t *testing.T) {
		fakeSubnets()

		info := newCreateInfo(ownSubnet, "")
		info.Spec.SNAT = []string{"10.10.0.0/24"}
		info.Spec.DNAT = append(info.Spec.DNAT, badDNAT)

		if err := info.CreateEip(context.Background()); !apierrors.IsBadRequest(err) {
			t.Fatalf("expected a BadRequest error, got %v", err)
		}
		eips, _ := config.KubeOvnClient.KubeovnV1().IptablesEIPs().List(context.Background(), metav1.ListOptions{})
		if len(eips.Items) != 0 {
			t.Errorf("no EIP must be created, found %d", len(eips.Items))
		}
	})
}

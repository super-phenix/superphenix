package server

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	adminAz "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/admin/az"
	adminBilling "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/admin/billing"
	adminPermission "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/admin/permission"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/session"
	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"
	baasctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/baas"
	diskctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/disk"
	eipctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/eip"
	firewallctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/firewall"
	instancectrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/instance"
	kaasctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/kaas"
	loadbalancerctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/loadbalancer"
	metadatactrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/metadata"
	snapshotctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/snapshot"
	sshctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/ssh"
	subnetctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/subnet"
	vmsnapshotctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/vmsnapshot"
	vpcctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/vpc"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/membership"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/permission"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/user"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/manager"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/region/az"

	"github.com/rs/zerolog/log"
)

// RegisterFunc registers one service's routes on reg. Every service's ProvideService
// has exactly this signature, so it is assigned directly (no adapter).
type RegisterFunc func(cfg *config.Config, reg *router.Registry)

// Providers holds one RegisterFunc per service. To swap an implementation, start from
// DefaultProviders() and reassign the field for that service before calling
// InitializeServerWith/InitializeAdminServerWith — the per-service equivalent of a DI bind.
type Providers struct {
	// public
	Organization RegisterFunc
	Session      RegisterFunc
	APIToken     RegisterFunc
	AZ           RegisterFunc
	User         RegisterFunc
	Group        RegisterFunc
	IAM          RegisterFunc
	Permission   RegisterFunc
	Project      RegisterFunc
	ProjectMgr   RegisterFunc
	Instance     RegisterFunc
	VmSnapshot   RegisterFunc
	Disk         RegisterFunc
	Snapshot     RegisterFunc
	BaaS         RegisterFunc
	VPC          RegisterFunc
	Subnet       RegisterFunc
	Eip          RegisterFunc
	LoadBalancer RegisterFunc
	Firewall     RegisterFunc
	SSH          RegisterFunc
	KaaS         RegisterFunc
	Metadata     RegisterFunc
	Argo         RegisterFunc

	// admin
	AdminAZ         RegisterFunc
	AdminPermission RegisterFunc
	AdminBilling    RegisterFunc
}

// DefaultProviders returns the default service set.
func DefaultProviders() Providers {
	return Providers{
		Organization: organization.ProvideService,
		Session:      session.ProvideService,
		APIToken:     apiToken.ProvideService,
		AZ:           az.ProvideService,
		User:         user.ProvideService,
		Group:        group.ProvideService,
		IAM:          membership.ProvideService,
		Permission:   permission.ProvideService,
		Project:      project.ProvideService,
		ProjectMgr:   manager.ProvideService,
		Instance:     instancectrl.ProvideService,
		VmSnapshot:   vmsnapshotctrl.ProvideService,
		Disk:         diskctrl.ProvideService,
		Snapshot:     snapshotctrl.ProvideService,
		BaaS:         baasctrl.ProvideService,
		VPC:          vpcctrl.ProvideService,
		Subnet:       subnetctrl.ProvideService,
		Eip:          eipctrl.ProvideService,
		LoadBalancer: loadbalancerctrl.ProvideService,
		Firewall:     firewallctrl.ProvideService,
		SSH:          sshctrl.ProvideService,
		KaaS:         kaasctrl.ProvideService,
		Metadata:     metadatactrl.ProvideService,
		Argo:         argoApp.ProvideService,

		AdminAZ:         adminAz.ProvideService,
		AdminPermission: adminPermission.ProvideService,
		AdminBilling:    adminBilling.ProvideService,
	}
}

// registerPublic registers every public service on reg, in a fixed order. A nil
// field is skipped, so an edition can drop a service by zeroing its slot.
func (p Providers) registerPublic(cfg *config.Config, reg *router.Registry) {
	for _, register := range []RegisterFunc{
		p.Organization, p.Session, p.APIToken, p.AZ, p.User, p.Group, p.IAM,
		p.Permission, p.Project, p.ProjectMgr,
		p.Instance, p.VmSnapshot, p.Disk, p.Snapshot, p.BaaS, p.VPC, p.Subnet,
		p.Eip, p.LoadBalancer, p.Firewall, p.SSH, p.KaaS, p.Metadata, p.Argo,
	} {
		if register == nil {
			log.Debug().Msg("server: skipping nil public provider")
			continue
		}
		register(cfg, reg)
	}
}

// registerAdmin registers every admin service on reg, in a fixed order. A nil
// field is skipped, so an edition can drop a service by zeroing its slot.
func (p Providers) registerAdmin(cfg *config.Config, reg *router.Registry) {
	for _, register := range []RegisterFunc{
		p.AdminAZ, p.AdminPermission, p.AdminBilling,
	} {
		if register == nil {
			log.Debug().Msg("server: skipping nil admin provider")
			continue
		}
		register(cfg, reg)
	}
}

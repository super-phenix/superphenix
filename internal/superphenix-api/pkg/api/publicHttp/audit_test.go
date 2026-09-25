package publicHttp

import (
	"net/http"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/session"
	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"
	baasctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/baas"
	bucketctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/bucket"
	diskctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/disk"
	eipctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/eip"
	instancectrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/instance"
	kaasctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/kaas"
	loadbalancerctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/loadbalancer"
	metadatactrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/metadata"
	securitygroupctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/securitygroup"
	snapshotctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/snapshot"
	sshctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/ssh"
	subnetctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/subnet"
	summaryctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/summary"
	vmsnapshotctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/vmsnapshot"
	vpcctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/vpc"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/auditlog"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/membership"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/permission"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/user"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/manager"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/region/az"

	"github.com/stretchr/testify/assert"
)

// declaredRegistry registers every public module without the infrastructure the providers
// connect to. A new service has to be added here, as in wirePublicRoutes.
func declaredRegistry() *router.Registry {
	cfg := &config.Global
	reg := router.New()

	for _, module := range []router.Module{
		organization.Module(organization.New(cfg)),
		session.Module(session.New(cfg)),
		apiToken.Module(apiToken.New(cfg)),
		az.Module(az.New(cfg)),
		user.Module(user.New(cfg)),
		group.Module(group.New(cfg)),
		membership.Module(membership.New(cfg)),
		permission.Module(permission.New(cfg)),
		project.Module(project.New(cfg)),
		manager.Module(cfg, manager.New(cfg)),
		auditlog.Module(auditlog.New(cfg)),

		instancectrl.Module(cfg, instancectrl.New(cfg)),
		vmsnapshotctrl.Module(cfg, vmsnapshotctrl.New(cfg)),
		diskctrl.Module(cfg, diskctrl.New(cfg)),
		bucketctrl.Module(cfg, bucketctrl.New(cfg)),
		snapshotctrl.Module(cfg, snapshotctrl.New(cfg)),
		baasctrl.Module(cfg, baasctrl.New(cfg, nil)),
		vpcctrl.Module(cfg, vpcctrl.New(cfg)),
		subnetctrl.Module(cfg, subnetctrl.New(cfg)),
		eipctrl.Module(cfg, eipctrl.New(cfg)),
		loadbalancerctrl.Module(cfg, loadbalancerctrl.New(cfg)),
		securitygroupctrl.Module(cfg, securitygroupctrl.New(cfg)),
		sshctrl.Module(cfg, sshctrl.New(cfg)),
		kaasctrl.Module(cfg, kaasctrl.New(cfg, nil)),
		metadatactrl.Module(cfg, metadatactrl.New(cfg)),
		summaryctrl.Module(cfg, summaryctrl.New(cfg)),
		argoApp.Module(cfg, argoApp.New(cfg)),
	} {
		reg.Register(module)
	}

	return reg
}

// mutatingGets are the GET routes that change state and must be audited like any write.
var mutatingGets = []string{
	"/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/start",
	"/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/stop",
	"/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/stop-force",
	"/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/restart",
	"/{orgaId}/api/spx-ctrl/{az}/{projectId}/disk/{effectiveId}/unmount",
	"/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance-snapshot/{effectiveId}/restore",
	"/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId}/reinstall-essentials",
	"/v1/session/token",
}

// TestEveryWriteRouteDeclaresAudit fails when a route that changes state is added without
// saying how it is audited. Use Audited, or NotAudited with a reason for a read behind a POST.
func TestEveryWriteRouteDeclaresAudit(t *testing.T) {
	declared := map[string]*router.Audit{}
	for _, info := range declaredRegistry().Declared() {
		declared[info.Method+" "+info.Pattern] = info.Audit
	}

	type routeCase struct {
		name  string
		key   string
		audit *router.Audit
		found bool
	}

	tests := make([]routeCase, 0, len(declared))
	for key, audit := range declared {
		if key[:len(http.MethodGet)+1] != http.MethodGet+" " {
			tests = append(tests, routeCase{name: key, key: key, audit: audit, found: true})
		}
	}
	for _, pattern := range mutatingGets {
		key := http.MethodGet + " " + pattern
		audit, found := declared[key]
		tests = append(tests, routeCase{name: key, key: key, audit: audit, found: found})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !assert.True(t, tt.found, "route is not declared anymore, update mutatingGets") {
				return
			}
			if !assert.NotNil(t, tt.audit, "declare the route with Audited or NotAudited") {
				return
			}
			if tt.audit.Skip {
				assert.NotEmpty(t, tt.audit.SkipReason, "NotAudited needs a reason")
				return
			}
			assert.NotEmpty(t, tt.audit.Resource.Name)
			assert.NotEmpty(t, tt.audit.Resource.Label, "Resource needs a Label")
			assert.NotEmpty(t, tt.audit.Action)
		})
	}
}

package bucket

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-bucket"

// API is the overridable seam for the bucket endpoints; the methods are
// the HTTP handlers (named handlers and SimpleRedirect passthroughs).
type API interface {
	ListBuckets(http.ResponseWriter, *http.Request)
	ListAZBuckets(http.ResponseWriter, *http.Request)
	GetBucket(http.ResponseWriter, *http.Request)
	GetS3Config(http.ResponseWriter, *http.Request)
	CreateBucket(http.ResponseWriter, *http.Request)
	UpdateBucket(http.ResponseWriter, *http.Request)
	DeleteBucket(http.ResponseWriter, *http.Request)
	GetBucketCredentials(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the bucket routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		bucketRead        = controller.Perm(pwPermission.ProjectBucketRead)
		bucketWrite       = controller.Perm(pwPermission.ProjectBucketWrite)
		bucketCredentials = controller.Perm(pwPermission.ProjectBucketCredentials)
		quota             = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(cfg, moduleName,
		nil,
		router.Group{
			Middlewares: []router.Middleware{bucketRead},
			Routes: []router.Route{
				router.Get("/{projectId}/bucket", s.ListBuckets),
				router.Get("/{az}/{projectId}/bucket", s.ListAZBuckets),
				router.Get("/{az}/{projectId}/bucket/{effectiveId}", s.GetBucket),
				router.Get("/{az}/{projectId}/s3-config", s.GetS3Config),
				router.Get("/{az}/{projectId}/bucket/{effectiveId}/credentials", s.GetBucketCredentials, bucketCredentials),
			},
			Groups: []router.Group{{
				Middlewares: []router.Middleware{bucketWrite},
				Routes: []router.Route{
					router.Post("/{az}/{projectId}/bucket", s.CreateBucket, quota),
					router.Post("/{az}/{projectId}/bucket/{effectiveId}", s.UpdateBucket),
					router.Delete("/{az}/{projectId}/bucket/{effectiveId}", s.DeleteBucket),
				},
			}},
		},
	)
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}

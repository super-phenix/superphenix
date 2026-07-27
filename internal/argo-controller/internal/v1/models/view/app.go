package view

import (
	"encoding/json"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/rs/zerolog/log"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Resource struct {
	ID             string `json:"id"`           // local ID
	EId            string `json:"eid"`          // effective ID
	ResourceName   string `json:"resourceName"` // human-readable name
	CodeAZ         string `json:"codeAZ"`
	ResourceTypeId string `json:"resourceTypeId"`
	Gitops         string `json:"gitops"`
}

type Application struct {
	Resource `json:",inline"`
	App      AppView `json:"app"`
}
type AppView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`

	Spec      ApplicationSpec   `json:"spec"`
	Status    ApplicationStatus `json:"status,omitempty"`
	Operation *v1.Operation     `json:"operation,omitempty"`
}

type ApplicationSpec struct {
	// Source is a reference to the location of the application's manifests or chart
	Source *ApplicationSource `json:"source,omitempty" protobuf:"bytes,1,opt,name=source"`
}

// ApplicationSource contains all required information about the source of an application
type ApplicationSource struct {
	// RepoURL is the URL to the repository (Git or Helm) that contains the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the Git repository, and is only valid for applications sourced from Git.
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
	// TargetRevision defines the revision of the source to sync the application to.
	// In case of Git, this can be commit, tag, or branch. If omitted, will equal to HEAD.
	// In case of Helm, this is a semver tag for the Chart's version.
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
	// Helm holds helm specific options
	Helm *v1.ApplicationSourceHelm `json:"helm,omitempty" protobuf:"bytes,7,opt,name=helm"`
	// Chart is a Helm chart name, and must be specified for applications sourced from a Helm repo.
	Chart string `json:"chart,omitempty" protobuf:"bytes,12,opt,name=chart"`
	// Ref is reference to another source within sources field. This field will not be used if used with a `source` tag.
	Ref string `json:"ref,omitempty" protobuf:"bytes,13,opt,name=ref"`
	// Name is used to refer to a source and is displayed in the UI. It is used in multi-source Applications.
	Name string `json:"name,omitempty" protobuf:"bytes,14,opt,name=name"`
	// Plugin holds config management plugin specific options
	Plugin *v1.ApplicationSourcePlugin `json:"plugin,omitempty" protobuf:"bytes,15,opt,name=plugin"`
}

// ApplicationStatus contains status information for the application
type ApplicationStatus struct {
	// Resources is a list of Kubernetes resources managed by this application
	Resources []v1.ResourceStatus `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Sync contains information about the application's current sync status
	Sync v1.SyncStatus `json:"sync,omitempty" protobuf:"bytes,2,opt,name=sync"`
	// Health contains information about the application's current health status
	Health v1.AppHealthStatus `json:"health,omitempty" protobuf:"bytes,3,opt,name=health"`

	// Conditions is a list of currently observed application conditions
	Conditions []v1.ApplicationCondition `json:"conditions,omitempty" protobuf:"bytes,5,opt,name=conditions"`
}

func AppToView(app v1.Application) AppView {
	app.Labels = filterLabels(app.GetLabels())

	vmiStr, err := json.Marshal(app)
	if err != nil {
		log.Error().AnErr("error converting vm to view", err).Any("vm", app).Send()
	}
	var view AppView
	err = json.Unmarshal(vmiStr, &view)
	if err != nil {
		log.Error().AnErr("error converting vm to view", err).Any("vm", app).Send()
	}
	return view
}

func AppsToView(apps []v1.Application) []AppView {
	views := make([]AppView, 0)
	for _, vpc := range apps {
		view := AppToView(vpc)
		if view.APIVersion != "" {
			views = append(views, view)
		}
	}
	return views
}

func AppToResource(appView AppView) Application {
	return Application{
		Resource: Resource{
			ID:           appView.Labels[spxId.SpxLabelResourceLocalID],
			EId:          appView.Name,
			ResourceName: appView.Labels[spxId.SpxLabelResourceName],
			Gitops:       appView.Labels[spxId.SpxLabelGitops],
		},
		App: appView,
	}
}

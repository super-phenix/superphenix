package argoApp

import spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

// AppArgoCtrlBody is the body send to Argo Ctrl to create or update an App
type AppArgoCtrlBody struct {
	spxId.Metadata
	General struct {
		AppName     string `json:"appName"`
		Destination string `json:"destination"`
	} `json:"general"`
	Spec struct {
		Source            AppSource         `json:"source"`
		IgnoreDifferences IgnoreDifferences `json:"ignoreDifferences"`
	} `json:"spec"`
}

type AppSource struct {
	RepoURL        string                  `json:"repoUrl"`
	TargetRevision string                  `json:"targetRevision"`
	Chart          string                  `json:"chart,omitempty"`
	Path           string                  `json:"path,omitempty"`
	Helm           ApplicationSourceHelm   `json:"helm,omitempty"`
	Plugin         ApplicationSourcePlugin `json:"plugin,omitempty"`
}

type IgnoreDifferences []ResourceIgnoreDifferences

// ResourceIgnoreDifferences contains resource filter and list of json paths which should be ignored during comparison with live state.
type ResourceIgnoreDifferences struct {
	Group             string   `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind              string   `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Name              string   `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
	Namespace         string   `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	JSONPointers      []string `json:"jsonPointers,omitempty" protobuf:"bytes,5,opt,name=jsonPointers"`
	JQPathExpressions []string `json:"jqPathExpressions,omitempty" protobuf:"bytes,6,opt,name=jqPathExpressions"`
	// ManagedFieldsManagers is a list of trusted managers. Fields mutated by those managers will take precedence over the
	// desired state defined in the SCM and won't be displayed in diffs
	ManagedFieldsManagers []string `json:"managedFieldsManagers,omitempty" protobuf:"bytes,7,opt,name=managedFieldsManagers"`
}

// ApplicationSourcePlugin holds options specific to config management plugins
type ApplicationSourcePlugin struct {
	Name string `json:"name,omitempty"`
	Env  `json:"env,omitempty"`
}

// Env is a list of environment variable entries
type Env []*EnvEntry

// EnvEntry represents an entry in the application's environment
type EnvEntry struct {
	// Name is the name of the variable, usually expressed in uppercase
	Name string `json:"name"`
	// Value is the value of the variable
	Value string `json:"value"`
}

// ApplicationSourceHelm holds helm specific options
type ApplicationSourceHelm struct {
	Parameters []HelmParameter `json:"parameters,omitempty"`
	Values     string          `json:"values,omitempty"`
}

// HelmParameter is a parameter that's passed to helm template during manifest generation
type HelmParameter struct {
	// Name is the name of the Helm parameter
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Value is the value for the Helm parameter
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// ForceString determines whether to tell Helm to interpret booleans and numbers as strings
	ForceString bool `json:"forceString,omitempty" protobuf:"bytes,3,opt,name=forceString"`
}

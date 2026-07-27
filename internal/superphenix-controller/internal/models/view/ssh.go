package view

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type SSHView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`

	// Immutable, if set to true, ensures that data stored in the Secret cannot
	// be updated (only object metadata can be modified).
	// If not set to true, the field can be modified at any time.
	// Defaulted to nil.
	// +optional
	Immutable *bool `json:"immutable,omitempty" protobuf:"varint,5,opt,name=immutable"`

	// Data contains the secret data. Each key must consist of alphanumeric
	// characters, '-', '_' or '.'. The serialized form of the secret data is a
	// base64 encoded string, representing the arbitrary (possibly non-string)
	// data value here. Described in https://tools.ietf.org/html/rfc4648#section-4
	// +optional
	Data map[string][]byte `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`

	// stringData allows specifying non-binary secret data in string form.
	// It is provided as a write-only input field for convenience.
	// All keys and values are merged into the data field on write, overwriting any existing values.
	// The stringData field is never output when reading from the API.
	// +k8s:conversion-gen=false
	// +optional
	StringData map[string]string `json:"stringData,omitempty" protobuf:"bytes,4,rep,name=stringData"`

	// Used to facilitate programmatic handling of secret data.
	// More info: https://kubernetes.io/docs/concepts/configuration/secret/#secret-types
	// +optional
	Type SecretType `json:"type,omitempty" protobuf:"bytes,3,opt,name=type,casttype=SecretType"`
}

type SecretType string

func UnstructuredSSHToView(ssh *unstructured.Unstructured) SSHView {
	var view SSHView
	err := utils.UnstructuredToStruct(ssh, &view)
	if err != nil {
		log.Error().AnErr("error converting ssh to view", err).Any("ssh", ssh).Send()
		return SSHView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	return view
}

func SSHToView(ssh v1.Secret) SSHView {
	ssh.Labels = utils.FilterLabels(ssh.GetLabels())
	ssh.Annotations = utils.FilterAnnotations(ssh.GetAnnotations())

	sshStr, err := json.Marshal(ssh)
	if err != nil {
		log.Error().AnErr("error converting ssh to view", err).Any("ssh", ssh).Send()
	}
	var view SSHView
	err = json.Unmarshal(sshStr, &view)
	if err != nil {
		log.Error().AnErr("error converting ssh to view", err).Any("ssh", ssh).Send()
	}
	return view
}

func SSHViewToResource(sshView SSHView) SSH {
	return SSH{
		Resource: Resource{
			ID:          sshView.Labels[spxId.SpxLabelResourceLocalID],
			EId:         sshView.Name,
			ProductName: sshView.Labels[spxId.SpxLabelResourceName],
			Gitops:      sshView.Labels[spxId.SpxLabelGitops],
		},
		SSH: sshView,
	}
}

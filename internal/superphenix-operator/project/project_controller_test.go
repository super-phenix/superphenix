package project

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

func TestBuildHelmValuesIgnoresLegacyValuesPath(t *testing.T) {
	var proj operatorv1alpha1.Project
	if err := json.Unmarshal([]byte(`{
		"spec": {
			"projectID": "project-id",
			"gitops": {
				"valuesLocation": {
					"repoURL": "https://example.com/values.git",
					"path": "./untrusted",
					"targetRevision": "production",
					"credentials": {
						"username": "git-user",
						"password": "git-password"
					}
				}
			}
		}
	}`), &proj); err != nil {
		t.Fatalf("unmarshal project: %v", err)
	}

	valuesYAML, err := (&Reconciler{}).buildHelmValues(&proj, &operatorv1alpha1.Organization{}, "organization", "project")
	if err != nil {
		t.Fatalf("build Helm values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYAML), &values); err != nil {
		t.Fatalf("unmarshal Helm values: %v", err)
	}
	valuesLocation := values["project"].(map[string]interface{})["gitops"].(map[string]interface{})["valuesLocation"].(map[string]interface{})
	if _, found := valuesLocation["path"]; found {
		t.Fatalf("values repository path must not be forwarded to Helm: %s", valuesYAML)
	}
	if got := valuesLocation["repoURL"]; got != "https://example.com/values.git" {
		t.Errorf("repoURL = %v, want values repository URL", got)
	}
	if got := valuesLocation["targetRevision"]; got != "production" {
		t.Errorf("targetRevision = %v, want production", got)
	}
	credentials := valuesLocation["credentials"].(map[string]interface{})
	if got := credentials["username"]; got != "git-user" {
		t.Errorf("credentials.username = %v, want git-user", got)
	}
	if got := credentials["password"]; got != "git-password" {
		t.Errorf("credentials.password = %v, want git-password", got)
	}
}

func TestSfsGitopsApplicationsDoNotSetValuesRepositoryPath(t *testing.T) {
	chart, err := loader.Load("../../../components/dependencies/sfs-gitops")
	if err != nil {
		t.Fatalf("load sfs-gitops chart: %v", err)
	}

	values := chartutil.Values{
		"global": map[string]interface{}{
			"organization": map[string]interface{}{
				"organizationID": "organization-id",
				"name":           "organization",
			},
			"project": map[string]interface{}{
				"projectID": "project-id",
				"name":      "project",
				"availabilityZones": []interface{}{map[string]interface{}{
					"name":    "fr01-agc01",
					"cluster": "cluster-1",
				}},
				"gitops": map[string]interface{}{
					"valuesLocation": map[string]interface{}{
						"repoURL":        "https://example.com/values.git",
						"path":           "./untrusted",
						"targetRevision": "production",
					},
				},
			},
		},
		"kubernetes": map[string]interface{}{
			"cluster-1": map[string]interface{}{
				"location":    "fr01-agc01",
				"name":        "cluster-1",
				"kubeVersion": "1.35.0",
			},
		},
		"backups": map[string]interface{}{
			"backup-1": map[string]interface{}{
				"location": "fr01-agc01",
				"name":     "backup-1",
			},
		},
	}
	renderValues, err := chartutil.ToRenderValues(chart, values, chartutil.ReleaseOptions{
		Name:      "values-path-test",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}, chartutil.DefaultCapabilities)
	if err != nil {
		t.Fatalf("prepare chart values: %v", err)
	}

	renderedTemplates, err := (engine.Engine{}).Render(chart, renderValues)
	if err != nil {
		t.Fatalf("render sfs-gitops chart: %v", err)
	}

	for _, templatePath := range []string{
		"sfs-gitops/templates/kaas/application.yaml",
		"sfs-gitops/templates/baas/application.yaml",
	} {
		template, found := renderedTemplates[templatePath]
		if !found {
			t.Fatalf("rendered chart is missing %s", templatePath)
		}
		assertValuesSourceHasNoPath(t, template)
	}
}

func assertValuesSourceHasNoPath(t *testing.T, rendered string) {
	t.Helper()

	decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(rendered), 4096)
	applicationCount := 0
	for {
		application := map[string]interface{}{}
		err := decoder.Decode(&application)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode rendered Application: %v", err)
		}
		if application["kind"] != "Application" {
			continue
		}
		applicationCount++

		sources, found, err := unstructured.NestedSlice(application, "spec", "sources")
		if err != nil || !found || len(sources) != 2 {
			t.Fatalf("Application sources = %v, found = %t, error = %v; want two sources", sources, found, err)
		}
		valuesSource, ok := sources[1].(map[string]interface{})
		if !ok {
			t.Fatalf("values source has unexpected type %T", sources[1])
		}
		if _, found := valuesSource["path"]; found {
			t.Errorf("values source must not set path: %#v", valuesSource)
		}
		if got := valuesSource["ref"]; got != "values" {
			t.Errorf("values source ref = %v, want values", got)
		}
		if got := valuesSource["repoURL"]; got != "https://example.com/values.git" {
			t.Errorf("values source repoURL = %v, want values repository URL", got)
		}
		if got := valuesSource["targetRevision"]; got != "production" {
			t.Errorf("values source targetRevision = %v, want production", got)
		}
	}
	if applicationCount != 1 {
		t.Errorf("rendered %d Applications, want exactly one", applicationCount)
	}
}

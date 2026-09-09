package kaas

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var nodeGroupNameRegex = regexp.MustCompile("^[a-zA-Z0-9-]*$")

// azDomainValues converts the configured per-AZ URLs into the shape the
// sfs-kaas chart expects. A nil or empty configuration yields nil so the
// azDomains key stays out of the rendered values.
func azDomainValues(cfg map[string]config.AzDomainConfig) map[string]AzDomain {
	if len(cfg) == 0 {
		return nil
	}
	out := make(map[string]AzDomain, len(cfg))
	for az, d := range cfg {
		out[az] = AzDomain{Internal: d.Internal, External: d.External}
	}
	return out
}

// CreateKaaSAppValues generates the Helm values YAML for a KaaS application.
func CreateKaaSAppValues(ctx context.Context, localId, location string, spec KaaSSpec, kaasConfig KaaSConfig, oldSpec *KaaSSpec) (string, []string, error) {
	log := logger.GetLogger(ctx)

	//Validation part
	if len(spec.Groups) > maxnodeGroup {
		log.Error().Any("spec", spec).Msgf("Too many node groups, limit %d found %d", maxnodeGroup, len(spec.Groups))
		return "", nil, fmt.Errorf("too many node groups, limit %d found %d", maxnodeGroup, len(spec.Groups))
	}

	if !slices.Contains(CpNetworkPolicies, spec.CPNetPol) {
		log.Error().Any("spec", spec).Msgf("control plane network policy is not a valid value : %s", spec.CPNetPol)
		return "", nil, fmt.Errorf("control plane network policy is not a valid value : %s", spec.CPNetPol)

	}

	if !slices.Contains(WorkersNetworkPolicies, spec.WorkersNetPol) {
		log.Error().Any("spec", spec).Msgf("workers network policy is not a valid value : %s", spec.WorkersNetPol)
		return "", nil, fmt.Errorf("workers network policy is not a valid value : %s", spec.WorkersNetPol)
	}

	mapGroupToRemove := make(map[int]string)
	if oldSpec != nil {
		for i, group := range oldSpec.Groups {
			mapGroupToRemove[i] = group.Name
		}
	}

	instances := make(map[string]Instance)
	for _, group := range spec.Groups {
		if !nodeGroupNameRegex.MatchString(group.Name) {
			log.Error().Any("spec", spec).Msgf("The name '%s' does not meet the criteria : %s", group.Name, nodeGroupNameRegex.String())
			return "", nil, fmt.Errorf("the name '%s' does not meet the criteria : %s", group.Name, nodeGroupNameRegex.String())
		}

		if group.Replicas < minReplicas || group.Replicas > maxReplicas {
			log.Error().Any("spec", spec).Msgf("Replicas count value (%d) must be between %d and %d for group '%s'", group.Replicas, minReplicas, maxReplicas, group.Name)
			return "", nil, fmt.Errorf("replicas count value (%d) must be between %d and %d for group '%s'", group.Replicas, minReplicas, maxReplicas, group.Name)
		}

		// Groups have a version starting at 1. When updating, each group is matched by name
		// against oldSpec: if unchanged the version is kept, otherwise it is incremented.
		// Unmatched groups in oldSpec are tracked in mapGroupToRemove for cleanup.
		group.Version = 1
		if oldSpec != nil {
			for i, oldGroup := range oldSpec.Groups {
				if oldGroup.Name == group.Name {
					delete(mapGroupToRemove, i)
					if isSameGroup(group, oldGroup) {
						group.Version = oldGroup.Version
					} else {
						group.Version = oldGroup.Version + 1
					}
					break
				}
			}
		}

		var interfaces []Interface
		subnets := group.Subnets
		sort.Slice(subnets, func(i, j int) bool {
			return subnets[i].Order < subnets[j].Order
		})
		for _, subnet := range subnets {
			interfaces = append(interfaces, Interface{Subnet: subnet.Id})
		}

		err := parseNodeGroup(instances, group, interfaces)
		if err != nil {
			log.Error().Err(err).
				Str("localId", localId).
				Any("spec", spec).
				Any("group", group).
				Msg("Failed to parse node group")
			return "", nil, err
		}
	}

	if _, supported := config.ResolveKubeVersionRepo(config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions, config.Global.ProductsConfig.ArgoApp.Kubernetes.Repo, spec.KubeVersion); !supported {
		log.Error().Str("kubeVersion", spec.KubeVersion).Msg("KubeVersion not supported")
		return "", nil, fmt.Errorf("KubeVersion not supported")
	}

	//// KaaS Essentials
	storageClasses := make(map[string]StorageClass)
	for i, class := range kaasConfig.StorageClasses {
		storageClasses[class.Shortname] = StorageClass{
			IsDefaultClass: i == 0,
			TenantClass:    class.Shortname,
			InfraClass:     class.Shortname,
		}
	}

	revision := 1
	essentialsValues := EssentialsValues{}

	if spec.KaasEssentials.CorednsValues != "" ||
		spec.KaasEssentials.CiliumValues != "" ||
		spec.KaasEssentials.MetricsServerValues != "" {

		if oldSpec != nil &&
			(spec.KaasEssentials.CorednsValues != oldSpec.KaasEssentials.CorednsValues ||
				spec.KaasEssentials.CiliumValues != oldSpec.KaasEssentials.CiliumValues ||
				spec.KaasEssentials.MetricsServerValues != oldSpec.KaasEssentials.MetricsServerValues) {
			revision = oldSpec.KaasEssentials.Revision + 1
		}

		var corednsInterface interface{}
		if spec.KaasEssentials.CorednsValues != "" {
			if err := yaml.Unmarshal([]byte(spec.KaasEssentials.CorednsValues), &corednsInterface); err != nil {
				log.Error().Err(err).Msg("Failed to unmarshal coredns values")
				return "", nil, fmt.Errorf("failed to unmarshal coredns values: %w", err)
			}
		}

		var ciliumInterface interface{}
		if spec.KaasEssentials.CiliumValues != "" {
			if err := yaml.Unmarshal([]byte(spec.KaasEssentials.CiliumValues), &ciliumInterface); err != nil {
				log.Error().Err(err).Msg("Failed to unmarshal cilium values")
				return "", nil, fmt.Errorf("failed to unmarshal cilium values: %w", err)
			}
		}

		var metricsServerInterface interface{}
		if spec.KaasEssentials.MetricsServerValues != "" {
			if err := yaml.Unmarshal([]byte(spec.KaasEssentials.MetricsServerValues), &metricsServerInterface); err != nil {
				log.Error().Err(err).Msg("Failed to unmarshal metrics server values")
				return "", nil, fmt.Errorf("failed to unmarshal metrics server values: %w", err)
			}
		}
		essentialsValues = EssentialsValues{
			Coredns:       corednsInterface,
			Cilium:        ciliumInterface,
			MetricsServer: metricsServerInterface,
		}

	} else if oldSpec != nil && (oldSpec.KaasEssentials.CorednsValues != "" ||
		oldSpec.KaasEssentials.CiliumValues != "" ||
		oldSpec.KaasEssentials.MetricsServerValues != "") {
		// If essentialsValues has been reset, update the revision version
		revision = oldSpec.KaasEssentials.Revision + 1
	}

	kaasEsssentials := Essentials{
		Revision:        revision,
		StorageClasses:  storageClasses,
		SnapshotClasses: storageClasses, // StorageClass always match with SnapshotClass
		Values:          essentialsValues,
	}

	// Post Install Chart Validation
	pic := PostInstallChart{}
	if spec.PostInstallChart.ChartName != "" ||
		spec.PostInstallChart.ChartVersion != "" ||
		spec.PostInstallChart.RepoUrl != "" {

		pic.Revision = 1
		pic.RepoUrl = spec.PostInstallChart.RepoUrl
		pic.ChartName = spec.PostInstallChart.ChartName
		pic.ChartVersion = spec.PostInstallChart.ChartVersion
		pic.Namespace = spec.PostInstallChart.Namespace

		var values interface{}
		if spec.PostInstallChart.Values != "" {
			if err := yaml.Unmarshal([]byte(spec.PostInstallChart.Values), &values); err != nil {
				log.Error().Err(err).Msg("Failed to unmarshal post install chart values")
				return "", nil, fmt.Errorf("failed to unmarshal post install chart values: %w", err)
			}
		}
		pic.Values = values

		if oldSpec != nil {
			if oldSpec.PostInstallChart.ChartName == spec.PostInstallChart.ChartName &&
				oldSpec.PostInstallChart.ChartVersion == spec.PostInstallChart.ChartVersion &&
				oldSpec.PostInstallChart.RepoUrl == spec.PostInstallChart.RepoUrl &&
				oldSpec.PostInstallChart.Namespace == spec.PostInstallChart.Namespace &&
				oldSpec.PostInstallChart.Values == spec.PostInstallChart.Values {
				pic.Revision = oldSpec.PostInstallChart.Revision
			} else {
				pic.Revision = oldSpec.PostInstallChart.Revision + 1
			}
		}

	}

	// upgrading a cluster and activating/deactivating the dedicated datastore at the same time is not handled by Kamaji
	if oldSpec != nil && spec.ControlPlane.DataStore.Dedicated != oldSpec.ControlPlane.DataStore.Dedicated &&
		spec.KubeVersion != oldSpec.KubeVersion {
		log.Error().Msg("Cannot change kubeversion and activating/deactivating dedicated datastore store due to Kamaji limitation")
		return "", nil, fmt.Errorf("cannot change kubeversion and activating/deactivating dedicated datastore store")
	}

	// Disaster recovery (PRA): a dedicated control plane datastore (etcd).
	var dataStore *DataStore
	if spec.ControlPlane.DataStore.Dedicated {
		if !kubeVersionAtLeast(spec.KubeVersion, dataStoreMinMajor, dataStoreMinMinor) {
			log.Error().Str("kubeVersion", spec.KubeVersion).Msgf("Disaster recovery requires Kubernetes >= %d.%d", dataStoreMinMajor, dataStoreMinMinor)
			return "", nil, fmt.Errorf("disaster recovery requires Kubernetes >= %d.%d", dataStoreMinMajor, dataStoreMinMinor)
		}
		storageClassName := spec.ControlPlane.DataStore.StorageClassName
		if storageClassName == "" {
			if len(kaasConfig.StorageClasses) == 0 {
				log.Error().Msg("Disaster recovery enabled but no storage classes are configured")
				return "", nil, fmt.Errorf("disaster recovery requires at least one configured storage class")
			}
			storageClassName = kaasConfig.StorageClasses[0].Shortname // default = first, mirrors essentials
		}

		storage := spec.ControlPlane.DataStore.Storage
		if storage <= 0 {
			storage = defaultDataStoreStorageGi
		}

		// PVC-backed storage cannot be shrunk.
		if oldSpec != nil && oldSpec.ControlPlane.DataStore.Dedicated {
			oldStorage := oldSpec.ControlPlane.DataStore.Storage
			if oldStorage <= 0 {
				oldStorage = defaultDataStoreStorageGi
			}
			if storage < oldStorage {
				log.Error().Int("oldStorage", oldStorage).Int("newStorage", storage).Msg("Disaster recovery storage cannot be reduced")
				return "", nil, fmt.Errorf("disaster recovery storage cannot be reduced (current %dGi, requested %dGi)", oldStorage, storage)
			}
		}

		q, err := resource.ParseQuantity(fmt.Sprintf(quantityFormat, storage))
		if err != nil {
			log.Error().Err(err).Int("storage", storage).Msg("Invalid datastore storage size")
			return "", nil, fmt.Errorf("invalid datastore storage size: %w", err)
		}

		dataStore = &DataStore{
			Dedicated:        true,
			StorageClassName: storageClassName,
			Storage:          q.String(),
		}
	}

	valuesObj := Values{
		AzDomains: azDomainValues(config.Global.ProductsConfig.ArgoApp.Kubernetes.AzDomains),
		Clusters: map[string]Cluster{
			localId: {
				Name:        localId,
				Location:    location,
				KubeVersion: spec.KubeVersion,
				ControlPlane: ControlPlane{
					DataStore: dataStore,
					Network: struct {
						DefaultPolicies string `yaml:"defaultPolicies,omitempty"`
						Fqdn            string `yaml:"fqdn,omitempty"`
					}{DefaultPolicies: spec.CPNetPol},
				},
				Workers: Workers{
					Instances: instances,
					Network: struct {
						DefaultPolicies string `yaml:"defaultPolicies,omitempty"`
					}{DefaultPolicies: spec.WorkersNetPol},
				},
				PostInstallChart: pic,
				KaaSEssentials:   kaasEsssentials,
			},
		},
	}
	valuesBytes, err := yaml.Marshal(valuesObj)
	if err != nil {
		return "", nil, err
	}

	groupToRemove := make([]string, 0)
	for _, s := range mapGroupToRemove {
		groupToRemove = append(groupToRemove, s)
	}

	return string(valuesBytes[:]), groupToRemove, nil
}

// CreateArgoApp builds the ArgoCD application body for a KaaS deployment.
func CreateArgoApp(ctx context.Context, localId string, az config.AZConfig, spec KaaSSpec, metadata spxId.Metadata, kaasConfig KaaSConfig, oldSpec *KaaSSpec) (argo.CreateAppInfo, []string, error) {
	log := logger.GetLogger(ctx)
	values, gtr, err := CreateKaaSAppValues(ctx, localId, az.Code, spec, kaasConfig, oldSpec)
	if err != nil {
		log.Err(err).Msg("Failed to create app values")
		return argo.CreateAppInfo{}, nil, fmt.Errorf("failed to create app values")
	}

	var helmParams strings.Builder
	helmParams.WriteString(fmt.Sprintf("--set location=%s  --set organizationID=%s  --set projectID=%s", az.Code, metadata.OrgId, metadata.ProjectId))
	for _, class := range kaasConfig.StorageClasses {
		helmParams.WriteString(fmt.Sprintf("  --set storageClassMapping.%s=%s", class.Shortname, class.Fullname))
	}

	appName := fmt.Sprintf("%s-%s", KaasPrefix, metadata.GetResourceEffectiveID())

	// Version support was already validated in CreateKaaSAppValues, so the repo is always resolved here.
	repo, _ := config.ResolveKubeVersionRepo(config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions, config.Global.ProductsConfig.ArgoApp.Kubernetes.Repo, spec.KubeVersion)

	return argo.CreateAppInfo{
		Metadata: metadata,
		General: argo.AppGeneral{
			AppName:     appName,
			Destination: az.Destination,
		},
		Spec: argo.AppSpec{
			Source: argo.AppSource{
				RepoURL:        repo.RepoURL,
				TargetRevision: repo.TargetRevision,
				Chart:          repo.Chart,
				Path:           repo.Path,
				Plugin: v1alpha1.ApplicationSourcePlugin{
					Name: "uuidv5",
					Env: v1alpha1.Env{
						{Name: "RELEASE", Value: appName},
						{Name: "REPO", Value: ""},
						{Name: "HELM_PARAMS", Value: helmParams.String()},
						{Name: "HELM_VALUEFILES", Value: ""},
						{Name: "HELM_VALUES", Value: values},
					},
				},
			},
			IgnoreDifferences: ignoreDifferences,
		},
	}, gtr, nil
}

// parseNodeGroup validates a node group and adds it to the instances map.
func parseNodeGroup(instances map[string]Instance, group Group, interfaces []Interface) error {
	if group.Version < 0 {
		return fmt.Errorf("version must be equals or greater than 0")
	}

	if !slices.Contains(MemoryValueList, group.Memory) {
		return fmt.Errorf("no such memory: %d", group.Memory)
	}
	if !slices.Contains(CpuValueList, group.Cpu) {
		return fmt.Errorf("no such cpu value: %d", group.Cpu)
	}

	minStorageSize := resource.MustParse(fmt.Sprintf(quantityFormat, 1))
	storage := resource.MustParse(fmt.Sprintf(quantityFormat, group.BootDiskSize))
	memory := resource.MustParse(fmt.Sprintf(quantityFormat, group.Memory))

	if storage.Cmp(minStorageSize) < 0 {
		return fmt.Errorf("storage size is incorrect, minStorageSize: %s", minStorageSize.String())
	}

	instances[group.Name] = Instance{
		Deployment: Deployment{
			Replicas: group.Replicas,
		},
		Template: Template{
			Version:    group.Version,
			Cores:      group.Cpu,
			Memory:     memory.String(),
			Interfaces: interfaces,
			BootDisk: BootDisk{
				Storage:          storage.String(),
				StorageClassName: group.StorageClass,
			},
		},
	}
	return nil
}

// isSameGroup compares two Groups ignoring Replicas and Version.
func isSameGroup(g1, g2 Group) bool {
	if g1.Cpu != g2.Cpu ||
		g1.Memory != g2.Memory ||
		g1.BootDiskSize != g2.BootDiskSize ||
		g1.StorageClass != g2.StorageClass ||
		g1.Name != g2.Name ||
		len(g1.Subnets) != len(g2.Subnets) {
		return false
	}

	for i := range g1.Subnets {
		if !isSameGroupSubnet(g1.Subnets[i], g2.Subnets[i]) {
			return false
		}
	}

	return true
}

// isSameGroupSubnet checks if two GroupSubnet objects have the same ID and Order.
func isSameGroupSubnet(s1, s2 GroupSubnet) bool {
	return s1.Id == s2.Id && s1.Order == s2.Order
}

// kubeVersionAtLeast checks if version is >= minMajor.minMinor.
func kubeVersionAtLeast(version string, minMajor, minMinor int) bool {
	v := version
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major > minMajor || (major == minMajor && minor >= minMinor)
}

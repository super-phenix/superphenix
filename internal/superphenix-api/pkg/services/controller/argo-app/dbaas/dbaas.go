package dbaas

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// CreateAppValues generates the sfs-dbaas values of a database. oldSpec is nil on creation.
func CreateAppValues(ctx context.Context, localId, location string, spec DBaaSSpec, subnet Subnet, oldSpec *DBaaSSpec) (string, error) {
	log := logger.GetLogger(ctx)
	spec = withImmutable(spec, oldSpec)

	if err := validateSpec(spec); err != nil {
		log.Error().Err(err).Any("spec", spec).Msg("Invalid database spec")
		return "", err
	}
	if err := validateUpdate(spec, oldSpec); err != nil {
		log.Error().Err(err).Any("spec", spec).Any("oldSpec", oldSpec).Msg("Invalid database update")
		return "", err
	}

	valuesObj := Values{
		Databases: map[string]Database{
			localId: {
				Name:             localId,
				Location:         location,
				Version:          spec.Version,
				Instances:        spec.Instances,
				Cpu:              strconv.Itoa(spec.Cpu),
				Memory:           fmt.Sprintf(quantityFormat, spec.Memory),
				Storage:          fmt.Sprintf(quantityFormat, spec.Storage),
				StorageClassName: spec.StorageClass,
				Subnet:           subnet.LocalID,
				SubnetCIDR:       subnet.CIDR,
				LoadBalancer:     LoadBalancer{VIP: spec.VIP},
				ExternalAccess:   spec.ExternalAccess,
			},
		},
	}
	valuesBytes, err := yaml.Marshal(valuesObj)
	if err != nil {
		return "", err
	}

	return string(valuesBytes), nil
}

// CreateArgoApp builds the Application rendering a database with the sfs-dbaas chart.
func CreateArgoApp(
	ctx context.Context, localId string, az config.AZConfig, spec DBaaSSpec, subnet Subnet, metadata spxId.Metadata,
	dbaasConfig DBaaSConfig, oldSpec *DBaaSSpec, chart config.RepoArgoAppConfig,
) (argo.CreateAppInfo, error) {
	log := logger.GetLogger(ctx)
	spec = withImmutable(spec, oldSpec)

	if !slices.ContainsFunc(dbaasConfig.StorageClasses, func(c ClassMapping) bool { return c.Shortname == spec.StorageClass }) {
		log.Error().Str("storageClass", spec.StorageClass).Msg("Storage class not available in this AZ")
		return argo.CreateAppInfo{}, fmt.Errorf("storage class %q is not available in this AZ", spec.StorageClass)
	}

	if spec.ExternalAccess && dbaasConfig.ExternalSubnet == "" {
		log.Error().Str("az", az.Code).Msg("No EIP external subnet configured in this AZ")
		return argo.CreateAppInfo{}, fmt.Errorf("external access is not available in this AZ")
	}

	values, err := CreateAppValues(ctx, localId, az.Code, spec, subnet, oldSpec)
	if err != nil {
		return argo.CreateAppInfo{}, err
	}

	var helmParams strings.Builder
	helmParams.WriteString(fmt.Sprintf("--set location=%s  --set organizationID=%s  --set projectID=%s", az.Code, metadata.OrgId, metadata.ProjectId))
	for _, class := range dbaasConfig.StorageClasses {
		helmParams.WriteString(fmt.Sprintf("  --set storageClassMapping.%s=%s", class.Shortname, class.Fullname))
	}
	if dbaasConfig.ExternalSubnet != "" {
		helmParams.WriteString(fmt.Sprintf("  --set externalSubnet=%s", dbaasConfig.ExternalSubnet))
	}

	return newAppInfo(AppName(metadata.GetResourceEffectiveID()), az, metadata, chart, helmParams.String(), values), nil
}

// AppName returns the name of the Application of a database.
func AppName(effectiveId string) string {
	return fmt.Sprintf("%s-%s", AppPrefix, effectiveId)
}

func newAppInfo(appName string, az config.AZConfig, metadata spxId.Metadata, chart config.RepoArgoAppConfig, helmParams, values string) argo.CreateAppInfo {
	return argo.CreateAppInfo{
		Metadata: metadata,
		General: argo.AppGeneral{
			AppName:     appName,
			Destination: az.Destination,
		},
		Spec: argo.AppSpec{
			Source: argo.AppSource{
				RepoURL:        chart.RepoURL,
				TargetRevision: chart.TargetRevision,
				Chart:          chart.Chart,
				Path:           chart.Path,
				Plugin: v1alpha1.ApplicationSourcePlugin{
					Name: "uuidv5",
					Env: v1alpha1.Env{
						{Name: "RELEASE", Value: appName},
						{Name: "REPO", Value: ""},
						{Name: "HELM_PARAMS", Value: helmParams},
						{Name: "HELM_VALUEFILES", Value: ""},
						{Name: "HELM_VALUES", Value: values},
					},
				},
			},
		},
	}
}

// withImmutable fills the fields an update may omit from oldSpec.
func withImmutable(spec DBaaSSpec, oldSpec *DBaaSSpec) DBaaSSpec {
	if oldSpec == nil {
		return spec
	}
	if spec.StorageClass == "" {
		spec.StorageClass = oldSpec.StorageClass
	}
	if spec.SubnetEId == "" {
		spec.SubnetEId = oldSpec.SubnetEId
	}
	if spec.VIP == "" {
		spec.VIP = oldSpec.VIP
	}
	return spec
}

func validateSpec(spec DBaaSSpec) error {
	if spec.Instances < minInstances || spec.Instances > maxInstances {
		return fmt.Errorf("instances (%d) must be between %d and %d", spec.Instances, minInstances, maxInstances)
	}
	if !slices.Contains(CpuValueList, spec.Cpu) {
		return fmt.Errorf("no such cpu value: %d", spec.Cpu)
	}
	if !slices.Contains(MemoryValueList, spec.Memory) {
		return fmt.Errorf("no such memory: %d", spec.Memory)
	}
	if spec.Storage < minStorage {
		return fmt.Errorf("storage must be at least %dGi", minStorage)
	}
	if spec.StorageClass == "" {
		return fmt.Errorf("storage class is required")
	}
	if !vipRegex.MatchString(spec.VIP) {
		return fmt.Errorf("vip %q must be part of 198.18.0.0/16", spec.VIP)
	}
	if _, ok := majorVersion(spec.Version); !ok {
		return fmt.Errorf("invalid version %q", spec.Version)
	}
	return nil
}

// validateUpdate checks the version is supported and, on update, that only mutable fields change.
func validateUpdate(spec DBaaSSpec, oldSpec *DBaaSSpec) error {
	if oldSpec == nil || spec.Version != oldSpec.Version {
		if _, supported := ChartFromConfig(spec.Version); !supported {
			return fmt.Errorf("version %q is not supported", spec.Version)
		}
	}
	if oldSpec == nil {
		return nil
	}

	newMajor, _ := majorVersion(spec.Version)
	oldMajor, _ := majorVersion(oldSpec.Version)
	switch {
	case newMajor != oldMajor:
		return fmt.Errorf("cannot change major version from %s to %s", oldSpec.Version, spec.Version)
	case spec.Storage < oldSpec.Storage:
		return fmt.Errorf("storage cannot be reduced (current %dGi, requested %dGi)", oldSpec.Storage, spec.Storage)
	case spec.StorageClass != oldSpec.StorageClass:
		return fmt.Errorf("storage class cannot be changed")
	case spec.SubnetEId != oldSpec.SubnetEId:
		return fmt.Errorf("subnet cannot be changed")
	case spec.VIP != oldSpec.VIP:
		return fmt.Errorf("vip cannot be changed")
	}
	return nil
}

// majorVersion returns the major of a PostgreSQL version such as "17.6".
func majorVersion(version string) (int, bool) {
	major, _, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(major)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

package dbaas

import (
	"fmt"
	"strconv"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ConvertAppToDBaaSSpec reads the spec and the subnet of a database back from its Application.
func ConvertAppToDBaaSSpec(app view.AppView, projectId string) (DBaaSSpec, Subnet, error) {
	if app.Spec.Source == nil || app.Spec.Source.Plugin == nil {
		return DBaaSSpec{}, Subnet{}, fmt.Errorf("failed to convert app to DBaaSSpec")
	}

	var helmValues Values
	for _, entry := range app.Spec.Source.Plugin.Env {
		if entry == nil || entry.Name != "HELM_VALUES" {
			continue
		}
		if err := yaml.Unmarshal([]byte(entry.Value), &helmValues); err != nil {
			return DBaaSSpec{}, Subnet{}, err
		}
	}

	for _, db := range helmValues.Databases {
		cpu, err := strconv.Atoi(db.Cpu)
		if err != nil {
			return DBaaSSpec{}, Subnet{}, fmt.Errorf("invalid cpu %q: %w", db.Cpu, err)
		}
		memory, err := gibibytes(db.Memory)
		if err != nil {
			return DBaaSSpec{}, Subnet{}, err
		}
		storage, err := gibibytes(db.Storage)
		if err != nil {
			return DBaaSSpec{}, Subnet{}, err
		}
		subnetEId, err := effectiveID(projectId, db.Subnet)
		if err != nil {
			return DBaaSSpec{}, Subnet{}, err
		}

		spec := DBaaSSpec{
			Version:      db.Version,
			Instances:    db.Instances,
			Cpu:          cpu,
			Memory:       memory,
			Storage:      storage,
			StorageClass: db.StorageClassName,
			SubnetEId:    subnetEId,
			VIP:          db.LoadBalancer.VIP,

			ExternalAccess: db.ExternalAccess,
		}
		return spec, Subnet{LocalID: db.Subnet, CIDR: db.SubnetCIDR}, nil // a single database per Application
	}

	return DBaaSSpec{}, Subnet{}, fmt.Errorf("no database in app values")
}

// gibibytes parses a quantity such as "20Gi" into GiB.
func gibibytes(value string) (int, error) {
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity %q: %w", value, err)
	}
	return int(q.Value() / (1 << 30)), nil
}

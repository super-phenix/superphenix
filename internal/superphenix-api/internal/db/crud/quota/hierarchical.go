package quota

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
)

var (
	ErrOrgQuotaExceeded     = errors.New("organization quota exceeded")
	ErrProjectQuotaExceeded = errors.New("project quota exceeded")
	ErrInvalidQuantity      = errors.New("invalid resource quantity")
)

// ParsedResources represents canonical numeric quantities of resources.
type ParsedResources struct {
	MilliCPU int64
	Memory   int64 // bytes
	Disk     int64 // bytes
}

// ParseResources parses a model.QuotaResources into canonical numeric quantities.
func ParseResources(r model.QuotaResources) (ParsedResources, error) {
	var res ParsedResources

	if r.CPU != "" {
		q, err := resource.ParseQuantity(r.CPU)
		if err != nil {
			return res, fmt.Errorf("%w: invalid cpu %q: %v", ErrInvalidQuantity, r.CPU, err)
		}
		if q.MilliValue() < 0 {
			return res, fmt.Errorf("%w: cpu cannot be negative", ErrInvalidQuantity)
		}
		res.MilliCPU = q.MilliValue()
	}

	if r.Memory != "" {
		q, err := resource.ParseQuantity(r.Memory)
		if err != nil {
			return res, fmt.Errorf("%w: invalid memory %q: %v", ErrInvalidQuantity, r.Memory, err)
		}
		if q.Value() < 0 {
			return res, fmt.Errorf("%w: memory cannot be negative", ErrInvalidQuantity)
		}
		res.Memory = q.Value()
	}

	if r.Disk != "" {
		q, err := resource.ParseQuantity(r.Disk)
		if err != nil {
			return res, fmt.Errorf("%w: invalid disk %q: %v", ErrInvalidQuantity, r.Disk, err)
		}
		if q.Value() < 0 {
			return res, fmt.Errorf("%w: disk cannot be negative", ErrInvalidQuantity)
		}
		res.Disk = q.Value()
	}

	return res, nil
}

// ToQuotaResources formats ParsedResources back into standard string representations.
func (p ParsedResources) ToQuotaResources() model.QuotaResources {
	var r model.QuotaResources
	if p.MilliCPU > 0 {
		if p.MilliCPU%1000 == 0 {
			r.CPU = fmt.Sprintf("%d", p.MilliCPU/1000)
		} else {
			r.CPU = fmt.Sprintf("%dm", p.MilliCPU)
		}
	}
	if p.Memory > 0 {
		r.Memory = resource.NewQuantity(p.Memory, resource.BinarySI).String()
	}
	if p.Disk > 0 {
		r.Disk = resource.NewQuantity(p.Disk, resource.BinarySI).String()
	}
	return r
}

// SplitEqually divides ParsedResources into count equal parts using integer floor division.
func (p ParsedResources) SplitEqually(count int) ParsedResources {
	if count <= 0 {
		return ParsedResources{}
	}
	return ParsedResources{
		MilliCPU: p.MilliCPU / int64(count),
		Memory:   p.Memory / int64(count),
		Disk:     p.Disk / int64(count),
	}
}

// Add sums two ParsedResources.
func (p ParsedResources) Add(other ParsedResources) ParsedResources {
	return ParsedResources{
		MilliCPU: p.MilliCPU + other.MilliCPU,
		Memory:   p.Memory + other.Memory,
		Disk:     p.Disk + other.Disk,
	}
}

// Sub subtracts other from p (clamping to 0).
func (p ParsedResources) Sub(other ParsedResources) ParsedResources {
	milliCPU := p.MilliCPU - other.MilliCPU
	if milliCPU < 0 {
		milliCPU = 0
	}
	mem := p.Memory - other.Memory
	if mem < 0 {
		mem = 0
	}
	disk := p.Disk - other.Disk
	if disk < 0 {
		disk = 0
	}
	return ParsedResources{
		MilliCPU: milliCPU,
		Memory:   mem,
		Disk:     disk,
	}
}

// Exceeds checks if p exceeds limit for any resource where limit > 0.
func (p ParsedResources) Exceeds(limit ParsedResources) (bool, string) {
	if limit.MilliCPU > 0 && p.MilliCPU > limit.MilliCPU {
		return true, fmt.Sprintf("CPU quota exceeded: requested %s exceeds limit %s",
			resource.NewMilliQuantity(p.MilliCPU, resource.DecimalSI).String(),
			resource.NewMilliQuantity(limit.MilliCPU, resource.DecimalSI).String())
	}
	if limit.Memory > 0 && p.Memory > limit.Memory {
		return true, fmt.Sprintf("Memory quota exceeded: requested %s exceeds limit %s",
			resource.NewQuantity(p.Memory, resource.BinarySI).String(),
			resource.NewQuantity(limit.Memory, resource.BinarySI).String())
	}
	if limit.Disk > 0 && p.Disk > limit.Disk {
		return true, fmt.Sprintf("Disk quota exceeded: requested %s exceeds limit %s",
			resource.NewQuantity(p.Disk, resource.BinarySI).String(),
			resource.NewQuantity(limit.Disk, resource.BinarySI).String())
	}
	return false, ""
}

// GetOrganizationQuota retrieves an organization's quota, or nil if unlimited.
func GetOrganizationQuota(ctx context.Context, orgId uuid.UUID) (*model.OrganizationQuota, error) {
	var oq model.OrganizationQuota
	err := db.Client.WithContext(ctx).Where("organization_id = ?", orgId).First(&oq).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &oq, nil
}

// GetProjectQuota retrieves a project's quota, or nil if not defined.
func GetProjectQuota(ctx context.Context, projectId uuid.UUID) (*model.ProjectQuota, error) {
	var pq model.ProjectQuota
	err := db.Client.WithContext(ctx).Where("project_id = ?", projectId).First(&pq).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &pq, nil
}

// ListProjectQuotasByOrg retrieves all project quotas within an organization.
func ListProjectQuotasByOrg(ctx context.Context, orgId uuid.UUID) ([]model.ProjectQuota, error) {
	var pqs []model.ProjectQuota
	err := db.Client.WithContext(ctx).Where("organization_id = ?", orgId).Find(&pqs).Error
	if err != nil {
		return nil, err
	}
	return pqs, nil
}

// GetProjectAZQuota retrieves an AZ quota for a project, or nil if not defined.
func GetProjectAZQuota(ctx context.Context, projectId uuid.UUID, codeAZ string) (*model.ProjectAZQuota, error) {
	var azq model.ProjectAZQuota
	err := db.Client.WithContext(ctx).Where("project_id = ? AND code_az = ?", projectId, codeAZ).First(&azq).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &azq, nil
}

// ListProjectAZQuotas retrieves all AZ quotas for a given project.
func ListProjectAZQuotas(ctx context.Context, projectId uuid.UUID) ([]model.ProjectAZQuota, error) {
	var azqs []model.ProjectAZQuota
	err := db.Client.WithContext(ctx).Where("project_id = ?", projectId).Find(&azqs).Error
	if err != nil {
		return nil, err
	}
	return azqs, nil
}

// SetOrganizationQuota creates or updates an organization quota.
// If activating quotas and no project quotas exist for pre-existing projects,
// the virtual quota is split equally among all pre-existing projects and immediately across their AZs.
func SetOrganizationQuota(ctx context.Context, orgId uuid.UUID, resources model.QuotaResources) (*model.OrganizationQuota, error) {
	parsedOrg, err := ParseResources(resources)
	if err != nil {
		return nil, err
	}

	var savedOQ model.OrganizationQuota

	err = db.Client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var projects []model.Project
		if err := tx.Where("orga_id = ?", orgId).Find(&projects).Error; err != nil {
			return err
		}

		var existingPQs []model.ProjectQuota
		if err := tx.Where("organization_id = ?", orgId).Find(&existingPQs).Error; err != nil {
			return err
		}

		// Auto-split rule: if activating quotas and none are defined for pre-existing projects
		if len(existingPQs) == 0 && len(projects) > 0 {
			projectSlice := parsedOrg.SplitEqually(len(projects))
			projectRes := projectSlice.ToQuotaResources()

			azConfigs := az.FindAll(orgId.String())

			for _, p := range projects {
				var existingPQ model.ProjectQuota
				err := tx.Where("project_id = ?", p.ID).First(&existingPQ).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if errors.Is(err, gorm.ErrRecordNotFound) {
					newPQ := model.ProjectQuota{
						ProjectId:      p.ID,
						OrganizationId: orgId,
						Resources:      projectRes,
					}
					if err := tx.Create(&newPQ).Error; err != nil {
						return err
					}
				} else {
					existingPQ.OrganizationId = orgId
					existingPQ.Resources = projectRes
					if err := tx.Save(&existingPQ).Error; err != nil {
						return err
					}
				}

				if len(azConfigs) > 0 {
					azSlice := projectSlice.SplitEqually(len(azConfigs))
					azRes := azSlice.ToQuotaResources()

					for _, azCfg := range azConfigs {
						var existingAZQ model.ProjectAZQuota
						err := tx.Where("project_id = ? AND code_az = ?", p.ID, azCfg.Code).First(&existingAZQ).Error
						if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
							return err
						}
						if errors.Is(err, gorm.ErrRecordNotFound) {
							newAZQ := model.ProjectAZQuota{
								ProjectId: p.ID,
								CodeAZ:    azCfg.Code,
								Resources: azRes,
							}
							if err := tx.Create(&newAZQ).Error; err != nil {
								return err
							}
						} else {
							existingAZQ.Resources = azRes
							if err := tx.Save(&existingAZQ).Error; err != nil {
								return err
							}
						}
					}
				}
			}
		} else if len(existingPQs) > 0 {
			// Validate that existing project quotas sum does not exceed new org quota
			var totalProjects ParsedResources
			for _, pq := range existingPQs {
				pParsed, err := ParseResources(pq.Resources)
				if err != nil {
					return err
				}
				totalProjects = totalProjects.Add(pParsed)
			}
			if exceeded, msg := totalProjects.Exceeds(parsedOrg); exceeded {
				return fmt.Errorf("%w: %s", ErrOrgQuotaExceeded, msg)
			}
		}

		var existingOQ model.OrganizationQuota
		err := tx.Where("organization_id = ?", orgId).First(&existingOQ).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			savedOQ = model.OrganizationQuota{
				OrganizationId: orgId,
				Resources:      resources,
			}
			if err := tx.Create(&savedOQ).Error; err != nil {
				return err
			}
		} else {
			existingOQ.Resources = resources
			savedOQ = existingOQ
			if err := tx.Save(&savedOQ).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &savedOQ, nil
}

// SetProjectQuota sets a project quota, validating against organization quota if defined,
// and immediately splitting the project quota equally across all active AZs of the project.
func SetProjectQuota(ctx context.Context, orgId uuid.UUID, projectId uuid.UUID, resources model.QuotaResources) (*model.ProjectQuota, error) {
	parsedProj, err := ParseResources(resources)
	if err != nil {
		return nil, err
	}

	var savedPQ model.ProjectQuota

	err = db.Client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Check org quota if present
		var oq model.OrganizationQuota
		err := tx.Where("organization_id = ?", orgId).First(&oq).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			parsedOrg, err := ParseResources(oq.Resources)
			if err != nil {
				return err
			}

			var otherPQs []model.ProjectQuota
			if err := tx.Where("organization_id = ? AND project_id != ?", orgId, projectId).Find(&otherPQs).Error; err != nil {
				return err
			}

			var totalProjects ParsedResources
			for _, pq := range otherPQs {
				pParsed, err := ParseResources(pq.Resources)
				if err != nil {
					return err
				}
				totalProjects = totalProjects.Add(pParsed)
			}
			totalProjects = totalProjects.Add(parsedProj)

			if exceeded, msg := totalProjects.Exceeds(parsedOrg); exceeded {
				return fmt.Errorf("%w: %s", ErrOrgQuotaExceeded, msg)
			}
		}

		var existingPQ model.ProjectQuota
		err = tx.Where("project_id = ?", projectId).First(&existingPQ).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			savedPQ = model.ProjectQuota{
				ProjectId:      projectId,
				OrganizationId: orgId,
				Resources:      resources,
			}
			if err := tx.Create(&savedPQ).Error; err != nil {
				return err
			}
		} else {
			existingPQ.OrganizationId = orgId
			existingPQ.Resources = resources
			savedPQ = existingPQ
			if err := tx.Save(&savedPQ).Error; err != nil {
				return err
			}
		}

		// Immediately split equally across available AZs
		azConfigs := az.FindAll(orgId.String())
		if len(azConfigs) > 0 {
			azSlice := parsedProj.SplitEqually(len(azConfigs))
			azRes := azSlice.ToQuotaResources()

			for _, azCfg := range azConfigs {
				var existingAZQ model.ProjectAZQuota
				err := tx.Where("project_id = ? AND code_az = ?", projectId, azCfg.Code).First(&existingAZQ).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if errors.Is(err, gorm.ErrRecordNotFound) {
					newAZQ := model.ProjectAZQuota{
						ProjectId: projectId,
						CodeAZ:    azCfg.Code,
						Resources: azRes,
					}
					if err := tx.Create(&newAZQ).Error; err != nil {
						return err
					}
				} else {
					existingAZQ.Resources = azRes
					if err := tx.Save(&existingAZQ).Error; err != nil {
						return err
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return &savedPQ, nil
}

// SetProjectAZQuota manually sets an AZ quota for a project, validating against project quota.
func SetProjectAZQuota(ctx context.Context, projectId uuid.UUID, codeAZ string, resources model.QuotaResources) (*model.ProjectAZQuota, error) {
	parsedAZ, err := ParseResources(resources)
	if err != nil {
		return nil, err
	}

	var savedAZQ model.ProjectAZQuota

	err = db.Client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Check project quota
		var pq model.ProjectQuota
		err := tx.Where("project_id = ?", projectId).First(&pq).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			parsedProj, err := ParseResources(pq.Resources)
			if err != nil {
				return err
			}

			var otherAZs []model.ProjectAZQuota
			if err := tx.Where("project_id = ? AND code_az != ?", projectId, codeAZ).Find(&otherAZs).Error; err != nil {
				return err
			}

			var totalAZs ParsedResources
			for _, azq := range otherAZs {
				aParsed, err := ParseResources(azq.Resources)
				if err != nil {
					return err
				}
				totalAZs = totalAZs.Add(aParsed)
			}
			totalAZs = totalAZs.Add(parsedAZ)

			if exceeded, msg := totalAZs.Exceeds(parsedProj); exceeded {
				return fmt.Errorf("%w: %s", ErrProjectQuotaExceeded, msg)
			}
		}

		var existingAZQ model.ProjectAZQuota
		err = tx.Where("project_id = ? AND code_az = ?", projectId, codeAZ).First(&existingAZQ).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			savedAZQ = model.ProjectAZQuota{
				ProjectId: projectId,
				CodeAZ:    codeAZ,
				Resources: resources,
			}
			if err := tx.Create(&savedAZQ).Error; err != nil {
				return err
			}
		} else {
			existingAZQ.Resources = resources
			savedAZQ = existingAZQ
			if err := tx.Save(&savedAZQ).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return &savedAZQ, nil
}

// DeleteOrganizationQuota deletes an organization quota.
func DeleteOrganizationQuota(ctx context.Context, orgId uuid.UUID) error {
	return db.Client.WithContext(ctx).Where("organization_id = ?", orgId).Delete(&model.OrganizationQuota{}).Error
}

// DeleteProjectQuota deletes a project quota.
func DeleteProjectQuota(ctx context.Context, projectId uuid.UUID) error {
	return db.Client.WithContext(ctx).Where("project_id = ?", projectId).Delete(&model.ProjectQuota{}).Error
}

// DeleteProjectAZQuota deletes an AZ quota.
func DeleteProjectAZQuota(ctx context.Context, projectId uuid.UUID, codeAZ string) error {
	return db.Client.WithContext(ctx).Where("project_id = ? AND code_az = ?", projectId, codeAZ).Delete(&model.ProjectAZQuota{}).Error
}

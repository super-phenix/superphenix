package product

import (
	"context"
	"strconv"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
)

func Save(product model.Product) (model.Product, error) {
	result := db.Client.Save(&product)
	return product, result.Error
}

func SaveAll(products []model.Product) ([]model.Product, error) {
	result := db.Client.Save(&products)
	return products, result.Error
}

func DeleteById(productId uuid.UUID) error {
	result := db.Client.Delete(&model.Product{}, productId)
	return result.Error
}

func DeleteByEId(productEId string) error {
	result := db.Client.Where("effective_id = ?", productEId).Delete(&model.Product{})
	return result.Error
}

func DeleteByEIdAndAZCodeAndProject(productEId, azCode string, projectId uuid.UUID) (int64, error) {
	result := db.Client.
		Where("effective_id = ? AND code_az = ? AND project_id = ?", productEId, azCode, projectId).
		Delete(&model.Product{})
	return result.RowsAffected, result.Error
}

func DeleteByProject(projectId string) error {
	result := db.Client.Where("project_id = ?", projectId).Delete(&model.Product{})
	return result.Error
}

func FindByEId(productEId string) (model.Product, error) {
	return crud.Find[model.Product, model.Product](model.Product{
		EffectiveID: productEId,
	})
}

func FindAllByProjectIdAndResourceType(projectId, productType string) ([]model.Product, error) {
	projectUuid, _ := uuid.Parse(projectId)
	return crud.FindAll[model.Product, model.Product](model.Product{
		ProjectId:     projectUuid,
		ProductTypeId: productType,
	})
}

func FindAllByProjectIdAndResourceTypeAndCodeAZ(projectId, productType, codeAZ string) ([]model.Product, error) {
	projectUuid, _ := uuid.Parse(projectId)
	return crud.FindAll[model.Product, model.Product](model.Product{
		ProjectId:     projectUuid,
		ProductTypeId: productType,
		CodeAZ:        codeAZ,
	})
}

// IsQuotaCreationReached compare quota with number of project inside the organization
// Return true if the quota is reached
func IsQuotaCreationReached(ctx context.Context, projectId uuid.UUID) (bool, error) {
	log := logger.GetLogger(ctx)
	limitQuota, err := quota.GetQuotaValue(quota.ProjectLimitProduct, projectId, quota.MapQuotaEntityType[quota.ProjectLimitProduct])
	if err != nil {
		log.Err(err).Str("projectId", projectId.String()).Msg("failed to get quota")
		return false, err
	}

	var count int64
	res := db.Client.Model(&model.Product{}).Where(&model.Product{ProjectId: projectId}).Count(&count)
	if res.Error != nil {
		log.Err(err).Str("projectId", projectId.String()).Msg("failed to count projects")
		return false, err
	}

	limit, err := strconv.ParseInt(limitQuota.Value, 10, 64)
	if err != nil {
		log.Err(err).Any("limitQuota", limitQuota).Msg("failed to parse quota value")
		return false, err
	}

	return count >= limit, nil
}

// CountByTypeForProject counts the project's products per product type. Requested types with no
// product are returned with a zero count.
func CountByTypeForProject(projectId uuid.UUID, productTypes []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(productTypes))
	for _, productType := range productTypes {
		counts[productType] = 0
	}

	if len(productTypes) == 0 {
		return counts, nil
	}

	var rows []struct {
		ProductTypeId string
		Count         int64
	}
	res := db.Client.Model(&model.Product{}).
		Select("product_type_id, count(*) as count").
		Where("project_id = ? AND product_type_id IN ?", projectId, productTypes).
		Group("product_type_id").
		Scan(&rows)
	if res.Error != nil {
		return nil, res.Error
	}

	for _, row := range rows {
		counts[row.ProductTypeId] = row.Count
	}

	return counts, nil
}

// CountByAZForProject counts the project's products of the given types per AZ. AZs without any
// product are absent.
func CountByAZForProject(projectId uuid.UUID, productTypes []string) (map[string]int64, error) {
	counts := make(map[string]int64)
	if len(productTypes) == 0 {
		return counts, nil
	}

	var rows []struct {
		CodeAZ string
		Count  int64
	}
	res := db.Client.Model(&model.Product{}).
		Select("code_az, count(*) as count").
		Where("project_id = ? AND product_type_id IN ?", projectId, productTypes).
		Group("code_az").
		Scan(&rows)
	if res.Error != nil {
		return nil, res.Error
	}

	for _, row := range rows {
		counts[row.CodeAZ] = row.Count
	}

	return counts, nil
}

// CountForProject counts every product of the project, whatever its type.
func CountForProject(projectId uuid.UUID) (int64, error) {
	var count int64
	res := db.Client.Model(&model.Product{}).Where(&model.Product{ProjectId: projectId}).Count(&count)
	return count, res.Error
}

// FindLastCreatedByProject returns the project's most recent products of the given types, newest first.
func FindLastCreatedByProject(projectId uuid.UUID, productTypes []string, limit int) ([]model.Product, error) {
	products := make([]model.Product, 0, limit)
	if len(productTypes) == 0 || limit <= 0 {
		return products, nil
	}

	res := db.Client.Model(&model.Product{}).
		Where("project_id = ? AND product_type_id IN ?", projectId, productTypes).
		Order("created_at DESC").
		Limit(limit).
		Find(&products)

	return products, res.Error
}

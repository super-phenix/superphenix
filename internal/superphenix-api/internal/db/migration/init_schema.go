package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

func initSchema(tx *gorm.DB) error {
	log.Info().Msg("Initializing schema")
	err := model.AutoMigrate(tx)
	if err != nil {
		return err
	}

	// Init quota

	userLimitOrganization := "User_Limit_Organization"
	orgaLimitProject := "Orga_Limit_Project"
	orgaLimitIAMGroup := "Orga_Limit_IAMGroup"
	projectLimitProduct := "Project_Limit_Product"

	var quotas = []model.Quota{
		{
			ID:    userLimitOrganization,
			Value: "5",
		}, {
			ID:    orgaLimitProject,
			Value: "10",
		}, {
			ID:    orgaLimitIAMGroup,
			Value: "15",
		}, {
			ID:    projectLimitProduct,
			Value: "100",
		},
	}

	if err := tx.CreateInBatches(quotas, 10).Error; err != nil {
		log.Err(err).Msg("Failed to initialize quotas")
		return err
	}

	// Init resources type
	var productTypes = []model.ProductType{
		{ID: "instance", Name: "Instance"},
		{ID: "vpc", Name: "VPC"},
		{ID: "subnet", Name: "Subnet"},
		{ID: "eip", Name: "Eip"},
		{ID: "disk", Name: "Disk"},
		{ID: "snapshot", Name: "Snapshot"},
		{ID: "ssh", Name: "SSH"},
		{ID: "vmSnapshot", Name: "Instance Snapshot"},
		{ID: "loadBalancer", Name: "Load Balancer"},
		{ID: "firewall", Name: "Firewall"},
		{ID: "kaas", Name: "KaaS"},
		{ID: "baas", Name: "BaaS"},
	}

	if err := tx.CreateInBatches(productTypes, 10).Error; err != nil {
		log.Err(err).Msg("Failed to initialize productTypes")
		return err
	}

	return nil
}

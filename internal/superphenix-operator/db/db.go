package db

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	Client *gorm.DB
)

// Organization represents a Superphenix organization in the database.
type Organization struct {
	ID uuid.UUID `gorm:"primaryKey;type:uuid"`
}

// Project represents a Superphenix project in the database.
type Project struct {
	ID uuid.UUID `gorm:"primaryKey;type:uuid"`
}

// InitDatabase initializes the connection to the Superphenix Database.
func InitDatabase(host, user, password, dbname, port string) error {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s", host, user, password, dbname, port)
	var err error
	Client, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	return nil
}

// OrganizationExists checks if an organization exists in the database.
func OrganizationExists(id string) (bool, error) {
	if Client == nil {
		return false, fmt.Errorf("database client not initialized")
	}
	orgUuid, err := uuid.Parse(id)
	if err != nil {
		return false, err
	}
	var org Organization
	err = Client.First(&org, "id = ?", orgUuid).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ProjectExists checks if a project exists in the database.
func ProjectExists(id string) (bool, error) {
	if Client == nil {
		return false, fmt.Errorf("database client not initialized")
	}
	projUuid, err := uuid.Parse(id)
	if err != nil {
		return false, err
	}
	var proj Project
	err = Client.First(&proj, "id = ?", projUuid).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}


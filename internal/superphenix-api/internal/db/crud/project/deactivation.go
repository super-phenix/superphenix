package project

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Deactivate deactivates a project for a reason (see model.ProjectDeactivation). Deactivating
// it again for the same reason only updates the message.
func Deactivate(ctx context.Context, projectId uuid.UUID, reason, message string) error {
	return db.Client.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "reason"}},
		DoUpdates: clause.AssignmentColumns([]string{"message", "updated_at"}),
	}).Create(&model.ProjectDeactivation{ProjectId: projectId, Reason: reason, Message: message}).Error
}

// Reactivate lifts the deactivation of a project for a reason. The project stays deactivated
// while other reasons remain.
func Reactivate(ctx context.Context, projectId uuid.UUID, reason string) error {
	return db.Client.WithContext(ctx).
		Where("project_id = ? AND reason = ?", projectId, reason).
		Delete(&model.ProjectDeactivation{}).Error
}

// ListDeactivations returns why a project of the organization is deactivated (empty when it
// is active, or when it is not a live project of the organization).
func ListDeactivations(ctx context.Context, orgaId, projectId uuid.UUID) ([]model.ProjectDeactivation, error) {
	tx := db.Client.WithContext(ctx)
	var deactivations []model.ProjectDeactivation
	err := tx.Where("project_id = ? AND project_id IN (?)", projectId,
		tx.Session(&gorm.Session{NewDB: true}).Model(&model.Project{}).Select("id").Where("orga_id = ?", orgaId)).
		Order("reason").Find(&deactivations).Error
	return deactivations, err
}

package postgres

import (
	"context"
	"errors"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConfirmImage records the owner's decision after a validated image is ready.
// Locking the task before its output serializes confirmation with revocation.
func (r *GenerationRepository) ConfirmImage(ctx context.Context, ownerID, taskID string, at time.Time) (generationapp.TaskView, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	if at.IsZero() {
		return generationapp.TaskView{}, generationapp.ErrInvalidGenerationInput
	}
	at = at.UTC()
	var view generationapp.TaskView
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, taskID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		if task.Purpose != generationapp.PurposeImage || task.Status != generationapp.StatusSucceeded || task.AccessRevokedAt != nil || task.ResultAssetID == "" {
			return generationapp.ErrImageNotConfirmable
		}
		var output generationOutputRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ? AND id = ? AND owner_id = ?", task.ID, task.ResultAssetID, ownerID).First(&output).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return generationapp.ErrGenerationUnavailable
			}
			return err
		}
		if _, err := generationOutputFromRecord(output, task); err != nil {
			return generationapp.ErrGenerationUnavailable
		}
		if output.ConfirmedAt == nil {
			if at.Before(output.PublishedAt) {
				return generationapp.ErrImageNotConfirmable
			}
			updated := tx.Model(&generationOutputRecord{}).Where("id = ? AND confirmed_at IS NULL", output.ID).Update("confirmed_at", at)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return generationapp.ErrGenerationUnavailable
			}
		}
		view, err = r.readTaskView(tx, task)
		return err
	})
	if err != nil {
		return generationapp.TaskView{}, generationGenerationError(err)
	}
	return view, nil
}

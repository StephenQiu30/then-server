package postgres

import (
	"context"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AcquireLease claims one active generation task for a bounded worker
// interval. It contains no provider call; the lease only fences later worker
// state writes.
func (r *GenerationRepository) AcquireLease(ctx context.Context, taskID, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.Lease{}, generationapp.ErrGenerationUnavailable
	}
	if _, err := uuid.Parse(taskID); err != nil {
		return generationapp.TaskView{}, generationapp.Lease{}, generationapp.ErrInvalidGenerationInput
	}
	var view generationapp.TaskView
	var lease generationapp.Lease
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		previousRevision := task.StatusRevision
		if _, err := task.AcquireLease(owner, at, ttl); err != nil {
			return err
		}
		if err := updateGenerationTask(tx, task, previousRevision); err != nil {
			return err
		}
		persisted, err := generationTaskByID(tx, taskID)
		if err != nil {
			return err
		}
		lease, err = persisted.CurrentLease()
		if err != nil {
			return err
		}
		view, err = r.readTaskView(tx, persisted)
		return err
	})
	if err != nil {
		return generationapp.TaskView{}, generationapp.Lease{}, generationGenerationError(err)
	}
	return view, lease, nil
}

// RenewLease extends the current lease without changing its fencing token.
// The conditional status revision keeps an old worker from renewing after a
// recovery worker has acquired a new lease.
func (r *GenerationRepository) RenewLease(ctx context.Context, lease generationapp.Lease, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.Lease{}, generationapp.ErrGenerationUnavailable
	}
	if _, err := uuid.Parse(lease.TaskID); err != nil {
		return generationapp.TaskView{}, generationapp.Lease{}, generationapp.ErrInvalidGenerationLease
	}
	var view generationapp.TaskView
	var renewed generationapp.Lease
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", lease.TaskID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		previousRevision := task.StatusRevision
		if _, err := task.RenewLease(lease, at, ttl); err != nil {
			return err
		}
		if err := updateGenerationTask(tx, task, previousRevision); err != nil {
			return err
		}
		persisted, err := generationTaskByID(tx, lease.TaskID)
		if err != nil {
			return err
		}
		renewed, err = persisted.CurrentLease()
		if err != nil {
			return err
		}
		view, err = r.readTaskView(tx, persisted)
		return err
	})
	if err != nil {
		return generationapp.TaskView{}, generationapp.Lease{}, generationGenerationError(err)
	}
	return view, renewed, nil
}

// ReleaseLease clears a lease only when its owner, attempt and fencing token
// are still current. A stale worker therefore cannot release a recovered task.
func (r *GenerationRepository) ReleaseLease(ctx context.Context, lease generationapp.Lease, at time.Time) (generationapp.TaskView, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	if _, err := uuid.Parse(lease.TaskID); err != nil {
		return generationapp.TaskView{}, generationapp.ErrInvalidGenerationLease
	}
	var view generationapp.TaskView
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", lease.TaskID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		previousRevision := task.StatusRevision
		if err := task.ReleaseLease(lease, at); err != nil {
			return err
		}
		if err := updateGenerationTask(tx, task, previousRevision); err != nil {
			return err
		}
		persisted, err := generationTaskByID(tx, lease.TaskID)
		if err != nil {
			return err
		}
		view, err = r.readTaskView(tx, persisted)
		return err
	})
	if err != nil {
		return generationapp.TaskView{}, generationGenerationError(err)
	}
	return view, nil
}

func generationTaskByID(database *gorm.DB, taskID string) (generationapp.Task, error) {
	var record generationJobRecord
	if err := database.Where("id = ?", taskID).First(&record).Error; err != nil {
		return generationapp.Task{}, generationLookupError(err)
	}
	return generationTaskFromRecord(record)
}

func updateGenerationTask(database *gorm.DB, task generationapp.Task, previousRevision int) error {
	updated := database.Model(&generationJobRecord{}).Where("id = ? AND status_revision = ?", task.ID, previousRevision).Updates(map[string]any{
		"status":                string(task.Status),
		"status_revision":       task.StatusRevision,
		"submission_state":      string(task.SubmissionState),
		"submission_attempt":    task.SubmissionAttempt,
		"submission_started_at": task.SubmissionStartedAt,
		"submission_unknown_at": task.SubmissionUnknownAt,
		"cancel_requested_at":   task.CancelRequestedAt,
		"external_task_id":      task.ExternalTaskID,
		"result_asset_id":       task.ResultAssetID,
		"failure_code":          task.FailureCode,
		"lease_owner":           task.LeaseOwner,
		"fencing_token":         task.FencingToken,
		"lease_attempt":         task.LeaseAttempt,
		"lease_until":           task.LeaseUntil,
		"updated_at":            task.UpdatedAt,
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return generationapp.ErrGenerationLeaseConflict
	}
	return nil
}

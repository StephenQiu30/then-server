package postgres

import (
	"context"
	"errors"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ generationapp.SubmissionWorkerRepository = (*GenerationRepository)(nil)
var _ generationapp.ObservationWorkerRepository = (*GenerationRepository)(nil)

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

// AcquireObservationLease claims only a task that already has an accepted
// provider identity. The query worker must not turn a queued task into
// running merely because it was selected by a broad scheduler.
func (r *GenerationRepository) AcquireObservationLease(ctx context.Context, taskID, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, error) {
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
		if task.ExternalTaskID == "" || task.SubmissionState != generationapp.SubmissionAccepted {
			return generationapp.ErrGenerationObservationUnavailable
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
		return generationapp.TaskView{}, generationapp.Lease{}, generationWorkerError(err)
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

// BeginSubmission marks a leased task in flight and returns the immutable
// provider-neutral submission payload. The caller must persist this mutation
// before making any external request.
func (r *GenerationRepository) BeginSubmission(ctx context.Context, lease generationapp.Lease, at time.Time) (generationapp.TaskView, generationapp.Submission, error) {
	var submission generationapp.Submission
	view, err := r.mutateLeasedTask(ctx, lease, at, func(task *generationapp.Task) error {
		var beginErr error
		submission, beginErr = task.BeginSubmission(at)
		return beginErr
	})
	if err != nil {
		return generationapp.TaskView{}, generationapp.Submission{}, err
	}
	return view, submission, nil
}

// MarkSubmissionUnknown persists a transport outcome that cannot prove
// whether the external service accepted the request. It deliberately keeps
// the task blocked until ReconcileSubmissionNotAccepted or RecordExternalTaskID.
func (r *GenerationRepository) MarkSubmissionUnknown(ctx context.Context, lease generationapp.Lease, at time.Time) (generationapp.TaskView, error) {
	return r.mutateLeasedTask(ctx, lease, at, func(task *generationapp.Task) error {
		return task.MarkSubmissionUnknown(at)
	})
}

// ReconcileSubmissionNotAccepted clears an unknown submission only after the
// caller has evidence that no external task was accepted.
func (r *GenerationRepository) ReconcileSubmissionNotAccepted(ctx context.Context, lease generationapp.Lease, at time.Time) (generationapp.TaskView, error) {
	return r.mutateLeasedTask(ctx, lease, at, func(task *generationapp.Task) error {
		return task.ReconcileSubmissionNotAccepted(at)
	})
}

// ScheduleSubmissionRetry records a finite backoff after explicit
// reconciliation proved that no external task was accepted. The worker can
// release the returned lease afterwards; no provider call occurs here.
func (r *GenerationRepository) ScheduleSubmissionRetry(ctx context.Context, lease generationapp.Lease, at time.Time, policy generationapp.RetryPolicy) (generationapp.TaskView, error) {
	return r.mutateLeasedTask(ctx, lease, at, func(task *generationapp.Task) error {
		return task.ScheduleSubmissionRetry(at, policy)
	})
}

// RecordExternalTaskID attaches the first external identity. A terminal task
// may retain a late identity for cleanup, but the fencing token must still
// match the worker attempt that submitted it.
func (r *GenerationRepository) RecordExternalTaskID(ctx context.Context, lease generationapp.Lease, externalID string, at time.Time) (generationapp.TaskView, error) {
	return r.mutateLeasedTask(ctx, lease, at, func(task *generationapp.Task) error {
		return task.RecordExternalTaskID(externalID, at)
	})
}

// ApplyProviderState records a non-terminal provider observation after the
// adapter has mapped it to the domain state machine. Terminal settlement is a
// separate atomic operation and is intentionally not performed here.
func (r *GenerationRepository) ApplyProviderState(ctx context.Context, lease generationapp.Lease, externalID string, next generationapp.Status, failureCode string, at time.Time) (generationapp.TaskView, error) {
	return r.mutateLeasedTask(ctx, lease, at, func(task *generationapp.Task) error {
		return task.ApplyProviderState(externalID, next, failureCode, at)
	})
}

func (r *GenerationRepository) mutateLeasedTask(ctx context.Context, lease generationapp.Lease, at time.Time, mutate func(*generationapp.Task) error) (generationapp.TaskView, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	if mutate == nil {
		return generationapp.TaskView{}, generationapp.ErrInvalidGenerationState
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
		if err := validateGenerationWorkerProof(task, lease, at); err != nil {
			return err
		}
		previousRevision := task.StatusRevision
		if err := mutate(&task); err != nil {
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
		return generationapp.TaskView{}, generationWorkerError(err)
	}
	return view, nil
}

func validateGenerationWorkerProof(task generationapp.Task, lease generationapp.Lease, at time.Time) error {
	if task.Status.Terminal() {
		if lease.TaskID != task.ID || lease.Owner == "" || lease.FencingToken == 0 || lease.Attempt < 1 || task.FencingToken != lease.FencingToken || task.LeaseAttempt != lease.Attempt {
			return generationapp.ErrGenerationLeaseConflict
		}
		return nil
	}
	return task.ValidateLease(lease, at)
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
		"next_attempt_at":       task.NextAttemptAt,
		"cancel_requested_at":   task.CancelRequestedAt,
		"access_revoked_at":     task.AccessRevokedAt,
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

func generationWorkerError(err error) error {
	if errors.Is(err, generationapp.ErrGenerationNotFound) ||
		errors.Is(err, generationapp.ErrGenerationUnavailable) ||
		errors.Is(err, generationapp.ErrInvalidGenerationInput) ||
		errors.Is(err, generationapp.ErrInvalidGenerationOutput) ||
		errors.Is(err, generationapp.ErrInvalidGenerationSettlement) ||
		errors.Is(err, generationapp.ErrGenerationSettlementConflict) ||
		errors.Is(err, generationapp.ErrInvalidQuotaReservation) ||
		errors.Is(err, generationapp.ErrQuotaReservationClosed) ||
		errors.Is(err, generationapp.ErrGenerationNotSubmittable) ||
		errors.Is(err, generationapp.ErrGenerationRetryNotReady) ||
		errors.Is(err, generationapp.ErrGenerationRetryExhausted) ||
		errors.Is(err, generationapp.ErrGenerationObservationUnavailable) ||
		errors.Is(err, generationapp.ErrSubmissionInProgress) ||
		errors.Is(err, generationapp.ErrSubmissionOutcomeUnknown) ||
		errors.Is(err, generationapp.ErrExternalTaskConflict) ||
		errors.Is(err, generationapp.ErrInvalidGenerationState) ||
		errors.Is(err, generationapp.ErrGenerationLeaseHeld) ||
		errors.Is(err, generationapp.ErrGenerationLeaseExpired) ||
		errors.Is(err, generationapp.ErrGenerationLeaseConflict) ||
		errors.Is(err, generationapp.ErrInvalidGenerationLease) {
		return err
	}
	return generationapp.ErrGenerationUnavailable
}

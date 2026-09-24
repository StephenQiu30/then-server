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
var _ generationapp.SubmissionWorkerQueue = (*GenerationRepository)(nil)
var _ generationapp.ObservationWorkerRepository = (*GenerationRepository)(nil)
var _ generationapp.ObservationWorkerQueue = (*GenerationRepository)(nil)
var _ generationapp.ResultWorkerRepository = (*GenerationRepository)(nil)
var _ generationapp.ResultWorkerQueue = (*GenerationRepository)(nil)

// AcquireLease claims one active generation task for a bounded worker
// interval. It contains no provider call; the lease only fences later worker
// state writes.
func (r *GenerationRepository) AcquireLease(ctx context.Context, taskID, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, error) {
	return r.acquireLease(ctx, taskID, owner, at, ttl, nil)
}

func (r *GenerationRepository) acquireLease(ctx context.Context, taskID, owner string, at time.Time, ttl time.Duration, eligible func(generationapp.Task) error) (generationapp.TaskView, generationapp.Lease, error) {
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
		if eligible != nil {
			if err := eligible(task); err != nil {
				return err
			}
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

// AcquireObservationLease claims only a task that already has an accepted
// provider identity. The query worker must not turn a queued task into
// running merely because it was selected by a broad scheduler.
func (r *GenerationRepository) AcquireObservationLease(ctx context.Context, taskID, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, error) {
	return r.acquireLease(ctx, taskID, owner, at, ttl, func(task generationapp.Task) error {
		if task.ExternalTaskID == "" || task.SubmissionState != generationapp.SubmissionAccepted {
			return generationapp.ErrGenerationObservationUnavailable
		}
		return nil
	})
}

// AcquireResultLease claims only a validating task with an accepted provider
// identity. It prevents a broad worker scan from publishing an output for a
// queued or merely running task.
func (r *GenerationRepository) AcquireResultLease(ctx context.Context, taskID, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, error) {
	return r.acquireLease(ctx, taskID, owner, at, ttl, func(task generationapp.Task) error {
		if task.Status != generationapp.StatusValidating || task.ExternalTaskID == "" || task.SubmissionState != generationapp.SubmissionAccepted {
			return generationapp.ErrGenerationResultUnavailable
		}
		return nil
	})
}

// ClaimNextSubmissionLease atomically selects one ready submission task. The
// row lock and lease predicate make concurrent workers skip each other, while
// the domain lease still fences a worker recovered after expiry.
func (r *GenerationRepository) ClaimNextSubmissionLease(ctx context.Context, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, bool, error) {
	return r.claimNextLease(ctx, owner, at, ttl, func(query *gorm.DB) *gorm.DB {
		return query.Where("status IN ? AND external_task_id = '' AND access_revoked_at IS NULL AND (lease_until IS NULL OR lease_until <= ?) AND ((submission_state = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (submission_state = ? AND lease_until IS NOT NULL))",
			[]string{string(generationapp.StatusQueued), string(generationapp.StatusRunning)}, at, string(generationapp.SubmissionNotStarted), at, string(generationapp.SubmissionInFlight))
	})
}

// ClaimNextObservationLease atomically selects one running task with an
// accepted provider identity. Validating tasks are left to the result queue;
// this prevents status polling from competing with output publication.
func (r *GenerationRepository) ClaimNextObservationLease(ctx context.Context, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, bool, error) {
	return r.claimNextLease(ctx, owner, at, ttl, func(query *gorm.DB) *gorm.DB {
		return query.Where("status = ? AND submission_state = ? AND external_task_id <> '' AND access_revoked_at IS NULL AND (lease_until IS NULL OR lease_until <= ?)",
			string(generationapp.StatusRunning), string(generationapp.SubmissionAccepted), at)
	})
}

// ClaimNextResultLease atomically selects one validating task whose provider
// identity is already accepted. Revoked tasks stay available to cleanup but
// can never trigger another output fetch.
func (r *GenerationRepository) ClaimNextResultLease(ctx context.Context, owner string, at time.Time, ttl time.Duration) (generationapp.TaskView, generationapp.Lease, bool, error) {
	return r.claimNextLease(ctx, owner, at, ttl, func(query *gorm.DB) *gorm.DB {
		return query.Where("status = ? AND submission_state = ? AND external_task_id <> '' AND access_revoked_at IS NULL AND (lease_until IS NULL OR lease_until <= ?)",
			string(generationapp.StatusValidating), string(generationapp.SubmissionAccepted), at)
	})
}

func (r *GenerationRepository) claimNextLease(ctx context.Context, owner string, at time.Time, ttl time.Duration, filter func(*gorm.DB) *gorm.DB) (generationapp.TaskView, generationapp.Lease, bool, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.Lease{}, false, generationapp.ErrGenerationUnavailable
	}
	if owner == "" || at.IsZero() || ttl <= 0 || filter == nil {
		return generationapp.TaskView{}, generationapp.Lease{}, false, generationapp.ErrInvalidGenerationLease
	}
	var view generationapp.TaskView
	var lease generationapp.Lease
	found := false
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		query := filter(tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})).
			Order("created_at ASC").Order("id ASC").Limit(1)
		if err := query.First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		found = true
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		previousRevision := task.StatusRevision
		lease, err = task.AcquireLease(owner, at, ttl)
		if err != nil {
			return err
		}
		if err := updateGenerationTask(tx, task, previousRevision); err != nil {
			return err
		}
		persisted, err := generationTaskByID(tx, task.ID)
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
		return generationapp.TaskView{}, generationapp.Lease{}, false, generationWorkerError(err)
	}
	return view, lease, found, nil
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
	return r.mutateLeasedTaskWithHook(ctx, lease, at, func(task *generationapp.Task) error {
		return task.RecordExternalTaskID(externalID, at)
	}, func(tx *gorm.DB, task generationapp.Task, mutationAt time.Time) error {
		return syncGenerationCleanupTargetsInTx(tx, task, mutationAt)
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
	return r.mutateLeasedTaskWithHook(ctx, lease, at, mutate, nil)
}

func (r *GenerationRepository) mutateLeasedTaskWithHook(ctx context.Context, lease generationapp.Lease, at time.Time, mutate func(*generationapp.Task) error, after func(*gorm.DB, generationapp.Task, time.Time) error) (generationapp.TaskView, error) {
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
		if after != nil {
			if err := after(tx, task, at); err != nil {
				return err
			}
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
		errors.Is(err, generationapp.ErrGenerationResultUnavailable) ||
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

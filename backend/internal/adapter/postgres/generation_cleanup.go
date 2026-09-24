package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type generationCleanupRequestRecord struct {
	ID                string     `gorm:"column:id;type:uuid;primaryKey"`
	OwnerID           string     `gorm:"column:owner_id;type:uuid;not null;index:generation_cleanup_owner_status_idx,priority:1"`
	TaskID            string     `gorm:"column:task_id;type:uuid;not null;index:generation_cleanup_task_idx;uniqueIndex:generation_cleanup_task_scope_unique,priority:1"`
	Scope             string     `gorm:"column:scope;type:text;not null;uniqueIndex:generation_cleanup_task_scope_unique,priority:2;check:generation_cleanup_scope_check,scope IN ('task','source','account')"`
	SourceMediaID     *string    `gorm:"column:source_media_id;type:uuid;index:generation_cleanup_source_media_idx"`
	AccountDeletionID *string    `gorm:"column:account_deletion_id;type:uuid;index:generation_cleanup_account_deletion_idx"`
	Status            string     `gorm:"column:status;type:text;not null;index:generation_cleanup_owner_status_idx,priority:2;check:generation_cleanup_status_check,status IN ('pending','running','complete','failed')"`
	AccessRevokedAt   time.Time  `gorm:"column:access_revoked_at;type:timestamptz;not null"`
	CompletedAt       *time.Time `gorm:"column:completed_at;type:timestamptz"`
	StableError       string     `gorm:"column:stable_error;type:text;not null;default:''"`
	Attempts          int        `gorm:"column:attempts;not null;default:0;check:generation_cleanup_attempts_check,attempts >= 0 AND attempts <= 100"`
	NextAttemptAt     *time.Time `gorm:"column:next_attempt_at;type:timestamptz;index:generation_cleanup_next_attempt_idx"`
	Targets           []byte     `gorm:"column:targets;type:jsonb;not null"`
	CreatedAt         time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;type:timestamptz;not null;check:generation_cleanup_timestamps_check,updated_at >= created_at"`
}

func (generationCleanupRequestRecord) TableName() string { return "generation_cleanup_requests" }

// RequestTaskCleanup revokes owner-facing visibility and records every known
// object/provider target in one transaction. It intentionally does not call a
// provider or object store; the durable request is the business boundary that
// a later local worker can execute without losing evidence.
func (r *GenerationRepository) RequestTaskCleanup(ctx context.Context, ownerID, taskID string, at time.Time) (generationapp.TaskView, generationapp.CleanupRequest, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.CleanupRequest{}, generationapp.ErrGenerationUnavailable
	}
	if _, err := uuid.Parse(taskID); err != nil {
		return generationapp.TaskView{}, generationapp.CleanupRequest{}, generationapp.ErrInvalidGenerationInput
	}
	var view generationapp.TaskView
	var request generationapp.CleanupRequest
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, taskID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		request, err = ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeTask, "", "", at)
		if err != nil {
			return err
		}
		if task.Purpose == generationapp.PurposeImage && task.ResultAssetID != "" {
			if err := requestDependentModelCleanupInTx(tx, task.OwnerID, task.ResultAssetID, at); err != nil {
				return err
			}
		}
		persisted, err := generationTaskByID(tx, task.ID)
		if err != nil {
			return err
		}
		view, err = r.readTaskView(tx, persisted)
		return err
	})
	if err != nil {
		return generationapp.TaskView{}, generationapp.CleanupRequest{}, generationGenerationError(err)
	}
	return view, request, nil
}

func ensureGenerationCleanupInTx(tx *gorm.DB, task generationapp.Task, scope generationapp.CleanupScope, sourceMediaID, accountDeletionID string, at time.Time) (generationapp.CleanupRequest, error) {
	var existing generationCleanupRequestRecord
	err := tx.Where("task_id = ? AND scope = ?", task.ID, string(scope)).First(&existing).Error
	if err == nil {
		return generationCleanupFromRecord(existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return generationapp.CleanupRequest{}, err
	}
	mutationAt := at
	if mutationAt.Before(task.UpdatedAt) {
		mutationAt = task.UpdatedAt
	}
	oldRevision := task.StatusRevision
	if task.AccessRevokedAt == nil {
		if err := task.RevokeAccess(mutationAt); err != nil {
			return generationapp.CleanupRequest{}, err
		}
	}
	if !task.Status.Terminal() && task.CancelRequestedAt == nil {
		if err := task.RequestCancel(mutationAt); err != nil {
			return generationapp.CleanupRequest{}, err
		}
	}
	if task.StatusRevision != oldRevision {
		if err := updateGenerationTask(tx, task, oldRevision); err != nil {
			return generationapp.CleanupRequest{}, err
		}
	}
	targets, err := generationCleanupTargets(tx, task)
	if err != nil {
		return generationapp.CleanupRequest{}, err
	}
	request, err := generationapp.NewCleanupRequest(uuid.NewString(), task, scope, targets, mutationAt)
	if err != nil {
		return generationapp.CleanupRequest{}, err
	}
	request.SourceMediaID = sourceMediaID
	request.AccountDeletionID = accountDeletionID
	if err := request.Validate(); err != nil {
		return generationapp.CleanupRequest{}, err
	}
	record, err := generationCleanupRecordFromDomain(request)
	if err != nil {
		return generationapp.CleanupRequest{}, err
	}
	if err := tx.Create(&record).Error; err != nil {
		return generationapp.CleanupRequest{}, err
	}
	return request, nil
}

func generationCleanupTargets(database *gorm.DB, task generationapp.Task) ([]generationapp.CleanupTarget, error) {
	targets := make([]generationapp.CleanupTarget, 0, 2)
	var output generationOutputRecord
	if err := database.Where("task_id = ?", task.ID).First(&output).Error; err == nil {
		targets = append(targets, generationapp.CleanupTarget{Kind: generationapp.CleanupTargetObject, ID: output.ID, ObjectVersionID: output.ObjectVersionID})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if task.ExternalTaskID != "" {
		targets = append(targets, generationapp.CleanupTarget{Kind: generationapp.CleanupTargetProvider, ID: task.ExternalTaskID})
	}
	return targets, nil
}

// requestGenerationSourceCleanupInTx revokes every task that captured a
// source media reference. It is called from the media deletion transaction so
// the source read right and all dependent generation reads close together.
func requestGenerationSourceCleanupInTx(tx *gorm.DB, ownerID, mediaID string, at time.Time) error {
	var records []generationJobRecord
	if err := tx.Where("owner_id = ?", ownerID).Order("id ASC").Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		if !generationTaskReferencesMedia(task, mediaID) {
			continue
		}
		if _, err := ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeSource, mediaID, "", at); err != nil {
			return err
		}
	}
	return nil
}

// RequestSourceCleanup is the provider-neutral hook used by media deletion
// and by integration workers. It returns the durable requests created for all
// tasks that captured the source media reference.
func (r *GenerationRepository) RequestSourceCleanup(ctx context.Context, ownerID, mediaID string, at time.Time) ([]generationapp.CleanupRequest, error) {
	if r == nil || r.database == nil {
		return nil, generationapp.ErrGenerationUnavailable
	}
	var requests []generationapp.CleanupRequest
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var records []generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ?", ownerID).Order("id ASC").Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			task, err := generationTaskFromRecord(record)
			if err != nil {
				return err
			}
			if !generationTaskReferencesMedia(task, mediaID) {
				continue
			}
			request, err := ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeSource, mediaID, "", at)
			if err != nil {
				return err
			}
			requests = append(requests, request)
		}
		return nil
	})
	if err != nil {
		return nil, generationGenerationError(err)
	}
	return requests, nil
}

func requestGenerationAccountCleanupInTx(tx *gorm.DB, ownerID, accountDeletionID string, at time.Time) error {
	var records []generationJobRecord
	if err := tx.Where("owner_id = ?", ownerID).Order("id ASC").Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		if _, err := ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeAccount, "", accountDeletionID, at); err != nil {
			return err
		}
	}
	return nil
}

func requestDependentModelCleanupInTx(tx *gorm.DB, ownerID, imageAssetID string, at time.Time) error {
	var records []generationJobRecord
	if err := tx.Where("owner_id = ? AND purpose = ?", ownerID, string(generationapp.PurposeModel)).Order("id ASC").Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		if task.Inputs.ImageAssetID != imageAssetID {
			continue
		}
		if _, err := ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeTask, "", "", at); err != nil {
			return err
		}
	}
	return nil
}

func generationTaskReferencesMedia(task generationapp.Task, mediaID string) bool {
	for _, reference := range task.Inputs.References {
		if reference.MediaID == mediaID {
			return true
		}
	}
	return false
}

// GetCleanup returns the durable cleanup state for an owner-scoped task.
func (r *GenerationRepository) GetTaskCleanup(ctx context.Context, ownerID, taskID string) (generationapp.CleanupRequest, error) {
	if r == nil || r.database == nil {
		return generationapp.CleanupRequest{}, generationapp.ErrGenerationUnavailable
	}
	var record generationCleanupRequestRecord
	if err := r.database.WithContext(ctx).Where("owner_id = ? AND task_id = ? AND scope = ?", ownerID, taskID, string(generationapp.CleanupScopeTask)).First(&record).Error; err != nil {
		return generationapp.CleanupRequest{}, generationLookupError(err)
	}
	request, err := generationCleanupFromRecord(record)
	if err != nil {
		return generationapp.CleanupRequest{}, generationGenerationError(err)
	}
	return request, nil
}

// BeginTaskCleanup claims a pending or failed request. The caller performs
// target deletion after this transaction and completes or fails it explicitly.
func (r *GenerationRepository) BeginTaskCleanup(ctx context.Context, requestID string, at time.Time) (generationapp.CleanupRequest, []generationapp.CleanupTarget, error) {
	if r == nil || r.database == nil {
		return generationapp.CleanupRequest{}, nil, generationapp.ErrGenerationUnavailable
	}
	var request generationapp.CleanupRequest
	var targets []generationapp.CleanupTarget
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationCleanupRequestRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", requestID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		loaded, err := generationCleanupFromRecord(record)
		if err != nil {
			return err
		}
		targets, err = loaded.Begin(at)
		if err != nil {
			return err
		}
		if loaded.Status != generationapp.CleanupComplete {
			if err := updateGenerationCleanup(tx, loaded, record.Status, record.Attempts, record.UpdatedAt); err != nil {
				return err
			}
		}
		request = loaded
		return nil
	})
	if err != nil {
		return generationapp.CleanupRequest{}, nil, generationGenerationError(err)
	}
	return request, targets, nil
}

// ClaimNextCleanup atomically claims the oldest ready cleanup request. A
// stale running request is first converted to a retryable failure inside the
// same transaction; the optimistic status/attempt/timestamp predicate then
// prevents the previous worker from completing over the new claim.
func (r *GenerationRepository) ClaimNextCleanup(ctx context.Context, at time.Time, staleAfter time.Duration) (generationapp.CleanupRequest, []generationapp.CleanupTarget, bool, error) {
	if r == nil || r.database == nil || at.IsZero() || staleAfter <= 0 {
		return generationapp.CleanupRequest{}, nil, false, generationapp.ErrGenerationUnavailable
	}
	var request generationapp.CleanupRequest
	var targets []generationapp.CleanupTarget
	found := false
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationCleanupRequestRecord
		readyBefore := at.Add(-staleAfter)
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND updated_at <= ?)", []string{string(generationapp.CleanupPending), string(generationapp.CleanupFailed)}, at, string(generationapp.CleanupRunning), readyBefore).
			Order("created_at ASC").Order("id ASC")
		if err := query.First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		found = true
		loaded, err := generationCleanupFromRecord(record)
		if err != nil {
			return err
		}
		if loaded.Status == generationapp.CleanupRunning {
			previousStatus := string(loaded.Status)
			previousAttempts := loaded.Attempts
			previousUpdatedAt := loaded.UpdatedAt
			if err := loaded.Fail(at, "worker_expired", at); err != nil {
				return err
			}
			if err := updateGenerationCleanup(tx, loaded, previousStatus, previousAttempts, previousUpdatedAt); err != nil {
				return err
			}
		}
		previousStatus := string(loaded.Status)
		previousAttempts := loaded.Attempts
		previousUpdatedAt := loaded.UpdatedAt
		targets, err = loaded.Begin(at)
		if err != nil {
			return err
		}
		if err := updateGenerationCleanup(tx, loaded, previousStatus, previousAttempts, previousUpdatedAt); err != nil {
			return err
		}
		request = loaded
		return nil
	})
	if err != nil {
		return generationapp.CleanupRequest{}, nil, false, generationGenerationError(err)
	}
	return request, targets, found, nil
}

// CompleteTaskCleanup records a successful deletion and removes the output
// metadata only after the worker has finished its external target work. The
// claimed attempts/timestamp act as a fencing proof after stale recovery.
func (r *GenerationRepository) CompleteTaskCleanup(ctx context.Context, claimed generationapp.CleanupRequest, at time.Time) (generationapp.CleanupRequest, error) {
	if r == nil || r.database == nil {
		return generationapp.CleanupRequest{}, generationapp.ErrGenerationUnavailable
	}
	if err := claimed.Validate(); err != nil {
		return generationapp.CleanupRequest{}, generationapp.ErrInvalidGenerationCleanup
	}
	var request generationapp.CleanupRequest
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationCleanupRequestRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", claimed.ID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		loaded, err := generationCleanupFromRecord(record)
		if err != nil {
			return err
		}
		if !generationCleanupClaimMatches(loaded, claimed) {
			return generationapp.ErrGenerationCleanupClaim
		}
		if err := loaded.Complete(at); err != nil {
			return err
		}
		if err := updateGenerationCleanup(tx, loaded, record.Status, record.Attempts, record.UpdatedAt); err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", loaded.TaskID).Delete(&generationOutputRecord{}).Error; err != nil {
			return err
		}
		if loaded.AccountDeletionID != "" {
			if err := finalizeAccountDeletionIfReady(ctx, tx, loaded.OwnerID, at); err != nil {
				return err
			}
		}
		request = loaded
		return nil
	})
	if err != nil {
		return generationapp.CleanupRequest{}, generationGenerationError(err)
	}
	return request, nil
}

func (r *GenerationRepository) FailTaskCleanup(ctx context.Context, claimed generationapp.CleanupRequest, at time.Time, stableError string, retryAt time.Time) (generationapp.CleanupRequest, error) {
	if r == nil || r.database == nil {
		return generationapp.CleanupRequest{}, generationapp.ErrGenerationUnavailable
	}
	if err := claimed.Validate(); err != nil {
		return generationapp.CleanupRequest{}, generationapp.ErrInvalidGenerationCleanup
	}
	var request generationapp.CleanupRequest
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationCleanupRequestRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", claimed.ID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		loaded, err := generationCleanupFromRecord(record)
		if err != nil {
			return err
		}
		if !generationCleanupClaimMatches(loaded, claimed) {
			return generationapp.ErrGenerationCleanupClaim
		}
		if err := loaded.Fail(at, stableError, retryAt); err != nil {
			return err
		}
		if err := updateGenerationCleanup(tx, loaded, record.Status, record.Attempts, record.UpdatedAt); err != nil {
			return err
		}
		request = loaded
		return nil
	})
	if err != nil {
		return generationapp.CleanupRequest{}, generationGenerationError(err)
	}
	return request, nil
}

func generationCleanupClaimMatches(current, claimed generationapp.CleanupRequest) bool {
	return current.ID == claimed.ID && current.Status == generationapp.CleanupRunning && claimed.Status == generationapp.CleanupRunning && current.Attempts == claimed.Attempts && current.UpdatedAt.Equal(claimed.UpdatedAt)
}

func generationCleanupRecordFromDomain(request generationapp.CleanupRequest) (generationCleanupRequestRecord, error) {
	if err := request.Validate(); err != nil {
		return generationCleanupRequestRecord{}, err
	}
	targets, err := json.Marshal(request.Targets)
	if err != nil {
		return generationCleanupRequestRecord{}, generationapp.ErrInvalidGenerationCleanup
	}
	var sourceMediaID, accountDeletionID *string
	if request.SourceMediaID != "" {
		value := request.SourceMediaID
		sourceMediaID = &value
	}
	if request.AccountDeletionID != "" {
		value := request.AccountDeletionID
		accountDeletionID = &value
	}
	return generationCleanupRequestRecord{ID: request.ID, OwnerID: request.OwnerID, TaskID: request.TaskID, Scope: string(request.Scope), SourceMediaID: sourceMediaID, AccountDeletionID: accountDeletionID, Status: string(request.Status), AccessRevokedAt: request.AccessRevokedAt, CompletedAt: request.CompletedAt, StableError: request.StableError, Attempts: request.Attempts, NextAttemptAt: request.NextAttemptAt, Targets: targets, CreatedAt: request.CreatedAt, UpdatedAt: request.UpdatedAt}, nil
}

func generationCleanupFromRecord(record generationCleanupRequestRecord) (generationapp.CleanupRequest, error) {
	var targets []generationapp.CleanupTarget
	if err := json.Unmarshal(record.Targets, &targets); err != nil {
		return generationapp.CleanupRequest{}, generationapp.ErrInvalidGenerationCleanup
	}
	request := generationapp.CleanupRequest{ID: record.ID, OwnerID: record.OwnerID, TaskID: record.TaskID, Scope: generationapp.CleanupScope(record.Scope), Status: generationapp.CleanupStatus(record.Status), AccessRevokedAt: record.AccessRevokedAt, CompletedAt: record.CompletedAt, StableError: record.StableError, Attempts: record.Attempts, NextAttemptAt: record.NextAttemptAt, Targets: targets, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if record.SourceMediaID != nil {
		request.SourceMediaID = *record.SourceMediaID
	}
	if record.AccountDeletionID != nil {
		request.AccountDeletionID = *record.AccountDeletionID
	}
	if err := request.Validate(); err != nil {
		return generationapp.CleanupRequest{}, err
	}
	return request, nil
}

func updateGenerationCleanup(database *gorm.DB, request generationapp.CleanupRequest, previousStatus string, previousAttempts int, previousUpdatedAt time.Time) error {
	record, err := generationCleanupRecordFromDomain(request)
	if err != nil {
		return err
	}
	updated := database.Model(&generationCleanupRequestRecord{}).Where("id = ? AND status = ? AND attempts = ? AND updated_at = ?", request.ID, previousStatus, previousAttempts, previousUpdatedAt).Updates(map[string]any{
		"status":          record.Status,
		"completed_at":    record.CompletedAt,
		"stable_error":    record.StableError,
		"attempts":        record.Attempts,
		"next_attempt_at": record.NextAttemptAt,
		"targets":         record.Targets,
		"updated_at":      record.UpdatedAt,
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return generationapp.ErrGenerationUnavailable
	}
	return nil
}

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
	Scope             string     `gorm:"column:scope;type:text;not null;uniqueIndex:generation_cleanup_task_scope_unique,priority:2;check:generation_cleanup_scope_check,scope IN ('task','source','account','orphan_output')"`
	SourceMediaID     *string    `gorm:"column:source_media_id;type:uuid;index:generation_cleanup_source_media_idx"`
	AccountDeletionID *string    `gorm:"column:account_deletion_id;type:uuid;index:generation_cleanup_account_deletion_idx"`
	Status            string     `gorm:"column:status;type:text;not null;index:generation_cleanup_owner_status_idx,priority:2;check:generation_cleanup_status_check,status IN ('pending','running','complete','failed')"`
	AccessRevokedAt   *time.Time `gorm:"column:access_revoked_at;type:timestamptz"`
	CompletedAt       *time.Time `gorm:"column:completed_at;type:timestamptz"`
	StableError       string     `gorm:"column:stable_error;type:text;not null;default:''"`
	Attempts          int        `gorm:"column:attempts;not null;default:0;check:generation_cleanup_attempts_check,attempts >= 0 AND attempts <= 100"`
	NextAttemptAt     *time.Time `gorm:"column:next_attempt_at;type:timestamptz;index:generation_cleanup_next_attempt_idx"`
	Targets           []byte     `gorm:"column:targets;type:jsonb;not null"`
	CreatedAt         time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;type:timestamptz;not null;check:generation_cleanup_timestamps_check,updated_at >= created_at"`
}

func (generationCleanupRequestRecord) TableName() string { return "generation_cleanup_requests" }

// RecordUnpublishedOutput durably queues one exact private object version when
// result verification or publication fails. The task lease fences the record;
// an already-published version is left untouched to handle ambiguous commits.
func (r *GenerationRepository) RecordUnpublishedOutput(ctx context.Context, lease generationapp.Lease, target generationapp.CleanupTarget, at time.Time) error {
	if r == nil || r.database == nil || at.IsZero() {
		return generationapp.ErrGenerationUnavailable
	}
	if _, err := parseGenerationLeaseTaskID(lease); err != nil {
		return err
	}
	if err := target.Validate(); err != nil || target.Kind != generationapp.CleanupTargetObject || target.ObjectKey == "" {
		return generationapp.ErrInvalidGenerationCleanup
	}
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var taskRecord generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", lease.TaskID).First(&taskRecord).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(taskRecord)
		if err != nil {
			return err
		}
		if err := validateGenerationWorkerProof(task, lease, at); err != nil {
			return err
		}
		expectedKey, err := generationapp.OutputObjectKey(task)
		if err != nil || target.ObjectKey != expectedKey {
			return generationapp.ErrInvalidGenerationCleanup
		}
		var published generationOutputRecord
		if err := tx.Where("task_id = ? AND object_key = ? AND object_version_id = ?", task.ID, target.ObjectKey, target.ObjectVersionID).First(&published).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		mutationAt := at.UTC()
		if mutationAt.Before(task.UpdatedAt) {
			mutationAt = task.UpdatedAt
		}
		if task.AccessRevokedAt == nil {
			return recordOrphanGenerationOutputInTx(tx, task, target, mutationAt)
		}
		return addTargetToRevokedGenerationCleanupInTx(tx, task, target, mutationAt)
	})
	if err != nil {
		return generationGenerationError(err)
	}
	return nil
}

func recordOrphanGenerationOutputInTx(tx *gorm.DB, task generationapp.Task, target generationapp.CleanupTarget, at time.Time) error {
	var record generationCleanupRequestRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ? AND scope = ?", task.ID, string(generationapp.CleanupScopeOrphanOutput)).First(&record).Error
	if err == nil {
		request, err := generationCleanupFromRecord(record)
		if err != nil {
			return err
		}
		if err := request.AddTarget(target, at); err != nil {
			return err
		}
		return updateGenerationCleanup(tx, request, record.Status, record.Attempts, record.UpdatedAt)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	request, err := generationapp.NewOrphanOutputCleanupRequest(uuid.NewString(), task, target, at)
	if err != nil {
		return err
	}
	record, err = generationCleanupRecordFromDomain(request)
	if err != nil {
		return err
	}
	return tx.Create(&record).Error
}

func addTargetToRevokedGenerationCleanupInTx(tx *gorm.DB, task generationapp.Task, target generationapp.CleanupTarget, at time.Time) error {
	var records []generationCleanupRequestRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ? AND scope IN ?", task.ID, []string{string(generationapp.CleanupScopeTask), string(generationapp.CleanupScopeSource), string(generationapp.CleanupScopeAccount)}).Order("created_at ASC").Find(&records).Error; err != nil {
		return err
	}
	if len(records) == 0 {
		if _, err := ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeTask, "", "", at); err != nil {
			return err
		}
		return addTargetToRevokedGenerationCleanupInTx(tx, task, target, at)
	}
	for _, record := range records {
		request, err := generationCleanupFromRecord(record)
		if err != nil {
			return err
		}
		mutationAt := at
		if mutationAt.Before(request.UpdatedAt) {
			mutationAt = request.UpdatedAt
		}
		previousTargetCount := len(request.Targets)
		if err := request.AddTarget(target, mutationAt); err != nil {
			return err
		}
		if len(request.Targets) == previousTargetCount {
			continue
		}
		if err := updateGenerationCleanup(tx, request, record.Status, record.Attempts, record.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

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
		request, err := generationCleanupFromRecord(existing)
		if err != nil {
			return generationapp.CleanupRequest{}, err
		}
		if scope == generationapp.CleanupScopeAccount && accountDeletionID != "" {
			if err := attachOrphanGenerationCleanupToAccountInTx(tx, task.ID, accountDeletionID, at); err != nil {
				return generationapp.CleanupRequest{}, err
			}
		}
		return request, nil
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
		settledTask, finalized, err := settleUnsubmittedGenerationCancellationInTx(tx, task, oldRevision, mutationAt)
		if err != nil {
			return generationapp.CleanupRequest{}, err
		}
		if finalized {
			task = settledTask
		} else if err := updateGenerationTask(tx, task, oldRevision); err != nil {
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
	if scope == generationapp.CleanupScopeAccount && accountDeletionID != "" {
		if err := attachOrphanGenerationCleanupToAccountInTx(tx, task.ID, accountDeletionID, mutationAt); err != nil {
			return generationapp.CleanupRequest{}, err
		}
	}
	return request, nil
}

func attachOrphanGenerationCleanupToAccountInTx(tx *gorm.DB, taskID, accountDeletionID string, at time.Time) error {
	var records []generationCleanupRequestRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ? AND scope = ?", taskID, string(generationapp.CleanupScopeOrphanOutput)).Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		if record.AccountDeletionID != nil && *record.AccountDeletionID == accountDeletionID {
			continue
		}
		mutationAt := at.UTC()
		if mutationAt.Before(record.UpdatedAt) {
			mutationAt = record.UpdatedAt
		}
		updated := tx.Model(&generationCleanupRequestRecord{}).Where("id = ? AND status = ? AND attempts = ? AND updated_at = ?", record.ID, record.Status, record.Attempts, record.UpdatedAt).Updates(map[string]any{
			"account_deletion_id": accountDeletionID,
			"updated_at":          mutationAt,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return generationapp.ErrGenerationUnavailable
		}
	}
	return nil
}

func generationCleanupTargets(database *gorm.DB, task generationapp.Task) ([]generationapp.CleanupTarget, error) {
	targets := make([]generationapp.CleanupTarget, 0, 2)
	var output generationOutputRecord
	if err := database.Where("task_id = ?", task.ID).First(&output).Error; err == nil {
		targets = append(targets, generationapp.CleanupTarget{Kind: generationapp.CleanupTargetObject, ID: output.ID, ObjectKey: output.ObjectKey, ObjectVersionID: output.ObjectVersionID})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if task.ExternalTaskID != "" {
		targets = append(targets, generationapp.CleanupTarget{Kind: generationapp.CleanupTargetProvider, ID: task.ExternalTaskID, Provider: task.Provider})
	}
	return targets, nil
}

// syncGenerationCleanupTargetsInTx closes the late-acceptance gap between a
// task cleanup request and a provider submission that was already in flight.
// The task row and every cleanup request are locked by the same transaction;
// adding a newly discovered target reopens a running or completed request so
// an older worker cannot complete over the new target.
func syncGenerationCleanupTargetsInTx(tx *gorm.DB, task generationapp.Task, at time.Time) error {
	if task.AccessRevokedAt == nil || task.ExternalTaskID == "" {
		return nil
	}
	targets, err := generationCleanupTargets(tx, task)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	var records []generationCleanupRequestRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ?", task.ID).Order("created_at ASC").Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		request, err := generationCleanupFromRecord(record)
		if err != nil {
			return err
		}
		mutationAt := at
		if mutationAt.Before(task.UpdatedAt) {
			mutationAt = task.UpdatedAt
		}
		if mutationAt.Before(request.UpdatedAt) {
			mutationAt = request.UpdatedAt
		}
		changed := false
		for _, target := range targets {
			if generationCleanupTargetExists(request.Targets, target) {
				continue
			}
			if err := request.AddTarget(target, mutationAt); err != nil {
				return err
			}
			changed = true
		}
		if !changed {
			continue
		}
		if err := updateGenerationCleanup(tx, request, record.Status, record.Attempts, record.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

func generationCleanupTargetExists(targets []generationapp.CleanupTarget, target generationapp.CleanupTarget) bool {
	for _, existing := range targets {
		if generationapp.SameCleanupTarget(existing, target) {
			return true
		}
	}
	return false
}

// requestGenerationSourceCleanupInTx revokes every task that captured a
// source media reference. It is called from the media deletion transaction so
// the source read right and all dependent generation reads close together.
func requestGenerationSourceCleanupInTx(tx *gorm.DB, ownerID, mediaID string, at time.Time) error {
	return requestGenerationSourceCleanupForMediaInTx(tx, ownerID, []string{mediaID}, at)
}

func requestGenerationSourceCleanupForMediaInTx(tx *gorm.DB, ownerID string, mediaIDs []string, at time.Time) error {
	if len(mediaIDs) == 0 {
		return nil
	}
	sourceIDs := make(map[string]struct{}, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		sourceIDs[mediaID] = struct{}{}
	}
	var records []generationJobRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ?", ownerID).Order("id ASC").Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		for _, reference := range task.Inputs.References {
			if _, ok := sourceIDs[reference.MediaID]; !ok {
				continue
			}
			if err := requestGenerationSourceCleanupForTaskInTx(tx, task, reference.MediaID, at); err != nil {
				return err
			}
			break
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
			if err := requestGenerationDependentModelCleanupInTx(tx, task, at); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, generationGenerationError(err)
	}
	return requests, nil
}

func requestGenerationSourceCleanupForTaskInTx(tx *gorm.DB, task generationapp.Task, mediaID string, at time.Time) error {
	if !generationTaskReferencesMedia(task, mediaID) {
		return nil
	}
	if _, err := ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeSource, mediaID, "", at); err != nil {
		return err
	}
	return requestGenerationDependentModelCleanupInTx(tx, task, at)
}

func requestGenerationDependentModelCleanupInTx(tx *gorm.DB, task generationapp.Task, at time.Time) error {
	if task.Purpose != generationapp.PurposeImage || task.ResultAssetID == "" {
		return nil
	}
	return requestDependentModelCleanupInTx(tx, task.OwnerID, task.ResultAssetID, at)
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
	at = at.UTC().Truncate(time.Microsecond)
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
		if err := bindGenerationCleanupProviderInTx(tx, &loaded); err != nil {
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
	// PostgreSQL stores microseconds. Return the same timestamp that the
	// database persists so completion can match the claim on every platform.
	at = at.UTC().Truncate(time.Microsecond)
	var request generationapp.CleanupRequest
	var targets []generationapp.CleanupTarget
	found := false
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationCleanupRequestRecord
		readyBefore := at.Add(-staleAfter)
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(status IN ? AND stable_error <> ? AND attempts < ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND updated_at <= ?)", []string{string(generationapp.CleanupPending), string(generationapp.CleanupFailed)}, generationapp.CleanupIdentityMismatchCode, generationapp.MaxCleanupAttempts, at, string(generationapp.CleanupRunning), readyBefore).
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
		if err := bindGenerationCleanupProviderInTx(tx, &loaded); err != nil {
			if !errors.Is(err, generationapp.ErrInvalidGenerationCleanup) {
				return err
			}
			if err := loaded.RejectUnsafeTarget(at); err != nil {
				return err
			}
			if err := updateGenerationCleanup(tx, loaded, record.Status, record.Attempts, record.UpdatedAt); err != nil {
				return err
			}
			found = false
			return nil
		}
		if loaded.Status == generationapp.CleanupRunning {
			previousStatus := string(loaded.Status)
			previousAttempts := loaded.Attempts
			previousUpdatedAt := loaded.UpdatedAt
			if loaded.Attempts >= generationapp.MaxCleanupAttempts {
				if err := loaded.Exhaust(at); err != nil {
					return err
				}
				if err := updateGenerationCleanup(tx, loaded, previousStatus, previousAttempts, previousUpdatedAt); err != nil {
					return err
				}
				found = false
				return nil
			}
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

// Legacy manifests predate provider-qualified targets. Bind them to the
// immutable task provider before any external side effect and persist the
// qualified snapshot with the claim.
func bindGenerationCleanupProviderInTx(tx *gorm.DB, request *generationapp.CleanupRequest) error {
	needsProvider := false
	for _, target := range request.Targets {
		if target.Kind == generationapp.CleanupTargetProvider {
			needsProvider = true
			break
		}
	}
	if !needsProvider {
		return nil
	}
	var task generationJobRecord
	if err := tx.Select("provider", "external_task_id").Where("owner_id = ? AND id = ?", request.OwnerID, request.TaskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return generationapp.ErrInvalidGenerationCleanup
		}
		return err
	}
	if task.Provider == "" || task.ExternalTaskID == "" {
		return generationapp.ErrInvalidGenerationCleanup
	}
	candidate := *request
	candidate.Targets = append([]generationapp.CleanupTarget(nil), request.Targets...)
	for index := range candidate.Targets {
		target := &candidate.Targets[index]
		if target.Kind != generationapp.CleanupTargetProvider {
			continue
		}
		if target.ID != task.ExternalTaskID || (target.Provider != "" && target.Provider != task.Provider) {
			return generationapp.ErrInvalidGenerationCleanup
		}
		target.Provider = task.Provider
	}
	if err := candidate.Validate(); err != nil {
		return err
	}
	request.Targets = candidate.Targets
	return nil
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
		if loaded.Scope != generationapp.CleanupScopeOrphanOutput {
			if err := tx.Where("task_id = ?", loaded.TaskID).Delete(&generationOutputRecord{}).Error; err != nil {
				return err
			}
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

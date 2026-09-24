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

// PublishOutput commits one verified private output, the succeeded task state,
// and quota consumption in one transaction. The repository accepts object
// metadata only; downloading or validating provider bytes belongs to the
// worker/object-store adapter before this call.
func (r *GenerationRepository) PublishOutput(ctx context.Context, lease generationapp.Lease, asset generationapp.OutputAsset, at time.Time) (generationapp.TaskView, error) {
	return r.settleGenerationTask(ctx, lease, at, func(task generationapp.Task, reservation *generationapp.QuotaReservation) (generationapp.Settlement, error) {
		return generationapp.PublishOutput(task, reservation, asset, at)
	})
}

// FinalizeWithoutOutput commits a failed, canceled, or expired task and
// releases its quota hold. The worker proof is checked before the domain
// settlement and is carried into the conditional task update.
func (r *GenerationRepository) FinalizeWithoutOutput(ctx context.Context, lease generationapp.Lease, next generationapp.Status, failureCode string, at time.Time) (generationapp.TaskView, error) {
	return r.settleGenerationTask(ctx, lease, at, func(task generationapp.Task, reservation *generationapp.QuotaReservation) (generationapp.Settlement, error) {
		return generationapp.FinalizeWithoutOutput(task, reservation, next, failureCode, at)
	})
}

func (r *GenerationRepository) settleGenerationTask(ctx context.Context, lease generationapp.Lease, at time.Time, settle func(generationapp.Task, *generationapp.QuotaReservation) (generationapp.Settlement, error)) (generationapp.TaskView, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	if settle == nil {
		return generationapp.TaskView{}, generationapp.ErrInvalidGenerationSettlement
	}
	if _, err := parseGenerationLeaseTaskID(lease); err != nil {
		return generationapp.TaskView{}, err
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
		reservation, err := lockedGenerationReservation(tx, task.ID)
		if err != nil {
			return err
		}
		settlement, err := settle(task, reservation)
		if err != nil {
			return err
		}
		if settlement.FencingToken == 0 || settlement.FencingToken != task.FencingToken {
			return generationapp.ErrGenerationSettlementConflict
		}
		previousTaskRevision := task.StatusRevision
		previousReservationRevision := 0
		if reservation != nil {
			previousReservationRevision = reservation.StateRevision
		}
		if err := persistGenerationSettlement(tx, task, previousTaskRevision, reservation, previousReservationRevision, settlement); err != nil {
			return err
		}
		persisted, err := generationTaskByID(tx, task.ID)
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

func parseGenerationLeaseTaskID(lease generationapp.Lease) (string, error) {
	if _, err := uuid.Parse(lease.TaskID); err != nil {
		return "", generationapp.ErrInvalidGenerationLease
	}
	return lease.TaskID, nil
}

func lockedGenerationReservation(database *gorm.DB, taskID string) (*generationapp.QuotaReservation, error) {
	var record generationQuotaReservationRecord
	err := database.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ?", taskID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, generationapp.ErrGenerationUnavailable
	}
	reservation, err := generationReservationFromRecord(record)
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func persistGenerationSettlement(tx *gorm.DB, task generationapp.Task, previousTaskRevision int, reservation *generationapp.QuotaReservation, previousReservationRevision int, settlement generationapp.Settlement) error {
	if settlement.Reservation == nil && reservation != nil {
		return generationapp.ErrGenerationSettlementConflict
	}
	if settlement.Reservation != nil {
		if reservation == nil || settlement.Reservation.ID != reservation.ID || settlement.Reservation.TaskID != task.ID {
			return generationapp.ErrGenerationSettlementConflict
		}
		if err := updateGenerationReservation(tx, *settlement.Reservation, previousReservationRevision); err != nil {
			return err
		}
	}
	if settlement.Asset != nil {
		if err := persistGenerationOutput(tx, task, settlement); err != nil {
			return err
		}
	} else if err := ensureNoGenerationOutput(tx, task.ID); err != nil {
		return err
	}
	return updateGenerationTaskWithFencing(tx, settlement.Task, previousTaskRevision, settlement.FencingToken)
}

func updateGenerationTaskWithFencing(database *gorm.DB, task generationapp.Task, previousRevision int, fencingToken uint64) error {
	updated := database.Model(&generationJobRecord{}).Where("id = ? AND status_revision = ? AND fencing_token = ?", task.ID, previousRevision, fencingToken).Updates(map[string]any{
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

func updateGenerationReservation(database *gorm.DB, reservation generationapp.QuotaReservation, previousRevision int) error {
	updated := database.Model(&generationQuotaReservationRecord{}).Where("id = ? AND task_id = ? AND state_revision = ?", reservation.ID, reservation.TaskID, previousRevision).Updates(map[string]any{
		"state":          string(reservation.State),
		"state_revision": reservation.StateRevision,
		"updated_at":     reservation.UpdatedAt,
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return generationapp.ErrGenerationSettlementConflict
	}
	return nil
}

func persistGenerationOutput(tx *gorm.DB, task generationapp.Task, settlement generationapp.Settlement) error {
	if settlement.Asset == nil {
		return generationapp.ErrGenerationSettlementConflict
	}
	assetRecord, err := generationOutputRecordFromDomain(*settlement.Asset, settlement.Task)
	if err != nil {
		return err
	}
	var existing generationOutputRecord
	err = tx.Where("task_id = ?", task.ID).First(&existing).Error
	if settlement.Reused {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return generationapp.ErrGenerationSettlementConflict
		}
		if err != nil {
			return err
		}
		persisted, err := generationOutputFromRecord(existing, settlement.Task)
		if err != nil || !sameGenerationOutput(persisted, *settlement.Asset) {
			return generationapp.ErrGenerationSettlementConflict
		}
		return nil
	}
	if err == nil {
		return generationapp.ErrGenerationSettlementConflict
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&assetRecord).Error
}

func ensureNoGenerationOutput(tx *gorm.DB, taskID string) error {
	var existing generationOutputRecord
	err := tx.Where("task_id = ?", taskID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return generationapp.ErrGenerationSettlementConflict
}

func generationOutputRecordFromDomain(asset generationapp.OutputAsset, task generationapp.Task) (generationOutputRecord, error) {
	if err := asset.ValidateFor(task); err != nil {
		return generationOutputRecord{}, err
	}
	return generationOutputRecord{
		ID:                 asset.ID,
		TaskID:             asset.Lineage.TaskID,
		OwnerID:            asset.Lineage.OwnerID,
		LookID:             asset.Lineage.LookID,
		LookRevision:       asset.Lineage.LookRevision,
		Purpose:            string(asset.Lineage.Purpose),
		SourceImageAssetID: asset.Lineage.SourceImageAssetID,
		SourceImageSHA256:  asset.Lineage.SourceImageSHA256,
		ContentType:        asset.ContentType,
		ByteSize:           asset.ByteSize,
		SHA256:             asset.SHA256,
		ObjectVersionID:    asset.ObjectVersionID,
		PublishedAt:        asset.PublishedAt,
	}, nil
}

func sameGenerationOutput(left, right generationapp.OutputAsset) bool {
	return left.ID == right.ID &&
		left.Lineage == right.Lineage &&
		left.ContentType == right.ContentType &&
		left.ByteSize == right.ByteSize &&
		left.SHA256 == right.SHA256 &&
		left.ObjectVersionID == right.ObjectVersionID &&
		left.PublishedAt.Equal(right.PublishedAt)
}

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

var _ interface {
	ListUnknown(context.Context, string, int, *string) (generationapp.UnknownSubmissionPage, error)
	ReconcileUnknown(context.Context, string, generationapp.ReconcileUnknownInput, time.Time) (generationapp.SubmissionReconciliation, error)
} = (*GenerationRepository)(nil)

type generationSubmissionReconciliationRecord struct {
	ID                string    `gorm:"column:id;type:uuid;primaryKey"`
	TaskID            string    `gorm:"column:task_id;type:uuid;not null;index:generation_submission_reconciliations_task_idx"`
	ActorID           string    `gorm:"column:actor_id;type:uuid;not null"`
	ExpectedRevision  int       `gorm:"column:expected_revision;not null;check:generation_reconciliation_expected_revision_check,expected_revision >= 1"`
	ResultingRevision int       `gorm:"column:resulting_revision;not null;check:generation_reconciliation_resulting_revision_check,resulting_revision >= expected_revision"`
	Attempt           int       `gorm:"column:submission_attempt;not null;check:generation_reconciliation_attempt_check,submission_attempt >= 1"`
	Decision          string    `gorm:"column:decision;type:text;not null;check:generation_reconciliation_decision_check,decision IN ('accepted','not_accepted')"`
	ExternalTaskID    string    `gorm:"column:external_task_id;type:text;not null;default:''"`
	EvidenceType      string    `gorm:"column:evidence_type;type:text;not null;check:generation_reconciliation_evidence_type_check,evidence_type IN ('provider_console','provider_query','support_case')"`
	EvidenceReference string    `gorm:"column:evidence_reference;type:text;not null;check:generation_reconciliation_evidence_reference_check,char_length(evidence_reference) BETWEEN 1 AND 128"`
	CreatedAt         time.Time `gorm:"column:created_at;type:timestamptz;not null"`
}

func (generationSubmissionReconciliationRecord) TableName() string {
	return "generation_submission_reconciliations"
}

func (r *GenerationRepository) ListUnknown(ctx context.Context, actorID string, limit int, afterID *string) (generationapp.UnknownSubmissionPage, error) {
	if r == nil || r.database == nil {
		return generationapp.UnknownSubmissionPage{}, generationapp.ErrGenerationUnavailable
	}
	if _, err := uuid.Parse(actorID); err != nil || limit < 1 || limit > 100 {
		return generationapp.UnknownSubmissionPage{}, generationapp.ErrInvalidGenerationInput
	}
	if afterID != nil {
		if _, err := uuid.Parse(*afterID); err != nil {
			return generationapp.UnknownSubmissionPage{}, generationapp.ErrInvalidGenerationInput
		}
	}
	var page generationapp.UnknownSubmissionPage
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireGenerationAdmin(tx, actorID, clause.Locking{Strength: "SHARE"}); err != nil {
			return err
		}
		query := tx.Model(&generationJobRecord{}).Where("submission_state = ?", string(generationapp.SubmissionUnknown))
		if afterID != nil {
			var cursor generationJobRecord
			if err := tx.Where("id = ? AND submission_state = ?", *afterID, string(generationapp.SubmissionUnknown)).First(&cursor).Error; err != nil {
				return generationLookupError(err)
			}
			query = query.Where("created_at < ? OR (created_at = ? AND id > ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		}
		var records []generationJobRecord
		if err := query.Order("created_at DESC").Order("id ASC").Limit(limit + 1).Find(&records).Error; err != nil {
			return err
		}
		hasMore := len(records) > limit
		if hasMore {
			records = records[:limit]
		}
		page.Items = make([]generationapp.UnknownSubmission, 0, len(records))
		for _, record := range records {
			if record.SubmissionUnknownAt == nil {
				return generationapp.ErrInvalidGenerationState
			}
			page.Items = append(page.Items, generationapp.UnknownSubmission{
				ID: record.ID, Purpose: generationapp.Purpose(record.Purpose), Provider: record.Provider, Model: record.Model,
				StatusRevision: record.StatusRevision, SubmissionAttempt: record.SubmissionAttempt,
				UnknownAt: record.SubmissionUnknownAt.UTC(), CancelRequestedAt: record.CancelRequestedAt,
				AccessRevokedAt: record.AccessRevokedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
			})
		}
		if hasMore && len(page.Items) != 0 {
			last := page.Items[len(page.Items)-1].ID
			page.NextAfterID = &last
		}
		return nil
	})
	if err != nil {
		return generationapp.UnknownSubmissionPage{}, generationReconciliationError(err)
	}
	return page, nil
}

func (r *GenerationRepository) ReconcileUnknown(ctx context.Context, actorID string, input generationapp.ReconcileUnknownInput, at time.Time) (generationapp.SubmissionReconciliation, error) {
	if r == nil || r.database == nil {
		return generationapp.SubmissionReconciliation{}, generationapp.ErrGenerationUnavailable
	}
	if _, err := uuid.Parse(actorID); err != nil || input.Validate() != nil || at.IsZero() {
		return generationapp.SubmissionReconciliation{}, generationapp.ErrInvalidGenerationInput
	}
	var result generationapp.SubmissionReconciliation
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireGenerationAdmin(tx, actorID, clause.Locking{Strength: "UPDATE"}); err != nil {
			return err
		}
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", input.TaskID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		if task.StatusRevision != input.ExpectedRevision {
			return generationapp.ErrGenerationRevisionConflict
		}
		if task.SubmissionState != generationapp.SubmissionUnknown || task.SubmissionUnknownAt == nil || task.ExternalTaskID != "" || task.Status.Terminal() {
			return generationapp.ErrInvalidGenerationState
		}
		if task.LeaseUntil != nil && at.Before(*task.LeaseUntil) {
			return generationapp.ErrGenerationLeaseHeld
		}

		previousRevision := task.StatusRevision
		lease, err := task.AcquireLease(actorID, at, time.Minute)
		if err != nil {
			return err
		}
		if input.Decision == generationapp.SubmissionDecisionAccepted {
			if err := task.RecordExternalTaskID(input.ExternalTaskID, at); err != nil {
				return err
			}
			if err := task.ReleaseLease(lease, at); err != nil {
				return err
			}
			if err := updateGenerationTask(tx, task, previousRevision); err != nil {
				return err
			}
			if task.AccessRevokedAt != nil {
				var cleanupCount int64
				if err := tx.Model(&generationCleanupRequestRecord{}).Where("task_id = ?", task.ID).Count(&cleanupCount).Error; err != nil {
					return err
				}
				if cleanupCount == 0 {
					if _, err := ensureGenerationCleanupInTx(tx, task, generationapp.CleanupScopeTask, "", "", at); err != nil {
						return err
					}
				}
				if err := syncGenerationCleanupTargetsInTx(tx, task, at); err != nil {
					return err
				}
			}
		} else {
			if err := task.ReconcileSubmissionNotAccepted(at); err != nil {
				return err
			}
			if err := updateGenerationTask(tx, task, previousRevision); err != nil {
				return err
			}
			next := generationapp.StatusFailed
			failureCode := "submission_not_accepted"
			if task.CancelRequestedAt != nil || task.AccessRevokedAt != nil {
				next = generationapp.StatusCanceled
				failureCode = ""
			}
			reservation, err := lockedGenerationReservation(tx, task.ID)
			if err != nil {
				return err
			}
			settlement, err := generationapp.FinalizeWithoutOutput(task, reservation, next, failureCode, at)
			if err != nil {
				return err
			}
			reservationRevision := 0
			if reservation != nil {
				reservationRevision = reservation.StateRevision
			}
			if err := persistGenerationSettlement(tx, task, task.StatusRevision, reservation, reservationRevision, settlement); err != nil {
				return err
			}
			task = settlement.Task
		}

		auditID := uuid.NewString()
		audit := generationSubmissionReconciliationRecord{
			ID: auditID, TaskID: task.ID, ActorID: actorID, ExpectedRevision: input.ExpectedRevision,
			ResultingRevision: task.StatusRevision, Attempt: task.SubmissionAttempt,
			Decision: string(input.Decision), ExternalTaskID: input.ExternalTaskID,
			EvidenceType: string(input.EvidenceType), EvidenceReference: input.EvidenceReference, CreatedAt: at.UTC(),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		persisted, err := generationTaskByID(tx, task.ID)
		if err != nil {
			return err
		}
		view, err := r.readTaskView(tx, persisted)
		if err != nil {
			return err
		}
		result = generationapp.SubmissionReconciliation{View: view, AuditID: auditID, Decision: input.Decision, RecordedAt: at.UTC()}
		return nil
	})
	if err != nil {
		return generationapp.SubmissionReconciliation{}, generationReconciliationError(err)
	}
	return result, nil
}

func requireGenerationAdmin(tx *gorm.DB, actorID string, locking clause.Locking) error {
	var actor userRecord
	err := tx.Clauses(locking).Where("id = ? AND status = ? AND role = ?", actorID, "active", "admin").First(&actor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return generationapp.ErrGenerationForbidden
	}
	if err != nil {
		return err
	}
	return nil
}

func generationReconciliationError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, generationapp.ErrGenerationNotFound) ||
		errors.Is(err, generationapp.ErrGenerationUnavailable) || errors.Is(err, generationapp.ErrInvalidGenerationInput) ||
		errors.Is(err, generationapp.ErrInvalidGenerationState) || errors.Is(err, generationapp.ErrGenerationForbidden) ||
		errors.Is(err, generationapp.ErrGenerationRevisionConflict) || errors.Is(err, generationapp.ErrGenerationLeaseHeld) ||
		errors.Is(err, generationapp.ErrGenerationLeaseExpired) || errors.Is(err, generationapp.ErrGenerationLeaseConflict) ||
		errors.Is(err, generationapp.ErrInvalidGenerationLease) || errors.Is(err, generationapp.ErrInvalidGenerationSettlement) ||
		errors.Is(err, generationapp.ErrGenerationSettlementConflict) || errors.Is(err, generationapp.ErrQuotaReservationClosed) ||
		errors.Is(err, generationapp.ErrInvalidQuotaReservation) || errors.Is(err, generationapp.ErrExternalTaskConflict) {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return generationapp.ErrGenerationNotFound
		}
		return err
	}
	return generationapp.ErrGenerationUnavailable
}

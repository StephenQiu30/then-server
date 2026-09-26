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

var _ generationapp.TaskTimeoutRepository = (*GenerationRepository)(nil)

func (r *GenerationRepository) ListDueTasks(ctx context.Context, provider string, cutoff, at time.Time, limit int) ([]generationapp.Task, error) {
	if r == nil || r.database == nil || provider == "" || cutoff.IsZero() || at.IsZero() || cutoff.After(at) || limit < 1 || limit > 100 {
		return nil, generationapp.ErrInvalidGenerationInput
	}
	var records []generationJobRecord
	err := r.database.WithContext(ctx).Where("provider = ? AND status IN ? AND created_at <= ? AND (lease_until IS NULL OR lease_until <= ?)",
		provider, []string{string(generationapp.StatusQueued), string(generationapp.StatusRunning), string(generationapp.StatusValidating)}, cutoff.UTC(), at.UTC()).
		Order("created_at ASC, id ASC").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, err
	}
	tasks := make([]generationapp.Task, 0, len(records))
	for _, record := range records {
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// ExpireTaskIfDue settles a local task, revokes access and records provider
// and exact object-version cleanup targets in the same transaction. A stale
// inventory cannot overtake a live fenced worker lease.
func (r *GenerationRepository) ExpireTaskIfDue(ctx context.Context, taskID, provider string, cutoff, at time.Time, versions []string) (bool, error) {
	if r == nil || r.database == nil || provider == "" || cutoff.IsZero() || at.IsZero() || cutoff.After(at) {
		return false, generationapp.ErrInvalidGenerationInput
	}
	if _, err := uuid.Parse(taskID); err != nil {
		return false, generationapp.ErrInvalidGenerationInput
	}
	for _, version := range versions {
		if version == "" || len(version) > 160 {
			return false, generationapp.ErrInvalidGenerationInput
		}
	}
	changed := false
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		if task.Provider != provider || task.CreatedAt.After(cutoff) || task.Status.Terminal() || (task.LeaseUntil != nil && at.Before(*task.LeaseUntil)) {
			return nil
		}
		key, err := generationapp.OutputObjectKey(task)
		if err != nil {
			return err
		}
		reservation, err := lockedGenerationReservation(tx, task.ID)
		if err != nil {
			return err
		}
		previousReservationRevision := 0
		if reservation != nil {
			previousReservationRevision = reservation.StateRevision
		}
		mutationAt := at.UTC()
		if mutationAt.Before(task.UpdatedAt) {
			mutationAt = task.UpdatedAt
		}
		settlement, err := generationapp.FinalizeWithoutOutput(task, reservation, generationapp.StatusExpired, "", mutationAt)
		if err != nil {
			return err
		}
		if err := persistGenerationSettlement(tx, task, task.StatusRevision, reservation, previousReservationRevision, settlement); err != nil {
			return err
		}
		if _, err := ensureGenerationCleanupInTx(tx, settlement.Task, generationapp.CleanupScopeTask, "", "", mutationAt); err != nil {
			return err
		}
		for _, version := range versions {
			target := generationapp.CleanupTarget{Kind: generationapp.CleanupTargetObject, ID: task.ID, ObjectKey: key, ObjectVersionID: version}
			if err := addTargetToRevokedGenerationCleanupInTx(tx, settlement.Task, target, mutationAt); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, generationWorkerError(err)
	}
	return changed, nil
}

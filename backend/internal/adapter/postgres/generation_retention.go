package postgres

import (
	"context"
	"errors"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
)

// ExpirePublishedOutputs uses the ordinary task deletion transaction for each
// due result. That transaction also cascades an expired image to its models.
func (r *GenerationRepository) ExpirePublishedOutputs(ctx context.Context, cutoff, at time.Time, limit int) (int, error) {
	if r == nil || r.database == nil || cutoff.IsZero() || at.IsZero() || cutoff.After(at) || limit < 1 || limit > 100 {
		return 0, generationapp.ErrInvalidGenerationInput
	}
	var due []struct {
		ID      string
		OwnerID string
	}
	err := r.database.WithContext(ctx).Table("generation_jobs AS jobs").
		Select("jobs.id, jobs.owner_id").
		Joins("JOIN generation_outputs AS outputs ON outputs.task_id = jobs.id").
		Where("jobs.status = ? AND jobs.access_revoked_at IS NULL AND outputs.published_at <= ?", string(generationapp.StatusSucceeded), cutoff.UTC()).
		Order("outputs.published_at ASC, jobs.id ASC").Limit(limit).Scan(&due).Error
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, task := range due {
		if _, _, err := r.RequestTaskCleanup(ctx, task.OwnerID, task.ID, at.UTC()); err != nil {
			if errors.Is(err, generationapp.ErrGenerationNotFound) {
				continue
			}
			return processed, err
		}
		processed++
	}
	return processed, nil
}

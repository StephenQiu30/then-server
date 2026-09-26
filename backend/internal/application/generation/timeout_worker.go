package generation

import (
	"context"
	"time"
)

// TaskTimeoutRepository keeps the due check and terminal settlement atomic.
type TaskTimeoutRepository interface {
	ListDueTasks(context.Context, string, time.Time, time.Time, int) ([]Task, error)
	ExpireTaskIfDue(context.Context, string, string, time.Time, time.Time, []string) (bool, error)
}

type TaskTimeoutWorker struct {
	repository TaskTimeoutRepository
	outputs    OutputVersionReader
	provider   string
	timeout    time.Duration
	now        func() time.Time
}

func NewTaskTimeoutWorker(repository TaskTimeoutRepository, outputs OutputVersionReader, provider string, timeout time.Duration) (*TaskTimeoutWorker, error) {
	if repository == nil || outputs == nil || !validToken(provider, 96) || timeout < time.Minute || timeout > 24*time.Hour {
		return nil, ErrInvalidGenerationWorker
	}
	return &TaskTimeoutWorker{repository: repository, outputs: outputs, provider: provider, timeout: timeout, now: time.Now}, nil
}

// RunOnce inventories exact object versions before settling an overdue task.
// The repository rechecks the task and its lease after inventory, so a worker
// that claimed it concurrently cannot be expired from this stale snapshot.
func (w *TaskTimeoutWorker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.repository == nil || w.outputs == nil || w.timeout <= 0 {
		return 0, ErrInvalidGenerationWorker
	}
	at := w.now().UTC()
	cutoff := at.Add(-w.timeout)
	tasks, err := w.repository.ListDueTasks(ctx, w.provider, cutoff, at, 50)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, task := range tasks {
		key, err := OutputObjectKey(task)
		if err != nil {
			return processed, err
		}
		versions, err := w.outputs.ListOutputVersions(ctx, key)
		if err != nil {
			return processed, ErrGenerationOutputInventoryUnknown
		}
		changed, err := w.repository.ExpireTaskIfDue(ctx, task.ID, w.provider, cutoff, at, versions)
		if err != nil {
			return processed, err
		}
		if changed {
			processed++
		}
	}
	return processed, nil
}

func (w *TaskTimeoutWorker) Run(ctx context.Context, pollInterval time.Duration) error {
	if w == nil || pollInterval <= 0 {
		return ErrInvalidGenerationWorker
	}
	for {
		if _, err := w.RunOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}

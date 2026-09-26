package generation

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidGenerationRetentionWorker = errors.New("invalid generation retention worker")

// ExpiredOutputRepository atomically revokes due results and records their
// existing cleanup targets. Successful task and quota facts stay unchanged.
type ExpiredOutputRepository interface {
	ExpirePublishedOutputs(context.Context, time.Time, time.Time, int) (int, error)
}

type RetentionWorker struct {
	repository ExpiredOutputRepository
	retention  time.Duration
	now        func() time.Time
}

func NewRetentionWorker(repository ExpiredOutputRepository, retention time.Duration) (*RetentionWorker, error) {
	if repository == nil || retention <= 0 || retention > 365*24*time.Hour {
		return nil, ErrInvalidGenerationRetentionWorker
	}
	return &RetentionWorker{repository: repository, retention: retention, now: time.Now}, nil
}

func (w *RetentionWorker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.repository == nil || w.retention <= 0 {
		return 0, ErrInvalidGenerationRetentionWorker
	}
	at := w.now().UTC()
	return w.repository.ExpirePublishedOutputs(ctx, at.Add(-w.retention), at, 50)
}

func (w *RetentionWorker) Run(ctx context.Context, pollInterval time.Duration) error {
	if w == nil || pollInterval <= 0 {
		return ErrInvalidGenerationRetentionWorker
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

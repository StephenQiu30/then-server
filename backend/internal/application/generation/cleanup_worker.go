package generation

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidGenerationCleanupWorker = errors.New("invalid generation cleanup worker")
	ErrGenerationCleanupTarget        = errors.New("generation cleanup target failed")
)

// CleanupRepository is the durable boundary used by the cleanup worker. A
// claim contains an immutable target snapshot; completion or failure is then
// persisted with the same request identity after the external operation.
type CleanupRepository interface {
	ClaimNextCleanup(context.Context, time.Time, time.Duration) (CleanupRequest, []CleanupTarget, bool, error)
	CompleteTaskCleanup(context.Context, CleanupRequest, time.Time) (CleanupRequest, error)
	FailTaskCleanup(context.Context, CleanupRequest, time.Time, string, time.Time) (CleanupRequest, error)
}

// CleanupExecutor is deliberately provider-neutral. Concrete object-store
// and Provider adapters are supplied by a later enabled worker; this package
// only owns ordering, stable failure classification and retry timing.
type CleanupExecutor interface {
	DeleteObject(context.Context, CleanupTarget) error
	DeleteProviderTask(context.Context, CleanupTarget) error
}

// CleanupTargetError lets an adapter expose a bounded, non-sensitive reason
// code without persisting transport details or provider responses.
type CleanupTargetError struct {
	Code string
	Err  error
}

func (e *CleanupTargetError) Error() string {
	if e == nil || e.Code == "" {
		return ErrGenerationCleanupTarget.Error()
	}
	return e.Code
}

func (e *CleanupTargetError) Unwrap() error {
	if e == nil || e.Err == nil {
		return ErrGenerationCleanupTarget
	}
	return e.Err
}

// CleanupRetryPolicy bounds cleanup lease recovery and target retry delay.
// MaxCleanupAttempts remains enforced by CleanupRequest.Begin; the repository
// excludes exhausted retries and settles stale exhausted claims durably.
type CleanupRetryPolicy struct {
	LeaseTTL  time.Duration
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

func (p CleanupRetryPolicy) Validate() error {
	if p.LeaseTTL <= 0 || p.BaseDelay <= 0 || p.MaxDelay < p.BaseDelay {
		return ErrInvalidGenerationCleanupWorker
	}
	return nil
}

func (p CleanupRetryPolicy) RetryAt(at time.Time, attempts int) (time.Time, error) {
	if err := p.Validate(); err != nil || at.IsZero() || attempts < 1 {
		return time.Time{}, ErrInvalidGenerationCleanupWorker
	}
	delay := p.BaseDelay
	for step := 1; step < attempts && delay < p.MaxDelay; step++ {
		if delay > p.MaxDelay/2 {
			delay = p.MaxDelay
			break
		}
		delay *= 2
	}
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return at.UTC().Add(delay), nil
}

// CleanupWorker executes one durable cleanup request per RunOnce call. Runtime
// bootstrap wires the local object-store adapter; unavailable Provider deletion
// remains a bounded retryable failure until an approved adapter exists.
type CleanupWorker struct {
	repository CleanupRepository
	executor   CleanupExecutor
	policy     CleanupRetryPolicy
	now        func() time.Time
}

func NewCleanupWorker(repository CleanupRepository, executor CleanupExecutor, policy CleanupRetryPolicy) (*CleanupWorker, error) {
	if repository == nil || executor == nil {
		return nil, ErrInvalidGenerationCleanupWorker
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &CleanupWorker{repository: repository, executor: executor, policy: policy, now: time.Now}, nil
}

// RunOnce returns true when a request was claimed. A target failure is
// recorded as a retryable request and therefore returns nil; repository or
// completion failures are returned so the caller can stop and alert.
func (w *CleanupWorker) RunOnce(ctx context.Context) (bool, error) {
	if w == nil {
		return false, ErrInvalidGenerationCleanupWorker
	}
	return w.runOnce(ctx, w.now)
}

// RunOnceAt executes one cleanup claim at a supplied timestamp. It keeps
// durable worker behavior deterministic for recovery and integration checks.
func (w *CleanupWorker) RunOnceAt(ctx context.Context, at time.Time) (bool, error) {
	if at.IsZero() {
		return false, ErrInvalidGenerationCleanupWorker
	}
	return w.runOnce(ctx, func() time.Time { return at })
}

func (w *CleanupWorker) runOnce(ctx context.Context, now func() time.Time) (bool, error) {
	if w == nil || w.repository == nil || w.executor == nil || w.policy.Validate() != nil {
		return false, ErrInvalidGenerationCleanupWorker
	}
	at := now().UTC()
	request, targets, found, err := w.repository.ClaimNextCleanup(ctx, at, w.policy.LeaseTTL)
	if err != nil || !found {
		return found, err
	}
	for _, target := range targets {
		if err := w.deleteTarget(ctx, request.OwnerID, request.TaskID, target); err != nil {
			failureCode := cleanupFailureCode(err)
			retryAt, retryErr := w.policy.RetryAt(now().UTC(), request.Attempts)
			if retryErr != nil {
				return true, retryErr
			}
			if _, failErr := w.repository.FailTaskCleanup(ctx, request, now().UTC(), failureCode, retryAt); failErr != nil {
				return true, failErr
			}
			return true, nil
		}
	}
	_, err = w.repository.CompleteTaskCleanup(ctx, request, now().UTC())
	return true, err
}

// Run polls the durable cleanup queue until cancellation. It drains ready
// work immediately and sleeps only when no request is currently claimable.
func (w *CleanupWorker) Run(ctx context.Context, pollInterval time.Duration) error {
	if w == nil || pollInterval <= 0 {
		return ErrInvalidGenerationCleanupWorker
	}
	for {
		found, err := w.RunOnce(ctx)
		if err != nil {
			return err
		}
		if found {
			continue
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

func (w *CleanupWorker) deleteTarget(ctx context.Context, ownerID, taskID string, target CleanupTarget) error {
	if err := target.validateForTask(ownerID, taskID); err != nil {
		return &CleanupTargetError{Code: "invalid_target", Err: err}
	}
	switch target.Kind {
	case CleanupTargetObject:
		if target.ObjectKey == "" {
			return &CleanupTargetError{Code: "object_key_unavailable", Err: ErrGenerationCleanupTarget}
		}
		return w.executor.DeleteObject(ctx, target)
	case CleanupTargetProvider:
		return w.executor.DeleteProviderTask(ctx, target)
	default:
		return &CleanupTargetError{Code: "invalid_target", Err: ErrGenerationCleanupTarget}
	}
}

func cleanupFailureCode(err error) string {
	var targetErr *CleanupTargetError
	if errors.As(err, &targetErr) && targetErr != nil && validToken(targetErr.Code, 96) {
		return targetErr.Code
	}
	return "target_delete_failed"
}

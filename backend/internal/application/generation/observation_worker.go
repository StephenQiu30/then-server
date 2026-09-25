package generation

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrGenerationObservationUnknown means the provider query did not produce
	// a trustworthy observation. The task is left in its previous state and a
	// later query may retry after the lease has been released.
	ErrGenerationObservationUnknown = errors.New("generation observation requires retry")
	// ErrGenerationObservationUnavailable means the task has not yet recorded
	// an accepted provider identity and therefore cannot be queried.
	ErrGenerationObservationUnavailable = errors.New("generation task has no provider identity")
)

// ObservationWorkerRepository is the durable boundary for one provider
// status observation. Terminal provider states are settled through the same
// quota and lease transaction used by submission failures; non-terminal
// states only update the task state before releasing the lease.
type ObservationWorkerRepository interface {
	AcquireObservationLease(context.Context, string, string, time.Time, time.Duration) (TaskView, Lease, error)
	ReleaseLease(context.Context, Lease, time.Time) (TaskView, error)
	ReleaseLeaseForRetry(context.Context, Lease, time.Time, time.Duration) (TaskView, error)
	ApplyProviderState(context.Context, Lease, string, Status, string, time.Time) (TaskView, error)
	FinalizeWithoutOutput(context.Context, Lease, Status, string, time.Time) (TaskView, error)
}

// ObservationWorkerQueue is the durable scheduler boundary used by RunNext.
// The repository only claims accepted, non-terminal tasks whose provider
// identity can be queried; result publication remains a separate queue.
type ObservationWorkerQueue interface {
	ClaimNextObservationLease(context.Context, string, time.Time, time.Duration) (TaskView, Lease, bool, error)
}

// ProviderScopedObservationWorkerQueue prevents a provider adapter from
// polling task identities owned by another provider.
type ProviderScopedObservationWorkerQueue interface {
	ClaimNextObservationLeaseForProvider(context.Context, string, string, time.Time, time.Duration) (TaskView, Lease, bool, error)
}

// ObservationWorkerPolicy bounds one provider query. Provider calls remain
// outside the transaction; the current runtime uses only the local fixture.
type ObservationWorkerPolicy struct {
	WorkerID        string
	Provider        string
	LeaseTTL        time.Duration
	ProviderTimeout time.Duration
	PollInterval    time.Duration
}

func (p ObservationWorkerPolicy) Validate() error {
	if !validID(p.WorkerID) || (p.Provider != "" && !validToken(p.Provider, 96)) || p.LeaseTTL <= 0 || p.LeaseTTL > 30*time.Minute || p.ProviderTimeout <= 0 || p.ProviderTimeout > 10*time.Minute || p.PollInterval <= 0 || p.PollInterval > maxWorkerRetryDelay {
		return ErrInvalidGenerationWorker
	}
	return nil
}

type ObservationOutcome string

const (
	ObservationOutcomeRunning    ObservationOutcome = "running"
	ObservationOutcomeValidating ObservationOutcome = "validating"
	ObservationOutcomeTerminal   ObservationOutcome = "terminal"
)

// ObservationResult contains only persisted task facts and a bounded outcome
// label. Provider response bodies never cross this boundary.
type ObservationResult struct {
	View    TaskView
	Outcome ObservationOutcome
}

// ObservationWorker queries one accepted provider task. A provider success
// observation intentionally stops at validating; fetching, validating and
// publishing an output are separate operations owned by a future object-store
// worker.
type ObservationWorker struct {
	repository ObservationWorkerRepository
	provider   Provider
	policy     ObservationWorkerPolicy
	now        func() time.Time
}

func NewObservationWorker(repository ObservationWorkerRepository, provider Provider, policy ObservationWorkerPolicy) (*ObservationWorker, error) {
	if repository == nil || provider == nil {
		return nil, ErrInvalidGenerationWorker
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &ObservationWorker{repository: repository, provider: provider, policy: policy, now: time.Now}, nil
}

// RunOnce claims a task, queries its accepted provider identity, persists a
// non-terminal observation or terminal no-output settlement, and releases the
// lease. Query failures and malformed observations never advance the task.
func (w *ObservationWorker) RunOnce(ctx context.Context, taskID string) (ObservationResult, error) {
	if w == nil || w.repository == nil || w.provider == nil || w.policy.Validate() != nil || !validID(taskID) {
		return ObservationResult{}, ErrInvalidGenerationWorker
	}
	at := w.now().UTC()
	view, lease, err := w.repository.AcquireObservationLease(ctx, taskID, w.policy.WorkerID, at, w.policy.LeaseTTL)
	if err != nil {
		return ObservationResult{}, err
	}
	return w.runClaimed(ctx, view, lease)
}

// RunNext claims and processes one accepted task that is ready for a provider
// status observation. It returns found=false when the durable queue is empty.
func (w *ObservationWorker) RunNext(ctx context.Context) (bool, ObservationResult, error) {
	if w == nil || w.repository == nil || w.provider == nil || w.policy.Validate() != nil {
		return false, ObservationResult{}, ErrInvalidGenerationWorker
	}
	var view TaskView
	var lease Lease
	var found bool
	var err error
	if w.policy.Provider != "" {
		queue, ok := w.repository.(ProviderScopedObservationWorkerQueue)
		if !ok {
			return false, ObservationResult{}, ErrInvalidGenerationWorker
		}
		view, lease, found, err = queue.ClaimNextObservationLeaseForProvider(ctx, w.policy.WorkerID, w.policy.Provider, w.now().UTC(), w.policy.LeaseTTL)
	} else {
		queue, ok := w.repository.(ObservationWorkerQueue)
		if !ok {
			return false, ObservationResult{}, ErrInvalidGenerationWorker
		}
		view, lease, found, err = queue.ClaimNextObservationLease(ctx, w.policy.WorkerID, w.now().UTC(), w.policy.LeaseTTL)
	}
	if err != nil || !found {
		return found, ObservationResult{View: view}, err
	}
	result, err := w.runClaimed(ctx, view, lease)
	return true, result, err
}

func (w *ObservationWorker) runClaimed(ctx context.Context, view TaskView, lease Lease) (ObservationResult, error) {
	providerContext, cancel := context.WithTimeout(ctx, w.policy.ProviderTimeout)
	remote, queryErr := w.provider.Query(providerContext, view.Task.ExternalTaskID)
	cancel()
	if queryErr != nil {
		return w.releaseWithError(ctx, lease, view, ErrGenerationObservationUnknown)
	}
	if err := validateObservation(view.Task.ExternalTaskID, remote); err != nil {
		return w.releaseWithError(ctx, lease, view, err)
	}
	if view.Task.CancelRequestedAt != nil && !remote.State.terminal() && remote.State != StatusSucceeded {
		cancelContext, stopCancel := context.WithTimeout(ctx, w.policy.ProviderTimeout)
		cancelErr := w.provider.Cancel(cancelContext, view.Task.ExternalTaskID)
		stopCancel()
		if cancelErr != nil {
			return w.releaseWithError(ctx, lease, view, ErrGenerationCancellationUnknown)
		}
		providerContext, stopQuery := context.WithTimeout(ctx, w.policy.ProviderTimeout)
		remote, queryErr = w.provider.Query(providerContext, view.Task.ExternalTaskID)
		stopQuery()
		if queryErr != nil {
			return w.releaseWithError(ctx, lease, view, ErrGenerationCancellationUnknown)
		}
		if err := validateObservation(view.Task.ExternalTaskID, remote); err != nil {
			return w.releaseWithError(ctx, lease, view, err)
		}
	}

	now := w.now().UTC()
	switch remote.State {
	case StatusQueued:
		return w.applyNonTerminal(ctx, lease, view, remote, StatusRunning, now, ObservationOutcomeRunning)
	case StatusRunning:
		return w.applyNonTerminal(ctx, lease, view, remote, StatusRunning, now, ObservationOutcomeRunning)
	case StatusValidating:
		return w.applyNonTerminal(ctx, lease, view, remote, StatusValidating, now, ObservationOutcomeValidating)
	case StatusSucceeded:
		// A provider success is only permission to fetch and validate an
		// output. Publishing an asset is a separate fenced settlement.
		return w.applyNonTerminal(ctx, lease, view, remote, StatusValidating, now, ObservationOutcomeValidating)
	case StatusFailed, StatusCanceled, StatusExpired:
		settled, settleErr := w.repository.FinalizeWithoutOutput(ctx, lease, remote.State, remote.FailureCode, now)
		if settleErr != nil {
			return ObservationResult{View: view, Outcome: ObservationOutcomeTerminal}, settleErr
		}
		return ObservationResult{View: settled, Outcome: ObservationOutcomeTerminal}, nil
	default:
		return w.releaseWithError(ctx, lease, view, ErrInvalidProviderObservation)
	}
}

func (w *ObservationWorker) applyNonTerminal(ctx context.Context, lease Lease, previous TaskView, remote RemoteTask, next Status, at time.Time, outcome ObservationOutcome) (ObservationResult, error) {
	updated, err := w.repository.ApplyProviderState(ctx, lease, remote.ExternalTaskID, next, "", at)
	if err != nil {
		return ObservationResult{View: previous, Outcome: outcome}, err
	}
	releasedAt := w.now().UTC()
	var released TaskView
	if next == StatusRunning {
		released, err = w.repository.ReleaseLeaseForRetry(ctx, lease, releasedAt, w.policy.PollInterval)
	} else {
		released, err = w.repository.ReleaseLease(ctx, lease, releasedAt)
	}
	if err != nil {
		return ObservationResult{View: updated, Outcome: outcome}, err
	}
	return ObservationResult{View: released, Outcome: outcome}, nil
}

func (w *ObservationWorker) releaseWithError(ctx context.Context, lease Lease, view TaskView, observationErr error) (ObservationResult, error) {
	released, releaseErr := w.repository.ReleaseLeaseForRetry(ctx, lease, w.now().UTC(), w.policy.PollInterval)
	if releaseErr != nil {
		return ObservationResult{View: view}, releaseErr
	}
	return ObservationResult{View: released}, observationErr
}

func validateObservation(expectedID string, remote RemoteTask) error {
	if !validToken(remote.ExternalTaskID, 256) || remote.ExternalTaskID != expectedID {
		return ErrInvalidProviderObservation
	}
	switch remote.State {
	case StatusQueued, StatusRunning, StatusValidating, StatusSucceeded:
		if remote.FailureCode != "" {
			return ErrInvalidProviderObservation
		}
	case StatusFailed:
		if !validToken(remote.FailureCode, 96) {
			return ErrInvalidProviderObservation
		}
	case StatusCanceled, StatusExpired:
		if remote.FailureCode != "" {
			return ErrInvalidProviderObservation
		}
	default:
		return ErrInvalidProviderObservation
	}
	return nil
}

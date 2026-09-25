package generation

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidGenerationWorker = errors.New("invalid generation worker")
	// ErrGenerationSubmissionUnknown is returned after the worker has durably
	// fenced blind resubmission. A later reconciliation must prove that no
	// remote task was created before another submit is allowed.
	ErrGenerationSubmissionUnknown = errors.New("generation submission requires reconciliation")
)

// SubmissionWorkerRepository is the durable boundary for one submission
// attempt. The repository implementation keeps every mutation in a short
// transaction and uses Lease as the fencing proof for the attempt.
//
// Querying an accepted provider task and publishing a validated output are
// intentionally separate operations. This worker only owns the submit edge;
// a future provider/object-store worker can consume the accepted identity.
type SubmissionWorkerRepository interface {
	AcquireLease(context.Context, string, string, time.Time, time.Duration) (TaskView, Lease, error)
	ReleaseLease(context.Context, Lease, time.Time) (TaskView, error)
	BeginSubmission(context.Context, Lease, time.Time) (TaskView, Submission, error)
	MarkSubmissionUnknown(context.Context, Lease, time.Time) (TaskView, error)
	ReconcileSubmissionNotAccepted(context.Context, Lease, time.Time) (TaskView, error)
	ScheduleSubmissionRetry(context.Context, Lease, time.Time, RetryPolicy) (TaskView, error)
	RecordExternalTaskID(context.Context, Lease, string, time.Time) (TaskView, error)
	FinalizeWithoutOutput(context.Context, Lease, Status, string, time.Time) (TaskView, error)
}

// SubmissionWorkerQueue is the durable scheduler boundary used by RunNext.
// A repository claims at most one eligible task with a short transaction and
// returns the same fencing proof used by RunOnce. Keeping this optional from
// SubmissionWorkerRepository preserves the explicit task-ID operation for
// queue consumers while allowing a restart-safe database scan.
type SubmissionWorkerQueue interface {
	ClaimNextSubmissionLease(context.Context, string, time.Time, time.Duration) (TaskView, Lease, bool, error)
}

// ProviderScopedSubmissionWorkerQueue lets a worker poll only tasks belonging
// to its configured provider adapter.
type ProviderScopedSubmissionWorkerQueue interface {
	ClaimNextSubmissionLeaseForProvider(context.Context, string, string, time.Time, time.Duration) (TaskView, Lease, bool, error)
}

// SubmissionWorkerPolicy bounds the worker's lease and provider request. The
// retry policy is applied only after ErrProviderNotAccepted; an unknown
// transport outcome never receives an automatic second submit.
type SubmissionWorkerPolicy struct {
	WorkerID        string
	Provider        string
	LeaseTTL        time.Duration
	ProviderTimeout time.Duration
	Retry           RetryPolicy
}

func (p SubmissionWorkerPolicy) Validate() error {
	if !validID(p.WorkerID) || (p.Provider != "" && !validToken(p.Provider, 96)) || p.LeaseTTL <= 0 || p.LeaseTTL > 30*time.Minute || p.ProviderTimeout <= 0 || p.ProviderTimeout > 10*time.Minute {
		return ErrInvalidGenerationWorker
	}
	if err := p.Retry.Validate(); err != nil {
		return ErrInvalidGenerationWorker
	}
	return nil
}

type SubmissionOutcome string

const (
	SubmissionOutcomeAccepted        SubmissionOutcome = "accepted"
	SubmissionOutcomeAlreadyAccepted SubmissionOutcome = "already_accepted"
	SubmissionOutcomeRetryScheduled  SubmissionOutcome = "retry_scheduled"
	SubmissionOutcomeCanceled        SubmissionOutcome = "canceled"
	SubmissionOutcomeFailed          SubmissionOutcome = "failed"
)

// SubmissionResult describes the durable state observed after RunOnce. It is
// safe to log the outcome and task ID; it contains no provider response body
// or private media.
type SubmissionResult struct {
	View    TaskView
	Outcome SubmissionOutcome
}

// SubmissionWorker performs one fenced submit attempt for a task ID. Bootstrap
// currently assembles it only with the zero-cost local fixture adapter.
type SubmissionWorker struct {
	repository SubmissionWorkerRepository
	provider   Provider
	policy     SubmissionWorkerPolicy
	now        func() time.Time
}

func NewSubmissionWorker(repository SubmissionWorkerRepository, provider Provider, policy SubmissionWorkerPolicy) (*SubmissionWorker, error) {
	if repository == nil || provider == nil {
		return nil, ErrInvalidGenerationWorker
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &SubmissionWorker{repository: repository, provider: provider, policy: policy, now: time.Now}, nil
}

// RunOnce claims one task and handles only its submit boundary. A successful
// provider receipt is recorded before the lease is released. A transport
// failure is persisted as unknown; an explicit ErrProviderNotAccepted is the
// only path that can reconcile and schedule a bounded retry.
func (w *SubmissionWorker) RunOnce(ctx context.Context, taskID string) (SubmissionResult, error) {
	if w == nil || w.repository == nil || w.provider == nil || w.policy.Validate() != nil || !validID(taskID) {
		return SubmissionResult{}, ErrInvalidGenerationWorker
	}
	at := w.now().UTC()
	view, lease, err := w.repository.AcquireLease(ctx, taskID, w.policy.WorkerID, at, w.policy.LeaseTTL)
	if err != nil {
		return SubmissionResult{}, err
	}
	return w.runClaimed(ctx, view, lease)
}

// RunNext claims and processes one queued submission. It returns found=false
// when the durable queue has no task ready at the supplied clock instant.
// Provider calls remain entirely behind the injected Provider port.
func (w *SubmissionWorker) RunNext(ctx context.Context) (bool, SubmissionResult, error) {
	if w == nil || w.repository == nil || w.provider == nil || w.policy.Validate() != nil {
		return false, SubmissionResult{}, ErrInvalidGenerationWorker
	}
	var view TaskView
	var lease Lease
	var found bool
	var err error
	if w.policy.Provider != "" {
		queue, ok := w.repository.(ProviderScopedSubmissionWorkerQueue)
		if !ok {
			return false, SubmissionResult{}, ErrInvalidGenerationWorker
		}
		view, lease, found, err = queue.ClaimNextSubmissionLeaseForProvider(ctx, w.policy.WorkerID, w.policy.Provider, w.now().UTC(), w.policy.LeaseTTL)
	} else {
		queue, ok := w.repository.(SubmissionWorkerQueue)
		if !ok {
			return false, SubmissionResult{}, ErrInvalidGenerationWorker
		}
		view, lease, found, err = queue.ClaimNextSubmissionLease(ctx, w.policy.WorkerID, w.now().UTC(), w.policy.LeaseTTL)
	}
	if err != nil || !found {
		return found, SubmissionResult{View: view}, err
	}
	result, err := w.runClaimed(ctx, view, lease)
	return true, result, err
}

func (w *SubmissionWorker) runClaimed(ctx context.Context, view TaskView, lease Lease) (SubmissionResult, error) {

	// A cancellation that arrived before submit must never create a provider
	// task. Settlement clears the lease and releases the quota atomically.
	if view.Task.CancelRequestedAt != nil && view.Task.ExternalTaskID == "" && view.Task.SubmissionState == SubmissionNotStarted {
		canceled, finalizeErr := w.repository.FinalizeWithoutOutput(ctx, lease, StatusCanceled, "", w.now().UTC())
		if finalizeErr != nil {
			return SubmissionResult{View: view}, finalizeErr
		}
		return SubmissionResult{View: canceled, Outcome: SubmissionOutcomeCanceled}, nil
	}

	// A previously accepted identity belongs to the query/settlement worker;
	// submitting again here would create a second billable remote task.
	if view.Task.ExternalTaskID != "" || view.Task.SubmissionState == SubmissionAccepted {
		released, releaseErr := w.repository.ReleaseLease(ctx, lease, w.now().UTC())
		if releaseErr != nil {
			return SubmissionResult{View: view, Outcome: SubmissionOutcomeAlreadyAccepted}, releaseErr
		}
		return SubmissionResult{View: released, Outcome: SubmissionOutcomeAlreadyAccepted}, nil
	}
	if view.Task.SubmissionState == SubmissionUnknown {
		released, releaseErr := w.repository.ReleaseLease(ctx, lease, w.now().UTC())
		if releaseErr != nil {
			return SubmissionResult{View: view}, releaseErr
		}
		return SubmissionResult{View: released}, ErrGenerationSubmissionUnknown
	}
	// An expired worker lease can leave the durable state in flight while its
	// provider request is still unresolved. Mark it unknown before releasing the
	// recovered lease so cancellation cannot hide an external side effect and a
	// later worker cannot submit the same request blindly.
	if view.Task.SubmissionState == SubmissionInFlight {
		return w.markUnknown(ctx, lease, view)
	}

	started, submission, err := w.repository.BeginSubmission(ctx, lease, w.now().UTC())
	if err != nil {
		return SubmissionResult{View: view}, err
	}
	providerContext, cancel := context.WithTimeout(ctx, w.policy.ProviderTimeout)
	receipt, submitErr := w.provider.Submit(providerContext, submission)
	cancel()
	if submitErr != nil {
		return w.handleSubmitError(ctx, lease, started, submitErr)
	}
	if !validToken(receipt.ExternalTaskID, 256) {
		unknown, unknownErr := w.markUnknown(ctx, lease, started)
		if unknownErr != nil && !errors.Is(unknownErr, ErrGenerationSubmissionUnknown) {
			return unknown, unknownErr
		}
		return unknown, ErrInvalidProviderReceipt
	}

	accepted, err := w.repository.RecordExternalTaskID(ctx, lease, receipt.ExternalTaskID, w.now().UTC())
	if err != nil {
		return SubmissionResult{View: started, Outcome: SubmissionOutcomeAccepted}, err
	}
	released, err := w.repository.ReleaseLease(ctx, lease, w.now().UTC())
	if err != nil {
		return SubmissionResult{View: accepted, Outcome: SubmissionOutcomeAccepted}, err
	}
	return SubmissionResult{View: released, Outcome: SubmissionOutcomeAccepted}, nil
}

func (w *SubmissionWorker) handleSubmitError(ctx context.Context, lease Lease, started TaskView, submitErr error) (SubmissionResult, error) {
	if errors.Is(submitErr, ErrProviderNotAccepted) {
		unknown, err := w.repository.MarkSubmissionUnknown(ctx, lease, w.now().UTC())
		if err != nil {
			return SubmissionResult{View: started}, err
		}
		reconciled, err := w.repository.ReconcileSubmissionNotAccepted(ctx, lease, w.now().UTC())
		if err != nil {
			return SubmissionResult{View: unknown}, err
		}
		scheduled, err := w.repository.ScheduleSubmissionRetry(ctx, lease, w.now().UTC(), w.policy.Retry)
		if errors.Is(err, ErrGenerationRetryExhausted) {
			failed, finalizeErr := w.repository.FinalizeWithoutOutput(ctx, lease, StatusFailed, "submission_retry_exhausted", w.now().UTC())
			if finalizeErr != nil {
				return SubmissionResult{View: reconciled}, finalizeErr
			}
			return SubmissionResult{View: failed, Outcome: SubmissionOutcomeFailed}, nil
		}
		if err != nil {
			return SubmissionResult{View: reconciled}, err
		}
		released, releaseErr := w.repository.ReleaseLease(ctx, lease, w.now().UTC())
		if releaseErr != nil {
			return SubmissionResult{View: scheduled, Outcome: SubmissionOutcomeRetryScheduled}, releaseErr
		}
		return SubmissionResult{View: released, Outcome: SubmissionOutcomeRetryScheduled}, nil
	}
	return w.markUnknown(ctx, lease, started)
}

func (w *SubmissionWorker) markUnknown(ctx context.Context, lease Lease, started TaskView) (SubmissionResult, error) {
	unknown, err := w.repository.MarkSubmissionUnknown(ctx, lease, w.now().UTC())
	if err != nil {
		return SubmissionResult{View: started}, err
	}
	released, releaseErr := w.repository.ReleaseLease(ctx, lease, w.now().UTC())
	if releaseErr != nil {
		return SubmissionResult{View: unknown}, releaseErr
	}
	return SubmissionResult{View: released}, ErrGenerationSubmissionUnknown
}

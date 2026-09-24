package generation

import (
	"context"
	"errors"
)

var (
	// ErrProviderNotAccepted marks a submission response that proves the
	// provider did not create a remote task. The worker may reconcile the
	// in-flight state and schedule a bounded retry only for this error.
	ErrProviderNotAccepted = errors.New("provider did not accept generation task")
	// ErrInvalidProviderReceipt is returned when an adapter claims success but
	// omits the remote identity required for later reconciliation.
	ErrInvalidProviderReceipt = errors.New("provider returned an invalid generation receipt")
	// ErrInvalidProviderObservation marks a provider response that cannot be
	// mapped to the task identified by the worker. The task remains leased
	// until the worker releases it, and no state transition is attempted.
	ErrInvalidProviderObservation = errors.New("provider returned an invalid generation observation")
	// ErrGenerationOutputFetchUnknown means the result fetch did not produce a
	// durable, validated object fact. The task remains validating and can be
	// retried by a later output worker.
	ErrGenerationOutputFetchUnknown = errors.New("generation output fetch requires retry")
	// ErrGenerationCancellationUnknown means a requested provider cancellation
	// did not produce a trustworthy outcome. The task remains visible as a
	// cancellation request and can be reconciled by a later observation.
	ErrGenerationCancellationUnknown = errors.New("generation cancellation requires retry")
)

// Provider is the only application port a future provider adapter may satisfy.
// It carries immutable metadata and never exposes HTTP or SDK types to the
// application package.
type Provider interface {
	Submit(context.Context, Submission) (Receipt, error)
	Query(context.Context, string) (RemoteTask, error)
	Cancel(context.Context, string) error
}

// Submission is assembled outside a database transaction by the worker.
type Submission struct {
	TaskID      string
	Purpose     Purpose
	Provider    string
	Model       string
	Parameters  []byte
	Inputs      InputSnapshot
	Idempotency string
	Attempt     int
}

// Receipt is the minimal fact returned when a provider accepts a task.
type Receipt struct {
	ExternalTaskID string
}

// RemoteTask is a provider-neutral observation used by the worker to map
// asynchronous provider state into the persisted task state machine.
type RemoteTask struct {
	ExternalTaskID string
	State          Status
	// FailureCode is required for a failed terminal observation and must be
	// empty for all other states. Adapters map provider-specific errors to this
	// stable, bounded code before returning it to the application layer.
	FailureCode string
}

// FetchRequest contains only immutable task facts needed by a future result
// adapter. It carries no provider SDK or object-store type.
type FetchRequest struct {
	TaskID         string
	ExternalTaskID string
	ObjectKey      string
	Purpose        Purpose
	LookID         string
	LookRevision   int
	Inputs         InputSnapshot
}

// FetchedResult is returned after an adapter has downloaded and validated the
// provider bytes into a private object version. The echoed immutable identity
// lets the application worker reject a result fetched for another task before
// it commits the output atomically.
type FetchedResult struct {
	ExternalTaskID string
	TaskID         string
	Purpose        Purpose
	LookID         string
	LookRevision   int
	Inputs         InputSnapshot
	Fact           OutputFact
}

// ResultFetcher is the only port needed by the provider-neutral output worker.
// It is intentionally not wired into bootstrap while Provider/object-store
// gates remain closed.
type ResultFetcher interface {
	Fetch(context.Context, FetchRequest) (FetchedResult, error)
}

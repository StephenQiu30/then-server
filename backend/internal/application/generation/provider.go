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

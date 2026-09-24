package generation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidGenerationResultWorker = errors.New("invalid generation result worker")
	// ErrGenerationResultUnavailable means the task is not yet ready for a
	// result fetch. A queued/running task must first receive a provider success
	// observation and enter validating.
	ErrGenerationResultUnavailable = errors.New("generation result is not ready")
)

// ResultWorkerRepository is the fenced persistence boundary for private
// output publication. AcquireResultLease only claims validating tasks with an
// accepted external identity; PublishOutput atomically writes the immutable
// asset, terminal task state and quota settlement.
type ResultWorkerRepository interface {
	AcquireResultLease(context.Context, string, string, time.Time, time.Duration) (TaskView, Lease, error)
	ReleaseLease(context.Context, Lease, time.Time) (TaskView, error)
	FinalizeWithoutOutput(context.Context, Lease, Status, string, time.Time) (TaskView, error)
	PublishOutput(context.Context, Lease, OutputAsset, time.Time) (TaskView, error)
}

// ResultWorkerQueue is the durable scheduler boundary used by RunNext. It
// only returns validating tasks with an accepted provider identity, so a
// restarted result worker cannot publish a queued or running task.
type ResultWorkerQueue interface {
	ClaimNextResultLease(context.Context, string, time.Time, time.Duration) (TaskView, Lease, bool, error)
}

// ResultWorkerPolicy bounds the private result fetch. No provider or
// object-store call occurs until a concrete adapter is injected by a future
// enabled worker.
type ResultWorkerPolicy struct {
	WorkerID     string
	LeaseTTL     time.Duration
	FetchTimeout time.Duration
}

func (p ResultWorkerPolicy) Validate() error {
	if !validID(p.WorkerID) || p.LeaseTTL <= 0 || p.LeaseTTL > 30*time.Minute || p.FetchTimeout <= 0 || p.FetchTimeout > 10*time.Minute {
		return ErrInvalidGenerationResultWorker
	}
	return nil
}

type ResultOutcome string

const (
	ResultOutcomePublished ResultOutcome = "published"
	ResultOutcomeCanceled  ResultOutcome = "canceled"
)

// ResultWorkerResult contains only persisted task facts and a bounded outcome.
type ResultWorkerResult struct {
	View    TaskView
	Outcome ResultOutcome
}

// ResultWorker fetches one validating result, verifies its immutable output
// fact against the task, and commits the private asset through the repository.
// It never calls Provider directly and is intentionally not started by
// bootstrap.
type ResultWorker struct {
	repository ResultWorkerRepository
	fetcher    ResultFetcher
	policy     ResultWorkerPolicy
	now        func() time.Time
}

func NewResultWorker(repository ResultWorkerRepository, fetcher ResultFetcher, policy ResultWorkerPolicy) (*ResultWorker, error) {
	if repository == nil || fetcher == nil {
		return nil, ErrInvalidGenerationResultWorker
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &ResultWorker{repository: repository, fetcher: fetcher, policy: policy, now: time.Now}, nil
}

// RunOnce claims only a validating task. Fetch failures or invalid lineage
// release the lease without changing task state; a successful fact is
// converted into a new immutable asset and atomically settled.
func (w *ResultWorker) RunOnce(ctx context.Context, taskID string) (ResultWorkerResult, error) {
	if w == nil || w.repository == nil || w.fetcher == nil || w.policy.Validate() != nil || !validID(taskID) {
		return ResultWorkerResult{}, ErrInvalidGenerationResultWorker
	}
	at := w.now().UTC()
	view, lease, err := w.repository.AcquireResultLease(ctx, taskID, w.policy.WorkerID, at, w.policy.LeaseTTL)
	if err != nil {
		return ResultWorkerResult{}, err
	}
	return w.runClaimed(ctx, view, lease)
}

// RunNext claims and processes one validating task. It returns found=false
// when no result is ready at the current durable queue snapshot.
func (w *ResultWorker) RunNext(ctx context.Context) (bool, ResultWorkerResult, error) {
	if w == nil || w.repository == nil || w.fetcher == nil || w.policy.Validate() != nil {
		return false, ResultWorkerResult{}, ErrInvalidGenerationResultWorker
	}
	queue, ok := w.repository.(ResultWorkerQueue)
	if !ok {
		return false, ResultWorkerResult{}, ErrInvalidGenerationResultWorker
	}
	view, lease, found, err := queue.ClaimNextResultLease(ctx, w.policy.WorkerID, w.now().UTC(), w.policy.LeaseTTL)
	if err != nil || !found {
		return found, ResultWorkerResult{View: view}, err
	}
	result, err := w.runClaimed(ctx, view, lease)
	return true, result, err
}

func (w *ResultWorker) runClaimed(ctx context.Context, view TaskView, lease Lease) (ResultWorkerResult, error) {
	if view.Task.CancelRequestedAt != nil {
		canceled, err := w.repository.FinalizeWithoutOutput(ctx, lease, StatusCanceled, "", w.now().UTC())
		if err != nil {
			return ResultWorkerResult{View: view, Outcome: ResultOutcomeCanceled}, err
		}
		return ResultWorkerResult{View: canceled, Outcome: ResultOutcomeCanceled}, nil
	}
	objectKey, err := OutputObjectKey(view.Task)
	if err != nil {
		return w.releaseWithError(ctx, lease, view, err)
	}
	request := FetchRequest{
		TaskID:         view.Task.ID,
		ExternalTaskID: view.Task.ExternalTaskID,
		ObjectKey:      objectKey,
		Purpose:        view.Task.Purpose,
		LookID:         view.Task.LookID,
		LookRevision:   view.Task.LookRevision,
		Inputs:         cloneSnapshot(view.Task.Inputs),
	}
	fetchContext, cancel := context.WithTimeout(ctx, w.policy.FetchTimeout)
	fetched, fetchErr := w.fetcher.Fetch(fetchContext, request)
	cancel()
	if fetchErr != nil {
		return w.releaseWithError(ctx, lease, view, ErrGenerationOutputFetchUnknown)
	}
	if !fetchedResultMatches(view.Task, fetched) {
		return w.releaseWithError(ctx, lease, view, ErrInvalidProviderObservation)
	}
	publishedAt := w.now().UTC()
	asset, err := NewOutputAsset(uuid.NewString(), view.Task, fetched.Fact, publishedAt)
	if err != nil {
		return w.releaseWithError(ctx, lease, view, err)
	}
	published, err := w.repository.PublishOutput(ctx, lease, asset, publishedAt)
	if err != nil {
		return w.releaseWithError(ctx, lease, view, err)
	}
	return ResultWorkerResult{View: published, Outcome: ResultOutcomePublished}, nil
}

func fetchedResultMatches(task Task, fetched FetchedResult) bool {
	return validID(fetched.TaskID) && fetched.TaskID == task.ID &&
		fetched.ExternalTaskID == task.ExternalTaskID && validToken(fetched.ExternalTaskID, 256) &&
		fetched.Purpose == task.Purpose && fetched.LookID == task.LookID && fetched.LookRevision == task.LookRevision &&
		inputSnapshotsEqual(task.Inputs, fetched.Inputs) && validOutputObjectKey(task, fetched.Fact.ObjectKey)
}

func inputSnapshotsEqual(left, right InputSnapshot) bool {
	if left.LookID != right.LookID || left.LookRevision != right.LookRevision || left.ImageAssetID != right.ImageAssetID || left.ImageSHA256 != right.ImageSHA256 || len(left.References) != len(right.References) {
		return false
	}
	for index := range left.References {
		if left.References[index] != right.References[index] {
			return false
		}
	}
	return true
}

func (w *ResultWorker) releaseWithError(ctx context.Context, lease Lease, view TaskView, resultErr error) (ResultWorkerResult, error) {
	released, releaseErr := w.repository.ReleaseLease(ctx, lease, w.now().UTC())
	if releaseErr != nil {
		return ResultWorkerResult{View: view}, releaseErr
	}
	return ResultWorkerResult{View: released}, resultErr
}

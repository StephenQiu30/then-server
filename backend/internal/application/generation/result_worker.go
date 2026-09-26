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
	// ErrGenerationOutputInventoryUnknown means object storage could not provide
	// a complete version inventory, so the worker must not create another output.
	ErrGenerationOutputInventoryUnknown = errors.New("generation output inventory requires retry")
	// ErrGenerationOutputCleanupPending means previously written versions were
	// durably queued for cleanup before another fetch can safely begin.
	ErrGenerationOutputCleanupPending = errors.New("generation output cleanup is pending")
)

// ResultWorkerRepository is the fenced persistence boundary for private
// output publication. AcquireResultLease only claims validating tasks with an
// accepted external identity; PublishOutput atomically writes the immutable
// asset, terminal task state and quota settlement.
type ResultWorkerRepository interface {
	AcquireResultLease(context.Context, string, string, time.Time, time.Duration) (TaskView, Lease, error)
	ReleaseLease(context.Context, Lease, time.Time) (TaskView, error)
	ReleaseLeaseForRetry(context.Context, Lease, time.Time, time.Duration) (TaskView, error)
	FinalizeWithoutOutput(context.Context, Lease, Status, string, time.Time) (TaskView, error)
	PublishOutput(context.Context, Lease, OutputAsset, time.Time) (TaskView, error)
	RecordUnpublishedOutput(context.Context, Lease, CleanupTarget, time.Time) error
}

// ResultWorkerQueue is the durable scheduler boundary used by RunNext. It
// only returns validating tasks with an accepted provider identity, so a
// restarted result worker cannot publish a queued or running task.
type ResultWorkerQueue interface {
	ClaimNextResultLease(context.Context, string, time.Time, time.Duration) (TaskView, Lease, bool, error)
}

// ProviderScopedResultWorkerQueue prevents an output adapter from fetching
// results for tasks owned by another provider.
type ProviderScopedResultWorkerQueue interface {
	ClaimNextResultLeaseForProvider(context.Context, string, string, time.Time, time.Duration) (TaskView, Lease, bool, error)
}

// ResultWorkerPolicy bounds output inventory, fetch, and object reads.
type ResultWorkerPolicy struct {
	WorkerID     string
	Provider     string
	LeaseTTL     time.Duration
	FetchTimeout time.Duration
	RetryDelay   time.Duration
}

func (p ResultWorkerPolicy) Validate() error {
	if !validID(p.WorkerID) || (p.Provider != "" && !validToken(p.Provider, 96)) || p.LeaseTTL <= 0 || p.LeaseTTL > 30*time.Minute || p.FetchTimeout <= 0 || p.FetchTimeout > 10*time.Minute || p.RetryDelay <= 0 || p.RetryDelay > maxWorkerRetryDelay {
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

// ResultWorker recovers unpublished versions before fetching, verifies the
// immutable output fact against the task, and commits the private asset. It
// never calls Provider directly; bootstrap currently supplies the local fixture.
type ResultWorker struct {
	repository ResultWorkerRepository
	fetcher    ResultFetcher
	outputs    OutputVersionReader
	policy     ResultWorkerPolicy
	now        func() time.Time
}

func NewResultWorker(repository ResultWorkerRepository, fetcher ResultFetcher, outputs OutputVersionReader, policy ResultWorkerPolicy) (*ResultWorker, error) {
	if repository == nil || fetcher == nil || outputs == nil {
		return nil, ErrInvalidGenerationResultWorker
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &ResultWorker{repository: repository, fetcher: fetcher, outputs: outputs, policy: policy, now: time.Now}, nil
}

// RunOnce claims only a validating task. Fetch failures or invalid lineage
// release the lease without changing task state; a successful fact is
// converted into a new immutable asset and atomically settled.
func (w *ResultWorker) RunOnce(ctx context.Context, taskID string) (ResultWorkerResult, error) {
	if w == nil {
		return ResultWorkerResult{}, ErrInvalidGenerationResultWorker
	}
	return w.RunOnceAt(ctx, taskID, w.now().UTC())
}

// RunOnceAt claims and processes one result at an explicit time. Runtime
// callers should use RunOnce; the explicit form keeps persistence tests
// deterministic across their staged task timestamps.
func (w *ResultWorker) RunOnceAt(ctx context.Context, taskID string, at time.Time) (ResultWorkerResult, error) {
	if w == nil || w.repository == nil || w.fetcher == nil || w.outputs == nil || w.policy.Validate() != nil || !validID(taskID) {
		return ResultWorkerResult{}, ErrInvalidGenerationResultWorker
	}
	if at.IsZero() {
		return ResultWorkerResult{}, ErrInvalidGenerationResultWorker
	}
	at = at.UTC()
	view, lease, err := w.repository.AcquireResultLease(ctx, taskID, w.policy.WorkerID, at, w.policy.LeaseTTL)
	if err != nil {
		return ResultWorkerResult{}, err
	}
	return w.runClaimed(ctx, view, lease, func() time.Time { return at })
}

// RunNext claims and processes one validating task. It returns found=false
// when no result is ready at the current durable queue snapshot.
func (w *ResultWorker) RunNext(ctx context.Context) (bool, ResultWorkerResult, error) {
	if w == nil || w.repository == nil || w.fetcher == nil || w.outputs == nil || w.policy.Validate() != nil {
		return false, ResultWorkerResult{}, ErrInvalidGenerationResultWorker
	}
	var view TaskView
	var lease Lease
	var found bool
	var err error
	if w.policy.Provider != "" {
		queue, ok := w.repository.(ProviderScopedResultWorkerQueue)
		if !ok {
			return false, ResultWorkerResult{}, ErrInvalidGenerationResultWorker
		}
		view, lease, found, err = queue.ClaimNextResultLeaseForProvider(ctx, w.policy.WorkerID, w.policy.Provider, w.now().UTC(), w.policy.LeaseTTL)
	} else {
		queue, ok := w.repository.(ResultWorkerQueue)
		if !ok {
			return false, ResultWorkerResult{}, ErrInvalidGenerationResultWorker
		}
		view, lease, found, err = queue.ClaimNextResultLease(ctx, w.policy.WorkerID, w.now().UTC(), w.policy.LeaseTTL)
	}
	if err != nil || !found {
		return found, ResultWorkerResult{View: view}, err
	}
	result, err := w.runClaimed(ctx, view, lease, w.now)
	return true, result, err
}

func (w *ResultWorker) runClaimed(ctx context.Context, view TaskView, lease Lease, now func() time.Time) (ResultWorkerResult, error) {
	objectKey, err := OutputObjectKey(view.Task)
	if err != nil {
		return w.releaseWithError(ctx, lease, view, err, now)
	}
	fetchContext, cancel := context.WithTimeout(ctx, w.policy.FetchTimeout)
	defer cancel()
	versions, err := w.outputs.ListOutputVersions(fetchContext, objectKey)
	if err != nil {
		return w.releaseWithError(ctx, lease, view, ErrGenerationOutputInventoryUnknown, now)
	}
	if len(versions) > 0 {
		for _, versionID := range versions {
			if !validToken(versionID, 160) {
				return w.releaseWithError(ctx, lease, view, ErrGenerationOutputInventoryUnknown, now)
			}
			target := CleanupTarget{Kind: CleanupTargetObject, ID: view.Task.ID, ObjectKey: objectKey, ObjectVersionID: versionID}
			if err := w.repository.RecordUnpublishedOutput(ctx, lease, target, now().UTC()); err != nil {
				return w.releaseWithError(ctx, lease, view, err, now)
			}
		}
		if view.Task.CancelRequestedAt == nil {
			return w.releaseWithError(ctx, lease, view, ErrGenerationOutputCleanupPending, now)
		}
	}
	// A previous fetch can have written an unreported version before cancellation.
	// Keep its exact cleanup target before the task becomes terminal.
	if view.Task.CancelRequestedAt != nil {
		canceled, err := w.repository.FinalizeWithoutOutput(ctx, lease, StatusCanceled, "", now().UTC())
		if err != nil {
			return ResultWorkerResult{View: view, Outcome: ResultOutcomeCanceled}, err
		}
		return ResultWorkerResult{View: canceled, Outcome: ResultOutcomeCanceled}, nil
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
	fetched, fetchErr := w.fetcher.Fetch(fetchContext, request)
	if fetchErr != nil {
		return w.releaseWithError(ctx, lease, view, ErrGenerationOutputFetchUnknown, now)
	}
	if !fetchedResultMatches(view.Task, fetched) {
		return w.releaseAfterUnpublishedOutput(ctx, lease, view, objectKey, fetched.Fact, ErrInvalidProviderObservation, now)
	}
	outputBytes, readErr := w.outputs.ReadOutputVersion(fetchContext, fetched.Fact.ObjectKey, fetched.Fact.ObjectVersionID, maxOutputBytes(view.Task.Purpose))
	if readErr != nil {
		return w.releaseWithError(ctx, lease, view, ErrGenerationOutputFetchUnknown, now)
	}
	verified, verifyErr := VerifyOutputContent(view.Task.Purpose, outputBytes)
	if verifyErr != nil || verified.ContentType != fetched.Fact.ContentType || verified.ByteSize != fetched.Fact.ByteSize || verified.SHA256 != fetched.Fact.SHA256 {
		return w.releaseAfterUnpublishedOutput(ctx, lease, view, objectKey, fetched.Fact, ErrInvalidGenerationOutput, now)
	}
	publishedAt := now().UTC()
	asset, err := NewOutputAsset(uuid.NewString(), view.Task, fetched.Fact, publishedAt)
	if err != nil {
		return w.releaseAfterUnpublishedOutput(ctx, lease, view, objectKey, fetched.Fact, err, now)
	}
	published, err := w.repository.PublishOutput(ctx, lease, asset, publishedAt)
	if err != nil {
		return w.releaseAfterUnpublishedOutput(ctx, lease, view, objectKey, fetched.Fact, err, now)
	}
	return ResultWorkerResult{View: published, Outcome: ResultOutcomePublished}, nil
}

func (w *ResultWorker) releaseAfterUnpublishedOutput(ctx context.Context, lease Lease, view TaskView, expectedObjectKey string, fact OutputFact, resultErr error, now func() time.Time) (ResultWorkerResult, error) {
	if fact.ObjectKey == expectedObjectKey && validToken(fact.ObjectVersionID, 160) {
		target := CleanupTarget{Kind: CleanupTargetObject, ID: view.Task.ID, ObjectKey: fact.ObjectKey, ObjectVersionID: fact.ObjectVersionID}
		if err := w.repository.RecordUnpublishedOutput(ctx, lease, target, now().UTC()); err != nil {
			released, releaseErr := w.repository.ReleaseLeaseForRetry(ctx, lease, now().UTC(), w.policy.RetryDelay)
			if releaseErr != nil {
				return ResultWorkerResult{View: view}, errors.Join(err, releaseErr)
			}
			return ResultWorkerResult{View: released}, err
		}
	}
	return w.releaseWithError(ctx, lease, view, resultErr, now)
}

func fetchedResultMatches(task Task, fetched FetchedResult) bool {
	return validID(fetched.TaskID) && fetched.TaskID == task.ID &&
		fetched.ExternalTaskID == task.ExternalTaskID && validToken(fetched.ExternalTaskID, 256) &&
		fetched.Purpose == task.Purpose && fetched.LookID == task.LookID && fetched.LookRevision == task.LookRevision &&
		inputSnapshotsEqual(task.Inputs, fetched.Inputs) && validOutputFact(task.Purpose, fetched.Fact) && validOutputObjectKey(task, fetched.Fact.ObjectKey)
}

func maxOutputBytes(purpose Purpose) int64 {
	if purpose == PurposeImage {
		return MaxGenerationImageOutputBytes
	}
	return MaxGenerationModelOutputBytes
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

func (w *ResultWorker) releaseWithError(ctx context.Context, lease Lease, view TaskView, resultErr error, now func() time.Time) (ResultWorkerResult, error) {
	released, releaseErr := w.repository.ReleaseLeaseForRetry(ctx, lease, now().UTC(), w.policy.RetryDelay)
	if releaseErr != nil {
		return ResultWorkerResult{View: view}, releaseErr
	}
	return ResultWorkerResult{View: released}, resultErr
}

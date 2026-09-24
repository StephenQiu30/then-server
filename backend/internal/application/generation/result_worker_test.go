package generation

import (
	"context"
	"errors"
	"testing"
	"time"
)

type resultWorkerFetcherStub struct {
	result  FetchedResult
	err     error
	fetches int
}

func (f *resultWorkerFetcherStub) Fetch(_ context.Context, _ FetchRequest) (FetchedResult, error) {
	f.fetches++
	if f.err != nil {
		return FetchedResult{}, f.err
	}
	return f.result, nil
}

func (r *observationWorkerRepositoryStub) AcquireResultLease(_ context.Context, _ string, owner string, at time.Time, ttl time.Duration) (TaskView, Lease, error) {
	if r.task.Status != StatusValidating || r.task.ExternalTaskID == "" || r.task.SubmissionState != SubmissionAccepted {
		return TaskView{}, Lease{}, ErrGenerationResultUnavailable
	}
	lease, err := r.task.AcquireLease(owner, at, ttl)
	if err != nil {
		return TaskView{}, Lease{}, err
	}
	return TaskView{Task: r.task, Reservation: r.reservation}, lease, nil
}

func (r *observationWorkerRepositoryStub) PublishOutput(_ context.Context, lease Lease, asset OutputAsset, at time.Time) (TaskView, error) {
	if err := r.task.ValidateLease(lease, at); err != nil {
		return TaskView{}, err
	}
	settlement, err := PublishOutput(r.task, r.reservation, asset, at)
	if err != nil {
		return TaskView{}, err
	}
	r.task = settlement.Task
	r.reservation = settlement.Reservation
	return TaskView{Task: r.task, Reservation: r.reservation, Asset: settlement.Asset}, nil
}

func validatingResultTask(t *testing.T) Task {
	t.Helper()
	task := acceptedObservationTask(t)
	lease, err := task.AcquireLease("observation-worker", generationTestNow.Add(4*time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.ApplyProviderState("provider-job-1", StatusValidating, "", generationTestNow.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.ReleaseLease(lease, generationTestNow.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	return task
}

func newResultWorker(t *testing.T, repository *observationWorkerRepositoryStub, fetcher *resultWorkerFetcherStub) *ResultWorker {
	t.Helper()
	worker, err := NewResultWorker(repository, fetcher, ResultWorkerPolicy{
		WorkerID:     "result-worker",
		LeaseTTL:     10 * time.Minute,
		FetchTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return generationTestNow.Add(6 * time.Minute) }
	return worker
}

func TestResultWorkerPublishesValidatedOutputAtomically(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: validatingResultTask(t)}
	fetcher := &resultWorkerFetcherStub{result: FetchedResult{ExternalTaskID: "provider-job-1", Fact: imageOutputFact()}}
	worker := newResultWorker(t, repository, fetcher)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Outcome != ResultOutcomePublished || result.View.Task.Status != StatusSucceeded || result.View.Task.ResultAssetID == "" || result.View.Asset == nil {
		t.Fatalf("unexpected result publication: %+v", result)
	}
	if result.View.Task.LeaseOwner != "" || fetcher.fetches != 1 {
		t.Fatalf("publication retained lease or fetched unexpected count: task=%+v fetches=%d", result.View.Task, fetcher.fetches)
	}
}

func TestResultWorkerKeepsValidatingTaskOnFetchFailure(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: validatingResultTask(t)}
	fetcher := &resultWorkerFetcherStub{err: errors.New("provider output unavailable")}
	worker := newResultWorker(t, repository, fetcher)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationOutputFetchUnknown) {
		t.Fatalf("RunOnce() error = %v, want fetch retry", err)
	}
	if result.View.Task.Status != StatusValidating || result.View.Task.ResultAssetID != "" || result.View.Task.LeaseOwner != "" {
		t.Fatalf("fetch failure changed task or retained lease: %+v", result.View.Task)
	}
}

func TestResultWorkerRejectsWrongLineageOrFact(t *testing.T) {
	tests := []struct {
		name   string
		result FetchedResult
	}{
		{name: "external identity", result: FetchedResult{ExternalTaskID: "other-task", Fact: imageOutputFact()}},
		{name: "wrong content type", result: FetchedResult{ExternalTaskID: "provider-job-1", Fact: modelOutputFact()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &observationWorkerRepositoryStub{task: validatingResultTask(t)}
			fetcher := &resultWorkerFetcherStub{result: test.result}
			worker := newResultWorker(t, repository, fetcher)

			result, err := worker.RunOnce(context.Background(), repository.task.ID)
			if !errors.Is(err, ErrInvalidProviderObservation) && !errors.Is(err, ErrInvalidGenerationOutput) {
				t.Fatalf("RunOnce() error = %v, want invalid result", err)
			}
			if result.View.Task.Status != StatusValidating || result.View.Task.ResultAssetID != "" || result.View.Task.LeaseOwner != "" {
				t.Fatalf("invalid result changed task or retained lease: %+v", result.View.Task)
			}
		})
	}
}

func TestResultWorkerRejectsTaskBeforeValidating(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	fetcher := &resultWorkerFetcherStub{result: FetchedResult{ExternalTaskID: "provider-job-1", Fact: imageOutputFact()}}
	worker := newResultWorker(t, repository, fetcher)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationResultUnavailable) {
		t.Fatalf("RunOnce() error = %v, want result unavailable", err)
	}
	if result.View.Task.ID != "" || fetcher.fetches != 0 {
		t.Fatalf("non-validating task was fetched or returned as claimed: %+v fetches=%d", result, fetcher.fetches)
	}
}

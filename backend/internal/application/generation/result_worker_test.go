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
	request FetchRequest
	fetches int
}

func (f *resultWorkerFetcherStub) Fetch(_ context.Context, request FetchRequest) (FetchedResult, error) {
	f.request = request
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

func (r *observationWorkerRepositoryStub) ClaimNextResultLease(_ context.Context, owner string, at time.Time, ttl time.Duration) (TaskView, Lease, bool, error) {
	if r.task.Status != StatusValidating || r.task.ExternalTaskID == "" || r.task.SubmissionState != SubmissionAccepted || r.task.AccessRevokedAt != nil {
		return TaskView{}, Lease{}, false, nil
	}
	view, lease, err := r.AcquireResultLease(context.Background(), r.task.ID, owner, at, ttl)
	return view, lease, err == nil, err
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
	fetcher := &resultWorkerFetcherStub{result: fetchedImageResult(repository.task)}
	worker := newResultWorker(t, repository, fetcher)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Outcome != ResultOutcomePublished || result.View.Task.Status != StatusSucceeded || result.View.Task.ResultAssetID == "" || result.View.Asset == nil {
		t.Fatalf("unexpected result publication: %+v", result)
	}
	if result.View.Task.LeaseOwner != "" || fetcher.fetches != 1 || fetcher.request.TaskID != repository.task.ID || fetcher.request.LookID != repository.task.LookID || fetcher.request.LookRevision != repository.task.LookRevision || !inputSnapshotsEqual(fetcher.request.Inputs, repository.task.Inputs) {
		t.Fatalf("publication retained lease or fetched unexpected count: task=%+v fetches=%d", result.View.Task, fetcher.fetches)
	}
}

func TestResultWorkerRunNextClaimsValidatingQueue(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: validatingResultTask(t)}
	fetcher := &resultWorkerFetcherStub{result: fetchedImageResult(repository.task)}
	worker := newResultWorker(t, repository, fetcher)

	found, result, err := worker.RunNext(context.Background())
	if err != nil || !found || result.Outcome != ResultOutcomePublished || repository.task.Status != StatusSucceeded {
		t.Fatalf("RunNext() = found=%v result=%+v err=%v", found, result, err)
	}
	if fetcher.fetches != 1 || repository.task.LeaseOwner != "" {
		t.Fatalf("RunNext() did not publish and release exactly once: fetches=%d task=%+v", fetcher.fetches, repository.task)
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
		mutate func(*FetchedResult)
	}{
		{name: "external identity", mutate: func(result *FetchedResult) { result.ExternalTaskID = "other-task" }},
		{name: "task identity", mutate: func(result *FetchedResult) { result.TaskID = "other-task" }},
		{name: "purpose", mutate: func(result *FetchedResult) { result.Purpose = PurposeModel }},
		{name: "look identity", mutate: func(result *FetchedResult) { result.LookID = "other-look" }},
		{name: "look revision", mutate: func(result *FetchedResult) { result.LookRevision++ }},
		{name: "input snapshot", mutate: func(result *FetchedResult) { result.Inputs.References[0].Revision++ }},
		{name: "wrong content type", mutate: func(result *FetchedResult) { result.Fact = modelOutputFact() }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &observationWorkerRepositoryStub{task: validatingResultTask(t)}
			resultFact := fetchedImageResult(repository.task)
			test.mutate(&resultFact)
			fetcher := &resultWorkerFetcherStub{result: resultFact}
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
	fetcher := &resultWorkerFetcherStub{result: fetchedImageResult(repository.task)}
	worker := newResultWorker(t, repository, fetcher)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationResultUnavailable) {
		t.Fatalf("RunOnce() error = %v, want result unavailable", err)
	}
	if result.View.Task.ID != "" || fetcher.fetches != 0 {
		t.Fatalf("non-validating task was fetched or returned as claimed: %+v fetches=%d", result, fetcher.fetches)
	}
}

func fetchedImageResult(task Task) FetchedResult {
	return FetchedResult{
		ExternalTaskID: task.ExternalTaskID,
		TaskID:         task.ID,
		Purpose:        task.Purpose,
		LookID:         task.LookID,
		LookRevision:   task.LookRevision,
		Inputs:         cloneSnapshot(task.Inputs),
		Fact:           imageOutputFact(),
	}
}

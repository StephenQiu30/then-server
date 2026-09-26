package generation

import (
	"context"
	"errors"
	"testing"
	"time"
)

type observationWorkerProviderStub struct {
	remote      RemoteTask
	queryErr    error
	cancelErr   error
	cancelState *Status
	queries     int
	cancels     int
	deadlines   []time.Time
}

func (p *observationWorkerProviderStub) Submit(context.Context, Submission) (Receipt, error) {
	return Receipt{}, errors.New("submit should not be called by observation worker")
}

func (p *observationWorkerProviderStub) Query(ctx context.Context, _ string) (RemoteTask, error) {
	p.queries++
	if deadline, ok := ctx.Deadline(); ok {
		p.deadlines = append(p.deadlines, deadline)
	}
	if p.queryErr != nil {
		return RemoteTask{}, p.queryErr
	}
	return p.remote, nil
}

func (p *observationWorkerProviderStub) Cancel(ctx context.Context, externalID string) error {
	p.cancels++
	if deadline, ok := ctx.Deadline(); ok {
		p.deadlines = append(p.deadlines, deadline)
	}
	if p.cancelErr != nil {
		return p.cancelErr
	}
	if p.remote.ExternalTaskID != externalID {
		return errors.New("unexpected external task")
	}
	if p.cancelState != nil {
		p.remote.State = *p.cancelState
	}
	return nil
}

type observationWorkerRepositoryStub struct {
	task               Task
	reservation        *QuotaReservation
	unpublishedTargets []CleanupTarget
}

func (r *observationWorkerRepositoryStub) AcquireObservationLease(_ context.Context, _ string, owner string, at time.Time, ttl time.Duration) (TaskView, Lease, error) {
	if r.task.ExternalTaskID == "" || r.task.SubmissionState != SubmissionAccepted {
		return TaskView{}, Lease{}, ErrGenerationObservationUnavailable
	}
	lease, err := r.task.AcquireLease(owner, at, ttl)
	if err != nil {
		return TaskView{}, Lease{}, err
	}
	return TaskView{Task: r.task, Reservation: r.reservation}, lease, nil
}

func (r *observationWorkerRepositoryStub) ClaimNextObservationLease(_ context.Context, owner string, at time.Time, ttl time.Duration) (TaskView, Lease, bool, error) {
	if r.task.Status != StatusRunning || r.task.ExternalTaskID == "" || r.task.SubmissionState != SubmissionAccepted || r.task.AccessRevokedAt != nil || (r.task.NextAttemptAt != nil && at.Before(*r.task.NextAttemptAt)) {
		return TaskView{}, Lease{}, false, nil
	}
	view, lease, err := r.AcquireObservationLease(context.Background(), r.task.ID, owner, at, ttl)
	return view, lease, err == nil, err
}

func (r *observationWorkerRepositoryStub) ReleaseLease(_ context.Context, lease Lease, at time.Time) (TaskView, error) {
	if err := r.task.ReleaseLease(lease, at); err != nil {
		return TaskView{}, err
	}
	return TaskView{Task: r.task, Reservation: r.reservation}, nil
}

func (r *observationWorkerRepositoryStub) ReleaseLeaseForRetry(_ context.Context, lease Lease, at time.Time, delay time.Duration) (TaskView, error) {
	if err := r.task.ReleaseLeaseForRetry(lease, at, delay); err != nil {
		return TaskView{}, err
	}
	return TaskView{Task: r.task, Reservation: r.reservation}, nil
}

func (r *observationWorkerRepositoryStub) ApplyProviderState(_ context.Context, lease Lease, externalID string, next Status, failureCode string, at time.Time) (TaskView, error) {
	if err := r.task.ValidateLease(lease, at); err != nil {
		return TaskView{}, err
	}
	if err := r.task.ApplyProviderState(externalID, next, failureCode, at); err != nil {
		return TaskView{}, err
	}
	return TaskView{Task: r.task, Reservation: r.reservation}, nil
}

func (r *observationWorkerRepositoryStub) FinalizeWithoutOutput(_ context.Context, lease Lease, next Status, failureCode string, at time.Time) (TaskView, error) {
	if err := r.task.ValidateLease(lease, at); err != nil {
		return TaskView{}, err
	}
	settlement, err := FinalizeWithoutOutput(r.task, r.reservation, next, failureCode, at)
	if err != nil {
		return TaskView{}, err
	}
	r.task = settlement.Task
	r.reservation = settlement.Reservation
	return TaskView{Task: r.task, Reservation: r.reservation}, nil
}

func acceptedObservationTask(t *testing.T) Task {
	t.Helper()
	task := mustTask(validCreateInput())
	lease, err := task.AcquireLease("submit-worker", generationTestNow.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.ReleaseLease(lease, generationTestNow.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	return task
}

func newObservationWorker(t *testing.T, repository *observationWorkerRepositoryStub, provider *observationWorkerProviderStub) *ObservationWorker {
	t.Helper()
	worker, err := NewObservationWorker(repository, provider, ObservationWorkerPolicy{
		WorkerID:        "observe-worker",
		LeaseTTL:        10 * time.Minute,
		ProviderTimeout: time.Minute,
		PollInterval:    10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return generationTestNow.Add(4 * time.Minute) }
	return worker
}

func TestObservationWorkerMapsProviderSuccessToValidatingWithoutPublishing(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	provider := &observationWorkerProviderStub{remote: RemoteTask{ExternalTaskID: "provider-job-1", State: StatusSucceeded}}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Outcome != ObservationOutcomeValidating || result.View.Task.Status != StatusValidating {
		t.Fatalf("unexpected observation result: %+v", result)
	}
	if result.View.Task.ResultAssetID != "" || result.View.Task.LeaseOwner != "" {
		t.Fatalf("provider success published or retained lease: %+v", result.View.Task)
	}
	if provider.queries != 1 {
		t.Fatalf("provider queries = %d, want 1", provider.queries)
	}
}

func TestObservationWorkerRunNextClaimsRunningQueue(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	provider := &observationWorkerProviderStub{remote: RemoteTask{ExternalTaskID: "provider-job-1", State: StatusRunning}}
	worker := newObservationWorker(t, repository, provider)

	found, result, err := worker.RunNext(context.Background())
	if err != nil || !found || result.Outcome != ObservationOutcomeRunning || result.View.Task.Status != StatusRunning {
		t.Fatalf("RunNext() = found=%v result=%+v err=%v", found, result, err)
	}
	if provider.queries != 1 || repository.task.LeaseOwner != "" || repository.task.NextAttemptAt == nil || !repository.task.NextAttemptAt.Equal(generationTestNow.Add(4*time.Minute+10*time.Second)) {
		t.Fatalf("RunNext() did not query and release exactly once: queries=%d task=%+v", provider.queries, repository.task)
	}
	worker.now = func() time.Time { return generationTestNow.Add(4*time.Minute + 9*time.Second) }
	if found, _, err := worker.RunNext(context.Background()); err != nil || found || provider.queries != 1 {
		t.Fatalf("RunNext() claimed before poll time: found=%v queries=%d err=%v", found, provider.queries, err)
	}
	worker.now = func() time.Time { return generationTestNow.Add(4*time.Minute + 10*time.Second) }
	provider.remote.State = StatusValidating
	if found, result, err := worker.RunNext(context.Background()); err != nil || !found || result.View.Task.Status != StatusValidating || provider.queries != 2 {
		t.Fatalf("RunNext() did not claim at poll time: found=%v result=%+v queries=%d err=%v", found, result, provider.queries, err)
	}
}

func TestObservationWorkerCancelsRequestedProviderTaskBeforeSettling(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	if err := repository.task.RequestCancel(generationTestNow.Add(3*time.Minute + 30*time.Second)); err != nil {
		t.Fatalf("RequestCancel() error = %v", err)
	}
	canceled := StatusCanceled
	provider := &observationWorkerProviderStub{
		remote:      RemoteTask{ExternalTaskID: "provider-job-1", State: StatusRunning},
		cancelState: &canceled,
	}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Outcome != ObservationOutcomeTerminal || result.View.Task.Status != StatusCanceled || result.View.Task.LeaseOwner != "" {
		t.Fatalf("requested cancellation did not settle task: %+v", result)
	}
	if provider.cancels != 1 || provider.queries != 2 {
		t.Fatalf("provider cancellation/reconciliation calls = cancel %d query %d, want 1/2", provider.cancels, provider.queries)
	}
	if len(provider.deadlines) != 3 || !provider.deadlines[0].Equal(provider.deadlines[1]) || !provider.deadlines[1].Equal(provider.deadlines[2]) {
		t.Fatalf("query, cancel and reconciliation did not share one deadline: %v", provider.deadlines)
	}
}

func TestObservationWorkerKeepsCancellationRequestWhenProviderCancelFails(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	if err := repository.task.RequestCancel(generationTestNow.Add(3*time.Minute + 30*time.Second)); err != nil {
		t.Fatalf("RequestCancel() error = %v", err)
	}
	provider := &observationWorkerProviderStub{
		remote:    RemoteTask{ExternalTaskID: "provider-job-1", State: StatusRunning},
		cancelErr: errors.New("provider timeout"),
	}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationCancellationUnknown) {
		t.Fatalf("RunOnce() error = %v, want cancellation retry", err)
	}
	if result.View.Task.Status != StatusRunning || result.View.Task.CancelRequestedAt == nil || result.View.Task.LeaseOwner != "" || result.View.Task.NextAttemptAt == nil {
		t.Fatalf("failed cancellation changed task or retained lease: %+v", result.View.Task)
	}
	if provider.cancels != 1 || provider.queries != 1 {
		t.Fatalf("provider calls = cancel %d query %d, want 1/1", provider.cancels, provider.queries)
	}
}

func TestObservationWorkerMapsQueuedProviderStateToRunning(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	provider := &observationWorkerProviderStub{remote: RemoteTask{ExternalTaskID: "provider-job-1", State: StatusQueued}}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil || result.Outcome != ObservationOutcomeRunning || result.View.Task.Status != StatusRunning {
		t.Fatalf("queued provider state = result=%+v err=%v", result, err)
	}
}

func TestObservationWorkerFinalizesProviderFailureAndReleasesQuota(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	provider := &observationWorkerProviderStub{remote: RemoteTask{ExternalTaskID: "provider-job-1", State: StatusFailed, FailureCode: "provider_error"}}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Outcome != ObservationOutcomeTerminal || result.View.Task.Status != StatusFailed || result.View.Task.FailureCode != "provider_error" {
		t.Fatalf("unexpected terminal result: %+v", result)
	}
	if result.View.Task.LeaseOwner != "" || result.View.Task.LeaseUntil != nil {
		t.Fatalf("terminal observation retained lease: %+v", result.View.Task)
	}
}

func TestObservationWorkerRejectsMalformedObservationWithoutAdvancingTask(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	provider := &observationWorkerProviderStub{remote: RemoteTask{ExternalTaskID: "other-task", State: StatusRunning}}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrInvalidProviderObservation) {
		t.Fatalf("RunOnce() error = %v, want invalid observation", err)
	}
	if result.View.Task.Status != StatusRunning || result.View.Task.LeaseOwner != "" || result.View.Task.NextAttemptAt == nil {
		t.Fatalf("malformed observation changed task or retained lease: %+v", result.View.Task)
	}
}

func TestObservationWorkerKeepsTaskStateOnQueryFailure(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	provider := &observationWorkerProviderStub{queryErr: errors.New("timeout")}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationObservationUnknown) {
		t.Fatalf("RunOnce() error = %v, want observation retry", err)
	}
	if result.View.Task.Status != StatusRunning || result.View.Task.LeaseOwner != "" {
		t.Fatalf("query failure changed task or retained lease: %+v", result.View.Task)
	}
}

func TestObservationWorkerDoesNotQueryUnacceptedTask(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: mustTask(validCreateInput())}
	provider := &observationWorkerProviderStub{}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationObservationUnavailable) {
		t.Fatalf("RunOnce() error = %v, want unavailable", err)
	}
	if provider.queries != 0 || result.View.Task.ID != "" {
		t.Fatalf("unaccepted task was queried or returned as claimed: queries=%d result=%+v", provider.queries, result)
	}
}

func TestObservationWorkerRejectsInvalidFailureCode(t *testing.T) {
	repository := &observationWorkerRepositoryStub{task: acceptedObservationTask(t)}
	provider := &observationWorkerProviderStub{remote: RemoteTask{ExternalTaskID: "provider-job-1", State: StatusFailed}}
	worker := newObservationWorker(t, repository, provider)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrInvalidProviderObservation) {
		t.Fatalf("RunOnce() error = %v, want invalid observation", err)
	}
	if result.View.Task.Status != StatusRunning || result.View.Task.LeaseOwner != "" {
		t.Fatalf("invalid failure observation changed task or retained lease: %+v", result.View.Task)
	}
}

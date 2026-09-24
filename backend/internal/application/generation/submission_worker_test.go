package generation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type submissionWorkerProviderStub struct {
	results []struct {
		receipt Receipt
		err     error
	}
	calls int
}

func (p *submissionWorkerProviderStub) Submit(context.Context, Submission) (Receipt, error) {
	p.calls++
	result := struct {
		receipt Receipt
		err     error
	}{receipt: Receipt{ExternalTaskID: fmt.Sprintf("provider-%d", p.calls)}}
	if p.calls <= len(p.results) {
		result = p.results[p.calls-1]
	}
	return result.receipt, result.err
}

func (*submissionWorkerProviderStub) Query(context.Context, string) (RemoteTask, error) {
	return RemoteTask{}, errors.New("query is outside submission worker")
}

func (*submissionWorkerProviderStub) Cancel(context.Context, string) error {
	return errors.New("cancel is outside submission worker")
}

type submissionWorkerRepositoryStub struct {
	task        Task
	reservation *QuotaReservation
	lease       *Lease
}

func (r *submissionWorkerRepositoryStub) view() TaskView {
	return TaskView{Task: cloneTask(r.task), Reservation: cloneReservation(r.reservation)}
}

func (r *submissionWorkerRepositoryStub) AcquireLease(_ context.Context, _ string, owner string, at time.Time, ttl time.Duration) (TaskView, Lease, error) {
	lease, err := r.task.AcquireLease(owner, at, ttl)
	if err != nil {
		return TaskView{}, Lease{}, err
	}
	r.lease = &lease
	return r.view(), lease, nil
}

func (r *submissionWorkerRepositoryStub) ClaimNextSubmissionLease(_ context.Context, owner string, at time.Time, ttl time.Duration) (TaskView, Lease, bool, error) {
	if r.task.Status != StatusQueued && r.task.Status != StatusRunning || r.task.SubmissionState != SubmissionNotStarted || r.task.ExternalTaskID != "" || r.task.AccessRevokedAt != nil || (r.task.NextAttemptAt != nil && at.Before(*r.task.NextAttemptAt)) {
		return TaskView{}, Lease{}, false, nil
	}
	view, lease, err := r.AcquireLease(context.Background(), r.task.ID, owner, at, ttl)
	return view, lease, err == nil, err
}

func (r *submissionWorkerRepositoryStub) ReleaseLease(_ context.Context, lease Lease, at time.Time) (TaskView, error) {
	if err := r.task.ReleaseLease(lease, at); err != nil {
		return TaskView{}, err
	}
	r.lease = nil
	return r.view(), nil
}

func (r *submissionWorkerRepositoryStub) BeginSubmission(_ context.Context, lease Lease, at time.Time) (TaskView, Submission, error) {
	submission, err := r.task.BeginSubmission(at)
	if err != nil {
		return TaskView{}, Submission{}, err
	}
	if err := r.assertLease(lease); err != nil {
		return TaskView{}, Submission{}, err
	}
	return r.view(), submission, nil
}

func (r *submissionWorkerRepositoryStub) MarkSubmissionUnknown(_ context.Context, lease Lease, at time.Time) (TaskView, error) {
	if err := r.assertLease(lease); err != nil {
		return TaskView{}, err
	}
	if err := r.task.MarkSubmissionUnknown(at); err != nil {
		return TaskView{}, err
	}
	return r.view(), nil
}

func (r *submissionWorkerRepositoryStub) ReconcileSubmissionNotAccepted(_ context.Context, lease Lease, at time.Time) (TaskView, error) {
	if err := r.assertLease(lease); err != nil {
		return TaskView{}, err
	}
	if err := r.task.ReconcileSubmissionNotAccepted(at); err != nil {
		return TaskView{}, err
	}
	return r.view(), nil
}

func (r *submissionWorkerRepositoryStub) ScheduleSubmissionRetry(_ context.Context, lease Lease, at time.Time, policy RetryPolicy) (TaskView, error) {
	if err := r.assertLease(lease); err != nil {
		return TaskView{}, err
	}
	if err := r.task.ScheduleSubmissionRetry(at, policy); err != nil {
		return TaskView{}, err
	}
	return r.view(), nil
}

func (r *submissionWorkerRepositoryStub) RecordExternalTaskID(_ context.Context, lease Lease, externalID string, at time.Time) (TaskView, error) {
	if err := r.assertLease(lease); err != nil {
		return TaskView{}, err
	}
	if err := r.task.RecordExternalTaskID(externalID, at); err != nil {
		return TaskView{}, err
	}
	return r.view(), nil
}

func (r *submissionWorkerRepositoryStub) FinalizeWithoutOutput(_ context.Context, lease Lease, status Status, failureCode string, at time.Time) (TaskView, error) {
	if err := r.assertLease(lease); err != nil {
		return TaskView{}, err
	}
	settlement, err := FinalizeWithoutOutput(r.task, r.reservation, status, failureCode, at)
	if err != nil {
		return TaskView{}, err
	}
	r.task = settlement.Task
	r.reservation = settlement.Reservation
	r.lease = nil
	return r.view(), nil
}

func (r *submissionWorkerRepositoryStub) assertLease(lease Lease) error {
	if r.lease == nil || r.lease.TaskID != lease.TaskID || r.lease.Owner != lease.Owner || r.lease.FencingToken != lease.FencingToken || r.lease.Attempt != lease.Attempt || !r.lease.ExpiresAt.Equal(lease.ExpiresAt) {
		return ErrGenerationLeaseConflict
	}
	return nil
}

func newSubmissionWorkerTest(t *testing.T, provider Provider, task Task, policy SubmissionWorkerPolicy) (*SubmissionWorker, *submissionWorkerRepositoryStub, *time.Time) {
	t.Helper()
	now := generationTestNow.Add(time.Minute)
	repository := &submissionWorkerRepositoryStub{task: task}
	worker, err := NewSubmissionWorker(repository, provider, policy)
	if err != nil {
		t.Fatalf("NewSubmissionWorker() error = %v", err)
	}
	worker.now = func() time.Time { return *(&now) }
	return worker, repository, &now
}

func submissionWorkerPolicy() SubmissionWorkerPolicy {
	return SubmissionWorkerPolicy{
		WorkerID:        "worker-a",
		LeaseTTL:        10 * time.Minute,
		ProviderTimeout: time.Second,
		Retry:           RetryPolicy{MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: 10 * time.Second},
	}
}

func TestSubmissionWorkerRecordsAcceptedIdentityBeforeReleasingLease(t *testing.T) {
	provider := &submissionWorkerProviderStub{}
	worker, repository, now := newSubmissionWorkerTest(t, provider, mustTask(validCreateInput()), submissionWorkerPolicy())

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Outcome != SubmissionOutcomeAccepted || repository.task.ExternalTaskID != "provider-1" || repository.task.SubmissionState != SubmissionAccepted || repository.lease != nil {
		t.Fatalf("accepted submission was not durably released: outcome=%s task=%+v lease=%+v", result.Outcome, repository.task, repository.lease)
	}

	*now = now.Add(time.Minute)
	replayed, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("replay RunOnce() error = %v", err)
	}
	if replayed.Outcome != SubmissionOutcomeAlreadyAccepted || provider.calls != 1 {
		t.Fatalf("accepted task was submitted again: outcome=%s calls=%d", replayed.Outcome, provider.calls)
	}
}

func TestSubmissionWorkerRunNextClaimsDurableQueue(t *testing.T) {
	provider := &submissionWorkerProviderStub{}
	worker, repository, _ := newSubmissionWorkerTest(t, provider, mustTask(validCreateInput()), submissionWorkerPolicy())

	found, result, err := worker.RunNext(context.Background())
	if err != nil || !found || result.Outcome != SubmissionOutcomeAccepted || repository.task.ExternalTaskID != "provider-1" {
		t.Fatalf("RunNext() = found=%v result=%+v err=%v", found, result, err)
	}
	if repository.lease != nil || provider.calls != 1 {
		t.Fatalf("RunNext() retained lease or submitted unexpected count: lease=%+v calls=%d", repository.lease, provider.calls)
	}
}

func TestSubmissionWorkerBlocksBlindRetryAfterUnknownTransport(t *testing.T) {
	provider := &submissionWorkerProviderStub{results: []struct {
		receipt Receipt
		err     error
	}{{err: errors.New("transport timeout")}}}
	worker, repository, now := newSubmissionWorkerTest(t, provider, mustTask(validCreateInput()), submissionWorkerPolicy())

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationSubmissionUnknown) {
		t.Fatalf("unknown transport error = %v", err)
	}
	if result.View.Task.SubmissionState != SubmissionUnknown || repository.task.SubmissionState != SubmissionUnknown || repository.lease != nil {
		t.Fatalf("unknown submit was not fenced and released: result=%+v task=%+v lease=%+v", result, repository.task, repository.lease)
	}

	*now = now.Add(time.Minute)
	if _, err := worker.RunOnce(context.Background(), repository.task.ID); !errors.Is(err, ErrGenerationSubmissionUnknown) {
		t.Fatalf("unknown task allowed blind retry: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("unknown task called provider %d times", provider.calls)
	}
}

func TestSubmissionWorkerInvalidReceiptBlocksRetry(t *testing.T) {
	provider := &submissionWorkerProviderStub{results: []struct {
		receipt Receipt
		err     error
	}{{receipt: Receipt{}}}}
	worker, repository, _ := newSubmissionWorkerTest(t, provider, mustTask(validCreateInput()), submissionWorkerPolicy())

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrInvalidProviderReceipt) {
		t.Fatalf("invalid receipt error = %v", err)
	}
	if result.View.Task.SubmissionState != SubmissionUnknown || repository.task.SubmissionState != SubmissionUnknown || provider.calls != 1 {
		t.Fatalf("invalid receipt did not block retry: result=%+v task=%+v calls=%d", result, repository.task, provider.calls)
	}
}

func TestSubmissionWorkerRetriesOnlyAfterExplicitNotAcceptedResult(t *testing.T) {
	provider := &submissionWorkerProviderStub{results: []struct {
		receipt Receipt
		err     error
	}{
		{err: fmt.Errorf("provider rejected request: %w", ErrProviderNotAccepted)},
		{receipt: Receipt{ExternalTaskID: "provider-after-retry"}},
	}}
	worker, repository, now := newSubmissionWorkerTest(t, provider, mustTask(validCreateInput()), submissionWorkerPolicy())

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("not-accepted RunOnce() error = %v", err)
	}
	if result.Outcome != SubmissionOutcomeRetryScheduled || repository.task.NextAttemptAt == nil || provider.calls != 1 || repository.task.ExternalTaskID != "" {
		t.Fatalf("not-accepted response did not schedule one retry: outcome=%s task=%+v calls=%d", result.Outcome, repository.task, provider.calls)
	}

	*now = now.Add(500 * time.Millisecond)
	if _, err := worker.RunOnce(context.Background(), repository.task.ID); !errors.Is(err, ErrGenerationRetryNotReady) {
		t.Fatalf("retry ran before its window: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("early retry called provider %d times", provider.calls)
	}

	*now = *repository.task.NextAttemptAt
	result, err = worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("due retry RunOnce() error = %v", err)
	}
	if result.Outcome != SubmissionOutcomeAccepted || provider.calls != 2 || repository.task.ExternalTaskID != "provider-after-retry" {
		t.Fatalf("due retry was not accepted: outcome=%s task=%+v calls=%d", result.Outcome, repository.task, provider.calls)
	}
}

func TestSubmissionWorkerCancelsBeforeProviderSubmit(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.RequestCancel(generationTestNow.Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	provider := &submissionWorkerProviderStub{}
	worker, repository, _ := newSubmissionWorkerTest(t, provider, task, submissionWorkerPolicy())

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("canceled RunOnce() error = %v", err)
	}
	if result.Outcome != SubmissionOutcomeCanceled || repository.task.Status != StatusCanceled || provider.calls != 0 {
		t.Fatalf("canceled task reached provider: outcome=%s status=%s calls=%d", result.Outcome, repository.task.Status, provider.calls)
	}
}

func TestSubmissionWorkerReconcilesRecoveredInFlightSubmissionBeforeCancellation(t *testing.T) {
	task := mustTask(validCreateInput())
	lease, err := task.AcquireLease("previous-worker", generationTestNow.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.ReleaseLease(lease, generationTestNow.Add(2*time.Minute+time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := task.RequestCancel(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	provider := &submissionWorkerProviderStub{}
	worker, repository, now := newSubmissionWorkerTest(t, provider, task, submissionWorkerPolicy())
	*now = generationTestNow.Add(5 * time.Minute)
	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationSubmissionUnknown) {
		t.Fatalf("recovered in-flight cancellation error = %v", err)
	}
	if result.View.Task.Status == StatusCanceled || repository.task.Status == StatusCanceled {
		t.Fatalf("recovered in-flight submission was finalized as canceled: result=%+v task=%+v", result, repository.task)
	}
	if repository.task.SubmissionState != SubmissionUnknown || repository.lease != nil || provider.calls != 0 {
		t.Fatalf("recovered in-flight submission was not fenced and released: task=%+v lease=%+v calls=%d", repository.task, repository.lease, provider.calls)
	}
}

func TestSubmissionWorkerReconcilesRecoveredInFlightSubmissionWithoutProviderRetry(t *testing.T) {
	task := mustTask(validCreateInput())
	lease, err := task.AcquireLease("previous-worker", generationTestNow.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.ReleaseLease(lease, generationTestNow.Add(2*time.Minute+time.Second)); err != nil {
		t.Fatal(err)
	}

	provider := &submissionWorkerProviderStub{}
	worker, repository, now := newSubmissionWorkerTest(t, provider, task, submissionWorkerPolicy())
	*now = generationTestNow.Add(5 * time.Minute)
	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationSubmissionUnknown) {
		t.Fatalf("recovered in-flight error = %v", err)
	}
	if result.View.Task.SubmissionState != SubmissionUnknown || repository.task.SubmissionState != SubmissionUnknown || repository.lease != nil || provider.calls != 0 {
		t.Fatalf("recovered in-flight submission was not fenced: result=%+v task=%+v lease=%+v calls=%d", result, repository.task, repository.lease, provider.calls)
	}
}

func TestSubmissionWorkerKeepsUnknownCancellationPendingReconciliation(t *testing.T) {
	task := mustTask(validCreateInput())
	lease, err := task.AcquireLease("previous-worker", generationTestNow.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.MarkSubmissionUnknown(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.ReleaseLease(lease, generationTestNow.Add(3*time.Minute+time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := task.RequestCancel(generationTestNow.Add(4 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	provider := &submissionWorkerProviderStub{}
	worker, repository, now := newSubmissionWorkerTest(t, provider, task, submissionWorkerPolicy())
	*now = generationTestNow.Add(5 * time.Minute)
	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if !errors.Is(err, ErrGenerationSubmissionUnknown) {
		t.Fatalf("unknown cancellation error = %v", err)
	}
	if result.View.Task.Status == StatusCanceled || repository.task.Status == StatusCanceled {
		t.Fatalf("unknown cancellation was finalized: result=%+v task=%+v", result, repository.task)
	}
	if repository.task.SubmissionState != SubmissionUnknown || repository.lease != nil || provider.calls != 0 {
		t.Fatalf("unknown cancellation changed reconciliation state: task=%+v lease=%+v calls=%d", repository.task, repository.lease, provider.calls)
	}
}

func TestSubmissionWorkerExhaustedNotAcceptedAttemptsFailTask(t *testing.T) {
	provider := &submissionWorkerProviderStub{results: []struct {
		receipt Receipt
		err     error
	}{{err: fmt.Errorf("rejected: %w", ErrProviderNotAccepted)}}}
	policy := submissionWorkerPolicy()
	policy.Retry.MaxAttempts = 1
	worker, repository, _ := newSubmissionWorkerTest(t, provider, mustTask(validCreateInput()), policy)

	result, err := worker.RunOnce(context.Background(), repository.task.ID)
	if err != nil {
		t.Fatalf("exhausted retry RunOnce() error = %v", err)
	}
	if result.Outcome != SubmissionOutcomeFailed || repository.task.Status != StatusFailed || repository.task.FailureCode != "submission_retry_exhausted" || repository.lease != nil {
		t.Fatalf("exhausted retry did not settle failure: outcome=%s task=%+v lease=%+v", result.Outcome, repository.task, repository.lease)
	}
}

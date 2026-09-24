package generation

import (
	"errors"
	"testing"
	"time"
)

func TestRetryPolicyUsesCappedExponentialDelay(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 4, BaseDelay: 30 * time.Second, MaxDelay: time.Minute}
	for _, test := range []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 30 * time.Second},
		{attempt: 2, want: time.Minute},
		{attempt: 3, want: time.Minute},
	} {
		got, err := policy.DelayFor(test.attempt)
		if err != nil || got != test.want {
			t.Errorf("DelayFor(%d) = %s, %v; want %s", test.attempt, got, err, test.want)
		}
	}
	if _, err := policy.DelayFor(4); !errors.Is(err, ErrGenerationRetryExhausted) {
		t.Fatalf("exhausted policy error = %v", err)
	}
	if _, err := (RetryPolicy{MaxAttempts: 4, BaseDelay: 0, MaxDelay: time.Minute}).DelayFor(1); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("invalid policy error = %v", err)
	}
}

func TestSubmissionRetryRequiresReconciliationAndWaitsForBackoff(t *testing.T) {
	task := mustTask(validCreateInput())
	lease, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.MarkSubmissionUnknown(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.ReconcileSubmissionNotAccepted(generationTestNow.Add(4 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	policy := RetryPolicy{MaxAttempts: 3, BaseDelay: 30 * time.Second, MaxDelay: time.Minute}
	scheduledAt := generationTestNow.Add(5 * time.Minute)
	if err := task.ScheduleSubmissionRetry(scheduledAt, policy); err != nil {
		t.Fatalf("ScheduleSubmissionRetry() error = %v", err)
	}
	if task.NextAttemptAt == nil || !task.NextAttemptAt.Equal(scheduledAt.Add(30*time.Second)) || task.SubmissionState != SubmissionNotStarted {
		t.Fatalf("retry schedule was not persisted: %+v", task)
	}
	if err := task.ReleaseLease(lease, scheduledAt); err != nil {
		t.Fatalf("release scheduled retry lease: %v", err)
	}
	if _, err := task.AcquireLease("worker-b", scheduledAt.Add(29*time.Second), time.Minute); !errors.Is(err, ErrGenerationRetryNotReady) {
		t.Fatalf("early retry lease error = %v", err)
	}
	retryLease, err := task.AcquireLease("worker-b", scheduledAt.Add(30*time.Second), time.Minute)
	if err != nil {
		t.Fatalf("acquire due retry lease: %v", err)
	}
	if _, err := task.BeginSubmission(scheduledAt.Add(31 * time.Second)); err != nil {
		t.Fatalf("begin due retry: %v", err)
	}
	if task.NextAttemptAt != nil || task.SubmissionAttempt != 2 || retryLease.Attempt != 2 {
		t.Fatalf("retry did not clear due window or advance attempt: task=%+v lease=%+v", task, retryLease)
	}
}

func TestSubmissionRetryStopsAtConfiguredAttemptLimit(t *testing.T) {
	task := mustTask(validCreateInput())
	if _, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.MarkSubmissionUnknown(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.ReconcileSubmissionNotAccepted(generationTestNow.Add(4 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	before := task
	if err := task.ScheduleSubmissionRetry(generationTestNow.Add(5*time.Minute), RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: time.Minute}); !errors.Is(err, ErrGenerationRetryExhausted) {
		t.Fatalf("retry exhaustion error = %v", err)
	}
	if task.NextAttemptAt != nil || task.StatusRevision != before.StatusRevision || task.UpdatedAt != before.UpdatedAt {
		t.Fatalf("exhausted retry mutated task: before=%+v after=%+v", before, task)
	}
}

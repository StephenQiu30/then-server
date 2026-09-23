package generation

import (
	"errors"
	"testing"
	"time"
)

func TestLeaseAcquisitionMovesQueuedTaskAndFencesRecovery(t *testing.T) {
	task := mustTask(validCreateInput())
	first, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), 2*time.Minute)
	if err != nil {
		t.Fatalf("AcquireLease() error = %v", err)
	}
	if task.Status != StatusRunning || first.FencingToken != 1 || first.Attempt != 1 || task.LeaseOwner != "worker-a" || task.LeaseUntil == nil {
		t.Fatalf("unexpected first lease: task=%+v lease=%+v", task, first)
	}
	if err := task.ValidateLease(first, generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatalf("ValidateLease() error = %v", err)
	}
	if _, err := task.AcquireLease("worker-b", generationTestNow.Add(2*time.Minute), time.Minute); !errors.Is(err, ErrGenerationLeaseHeld) {
		t.Fatalf("held lease error = %v", err)
	}

	second, err := task.AcquireLease("worker-b", generationTestNow.Add(4*time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("recovery AcquireLease() error = %v", err)
	}
	if second.FencingToken != 2 || second.Attempt != 2 || task.LeaseOwner != "worker-b" {
		t.Fatalf("recovery did not advance fencing: task=%+v lease=%+v", task, second)
	}
	if err := task.ValidateLease(first, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrGenerationLeaseConflict) {
		t.Fatalf("stale lease validation error = %v", err)
	}
	if err := task.ReleaseLease(first, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrGenerationLeaseConflict) {
		t.Fatalf("stale lease release error = %v", err)
	}
	if err := task.ValidateLease(second, generationTestNow.Add(4*time.Minute)); err != nil {
		t.Fatalf("recovery lease validation error = %v", err)
	}
}

func TestLeaseRenewalAndExpiryRejectStaleMutation(t *testing.T) {
	task := mustTask(validCreateInput())
	lease, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := task.RenewLease(lease, generationTestNow.Add(90*time.Second), 2*time.Minute)
	if err != nil {
		t.Fatalf("RenewLease() error = %v", err)
	}
	if !renewed.ExpiresAt.After(lease.ExpiresAt) || renewed.FencingToken != lease.FencingToken || renewed.Attempt != lease.Attempt {
		t.Fatalf("renewal changed fencing identity: old=%+v new=%+v", lease, renewed)
	}
	if _, err := task.RenewLease(renewed, generationTestNow.Add(2*time.Minute), time.Minute); !errors.Is(err, ErrInvalidGenerationLease) {
		t.Fatalf("shortening renewal error = %v", err)
	}
	if err := task.ValidateLease(renewed, renewed.ExpiresAt); !errors.Is(err, ErrGenerationLeaseExpired) {
		t.Fatalf("expired validation error = %v", err)
	}
	if err := task.ReleaseLease(renewed, renewed.ExpiresAt); !errors.Is(err, ErrGenerationLeaseExpired) {
		t.Fatalf("expired release error = %v", err)
	}
	if task.LeaseOwner != "worker-a" || task.LeaseUntil == nil {
		t.Fatal("expired worker mutated the active lease")
	}
}

func TestSettlementCarriesFencingTokenAndClearsLease(t *testing.T) {
	task := mustTask(validCreateInput())
	lease, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	settled, err := PublishOutput(task, nil, asset, generationTestNow.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("PublishOutput() error = %v", err)
	}
	if settled.FencingToken != lease.FencingToken || settled.Task.LeaseOwner != "" || settled.Task.LeaseUntil != nil {
		t.Fatalf("settlement lost fencing or retained lease: %+v", settled)
	}
	if task.LeaseOwner != "worker-a" || task.LeaseUntil == nil {
		t.Fatal("settlement mutated the input lease")
	}
}

func TestTerminalTaskCannotAcquireLease(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.Transition(StatusFailed, "provider_error", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := task.AcquireLease("worker-a", generationTestNow.Add(2*time.Minute), time.Minute); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("terminal lease error = %v", err)
	}
}

func TestLeaseRejectsMalformedPersistedState(t *testing.T) {
	cases := []func(*Task){
		func(task *Task) { task.LeaseOwner = "worker-a" },
		func(task *Task) { task.LeaseUntil = timePtr(generationTestNow.Add(time.Minute)) },
		func(task *Task) { task.LeaseAttempt = -1 },
	}
	for index, mutate := range cases {
		task := mustTask(validCreateInput())
		mutate(&task)
		if _, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), time.Minute); !errors.Is(err, ErrInvalidGenerationLease) {
			t.Errorf("case %d malformed lease error = %v", index, err)
		}
	}
}

package generation

import (
	"context"
	"errors"
	"testing"
	"time"
)

type cleanupWorkerRepositoryStub struct {
	request     CleanupRequest
	targets     []CleanupTarget
	found       bool
	claimTTL    time.Duration
	claimAt     time.Time
	completed   bool
	failureCode string
	retryAt     time.Time
}

func (s *cleanupWorkerRepositoryStub) ClaimNextCleanup(_ context.Context, at time.Time, ttl time.Duration) (CleanupRequest, []CleanupTarget, bool, error) {
	s.claimAt = at
	s.claimTTL = ttl
	return s.request, append([]CleanupTarget(nil), s.targets...), s.found, nil
}

func (s *cleanupWorkerRepositoryStub) CompleteTaskCleanup(_ context.Context, claimed CleanupRequest, _ time.Time) (CleanupRequest, error) {
	if claimed.ID != s.request.ID {
		return CleanupRequest{}, errors.New("wrong cleanup request")
	}
	s.completed = true
	return s.request, nil
}

func (s *cleanupWorkerRepositoryStub) FailTaskCleanup(_ context.Context, claimed CleanupRequest, _ time.Time, code string, retryAt time.Time) (CleanupRequest, error) {
	if claimed.ID != s.request.ID {
		return CleanupRequest{}, errors.New("wrong cleanup request")
	}
	s.failureCode = code
	s.retryAt = retryAt
	return s.request, nil
}

type cleanupWorkerExecutorStub struct {
	objects   []CleanupTarget
	providers []CleanupTarget
	err       error
}

func (s *cleanupWorkerExecutorStub) DeleteObject(_ context.Context, target CleanupTarget) error {
	s.objects = append(s.objects, target)
	return s.err
}

func (s *cleanupWorkerExecutorStub) DeleteProviderTask(_ context.Context, target CleanupTarget) error {
	s.providers = append(s.providers, target)
	return s.err
}

func cleanupWorkerRequest(at time.Time) CleanupRequest {
	return CleanupRequest{ID: "00000000-0000-4000-8000-000000000001", OwnerID: "00000000-0000-4000-8000-000000000002", TaskID: "00000000-0000-4000-8000-000000000003", Scope: CleanupScopeTask, Status: CleanupRunning, AccessRevokedAt: timePtr(at), Attempts: 2, CreatedAt: at, UpdatedAt: at}
}

func TestCleanupWorkerExecutesAllTargetsBeforeCompletion(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	repository := &cleanupWorkerRepositoryStub{request: cleanupWorkerRequest(at), found: true, targets: []CleanupTarget{{Kind: CleanupTargetObject, ID: "asset", ObjectKey: "owners/00000000-0000-4000-8000-000000000002/generation/00000000-0000-4000-8000-000000000003/output.jpg", ObjectVersionID: "version"}, {Kind: CleanupTargetProvider, ID: "provider-task"}}}
	executor := &cleanupWorkerExecutorStub{}
	worker, err := NewCleanupWorker(repository, executor, CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: 10 * time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return at }

	found, err := worker.RunOnce(context.Background())
	if err != nil || !found || !repository.completed {
		t.Fatalf("cleanup worker did not complete request: found=%v completed=%v err=%v", found, repository.completed, err)
	}
	if len(executor.objects) != 1 || len(executor.providers) != 1 || repository.claimTTL != time.Minute || !repository.claimAt.Equal(at) {
		t.Fatalf("cleanup worker did not execute the claimed target set: objects=%+v providers=%+v ttl=%v at=%v", executor.objects, executor.providers, repository.claimTTL, repository.claimAt)
	}
}

func TestCleanupWorkerRecordsBoundedRetryWithoutCompleting(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	repository := &cleanupWorkerRepositoryStub{request: cleanupWorkerRequest(at), found: true, targets: []CleanupTarget{{Kind: CleanupTargetObject, ID: "asset", ObjectKey: "owners/00000000-0000-4000-8000-000000000002/generation/00000000-0000-4000-8000-000000000003/output.jpg", ObjectVersionID: "version"}}}
	executor := &cleanupWorkerExecutorStub{err: &CleanupTargetError{Code: "object_unavailable", Err: errors.New("transport detail")}}
	worker, err := NewCleanupWorker(repository, executor, CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: 10 * time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return at }

	found, err := worker.RunOnce(context.Background())
	if err != nil || !found || repository.completed || repository.failureCode != "object_unavailable" {
		t.Fatalf("cleanup failure was not persisted as retry: found=%v completed=%v code=%q err=%v", found, repository.completed, repository.failureCode, err)
	}
	if !repository.retryAt.Equal(at.Add(20 * time.Second)) {
		t.Fatalf("cleanup retry was not capped exponential: got %v", repository.retryAt)
	}
}

func TestCleanupWorkerNormalizesUnknownTargetErrors(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	repository := &cleanupWorkerRepositoryStub{request: cleanupWorkerRequest(at), found: true, targets: []CleanupTarget{{Kind: CleanupTargetObject, ID: "asset", ObjectKey: "owners/00000000-0000-4000-8000-000000000002/generation/00000000-0000-4000-8000-000000000003/output.jpg", ObjectVersionID: "version"}}}
	executor := &cleanupWorkerExecutorStub{err: errors.New("secret provider response")}
	worker, err := NewCleanupWorker(repository, executor, CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second, MaxDelay: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return at }
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.failureCode != "target_delete_failed" {
		t.Fatalf("unknown target error leaked or used unstable code: %q", repository.failureCode)
	}
}

func TestCleanupWorkerDoesNotCompleteLegacyObjectTargetWithoutKey(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	repository := &cleanupWorkerRepositoryStub{request: cleanupWorkerRequest(at), found: true, targets: []CleanupTarget{{Kind: CleanupTargetObject, ID: "legacy-asset", ObjectVersionID: "version"}}}
	executor := &cleanupWorkerExecutorStub{}
	worker, err := NewCleanupWorker(repository, executor, CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return at }

	found, err := worker.RunOnce(context.Background())
	if err != nil || !found || repository.completed || repository.failureCode != "object_key_unavailable" || len(executor.objects) != 0 {
		t.Fatalf("legacy object target was deleted or completed without a key: found=%v completed=%v failure=%q objects=%+v err=%v", found, repository.completed, repository.failureCode, executor.objects, err)
	}
}

func TestCleanupWorkerRejectsInvalidPolicyAndDependencies(t *testing.T) {
	if _, err := NewCleanupWorker(nil, &cleanupWorkerExecutorStub{}, CleanupRetryPolicy{}); !errors.Is(err, ErrInvalidGenerationCleanupWorker) {
		t.Fatalf("nil repository was accepted: %v", err)
	}
	if _, err := NewCleanupWorker(&cleanupWorkerRepositoryStub{}, &cleanupWorkerExecutorStub{}, CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second}); !errors.Is(err, ErrInvalidGenerationCleanupWorker) {
		t.Fatalf("incomplete retry policy was accepted: %v", err)
	}
}

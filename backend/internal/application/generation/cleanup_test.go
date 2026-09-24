package generation

import (
	"errors"
	"testing"
	"time"
)

func TestRevokeAccessBlocksLateOutputPublication(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RevokeAccess(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if task.AccessRevokedAt == nil || task.StatusRevision != 4 {
		t.Fatalf("access revocation was not persisted in the task state: %+v", task)
	}
	if err := task.RevokeAccess(generationTestNow.Add(4 * time.Minute)); err != nil {
		t.Fatalf("replaying access revocation should be idempotent: %v", err)
	}
	if _, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("revoked task accepted a late output asset: %v", err)
	}
}

func TestCleanupRequestRetainsTargetsAcrossRetry(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.RevokeAccess(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	targets := []CleanupTarget{
		{Kind: CleanupTargetObject, ID: "asset-image-1", ObjectVersionID: "object-version-1"},
		{Kind: CleanupTargetProvider, ID: "provider-task-1"},
	}
	request, err := NewCleanupRequest("cleanup-1", task, CleanupScopeTask, targets, generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := request.Begin(generationTestNow.Add(2 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != len(targets) || claimed[0] != targets[0] || claimed[1] != targets[1] {
		t.Fatalf("cleanup claim lost target snapshot: %+v", claimed)
	}
	retryAt := generationTestNow.Add(5 * time.Minute)
	if err := request.Fail(generationTestNow.Add(3*time.Minute), "object_store_unavailable", retryAt); err != nil {
		t.Fatal(err)
	}
	if _, err := request.Begin(generationTestNow.Add(4 * time.Minute)); !errors.Is(err, ErrGenerationCleanupNotReady) {
		t.Fatalf("cleanup retry ignored next attempt time: %v", err)
	}
	claimed, err = request.Begin(retryAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 2 || claimed[0] != targets[0] || claimed[1] != targets[1] {
		t.Fatalf("cleanup retry changed target snapshot: %+v", claimed)
	}
	if err := request.Complete(generationTestNow.Add(6 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if request.Status != CleanupComplete || request.CompletedAt == nil || request.NextAttemptAt != nil {
		t.Fatalf("cleanup did not complete cleanly: %+v", request)
	}
}

func TestCleanupRequestExhaustsAfterMaximumAttempts(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.RevokeAccess(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	request, err := NewCleanupRequest("cleanup-exhausted", task, CleanupScopeTask, nil, generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	request.Status = CleanupFailed
	request.Attempts = MaxCleanupAttempts
	request.StableError = "target_delete_failed"
	request.NextAttemptAt = timePtr(generationTestNow.Add(2 * time.Minute))
	request.UpdatedAt = generationTestNow.Add(2 * time.Minute)
	if err := request.Validate(); err != nil {
		t.Fatalf("maximum-attempt running request became invalid: %v", err)
	}
	if _, err := request.Begin(generationTestNow.Add(3 * time.Minute)); !errors.Is(err, ErrGenerationCleanupExhausted) {
		t.Fatalf("exhausted cleanup was claimable: %v", err)
	}
	request.Status = CleanupRunning
	request.StableError = ""
	request.NextAttemptAt = nil
	if err := request.Exhaust(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatalf("exhausted cleanup was not recorded: %v", err)
	}
	if request.Status != CleanupFailed || request.StableError != "cleanup_retry_exhausted" || request.NextAttemptAt != nil {
		t.Fatalf("exhausted cleanup retained retry state: %+v", request)
	}
}

func TestCleanupRequestNormalizesFinalTargetFailure(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.RevokeAccess(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	request, err := NewCleanupRequest("cleanup-final-failure", task, CleanupScopeTask, nil, generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	request.Status = CleanupRunning
	request.Attempts = MaxCleanupAttempts
	if err := request.Fail(generationTestNow.Add(2*time.Minute), "target_delete_failed", generationTestNow.Add(3*time.Minute)); err != nil {
		t.Fatalf("final target failure was rejected: %v", err)
	}
	if request.Status != CleanupFailed || request.StableError != "cleanup_retry_exhausted" || request.NextAttemptAt != nil {
		t.Fatalf("final target failure retained retry state: %+v", request)
	}
}

func TestCleanupRequestReopensWhenLateTargetArrives(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.RevokeAccess(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	request, err := NewCleanupRequest("cleanup-late-target", task, CleanupScopeTask, nil, generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := request.Begin(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := request.Complete(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	lateAt := generationTestNow.Add(4 * time.Minute)
	if err := request.AddTarget(CleanupTarget{Kind: CleanupTargetProvider, ID: "provider-late-1"}, lateAt); err != nil {
		t.Fatalf("late cleanup target was rejected: %v", err)
	}
	if request.Status != CleanupPending || request.CompletedAt != nil || request.NextAttemptAt != nil || request.StableError != "" || len(request.Targets) != 1 {
		t.Fatalf("late target did not reopen cleanup: %+v", request)
	}
	if _, err := request.Begin(lateAt); err != nil {
		t.Fatalf("reopened cleanup was not claimable: %v", err)
	}
	if err := request.AddTarget(CleanupTarget{Kind: CleanupTargetProvider, ID: "provider-late-1"}, generationTestNow); err != nil {
		t.Fatalf("replaying late target was not idempotent: %v", err)
	}
	if len(request.Targets) != 1 {
		t.Fatalf("replaying late target duplicated the manifest: %+v", request.Targets)
	}
}

func TestCleanupRequestAllowsLateScopeCreationAfterAccessRevocation(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.RevokeAccess(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	request, err := NewCleanupRequest("cleanup-late-scope", task, CleanupScopeSource, nil, generationTestNow.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("late cleanup scope was rejected: %v", err)
	}
	if request.AccessRevokedAt.After(request.CreatedAt) {
		t.Fatalf("late cleanup scope moved access revocation forward: %+v", request)
	}
}

func TestCleanupRequestKeepsDistinctObjectVersions(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.RevokeAccess(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	request, err := NewCleanupRequest("cleanup-object-versions", task, CleanupScopeTask, []CleanupTarget{
		{Kind: CleanupTargetObject, ID: "asset-image-1", ObjectVersionID: "object-version-1"},
	}, generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := request.AddTarget(CleanupTarget{Kind: CleanupTargetObject, ID: "asset-image-1", ObjectVersionID: "object-version-2"}, generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatalf("distinct object version was rejected: %v", err)
	}
	if len(request.Targets) != 2 {
		t.Fatalf("distinct object version was dropped: %+v", request.Targets)
	}
	if err := request.AddTarget(CleanupTarget{Kind: CleanupTargetObject, ID: "asset-image-1", ObjectVersionID: "object-version-2"}, generationTestNow); err != nil {
		t.Fatalf("same object version replay was not idempotent: %v", err)
	}
	if len(request.Targets) != 2 {
		t.Fatalf("same object version replay duplicated the manifest: %+v", request.Targets)
	}
}

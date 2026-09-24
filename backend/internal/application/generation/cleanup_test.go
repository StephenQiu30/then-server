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

package generation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var generationTestNow = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func validCreateInput() CreateInput {
	return CreateInput{
		ID:             "job-1",
		OwnerID:        "owner-1",
		IdempotencyKey: "request-1",
		LookID:         "look-1",
		LookRevision:   3,
		Purpose:        PurposeImage,
		Provider:       "seedream",
		Model:          "seedream-5",
		Parameters:     []byte(`{"temperature":1,"seed":"fixed"}`),
		Inputs: InputSnapshot{
			LookID:       "look-1",
			LookRevision: 3,
			References: []InputReference{
				{MediaID: "person-1", Role: InputRolePerson, Ordinal: 0, Revision: 2, SHA256: strings.Repeat("a", 64)},
				{MediaID: "garment-1", Role: InputRoleGarment, Ordinal: 1, Revision: 4, SHA256: strings.Repeat("b", 64)},
			},
		},
		Consent: ConsentReceipt{ID: "consent-1", Purpose: PurposeImage, PolicyVersion: "generation-v1", AcceptedAt: generationTestNow.Add(-time.Minute)},
	}
}

func acquireTestLease(t *testing.T, task *Task, at time.Time, ttl time.Duration) Lease {
	t.Helper()
	lease, err := task.AcquireLease("worker-a", at, ttl)
	if err != nil {
		t.Fatalf("AcquireLease() error = %v", err)
	}
	return lease
}

func TestNewTaskCanonicalizesParametersAndCopiesInputs(t *testing.T) {
	input := validCreateInput()
	task, err := NewTask(input, generationTestNow)
	if err != nil {
		t.Fatalf("NewTask() error = %v", err)
	}
	if string(task.Parameters) != `{"seed":"fixed","temperature":1}` {
		t.Fatalf("parameters were not canonicalized: %s", task.Parameters)
	}
	if task.Status != StatusQueued || task.StatusRevision != 1 || task.SubmissionState != SubmissionNotStarted || task.SubmissionAttempt != 0 || task.DedupeKey == "" || task.IdempotencyKeyHash == "" {
		t.Fatalf("unexpected initial task: %+v", task)
	}
	input.Inputs.References[0].MediaID = "mutated"
	if task.Inputs.References[0].MediaID != "person-1" {
		t.Fatal("task retained mutable input slice")
	}
}

func TestSubmissionGuardRequiresExplicitReconciliation(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	_ = acquireTestLease(t, &task, generationTestNow.Add(time.Minute), 10*time.Minute)
	first, err := task.BeginSubmission(generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("BeginSubmission() error = %v", err)
	}
	if first.Attempt != 1 || task.SubmissionState != SubmissionInFlight || task.SubmissionAttempt != 1 {
		t.Fatalf("submission was not durably marked in flight: %+v", task)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(2 * time.Minute)); !errors.Is(err, ErrSubmissionInProgress) {
		t.Fatalf("in-flight task allowed a second submission: %v", err)
	}
	if err := task.MarkSubmissionUnknown(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatalf("MarkSubmissionUnknown() error = %v", err)
	}
	if _, err := task.PrepareSubmission(); !errors.Is(err, ErrGenerationNotSubmittable) {
		t.Fatalf("unknown submission produced a retry payload: %v", err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(3 * time.Minute)); !errors.Is(err, ErrSubmissionOutcomeUnknown) {
		t.Fatalf("unknown submission was retried blindly: %v", err)
	}
	if err := task.ReconcileSubmissionNotAccepted(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatalf("ReconcileSubmissionNotAccepted() error = %v", err)
	}
	second, err := task.BeginSubmission(generationTestNow.Add(4 * time.Minute))
	if err != nil {
		t.Fatalf("reconciled submission did not allow an explicit retry: %v", err)
	}
	if second.Attempt != 2 || task.SubmissionAttempt != 2 || task.SubmissionState != SubmissionInFlight {
		t.Fatalf("retry attempt was not recorded: %+v", task)
	}
}

func TestWorkerStateWritesRequireAnActiveLease(t *testing.T) {
	task := mustTask(validCreateInput())
	if _, err := task.BeginSubmission(generationTestNow.Add(time.Minute)); !errors.Is(err, ErrGenerationLeaseConflict) {
		t.Fatalf("submission without lease error = %v", err)
	}
	_ = acquireTestLease(t, &task, generationTestNow.Add(time.Minute), time.Minute)
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrGenerationLeaseExpired) {
		t.Fatalf("expired provider acceptance error = %v", err)
	}

	// A fresh task proves the provider callback path independently of the
	// acceptance guard above.
	task = mustTask(validCreateInput())
	_ = acquireTestLease(t, &task, generationTestNow.Add(time.Minute), time.Minute)
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(90*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := task.ApplyProviderState("provider-job-1", StatusValidating, "", generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrGenerationLeaseExpired) {
		t.Fatalf("expired provider callback error = %v", err)
	}
	if task.Status != StatusRunning {
		t.Fatalf("expired provider callback changed task: %+v", task)
	}
}

func TestLateAcceptanceResolvesUnknownSubmissionWithoutResubmission(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	_ = acquireTestLease(t, &task, generationTestNow.Add(30*time.Second), 10*time.Minute)
	if _, err := task.BeginSubmission(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.MarkSubmissionUnknown(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RecordExternalTaskID("provider-after-timeout", generationTestNow.Add(3*time.Minute)); err != nil {
		t.Fatalf("late acceptance was not recorded: %v", err)
	}
	if task.SubmissionState != SubmissionAccepted || task.SubmissionUnknownAt != nil || task.ExternalTaskID == "" {
		t.Fatalf("late acceptance did not resolve uncertainty: %+v", task)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(4 * time.Minute)); !errors.Is(err, ErrGenerationNotSubmittable) {
		t.Fatalf("accepted task allowed a duplicate submission: %v", err)
	}
}

func TestIdentityIsStableAcrossParameterObjectOrder(t *testing.T) {
	first := validCreateInput()
	second := validCreateInput()
	second.Parameters = []byte(`{"seed":"fixed","temperature":1}`)
	firstID, firstDedupe, err := Identity(first)
	if err != nil {
		t.Fatalf("Identity(first) error = %v", err)
	}
	secondID, secondDedupe, err := Identity(second)
	if err != nil {
		t.Fatalf("Identity(second) error = %v", err)
	}
	if firstID != secondID || firstDedupe != secondDedupe {
		t.Fatal("equivalent requests did not produce stable identities")
	}
	second.IdempotencyKey = "request-2"
	thirdID, thirdDedupe, err := Identity(second)
	if err != nil {
		t.Fatalf("Identity(third) error = %v", err)
	}
	if firstID == thirdID || secondDedupe != thirdDedupe {
		t.Fatal("idempotency and content identities are not independent")
	}
	otherOwner := second
	otherOwner.OwnerID = "owner-2"
	otherID, otherDedupe, err := Identity(otherOwner)
	if err != nil {
		t.Fatalf("Identity(other owner) error = %v", err)
	}
	if otherID == secondID || otherDedupe == secondDedupe {
		t.Fatal("owner scope was omitted from generation identity")
	}
}

func TestModelRequiresConfirmedImageSnapshot(t *testing.T) {
	input := validCreateInput()
	input.Purpose = PurposeModel
	input.Consent.Purpose = PurposeModel
	if _, err := NewTask(input, generationTestNow); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("model without source image error = %v", err)
	}
	input.Inputs.ImageAssetID = "image-1"
	input.Inputs.ImageSHA256 = strings.Repeat("c", 64)
	input.Inputs.References = []InputReference{{MediaID: "image-1", Role: InputRoleLookImage, Ordinal: 0, Revision: 5, SHA256: strings.Repeat("c", 64)}}
	task, err := NewTask(input, generationTestNow)
	if err != nil {
		t.Fatalf("valid model NewTask() error = %v", err)
	}
	if task.Purpose != PurposeModel || task.Inputs.ImageAssetID != "image-1" {
		t.Fatalf("model source was not retained: %+v", task)
	}
}

func TestImageSnapshotRequiresOnePersonAndNoLookImage(t *testing.T) {
	input := validCreateInput()
	input.Inputs.References = []InputReference{{MediaID: "garment-1", Role: InputRoleGarment, Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64)}}
	if _, err := NewTask(input, generationTestNow); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("image without person was accepted: %v", err)
	}
	input = validCreateInput()
	input.Inputs.References = append(input.Inputs.References, InputReference{MediaID: "person-2", Role: InputRolePerson, Ordinal: 2, Revision: 1, SHA256: strings.Repeat("c", 64)})
	if _, err := NewTask(input, generationTestNow); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("image with two people was accepted: %v", err)
	}
	input = validCreateInput()
	input.Inputs.References[0].Role = InputRoleLookImage
	if _, err := NewTask(input, generationTestNow); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("image with a look image reference was accepted: %v", err)
	}
}

func TestTaskStateTransitionsAndCancellationAreMonotonic(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("queued -> validating error = %v", err)
	}
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(-time.Second)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("transition moved task time backwards: %v", err)
	}
	if err := task.RequestCancel(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatalf("RequestCancel() error = %v", err)
	}
	if task.CancelRequestedAt == nil || task.Status != StatusQueued {
		t.Fatalf("cancel request changed wrong facts: %+v", task)
	}
	if err := task.Transition(StatusCanceled, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatalf("queued -> canceled error = %v", err)
	}
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("terminal task accepted transition: %v", err)
	}
	if err := task.RequestCancel(generationTestNow.Add(4 * time.Minute)); !errors.Is(err, ErrGenerationNotCancellable) {
		t.Fatalf("terminal task accepted cancellation: %v", err)
	}
}

func TestTaskCannotSucceedWithoutValidatedOutputReference(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusSucceeded, "", generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrGenerationOutputRequired) {
		t.Fatalf("direct success transition error = %v", err)
	}
	if task.Status != StatusValidating || task.ResultAssetID != "" {
		t.Fatalf("direct success transition changed task: %+v", task)
	}
}

func TestTaskCannotSucceedAfterCancellationRequest(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RequestCancel(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	task.ResultAssetID = "asset-image-1"
	if err := task.Transition(StatusSucceeded, "", generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("canceled task accepted direct success: %v", err)
	}
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(5*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("canceled task accepted output settlement: %v", err)
	}
}

func TestValidatingTaskCanBeCanceled(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RequestCancel(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusCanceled, "", generationTestNow.Add(4*time.Minute)); err != nil {
		t.Fatalf("validating -> canceled error = %v", err)
	}
	if task.Status != StatusCanceled || task.CancelRequestedAt == nil {
		t.Fatalf("validation cancellation was not retained: %+v", task)
	}
}

func TestProviderAcceptanceAndCallbacksAreMonotonic(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	acquireTestLease(t, &task, generationTestNow.Add(time.Minute), 10*time.Minute)
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatalf("RecordExternalTaskID() error = %v", err)
	}
	revision := task.StatusRevision
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatalf("repeated RecordExternalTaskID() error = %v", err)
	}
	if task.StatusRevision != revision {
		t.Fatal("repeated provider acceptance changed the task")
	}
	if err := task.ApplyProviderState("provider-job-1", StatusRunning, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatalf("running callback error = %v", err)
	}
	if err := task.ApplyProviderState("provider-job-1", StatusValidating, "", generationTestNow.Add(3*time.Minute)); err != nil {
		t.Fatalf("validating callback error = %v", err)
	}
	if err := task.ApplyProviderState("provider-job-1", StatusSucceeded, "", generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrGenerationOutputRequired) {
		t.Fatalf("success callback without output error = %v", err)
	}
	if task.Status != StatusValidating {
		t.Fatalf("success callback changed task before output validation: %+v", task)
	}
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("NewOutputAsset() error = %v", err)
	}
	settled, err := PublishOutput(task, nil, asset, generationTestNow.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("PublishOutput() error = %v", err)
	}
	task = settled.Task
	if err := task.ApplyProviderState("provider-job-1", StatusSucceeded, "", generationTestNow.Add(6*time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("repeated success callback error = %v", err)
	}
	if task.Status != StatusSucceeded || task.ExternalTaskID != "provider-job-1" {
		t.Fatalf("unexpected reconciled task: %+v", task)
	}
	if err := task.ApplyProviderState("provider-job-2", StatusFailed, "provider_error", generationTestNow.Add(6*time.Minute)); !errors.Is(err, ErrExternalTaskConflict) {
		t.Fatalf("cross-task callback error = %v", err)
	}
	if task.Status != StatusSucceeded {
		t.Fatal("cross-task callback changed terminal task")
	}
	if err := task.ApplyProviderState("provider-job-1", StatusRunning, "", generationTestNow.Add(7*time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("late stale callback error = %v", err)
	}
}

func TestLateProviderAcceptanceAfterCancellationIsRetainedForCleanup(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.RequestCancel(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusCanceled, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RecordExternalTaskID("provider-late-1", generationTestNow.Add(90*time.Second)); err != nil {
		t.Fatalf("late acceptance was not retained: %v", err)
	}
	if task.ExternalTaskID != "provider-late-1" || task.Status != StatusCanceled {
		t.Fatalf("late acceptance changed cancellation state: %+v", task)
	}
	if task.UpdatedAt != generationTestNow.Add(2*time.Minute) {
		t.Fatalf("late acceptance moved local time backwards: %v", task.UpdatedAt)
	}
	if err := task.ApplyProviderState("provider-late-1", StatusSucceeded, "", generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("late success resurrected canceled task: %v", err)
	}
}

func TestProviderCallbacksRequireTheRecordedExternalTask(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.ApplyProviderState("unrecorded", StatusRunning, "", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrExternalTaskConflict) {
		t.Fatalf("unrecorded callback error = %v", err)
	}
	acquireTestLease(t, &task, generationTestNow.Add(time.Minute), 10*time.Minute)
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.RecordExternalTaskID("provider-job-2", generationTestNow.Add(2*time.Minute)); !errors.Is(err, ErrExternalTaskConflict) {
		t.Fatalf("replacement external ID error = %v", err)
	}
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(-time.Second)); err != nil {
		// Replaying the same acceptance is idempotent even if the transport
		// reports an older observation timestamp.
		t.Fatalf("same external ID replay error = %v", err)
	}
}

func TestClassifyRequestSeparatesReplayConflictAndContentDedupe(t *testing.T) {
	existing, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	match, err := ClassifyRequest(existing, validCreateInput())
	if err != nil || match != RequestMatchIdempotentReplay {
		t.Fatalf("same request classified as %q, error=%v", match, err)
	}
	conflicting := validCreateInput()
	conflicting.Parameters = []byte(`{"seed":"different"}`)
	match, err = ClassifyRequest(existing, conflicting)
	if err != nil || match != RequestMatchIdempotencyConflict {
		t.Fatalf("same key with changed request classified as %q, error=%v", match, err)
	}
	deduped := validCreateInput()
	deduped.IdempotencyKey = "request-2"
	match, err = ClassifyRequest(existing, deduped)
	if err != nil || match != RequestMatchContentDedupe {
		t.Fatalf("new key with same content classified as %q, error=%v", match, err)
	}
	otherOwner := validCreateInput()
	otherOwner.OwnerID = "owner-2"
	match, err = ClassifyRequest(existing, otherOwner)
	if err != nil || match != RequestMatchNone {
		t.Fatalf("cross-owner request classified as %q, error=%v", match, err)
	}
	failed := existing
	if err := failed.Transition(StatusRunning, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := failed.Transition(StatusFailed, "provider_error", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	match, err = ClassifyRequest(failed, deduped)
	if err != nil || match != RequestMatchNone {
		t.Fatalf("failed task blocked a new attempt as %q, error=%v", match, err)
	}
}

func TestTaskRejectsInvalidSnapshotsAndConsent(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CreateInput)
	}{
		{name: "duplicate media", mutate: func(input *CreateInput) { input.Inputs.References[1].MediaID = input.Inputs.References[0].MediaID }},
		{name: "duplicate ordinal", mutate: func(input *CreateInput) { input.Inputs.References[1].Ordinal = input.Inputs.References[0].Ordinal }},
		{name: "invalid hash", mutate: func(input *CreateInput) { input.Inputs.References[0].SHA256 = "not-a-hash" }},
		{name: "mismatched look revision", mutate: func(input *CreateInput) { input.Inputs.LookRevision++ }},
		{name: "wrong consent purpose", mutate: func(input *CreateInput) { input.Consent.Purpose = PurposeModel }},
		{name: "future consent", mutate: func(input *CreateInput) { input.Consent.AcceptedAt = generationTestNow.Add(time.Second) }},
		{name: "trailing JSON", mutate: func(input *CreateInput) { input.Parameters = []byte(`{"seed":"fixed"}{"extra":true}`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := validCreateInput()
			tc.mutate(&input)
			if _, err := NewTask(input, generationTestNow); !errors.Is(err, ErrInvalidGenerationInput) {
				t.Fatalf("NewTask() error = %v", err)
			}
		})
	}
}

func TestTransitionRequiresFailureCodeOnlyForFailedState(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.Transition(StatusRunning, "provider_error", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("non-failed transition accepted a failure code: %v", err)
	}
	if task.Status != StatusQueued || task.StatusRevision != 1 {
		t.Fatalf("invalid transition mutated task: %+v", task)
	}
	if err := task.Transition(StatusFailed, "", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("failed transition accepted an empty failure code: %v", err)
	}
	if task.Status != StatusQueued || task.StatusRevision != 1 {
		t.Fatalf("invalid failed transition mutated task: %+v", task)
	}

	acquireTestLease(t, &task, generationTestNow.Add(time.Minute), 10*time.Minute)
	if err := task.RecordExternalTaskID("provider-job-1", generationTestNow.Add(90*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := task.ApplyProviderState("provider-job-1", StatusFailed, "", generationTestNow.Add(2*time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("provider failure without a code was accepted: %v", err)
	}
	if task.Status != StatusRunning || task.FailureCode != "" {
		t.Fatalf("invalid provider failure mutated task: %+v", task)
	}
}

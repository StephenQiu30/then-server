package generation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPublishOutputSettlesTaskAndReservationWithoutMutatingInputs(t *testing.T) {
	input := validCreateInput()
	input.Cost = CostEstimate{Currency: "USD", EstimatedMinorUnits: 25, ReservedQuotaUnits: 1}
	task := validatingTask(t, input)
	reservation, err := NewQuotaReservation("reservation-1", task, generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	settled, err := PublishOutput(task, &reservation, asset, generationTestNow.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("PublishOutput() error = %v", err)
	}
	if settled.Task.Status != StatusSucceeded || settled.Task.ResultAssetID != asset.ID || settled.Task.StatusRevision != task.StatusRevision+1 || settled.Reservation == nil || settled.Reservation.State != ReservationConsumed || settled.Asset == nil || settled.Asset.ID != asset.ID || settled.Reused {
		t.Fatalf("unexpected success settlement: %+v", settled)
	}
	if task.Status != StatusValidating || task.ResultAssetID != "" || reservation.State != ReservationReserved {
		t.Fatalf("settlement mutated its inputs: task=%+v reservation=%+v", task, reservation)
	}
}

func TestPublishOutputIsIdempotentForSameSettledAsset(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	task.ResultAssetID = asset.ID
	task.LeaseOwner = ""
	task.LeaseUntil = nil
	if err := task.Transition(StatusSucceeded, "", generationTestNow.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	settled, err := PublishOutput(task, nil, asset, generationTestNow.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("replayed PublishOutput() error = %v", err)
	}
	if !settled.Reused || settled.Task.Status != StatusSucceeded || settled.Asset == nil || settled.Asset.ID != asset.ID {
		t.Fatalf("success replay was not idempotent: %+v", settled)
	}
	asset.ID = "asset-image-2"
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(5*time.Minute)); !errors.Is(err, ErrGenerationSettlementConflict) {
		t.Fatalf("different success asset error = %v", err)
	}
}

func TestPublishOutputRequiresMatchingReservationAndLineage(t *testing.T) {
	input := validCreateInput()
	input.Cost = CostEstimate{Currency: "USD", EstimatedMinorUnits: 25, ReservedQuotaUnits: 1}
	task := validatingTask(t, input)
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("missing reservation error = %v", err)
	}
	reservation, err := NewQuotaReservation("reservation-1", task, generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	asset.Lineage.LookRevision++
	if _, err := PublishOutput(task, &reservation, asset, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("lineage mismatch error = %v", err)
	}
}

func TestFinalizeWithoutOutputReleasesReservationAndReplays(t *testing.T) {
	input := validCreateInput()
	input.Cost = CostEstimate{Currency: "USD", EstimatedMinorUnits: 25, ReservedQuotaUnits: 1}
	task := mustTask(input)
	reservation, err := NewQuotaReservation("reservation-1", task, generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	settled, err := FinalizeWithoutOutput(task, &reservation, StatusFailed, "provider_error", generationTestNow.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("FinalizeWithoutOutput() error = %v", err)
	}
	if settled.Task.Status != StatusFailed || settled.Task.FailureCode != "provider_error" || settled.Reservation == nil || settled.Reservation.State != ReservationReleased || settled.Reused {
		t.Fatalf("unexpected failure settlement: %+v", settled)
	}
	replayed, err := FinalizeWithoutOutput(settled.Task, settled.Reservation, StatusFailed, "provider_error", generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("replayed FinalizeWithoutOutput() error = %v", err)
	}
	if !replayed.Reused || replayed.Task.Status != StatusFailed || replayed.Reservation.State != ReservationReleased {
		t.Fatalf("failure replay was not idempotent: %+v", replayed)
	}
	if _, err := FinalizeWithoutOutput(settled.Task, settled.Reservation, StatusExpired, "", generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("different terminal status error = %v", err)
	}
}

func TestFinalizeWithoutOutputSupportsZeroCostCancelAndRejectsInvalidFailure(t *testing.T) {
	task := mustTask(validCreateInput())
	settled, err := FinalizeWithoutOutput(task, nil, StatusCanceled, "", generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("zero-cost cancellation error = %v", err)
	}
	if settled.Task.Status != StatusCanceled || settled.Reservation != nil {
		t.Fatalf("unexpected zero-cost cancellation: %+v", settled)
	}
	if _, err := FinalizeWithoutOutput(task, nil, StatusFailed, "", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("empty failure code error = %v", err)
	}
	if _, err := FinalizeWithoutOutput(task, nil, StatusCanceled, "unexpected", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("cancellation failure code error = %v", err)
	}
	if _, err := FinalizeWithoutOutput(task, nil, StatusSucceeded, "", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("success without output error = %v", err)
	}
}

func TestPublishOutputRejectsMalformedAssetAndStaleSettlement(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	asset.SHA256 = strings.Repeat("x", 64)
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("malformed asset error = %v", err)
	}
	asset, err = NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	asset.PublishedAt = generationTestNow.Add(5 * time.Minute)
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("future asset publication error = %v", err)
	}
	asset.PublishedAt = generationTestNow.Add(3 * time.Minute)
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(90*time.Second)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("stale settlement error = %v", err)
	}
}

func TestSettlementRejectsExpiredActiveLease(t *testing.T) {
	task := mustTask(validCreateInput())
	if _, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(90*time.Second)); err != nil {
		t.Fatal(err)
	}
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(90*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("expired output settlement error = %v", err)
	}

	task = mustTask(validCreateInput())
	if _, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeWithoutOutput(task, nil, StatusFailed, "provider_error", generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("expired failure settlement error = %v", err)
	}
}

func TestPublishOutputRequiresAnActiveLease(t *testing.T) {
	task := mustTask(validCreateInput())
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("released validating task accepted output settlement: %v", err)
	}

	task = mustTask(validCreateInput())
	if _, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	asset, err = NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(4*time.Minute)); err != nil {
		t.Fatalf("active lease rejected output settlement: %v", err)
	}
}

func TestSettlementRejectsMalformedPersistedLease(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	task.LeaseOwner = " "
	task.LeaseAttempt = 1
	task.LeaseUntil = timePtr(generationTestNow.Add(10 * time.Minute))
	task.FencingToken = 1
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(4*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("malformed active lease error = %v", err)
	}

	task = validatingTask(t, validCreateInput())
	task.LeaseAttempt = -1
	if _, err := FinalizeWithoutOutput(task, nil, StatusCanceled, "", generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("malformed released lease error = %v", err)
	}
}

func TestSettlementRejectsActiveLeaseOnTerminalTask(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	task.ResultAssetID = asset.ID
	task.LeaseOwner = ""
	task.LeaseUntil = nil
	if err := task.Transition(StatusSucceeded, "", generationTestNow.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	task.LeaseOwner = "worker-a"
	task.LeaseAttempt = 1
	task.FencingToken = 1
	task.LeaseUntil = timePtr(generationTestNow.Add(10 * time.Minute))
	if _, err := PublishOutput(task, nil, asset, generationTestNow.Add(5*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("terminal active lease success replay error = %v", err)
	}

	task = mustTask(validCreateInput())
	if err := task.Transition(StatusCanceled, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	task.LeaseOwner = "worker-a"
	task.LeaseAttempt = 1
	task.FencingToken = 1
	task.LeaseUntil = timePtr(generationTestNow.Add(10 * time.Minute))
	if _, err := FinalizeWithoutOutput(task, nil, StatusCanceled, "", generationTestNow.Add(2*time.Minute)); !errors.Is(err, ErrInvalidGenerationSettlement) {
		t.Fatalf("terminal active lease cancellation replay error = %v", err)
	}
}

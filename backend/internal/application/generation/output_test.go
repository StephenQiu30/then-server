package generation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func imageOutputFact() OutputFact {
	return OutputFact{
		ObjectKey:       "owners/owner-1/generation/job-1/output.jpg",
		ContentType:     OutputContentTypeJPEG,
		ByteSize:        1024,
		SHA256:          strings.Repeat("d", 64),
		ObjectVersionID: "version-image-1",
	}
}

func modelOutputFact() OutputFact {
	fact := imageOutputFact()
	fact.ObjectKey = "owners/owner-1/generation/job-1/output.glb"
	fact.ContentType = OutputContentTypeGLB
	fact.SHA256 = strings.Repeat("e", 64)
	fact.ObjectVersionID = "version-model-1"
	return fact
}

func validatingTask(t *testing.T, input CreateInput) Task {
	t.Helper()
	task, err := NewTask(input, generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestNewOutputAssetPreservesImageLineage(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("NewOutputAsset() error = %v", err)
	}
	if asset.Lineage.TaskID != task.ID || asset.Lineage.OwnerID != task.OwnerID || asset.Lineage.LookID != task.LookID || asset.Lineage.LookRevision != task.LookRevision || asset.Lineage.Purpose != PurposeImage {
		t.Fatalf("image lineage was not preserved: %+v", asset.Lineage)
	}
	if asset.Lineage.SourceImageAssetID != "" || asset.ContentType != OutputContentTypeJPEG || asset.PublishedAt.Location() != time.UTC {
		t.Fatalf("unexpected image output: %+v", asset)
	}
	if asset.ObjectKey != "owners/owner-1/generation/job-1/output.jpg" {
		t.Fatalf("image output object key was not bound to its task: %q", asset.ObjectKey)
	}
}

func TestNewOutputAssetBindsModelToConfirmedImage(t *testing.T) {
	input := validCreateInput()
	input.Purpose = PurposeModel
	input.Consent.Purpose = PurposeModel
	input.Inputs.ImageAssetID = "image-1"
	input.Inputs.ImageSHA256 = strings.Repeat("c", 64)
	input.Inputs.References = []InputReference{{MediaID: "image-1", Role: InputRoleLookImage, Ordinal: 0, Revision: 5, SHA256: strings.Repeat("c", 64)}}
	task := validatingTask(t, input)
	asset, err := NewOutputAsset("asset-model-1", task, modelOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("NewOutputAsset() error = %v", err)
	}
	if asset.Lineage.SourceImageAssetID != "image-1" || asset.Lineage.SourceImageSHA256 != strings.Repeat("c", 64) || asset.Lineage.LookRevision != input.LookRevision {
		t.Fatalf("model source lineage was not frozen: %+v", asset.Lineage)
	}
	if asset.ContentType != OutputContentTypeGLB || asset.ObjectVersionID != "version-model-1" {
		t.Fatalf("unexpected model output: %+v", asset)
	}
}

func TestNewOutputAssetRejectsWrongPurposeFactAndTaskState(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	wrong := modelOutputFact()
	if _, err := NewOutputAsset("asset-image-1", task, wrong, generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("image accepted GLB output: %v", err)
	}
	if _, err := NewOutputAsset("asset-image-1", task, OutputFact{ContentType: OutputContentTypeJPEG, ByteSize: 0, SHA256: strings.Repeat("d", 64), ObjectVersionID: "version-image-1"}, generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("zero-size output error = %v", err)
	}

	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("queued task produced an output: %v", err)
	}
	if err := task.Transition(StatusRunning, "", generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := task.Transition(StatusValidating, "", generationTestNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(30*time.Second)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("stale output publication was accepted: %v", err)
	}
}

func TestNewOutputAssetRejectsUntrustedObjectFact(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	fact := imageOutputFact()
	fact.ContentType = "image/png"
	if _, err := NewOutputAsset("asset-image-1", task, fact, generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("non-normalized image output was accepted: %v", err)
	}
	fact = imageOutputFact()
	fact.SHA256 = strings.Repeat("x", 64)
	if _, err := NewOutputAsset("asset-image-1", task, fact, generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("invalid digest output was accepted: %v", err)
	}
	fact = imageOutputFact()
	fact.ObjectKey = "owners/another-owner/generation/job-1/output.jpg"
	if _, err := NewOutputAsset("asset-image-1", task, fact, generationTestNow.Add(3*time.Minute)); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("output stored outside its task-bound key was accepted: %v", err)
	}
}

func TestOutputObjectKeyBindsPurposeAndRejectsUnsafeIDs(t *testing.T) {
	task := mustTask(validCreateInput())
	imageKey, err := OutputObjectKey(task)
	if err != nil || imageKey != "owners/owner-1/generation/job-1/output.jpg" {
		t.Fatalf("image OutputObjectKey() = %q, %v", imageKey, err)
	}

	input := validCreateInput()
	input.Purpose = PurposeModel
	input.Consent.Purpose = PurposeModel
	input.Inputs.ImageAssetID = "image-1"
	input.Inputs.ImageSHA256 = strings.Repeat("c", 64)
	input.Inputs.References = []InputReference{{MediaID: "image-1", Role: InputRoleLookImage, Ordinal: 0, Revision: 5, SHA256: strings.Repeat("c", 64)}}
	modelTask := mustTask(input)
	modelKey, err := OutputObjectKey(modelTask)
	if err != nil || modelKey != "owners/owner-1/generation/job-1/output.glb" {
		t.Fatalf("model OutputObjectKey() = %q, %v", modelKey, err)
	}

	task.ID = "../outside"
	if _, err := OutputObjectKey(task); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("unsafe task id produced an object key: %v", err)
	}
}

func TestPersistedLegacyOutputWithoutObjectKeyIsReadableButNotPublishable(t *testing.T) {
	task := validatingTask(t, validCreateInput())
	asset, err := NewOutputAsset("asset-image-1", task, imageOutputFact(), generationTestNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	asset.ObjectKey = ""
	if err := asset.ValidateFor(task); !errors.Is(err, ErrInvalidGenerationOutput) {
		t.Fatalf("legacy output passed the new publication validation: %v", err)
	}
	if err := asset.ValidatePersistedFor(task); err != nil {
		t.Fatalf("legacy persisted output became unreadable: %v", err)
	}
}

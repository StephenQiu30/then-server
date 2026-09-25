package eventworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"
	"time"

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
)

type workerObjectStoreStub struct{ data []byte }

type shutdownRepository struct {
	Repository
	started chan struct{}
	release chan struct{}
}

func (r shutdownRepository) PendingOutbox(ctx context.Context, _ int) ([]mediaapp.OutboxEvent, error) {
	r.started <- struct{}{}
	<-ctx.Done()
	<-r.release
	return nil, ctx.Err()
}

func (r shutdownRepository) SourceCleanupCandidates(ctx context.Context, _ time.Time, _ int) ([]mediaapp.MediaAsset, error) {
	r.started <- struct{}{}
	<-ctx.Done()
	<-r.release
	return nil, ctx.Err()
}

type shutdownBroker struct {
	started chan struct{}
	release chan struct{}
}

type generationWakeRecorder struct{ taskID string }

func (r *generationWakeRecorder) WakeGeneration(_ context.Context, taskID string) error {
	r.taskID = taskID
	return nil
}

func (shutdownBroker) Publish(context.Context, mediaapp.OutboxEvent) error { return nil }
func (b shutdownBroker) Consume(ctx context.Context, _ string, _ func(context.Context, mediaapp.OutboxEvent) error) error {
	b.started <- struct{}{}
	<-ctx.Done()
	<-b.release
	return nil
}

func TestRunnerWaitsForAllWorkersBeforeReturning(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started, release := make(chan struct{}, 6), make(chan struct{})
	runner, err := New(shutdownRepository{started: started, release: release}, shutdownBroker{started: started, release: release}, &workerObjectStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	for range 6 {
		select {
		case <-started:
		case <-ctx.Done():
			close(release)
			t.Fatal("worker did not start")
		}
	}
	cancel()
	select {
	case <-done:
		close(release)
		t.Fatal("runner returned before workers released dependencies")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("runner shutdown blocked")
	}
}

func TestGenerationWakeValidatesAndSignalsTask(t *testing.T) {
	recorder := &generationWakeRecorder{}
	runner := &Runner{generation: recorder}
	event := mediaapp.OutboxEvent{ID: "event", EventType: "generation.task_requested", AggregateID: "task"}
	if err := runner.wakeGeneration(context.Background(), event); err != nil || recorder.taskID != event.AggregateID {
		t.Fatalf("generation wake was not delivered: task=%q err=%v", recorder.taskID, err)
	}
	event.EventType = "media.uploaded"
	if err := runner.wakeGeneration(context.Background(), event); err == nil {
		t.Fatal("wrong event type was accepted by generation wake handler")
	}
}

func (s *workerObjectStoreStub) OpenVersion(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
func (*workerObjectStoreStub) PutDerived(context.Context, string, io.Reader, int64) (string, error) {
	return "derived-version", nil
}
func (*workerObjectStoreStub) DeleteAllVersions(context.Context, string, string) error { return nil }

func TestNormalizedJPEGChecksDigestAndPixelBudget(t *testing.T) {
	imageData := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := range 6 {
		for x := range 8 {
			imageData.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 25), B: 100, A: 255})
		}
	}
	var source bytes.Buffer
	if err := jpeg.Encode(&source, imageData, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	data := source.Bytes()
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	runner := &Runner{objects: &workerObjectStoreStub{data: data}}
	media := mediaapp.MediaAsset{RawObjectKey: "source.jpg", ObjectVersionID: "fixed-version", ByteSize: int64(len(data)), SHA256: digest}
	normalized, width, height, reason, err := runner.normalizedJPEG(context.Background(), media)
	if err != nil || width != 8 || height != 6 || reason != "" || len(normalized) == 0 {
		t.Fatalf("valid synthetic JPEG rejected: size=%dx%d reason=%s err=%v", width, height, reason, err)
	}
	media.SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, _, _, reason, err = runner.normalizedJPEG(context.Background(), media); err == nil || reason != "digest_mismatch" {
		t.Fatal("digest mismatch was not rejected with a stable reason")
	}
}

func TestNormalizedJPEGRejectsAppendedSecondFrame(t *testing.T) {
	var source bytes.Buffer
	if err := jpeg.Encode(&source, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	data := append(bytes.Clone(source.Bytes()), source.Bytes()...)
	runner := &Runner{objects: &workerObjectStoreStub{data: data}}
	media := mediaapp.MediaAsset{RawObjectKey: "source.jpg", ObjectVersionID: "fixed-version", ByteSize: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	if _, _, _, reason, err := runner.normalizedJPEG(context.Background(), media); err == nil || reason != "invalid_jpeg" {
		t.Fatal("multi-frame JPEG input was accepted")
	}
}

func TestProfileAvatarNormalizationDropsSourceMetadata(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	metadata := []byte("Exif\x00\x00GPS test location")
	segment := append([]byte{0xff, 0xe1, 0, byte(len(metadata) + 2)}, metadata...)
	source := append(append(bytes.Clone(encoded.Bytes()[:2]), segment...), encoded.Bytes()[2:]...)
	runner := &Runner{objects: &workerObjectStoreStub{data: source}}
	media := mediaapp.MediaAsset{Purpose: mediaapp.MediaPurposeProfileAvatar, RawObjectKey: "source.jpg", ObjectVersionID: "fixed-version", ByteSize: int64(len(source)), SHA256: fmt.Sprintf("%x", sha256.Sum256(source))}
	normalized, _, _, reason, err := runner.normalizedJPEG(context.Background(), media)
	if err != nil || reason != "" || bytes.Contains(normalized, []byte("GPS test location")) || bytes.Contains(normalized, []byte("Exif")) {
		t.Fatalf("profile avatar normalization retained metadata: reason=%s err=%v", reason, err)
	}
}

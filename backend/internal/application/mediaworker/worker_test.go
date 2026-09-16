package mediaworker

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

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
)

type workerObjectStoreStub struct{ data []byte }

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

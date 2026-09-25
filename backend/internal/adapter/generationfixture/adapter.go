// Package generationfixture provides the zero-cost local image task adapter.
package generationfixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"strings"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/google/uuid"
)

const (
	ProviderName = "fixture"
	ImageModel   = "fixture-image-v1"
)

// PrivateOutputWriter writes one immutable result version into private storage.
type PrivateOutputWriter interface {
	PutDerived(context.Context, string, io.Reader, int64) (string, error)
}

// Adapter runs image tasks entirely in process and stores a deterministic
// placeholder in private object storage. It is for local workflow validation,
// not a generated Look or a production image provider.
type Adapter struct {
	objects PrivateOutputWriter
	image   []byte
}

var _ generationapp.Provider = (*Adapter)(nil)
var _ generationapp.ResultFetcher = (*Adapter)(nil)

func New(objects PrivateOutputWriter) (*Adapter, error) {
	if objects == nil {
		return nil, errors.New("fixture generation object store unavailable")
	}
	data, err := placeholderJPEG()
	if err != nil {
		return nil, errors.New("fixture image preparation failed")
	}
	return &Adapter{objects: objects, image: data}, nil
}

func (a *Adapter) Submit(ctx context.Context, submission generationapp.Submission) (generationapp.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return generationapp.Receipt{}, err
	}
	if a == nil || submission.Provider != ProviderName || submission.Model != ImageModel || submission.Purpose != generationapp.PurposeImage {
		return generationapp.Receipt{}, generationapp.ErrProviderNotAccepted
	}
	return generationapp.Receipt{ExternalTaskID: fixtureTaskID(uuid.NewString())}, nil
}

func (a *Adapter) Query(ctx context.Context, externalTaskID string) (generationapp.RemoteTask, error) {
	if err := ctx.Err(); err != nil {
		return generationapp.RemoteTask{}, err
	}
	if a == nil || !validFixtureTaskID(externalTaskID) {
		return generationapp.RemoteTask{}, generationapp.ErrInvalidProviderObservation
	}
	return generationapp.RemoteTask{ExternalTaskID: externalTaskID, State: generationapp.StatusSucceeded}, nil
}

func (a *Adapter) Cancel(ctx context.Context, externalTaskID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a == nil || !validFixtureTaskID(externalTaskID) {
		return generationapp.ErrInvalidProviderObservation
	}
	return nil
}

func (a *Adapter) Fetch(ctx context.Context, request generationapp.FetchRequest) (generationapp.FetchedResult, error) {
	if err := ctx.Err(); err != nil {
		return generationapp.FetchedResult{}, err
	}
	if a == nil || a.objects == nil || len(a.image) == 0 || request.Purpose != generationapp.PurposeImage ||
		!validFixtureTaskID(request.ExternalTaskID) || request.ObjectKey == "" || request.LookRevision < 1 {
		return generationapp.FetchedResult{}, errors.New("fixture generation result unavailable")
	}
	if _, err := uuid.Parse(request.TaskID); err != nil {
		return generationapp.FetchedResult{}, errors.New("fixture generation task invalid")
	}
	if _, err := uuid.Parse(request.LookID); err != nil {
		return generationapp.FetchedResult{}, errors.New("fixture generation look invalid")
	}

	data := append([]byte(nil), a.image...)
	versionID, err := a.objects.PutDerived(ctx, request.ObjectKey, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return generationapp.FetchedResult{}, err
	}
	digest := sha256.Sum256(data)
	return generationapp.FetchedResult{
		ExternalTaskID: request.ExternalTaskID,
		TaskID:         request.TaskID,
		Purpose:        request.Purpose,
		LookID:         request.LookID,
		LookRevision:   request.LookRevision,
		Inputs:         cloneInputSnapshot(request.Inputs),
		Fact: generationapp.OutputFact{
			ObjectKey:       request.ObjectKey,
			ObjectVersionID: versionID,
			ContentType:     generationapp.OutputContentTypeJPEG,
			ByteSize:        int64(len(data)),
			SHA256:          hex.EncodeToString(digest[:]),
		},
	}, nil
}

func placeholderJPEG() ([]byte, error) {
	const side = 64
	canvas := image.NewRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			shade := uint8(232)
			if (x/8+y/8)%2 == 1 {
				shade = 184
			}
			canvas.SetRGBA(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, canvas, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return generationapp.NormalizeGenerationImage(encoded.Bytes())
}

func fixtureTaskID(value string) string { return "fixture:" + value }

func validFixtureTaskID(value string) bool {
	if !strings.HasPrefix(value, "fixture:") {
		return false
	}
	_, err := uuid.Parse(strings.TrimPrefix(value, "fixture:"))
	return err == nil
}

func cloneInputSnapshot(snapshot generationapp.InputSnapshot) generationapp.InputSnapshot {
	snapshot.References = append([]generationapp.InputReference(nil), snapshot.References...)
	return snapshot
}

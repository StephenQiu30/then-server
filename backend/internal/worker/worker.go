// Package worker runs the approved private-media outbox and consumers.
package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type Repository interface {
	PendingOutbox(context.Context, int) ([]model.OutboxEvent, error)
	MarkOutboxPublished(context.Context, string, time.Time) error
	BeginMediaCheck(context.Context, string, time.Time) (model.MediaAsset, bool, error)
	CompleteMediaCheck(context.Context, string, string, *model.MediaDerivation, int, int, model.MediaStatus, string, time.Time) error
	BeginDeletion(context.Context, string, time.Time) (model.MediaAsset, []model.MediaDerivation, model.DeletionRequest, bool, error)
	CompleteDeletion(context.Context, string, string, time.Time) error
	SourceCleanupCandidates(context.Context, time.Time, int) ([]model.MediaAsset, error)
	MarkSourceDeleted(context.Context, string, time.Time) error
}

type Broker interface {
	Publish(context.Context, model.OutboxEvent) error
	Consume(context.Context, string, func(context.Context, model.OutboxEvent) error) error
}

type ObjectStore interface {
	OpenVersion(context.Context, string, string) (io.ReadCloser, error)
	PutDerived(context.Context, string, io.Reader, int64) (string, error)
	DeleteAllVersions(context.Context, string, string) error
}

type Runner struct {
	repository Repository
	broker     Broker
	objects    ObjectStore
	now        func() time.Time
}

func New(repository Repository, broker Broker, objects ObjectStore) (*Runner, error) {
	if repository == nil || broker == nil || objects == nil {
		return nil, model.ErrMediaUnavailable
	}
	return &Runner{repository: repository, broker: broker, objects: objects, now: time.Now}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errorsChannel := make(chan error, 4)
	go func() { errorsChannel <- r.relay(ctx) }()
	go func() { errorsChannel <- r.broker.Consume(ctx, "then.media-check", r.checkMedia) }()
	go func() { errorsChannel <- r.broker.Consume(ctx, "then.media-delete", r.deleteMedia) }()
	go func() { errorsChannel <- r.cleanupSources(ctx) }()
	err := <-errorsChannel
	if err == nil && ctx.Err() != nil {
		return nil
	}
	return err
}

func (r *Runner) cleanupSources(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		candidates, err := r.repository.SourceCleanupCandidates(ctx, r.now().UTC(), 32)
		if err != nil {
			return err
		}
		for _, media := range candidates {
			rawKey := media.RawObjectKey
			if media.Status == model.MediaDeleted {
				rawKey = "owners/" + media.OwnerID + "/media/" + media.ID + "/source.jpg"
			}
			if err := r.objects.DeleteAllVersions(ctx, "raw-private", rawKey); err != nil {
				return err
			}
			if media.Status == model.MediaDeleted {
				if err := r.objects.DeleteAllVersions(ctx, "derived-private", "media/"+media.ID+"/normalized.jpg"); err != nil {
					return err
				}
			}
			if err := r.repository.MarkSourceDeleted(ctx, media.ID, r.now().UTC()); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Runner) relay(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := r.publishPending(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Runner) publishPending(ctx context.Context) error {
	events, err := r.repository.PendingOutbox(ctx, 32)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := r.broker.Publish(ctx, event); err != nil {
			return err
		}
		if err := r.repository.MarkOutboxPublished(ctx, event.ID, r.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) checkMedia(ctx context.Context, event model.OutboxEvent) error {
	if event.EventType != "media.uploaded" {
		return errors.New("unexpected media-check event")
	}
	workContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	media, process, err := r.repository.BeginMediaCheck(workContext, event.AggregateID, r.now().UTC())
	if err != nil || !process {
		return err
	}
	data, width, height, reason, err := r.normalizedJPEG(workContext, media)
	if err != nil {
		if completionErr := r.repository.CompleteMediaCheck(ctx, event.ID, media.ID, nil, width, height, model.MediaRejected, reason, r.now().UTC()); completionErr != nil {
			return completionErr
		}
		return nil
	}
	key := "media/" + media.ID + "/normalized.jpg"
	versionID, err := r.objects.PutDerived(workContext, key, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	derivation := &model.MediaDerivation{ObjectKey: key, ObjectVersionID: versionID}
	if err := r.repository.CompleteMediaCheck(workContext, event.ID, media.ID, derivation, width, height, model.MediaReady, "ready", r.now().UTC()); err != nil {
		cleanupContext, stopCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		_ = r.objects.DeleteAllVersions(cleanupContext, "derived-private", key)
		stopCleanup()
		return err
	}
	return nil
}

func (r *Runner) normalizedJPEG(ctx context.Context, media model.MediaAsset) ([]byte, int, int, string, error) {
	reader, err := r.objects.OpenVersion(ctx, media.RawObjectKey, media.ObjectVersionID)
	if err != nil {
		return nil, 0, 0, "source_unavailable", err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, model.MaxPersonPhotoBytes+1))
	if err != nil || int64(len(data)) != media.ByteSize || len(data) > int(model.MaxPersonPhotoBytes) {
		return nil, 0, 0, "size_mismatch", errors.New("source size mismatch")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest != media.SHA256 {
		return nil, 0, 0, "digest_mismatch", errors.New("source digest mismatch")
	}
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 || bytes.Count(data, []byte{0xff, 0xd8}) != 1 {
		return nil, 0, 0, "invalid_jpeg", errors.New("invalid JPEG framing")
	}
	configuration, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || configuration.Width < 1 || configuration.Height < 1 || int64(configuration.Width)*int64(configuration.Height) > model.MaxPersonPhotoPixels {
		return nil, 0, 0, "pixel_limit", errors.New("JPEG pixel budget exceeded")
	}
	image, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, configuration.Width, configuration.Height, "invalid_jpeg", errors.New("JPEG decode failed")
	}
	var normalized bytes.Buffer
	if err := jpeg.Encode(&normalized, image, &jpeg.Options{Quality: 90}); err != nil {
		return nil, configuration.Width, configuration.Height, "normalization_failed", errors.New("JPEG normalization failed")
	}
	return normalized.Bytes(), configuration.Width, configuration.Height, "", nil
}

func (r *Runner) deleteMedia(ctx context.Context, event model.OutboxEvent) error {
	if event.EventType != "media.deletion_requested" {
		return errors.New("unexpected media-delete event")
	}
	media, derivations, _, process, err := r.repository.BeginDeletion(ctx, event.AggregateID, r.now().UTC())
	if err != nil || !process {
		return err
	}
	if media.RawObjectKey != "" {
		if err := r.objects.DeleteAllVersions(ctx, "raw-private", media.RawObjectKey); err != nil {
			return err
		}
	}
	for _, derivation := range derivations {
		if err := r.objects.DeleteAllVersions(ctx, "derived-private", derivation.ObjectKey); err != nil {
			return err
		}
	}
	if err := r.objects.DeleteAllVersions(ctx, "derived-private", "media/"+media.ID+"/normalized.jpg"); err != nil {
		return err
	}
	return r.repository.CompleteDeletion(ctx, event.ID, media.ID, r.now().UTC())
}

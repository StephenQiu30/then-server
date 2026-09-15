package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type mediaAuthenticatorStub struct {
	user model.User
	err  error
}

func (s *mediaAuthenticatorStub) CurrentUser(context.Context, string) (model.User, error) {
	return s.user, s.err
}

type mediaRepositoryStub struct {
	adult       bool
	consent     model.ConsentRecord
	media       model.MediaAsset
	deletion    model.DeletionRequest
	createCalls int
}

func (s *mediaRepositoryStub) CreateConsent(_ context.Context, ownerID string, input model.CreateConsentInput, at time.Time) (model.ConsentRecord, error) {
	s.createCalls++
	s.consent.OwnerID, s.consent.AgreedAt = ownerID, at
	return s.consent, nil
}
func (s *mediaRepositoryStub) GetConsent(context.Context, string, string) (model.ConsentRecord, error) {
	return s.consent, nil
}
func (s *mediaRepositoryStub) WithdrawConsent(context.Context, string, string, time.Time) (model.ConsentRecord, error) {
	return s.consent, nil
}
func (s *mediaRepositoryStub) IsSelfAdultConfirmed(context.Context, string) (bool, error) {
	return s.adult, nil
}
func (s *mediaRepositoryStub) CreateMedia(context.Context, string, model.CreateMediaUploadInput, time.Time) (model.MediaAsset, error) {
	return s.media, nil
}
func (s *mediaRepositoryStub) GetMedia(context.Context, string, string) (model.MediaAsset, error) {
	return s.media, nil
}
func (s *mediaRepositoryStub) CompleteMedia(context.Context, string, string, string, model.ObjectFact, time.Time) (model.MediaAsset, error) {
	return s.media, nil
}
func (s *mediaRepositoryStub) DeleteMedia(context.Context, string, string, time.Time) (model.DeletionRequest, error) {
	return s.deletion, nil
}
func (s *mediaRepositoryStub) GetDeletionRequest(context.Context, string, string) (model.DeletionRequest, error) {
	return s.deletion, nil
}

type mediaObjectStoreStub struct {
	upload model.SignedUpload
	fact   model.ObjectFact
	err    error
}

func (s *mediaObjectStoreStub) SignUpload(context.Context, model.MediaAsset) (model.SignedUpload, error) {
	return s.upload, s.err
}
func (s *mediaObjectStoreStub) HeadVersion(context.Context, model.MediaAsset, string) (model.ObjectFact, error) {
	return s.fact, s.err
}

func TestMediaServiceAuthenticatesBeforeValidatingConsent(t *testing.T) {
	repository := &mediaRepositoryStub{}
	service, err := NewMediaService(&mediaAuthenticatorStub{err: model.ErrAuthentication}, repository, &mediaObjectStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateConsent(context.Background(), "invalid", model.CreateConsentInput{})
	if !errors.Is(err, model.ErrAuthentication) || repository.createCalls != 0 {
		t.Fatal("authentication was not enforced before input and persistence")
	}
}

func TestMediaServiceRequiresFixedConsentAndAdultDeclaration(t *testing.T) {
	repository := &mediaRepositoryStub{adult: false}
	service, err := NewMediaService(&mediaAuthenticatorStub{user: model.User{ID: "owner"}}, repository, &mediaObjectStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateConsent(context.Background(), "session", model.CreateConsentInput{Purpose: "other", Category: model.MediaCategoryPersonPhoto, PolicyVersion: model.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	if !errors.Is(err, model.ErrInvalidMediaInput) {
		t.Fatal("unexpected consent purpose was accepted")
	}
	_, err = service.CreateMediaUpload(context.Background(), "session", model.CreateMediaUploadInput{ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Purpose: model.MediaPurposeAvatarSourcePreparation, ContentType: model.MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if !errors.Is(err, model.ErrConsentRequired) {
		t.Fatal("upload did not require the current adult declaration")
	}
}

func TestMediaServiceReturnsOnlySignedUploadResponse(t *testing.T) {
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	repository := &mediaRepositoryStub{adult: true, media: model.MediaAsset{ID: "media-id", OwnerID: "owner", Status: model.MediaPendingUpload, UploadExpiresAt: now.Add(model.UploadIntentLifetime)}}
	objects := &mediaObjectStoreStub{upload: model.SignedUpload{Method: "PUT", URL: "http://127.0.0.1/signed", Headers: map[string]string{"Content-Type": model.MediaContentTypeJPEG}, ExpiresAt: now.Add(model.UploadIntentLifetime)}}
	service, err := NewMediaService(&mediaAuthenticatorStub{user: model.User{ID: "owner"}}, repository, objects)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	upload, err := service.CreateMediaUpload(context.Background(), "session", model.CreateMediaUploadInput{ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Purpose: model.MediaPurposeAvatarSourcePreparation, ContentType: model.MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err != nil || upload.URL == "" || upload.Media.ID != "media-id" || upload.ExpiresAt.Sub(now) != model.UploadIntentLifetime {
		t.Fatalf("unexpected upload result: %+v err=%v", upload, err)
	}
}

package media

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
)

type mediaAuthenticatorStub struct {
	user domain.User
	err  error
}

func (s *mediaAuthenticatorStub) CurrentUser(context.Context, string) (domain.User, error) {
	return s.user, s.err
}

type mediaRepositoryStub struct {
	adult       bool
	consent     domain.ConsentRecord
	media       domain.MediaAsset
	deletion    domain.DeletionRequest
	createCalls int
	adultCalls  int
	mediaInput  domain.CreateMediaUploadInput
}

func (s *mediaRepositoryStub) CreateConsent(_ context.Context, ownerID string, input domain.CreateConsentInput, at time.Time) (domain.ConsentRecord, error) {
	s.createCalls++
	s.consent.OwnerID, s.consent.AgreedAt = ownerID, at
	return s.consent, nil
}
func (s *mediaRepositoryStub) GetConsent(context.Context, string, string) (domain.ConsentRecord, error) {
	return s.consent, nil
}
func (s *mediaRepositoryStub) WithdrawConsent(context.Context, string, string, time.Time) (domain.ConsentRecord, error) {
	return s.consent, nil
}
func (s *mediaRepositoryStub) IsSelfAdultConfirmed(context.Context, string) (bool, error) {
	s.adultCalls++
	return s.adult, nil
}
func (s *mediaRepositoryStub) CreateMedia(_ context.Context, _ string, input domain.CreateMediaUploadInput, _ time.Time) (domain.MediaAsset, error) {
	s.mediaInput = input
	return s.media, nil
}
func (s *mediaRepositoryStub) GetMedia(context.Context, string, string) (domain.MediaAsset, error) {
	return s.media, nil
}
func (s *mediaRepositoryStub) CompleteMedia(context.Context, string, string, string, domain.ObjectFact, time.Time) (domain.MediaAsset, error) {
	return s.media, nil
}
func (s *mediaRepositoryStub) DeleteMedia(context.Context, string, string, time.Time) (domain.DeletionRequest, error) {
	return s.deletion, nil
}
func (s *mediaRepositoryStub) GetDeletionRequest(context.Context, string, string) (domain.DeletionRequest, error) {
	return s.deletion, nil
}

type mediaObjectStoreStub struct {
	upload domain.SignedUpload
	fact   domain.ObjectFact
	err    error
}

func (s *mediaObjectStoreStub) SignUpload(context.Context, domain.MediaAsset) (domain.SignedUpload, error) {
	return s.upload, s.err
}
func (s *mediaObjectStoreStub) HeadVersion(context.Context, domain.MediaAsset, string) (domain.ObjectFact, error) {
	return s.fact, s.err
}

func TestMediaServiceAuthenticatesBeforeValidatingConsent(t *testing.T) {
	repository := &mediaRepositoryStub{}
	service, err := NewMediaService(&mediaAuthenticatorStub{err: domain.ErrAuthentication}, repository, &mediaObjectStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateConsent(context.Background(), "invalid", domain.CreateConsentInput{})
	if !errors.Is(err, domain.ErrAuthentication) || repository.createCalls != 0 {
		t.Fatal("authentication was not enforced before input and persistence")
	}
}

func TestMediaServiceRequiresFixedConsentAndAdultDeclaration(t *testing.T) {
	repository := &mediaRepositoryStub{adult: false}
	service, err := NewMediaService(&mediaAuthenticatorStub{user: domain.User{ID: "owner"}}, repository, &mediaObjectStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateConsent(context.Background(), "session", domain.CreateConsentInput{Purpose: "other", Category: domain.MediaCategoryPersonPhoto, PolicyVersion: domain.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	if !errors.Is(err, domain.ErrInvalidMediaInput) {
		t.Fatal("unexpected consent purpose was accepted")
	}
	_, err = service.CreateMediaUpload(context.Background(), "session", domain.CreateMediaUploadInput{ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Purpose: domain.MediaPurposeAvatarSourcePreparation, ContentType: domain.MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if !errors.Is(err, domain.ErrConsentRequired) {
		t.Fatal("upload did not require the current adult declaration")
	}
}

func TestMediaServiceReturnsOnlySignedUploadResponse(t *testing.T) {
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	repository := &mediaRepositoryStub{adult: true, media: domain.MediaAsset{ID: "media-id", OwnerID: "owner", Status: domain.MediaPendingUpload, UploadExpiresAt: now.Add(domain.UploadIntentLifetime)}}
	objects := &mediaObjectStoreStub{upload: domain.SignedUpload{Method: "PUT", URL: "http://127.0.0.1/signed", Headers: map[string]string{"Content-Type": domain.MediaContentTypeJPEG}, ExpiresAt: now.Add(domain.UploadIntentLifetime)}}
	service, err := NewMediaService(&mediaAuthenticatorStub{user: domain.User{ID: "owner"}}, repository, objects)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	upload, err := service.CreateMediaUpload(context.Background(), "session", domain.CreateMediaUploadInput{ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Purpose: domain.MediaPurposeAvatarSourcePreparation, ContentType: domain.MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err != nil || upload.URL == "" || upload.Media.ID != "media-id" || upload.ExpiresAt.Sub(now) != domain.UploadIntentLifetime {
		t.Fatalf("unexpected upload result: %+v err=%v", upload, err)
	}
}

func TestDiaryImageDoesNotReusePersonPhotoConsentGate(t *testing.T) {
	repository := &mediaRepositoryStub{adult: false, media: domain.MediaAsset{ID: "media-id", Purpose: domain.MediaPurposeDiaryImage, Category: domain.MediaCategoryOrdinaryImage}}
	objects := &mediaObjectStoreStub{upload: domain.SignedUpload{Method: "PUT", URL: "http://127.0.0.1/signed"}}
	service, err := NewMediaService(&mediaAuthenticatorStub{user: domain.User{ID: "owner"}}, repository, objects)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateMediaUpload(context.Background(), "session", domain.CreateMediaUploadInput{Purpose: domain.MediaPurposeDiaryImage, ContentType: domain.MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err != nil {
		t.Fatal(err)
	}
	if repository.adultCalls != 0 || repository.mediaInput.ConsentID != "" {
		t.Fatalf("diary image reused person-photo gate: adult_calls=%d input=%+v", repository.adultCalls, repository.mediaInput)
	}
}

package media

import (
	"context"
	"errors"
	"testing"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type mediaAuthenticatorStub struct {
	user accountapp.User
	err  error
}

func (s *mediaAuthenticatorStub) CurrentUser(context.Context, string) (accountapp.User, error) {
	return s.user, s.err
}

type mediaRepositoryStub struct {
	adult       bool
	consent     ConsentRecord
	media       MediaAsset
	deletion    DeletionRequest
	createCalls int
	adultCalls  int
	mediaInput  CreateMediaUploadInput
}

func (s *mediaRepositoryStub) CreateConsent(_ context.Context, ownerID string, input CreateConsentInput, at time.Time) (ConsentRecord, error) {
	s.createCalls++
	s.consent.OwnerID, s.consent.AgreedAt = ownerID, at
	return s.consent, nil
}
func (s *mediaRepositoryStub) GetConsent(context.Context, string, string) (ConsentRecord, error) {
	return s.consent, nil
}
func (s *mediaRepositoryStub) WithdrawConsent(context.Context, string, string, time.Time) (ConsentRecord, error) {
	return s.consent, nil
}
func (s *mediaRepositoryStub) IsSelfAdultConfirmed(context.Context, string) (bool, error) {
	s.adultCalls++
	return s.adult, nil
}
func (s *mediaRepositoryStub) CreateMedia(_ context.Context, _ string, input CreateMediaUploadInput, _ time.Time) (MediaAsset, error) {
	s.mediaInput = input
	return s.media, nil
}
func (s *mediaRepositoryStub) GetMedia(context.Context, string, string) (MediaAsset, error) {
	return s.media, nil
}
func (s *mediaRepositoryStub) CompleteMedia(context.Context, string, string, string, ObjectFact, time.Time) (MediaAsset, error) {
	return s.media, nil
}
func (s *mediaRepositoryStub) DeleteMedia(context.Context, string, string, time.Time) (DeletionRequest, error) {
	return s.deletion, nil
}
func (s *mediaRepositoryStub) GetDeletionRequest(context.Context, string, string) (DeletionRequest, error) {
	return s.deletion, nil
}

type mediaObjectStoreStub struct {
	upload SignedUpload
	fact   ObjectFact
	err    error
}

func (s *mediaObjectStoreStub) SignUpload(context.Context, MediaAsset) (SignedUpload, error) {
	return s.upload, s.err
}
func (s *mediaObjectStoreStub) HeadVersion(context.Context, MediaAsset, string) (ObjectFact, error) {
	return s.fact, s.err
}

func TestMediaServiceAuthenticatesBeforeValidatingConsent(t *testing.T) {
	repository := &mediaRepositoryStub{}
	service, err := NewMediaService(&mediaAuthenticatorStub{err: accountapp.ErrAuthentication}, repository, &mediaObjectStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateConsent(context.Background(), "invalid", CreateConsentInput{})
	if !errors.Is(err, accountapp.ErrAuthentication) || repository.createCalls != 0 {
		t.Fatal("authentication was not enforced before input and persistence")
	}
}

func TestMediaServiceRequiresFixedConsentAndAdultDeclaration(t *testing.T) {
	repository := &mediaRepositoryStub{adult: false}
	service, err := NewMediaService(&mediaAuthenticatorStub{user: accountapp.User{ID: "owner"}}, repository, &mediaObjectStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateConsent(context.Background(), "session", CreateConsentInput{Purpose: "other", Category: MediaCategoryPersonPhoto, PolicyVersion: CurrentMediaPolicyVersion, ActivelyAgreed: true})
	if !errors.Is(err, ErrInvalidMediaInput) {
		t.Fatal("unexpected consent purpose was accepted")
	}
	_, err = service.CreateMediaUpload(context.Background(), "session", CreateMediaUploadInput{ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Purpose: MediaPurposeAvatarSourcePreparation, ContentType: MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if !errors.Is(err, ErrConsentRequired) {
		t.Fatal("upload did not require the current adult declaration")
	}
}

func TestMediaServiceReturnsOnlySignedUploadResponse(t *testing.T) {
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	repository := &mediaRepositoryStub{adult: true, media: MediaAsset{ID: "media-id", OwnerID: "owner", Status: MediaPendingUpload, UploadExpiresAt: now.Add(UploadIntentLifetime)}}
	objects := &mediaObjectStoreStub{upload: SignedUpload{Method: "PUT", URL: "http://127.0.0.1/signed", Headers: map[string]string{"Content-Type": MediaContentTypeJPEG}, ExpiresAt: now.Add(UploadIntentLifetime)}}
	service, err := NewMediaService(&mediaAuthenticatorStub{user: accountapp.User{ID: "owner"}}, repository, objects)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	upload, err := service.CreateMediaUpload(context.Background(), "session", CreateMediaUploadInput{ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Purpose: MediaPurposeAvatarSourcePreparation, ContentType: MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err != nil || upload.URL == "" || upload.Media.ID != "media-id" || upload.ExpiresAt.Sub(now) != UploadIntentLifetime {
		t.Fatalf("unexpected upload result: %+v err=%v", upload, err)
	}
}

func TestGenerationInputUploadsRequirePurposeScopedConsentAndCategory(t *testing.T) {
	for _, category := range []string{MediaCategoryPersonPhoto, MediaCategoryOrdinaryImage} {
		t.Run(category, func(t *testing.T) {
			repository := &mediaRepositoryStub{adult: true, media: MediaAsset{ID: "media-id", Purpose: MediaPurposeGenerationInput, Category: category}}
			objects := &mediaObjectStoreStub{upload: SignedUpload{Method: "PUT", URL: "http://127.0.0.1/signed"}}
			service, err := NewMediaService(&mediaAuthenticatorStub{user: accountapp.User{ID: "owner"}}, repository, objects)
			if err != nil {
				t.Fatal(err)
			}
			consent, err := service.CreateConsent(context.Background(), "session", CreateConsentInput{
				Purpose: MediaPurposeGenerationInput, Category: category,
				PolicyVersion: CurrentGenerationInputPolicyVersion, ActivelyAgreed: true,
			})
			if err != nil || repository.createCalls != 1 {
				t.Fatalf("generation input consent was not accepted: consent=%+v err=%v", consent, err)
			}
			_, err = service.CreateMediaUpload(context.Background(), "session", CreateMediaUploadInput{
				ConsentID: "consent-id", Purpose: MediaPurposeGenerationInput, Category: category,
				ContentType: MediaContentTypeJPEG, ByteSize: 1024,
				SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			})
			if err != nil || repository.mediaInput.Category != category || repository.mediaInput.ConsentID != "consent-id" {
				t.Fatalf("generation input upload lost its category or consent: input=%+v err=%v", repository.mediaInput, err)
			}
			if category == MediaCategoryPersonPhoto && repository.adultCalls != 1 {
				t.Fatalf("person input did not require current adult declaration: calls=%d", repository.adultCalls)
			}
			if category == MediaCategoryOrdinaryImage && repository.adultCalls != 0 {
				t.Fatalf("ordinary garment input unexpectedly required adult declaration: calls=%d", repository.adultCalls)
			}
		})
	}
}

func TestGenerationInputConsentRejectsTrainingOrUnsupportedCategory(t *testing.T) {
	for _, input := range []CreateConsentInput{
		{Purpose: MediaPurposeGenerationInput, Category: MediaCategoryPersonPhoto, PolicyVersion: CurrentGenerationInputPolicyVersion, ActivelyAgreed: true, TrainingAllowed: true},
		{Purpose: MediaPurposeGenerationInput, Category: "", PolicyVersion: CurrentGenerationInputPolicyVersion, ActivelyAgreed: true},
	} {
		service, err := NewMediaService(&mediaAuthenticatorStub{user: accountapp.User{ID: "owner"}}, &mediaRepositoryStub{}, &mediaObjectStoreStub{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.CreateConsent(context.Background(), "session", input); !errors.Is(err, ErrInvalidMediaInput) {
			t.Fatalf("invalid generation-input consent was accepted: %+v err=%v", input, err)
		}
	}
}

func TestOrdinaryImagesDoNotReusePersonPhotoConsentGate(t *testing.T) {
	for _, purpose := range []string{MediaPurposeDiaryImage, MediaPurposeProfileAvatar} {
		repository := &mediaRepositoryStub{adult: false, media: MediaAsset{ID: "media-id", Purpose: purpose, Category: MediaCategoryOrdinaryImage}}
		objects := &mediaObjectStoreStub{upload: SignedUpload{Method: "PUT", URL: "http://127.0.0.1/signed"}}
		service, err := NewMediaService(&mediaAuthenticatorStub{user: accountapp.User{ID: "owner"}}, repository, objects)
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.CreateMediaUpload(context.Background(), "session", CreateMediaUploadInput{Purpose: purpose, ContentType: MediaContentTypeJPEG, ByteSize: 1024, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
		if err != nil {
			t.Fatal(err)
		}
		if repository.adultCalls != 0 || repository.mediaInput.ConsentID != "" {
			t.Fatalf("%s reused person-photo gate: adult_calls=%d input=%+v", purpose, repository.adultCalls, repository.mediaInput)
		}
	}
}

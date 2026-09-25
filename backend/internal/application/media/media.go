package media

import (
	"context"
	"regexp"
	"time"
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type MediaRepository interface {
	CreateConsent(context.Context, string, CreateConsentInput, time.Time) (ConsentRecord, error)
	GetConsent(context.Context, string, string) (ConsentRecord, error)
	WithdrawConsent(context.Context, string, string, time.Time) (ConsentRecord, error)
	IsSelfAdultConfirmed(context.Context, string) (bool, error)
	CreateMedia(context.Context, string, CreateMediaUploadInput, time.Time) (MediaAsset, error)
	GetMedia(context.Context, string, string) (MediaAsset, error)
	CompleteMedia(context.Context, string, string, string, ObjectFact, time.Time) (MediaAsset, error)
	DeleteMedia(context.Context, string, string, time.Time) (DeletionRequest, error)
	GetDeletionRequest(context.Context, string, string) (DeletionRequest, error)
}

type MediaObjectStore interface {
	SignUpload(context.Context, MediaAsset) (SignedUpload, error)
	HeadVersion(context.Context, MediaAsset, string) (ObjectFact, error)
}

type MediaService struct {
	authenticator PrivacyAuthenticator
	repository    MediaRepository
	objects       MediaObjectStore
	now           func() time.Time
}

func NewMediaService(authenticator PrivacyAuthenticator, repository MediaRepository, objects MediaObjectStore) (*MediaService, error) {
	if authenticator == nil || repository == nil || objects == nil {
		return nil, ErrMediaUnavailable
	}
	return &MediaService{authenticator: authenticator, repository: repository, objects: objects, now: time.Now}, nil
}

func (s *MediaService) CreateConsent(ctx context.Context, token string, input CreateConsentInput) (ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return ConsentRecord{}, err
	}
	if !validConsentInput(input) {
		return ConsentRecord{}, ErrInvalidMediaInput
	}
	return s.repository.CreateConsent(ctx, user.ID, input, s.now().UTC())
}

func (s *MediaService) GetConsent(ctx context.Context, token, consentID string) (ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return ConsentRecord{}, err
	}
	if consentID == "" {
		return ConsentRecord{}, ErrInvalidMediaInput
	}
	return s.repository.GetConsent(ctx, user.ID, consentID)
}

func (s *MediaService) WithdrawConsent(ctx context.Context, token, consentID string) (ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return ConsentRecord{}, err
	}
	if consentID == "" {
		return ConsentRecord{}, ErrInvalidMediaInput
	}
	return s.repository.WithdrawConsent(ctx, user.ID, consentID, s.now().UTC())
}

func (s *MediaService) CreateMediaUpload(ctx context.Context, token string, input CreateMediaUploadInput) (MediaUpload, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return MediaUpload{}, err
	}
	if !validUploadInput(input) {
		if input.ByteSize > MaxPersonPhotoBytes {
			return MediaUpload{}, ErrMediaTooLarge
		}
		return MediaUpload{}, ErrInvalidMediaInput
	}
	if input.Purpose == MediaPurposeAvatarSourcePreparation || input.Purpose == MediaPurposeGenerationInput && input.Category == MediaCategoryPersonPhoto {
		confirmed, err := s.repository.IsSelfAdultConfirmed(ctx, user.ID)
		if err != nil {
			return MediaUpload{}, err
		}
		if !confirmed {
			return MediaUpload{}, ErrConsentRequired
		}
	}
	media, err := s.repository.CreateMedia(ctx, user.ID, input, s.now().UTC())
	if err != nil {
		return MediaUpload{}, err
	}
	signed, err := s.objects.SignUpload(ctx, media)
	if err != nil {
		return MediaUpload{}, ErrMediaUnavailable
	}
	return MediaUpload{Media: media, Method: signed.Method, URL: signed.URL, Headers: signed.Headers, ExpiresAt: signed.ExpiresAt}, nil
}

func (s *MediaService) CompleteMediaUpload(ctx context.Context, token, mediaID string, input CompleteMediaUploadInput) (MediaAsset, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return MediaAsset{}, err
	}
	if mediaID == "" || input.VersionID == "" || len(input.VersionID) > 160 {
		return MediaAsset{}, ErrInvalidMediaInput
	}
	media, err := s.repository.GetMedia(ctx, user.ID, mediaID)
	if err != nil {
		return MediaAsset{}, err
	}
	fact, err := s.objects.HeadVersion(ctx, media, input.VersionID)
	if err != nil {
		return MediaAsset{}, ErrMediaConflict
	}
	if fact.ContentType != media.ContentType || fact.ByteSize != media.ByteSize || fact.SHA256 != media.SHA256 {
		return MediaAsset{}, ErrMediaConflict
	}
	return s.repository.CompleteMedia(ctx, user.ID, mediaID, input.VersionID, fact, s.now().UTC())
}

func (s *MediaService) GetMedia(ctx context.Context, token, mediaID string) (MediaAsset, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return MediaAsset{}, err
	}
	if mediaID == "" {
		return MediaAsset{}, ErrInvalidMediaInput
	}
	return s.repository.GetMedia(ctx, user.ID, mediaID)
}

func (s *MediaService) DeleteMedia(ctx context.Context, token, mediaID string) (DeletionRequest, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return DeletionRequest{}, err
	}
	if mediaID == "" {
		return DeletionRequest{}, ErrInvalidMediaInput
	}
	return s.repository.DeleteMedia(ctx, user.ID, mediaID, s.now().UTC())
}

func (s *MediaService) GetDeletionRequest(ctx context.Context, token, requestID string) (DeletionRequest, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return DeletionRequest{}, err
	}
	if requestID == "" {
		return DeletionRequest{}, ErrInvalidMediaInput
	}
	return s.repository.GetDeletionRequest(ctx, user.ID, requestID)
}

func validConsentInput(input CreateConsentInput) bool {
	if !input.ActivelyAgreed || input.TrainingAllowed {
		return false
	}
	switch input.Purpose {
	case MediaPurposeAvatarSourcePreparation:
		return input.Category == MediaCategoryPersonPhoto && input.PolicyVersion == CurrentMediaPolicyVersion
	case MediaPurposeGenerationInput:
		return (input.Category == MediaCategoryPersonPhoto || input.Category == MediaCategoryOrdinaryImage) && input.PolicyVersion == CurrentGenerationInputPolicyVersion
	default:
		return false
	}
}

func validUploadInput(input CreateMediaUploadInput) bool {
	if input.ContentType != MediaContentTypeJPEG || input.ByteSize <= 0 || input.ByteSize > MaxPersonPhotoBytes || !sha256Pattern.MatchString(input.SHA256) {
		return false
	}
	switch input.Purpose {
	case MediaPurposeAvatarSourcePreparation:
		return input.ConsentID != "" && (input.Category == "" || input.Category == MediaCategoryPersonPhoto)
	case MediaPurposeGenerationInput:
		return input.ConsentID != "" && (input.Category == MediaCategoryPersonPhoto || input.Category == MediaCategoryOrdinaryImage)
	case MediaPurposeDiaryImage, MediaPurposeCommunityPublish, MediaPurposeProfileAvatar:
		return input.ConsentID == "" && input.Category == ""
	default:
		return false
	}
}

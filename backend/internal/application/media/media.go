package media

import (
	"context"
	"regexp"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type MediaRepository interface {
	CreateConsent(context.Context, string, domain.CreateConsentInput, time.Time) (domain.ConsentRecord, error)
	GetConsent(context.Context, string, string) (domain.ConsentRecord, error)
	WithdrawConsent(context.Context, string, string, time.Time) (domain.ConsentRecord, error)
	IsSelfAdultConfirmed(context.Context, string) (bool, error)
	CreateMedia(context.Context, string, domain.CreateMediaUploadInput, time.Time) (domain.MediaAsset, error)
	GetMedia(context.Context, string, string) (domain.MediaAsset, error)
	CompleteMedia(context.Context, string, string, string, domain.ObjectFact, time.Time) (domain.MediaAsset, error)
	DeleteMedia(context.Context, string, string, time.Time) (domain.DeletionRequest, error)
	GetDeletionRequest(context.Context, string, string) (domain.DeletionRequest, error)
}

type MediaObjectStore interface {
	SignUpload(context.Context, domain.MediaAsset) (domain.SignedUpload, error)
	HeadVersion(context.Context, domain.MediaAsset, string) (domain.ObjectFact, error)
}

type MediaService struct {
	authenticator PrivacyAuthenticator
	repository    MediaRepository
	objects       MediaObjectStore
	now           func() time.Time
}

func NewMediaService(authenticator PrivacyAuthenticator, repository MediaRepository, objects MediaObjectStore) (*MediaService, error) {
	if authenticator == nil || repository == nil || objects == nil {
		return nil, domain.ErrMediaUnavailable
	}
	return &MediaService{authenticator: authenticator, repository: repository, objects: objects, now: time.Now}, nil
}

func (s *MediaService) CreateConsent(ctx context.Context, token string, input domain.CreateConsentInput) (domain.ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.ConsentRecord{}, err
	}
	if !validConsentInput(input) {
		return domain.ConsentRecord{}, domain.ErrInvalidMediaInput
	}
	return s.repository.CreateConsent(ctx, user.ID, input, s.now().UTC())
}

func (s *MediaService) GetConsent(ctx context.Context, token, consentID string) (domain.ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.ConsentRecord{}, err
	}
	if consentID == "" {
		return domain.ConsentRecord{}, domain.ErrInvalidMediaInput
	}
	return s.repository.GetConsent(ctx, user.ID, consentID)
}

func (s *MediaService) WithdrawConsent(ctx context.Context, token, consentID string) (domain.ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.ConsentRecord{}, err
	}
	if consentID == "" {
		return domain.ConsentRecord{}, domain.ErrInvalidMediaInput
	}
	return s.repository.WithdrawConsent(ctx, user.ID, consentID, s.now().UTC())
}

func (s *MediaService) CreateMediaUpload(ctx context.Context, token string, input domain.CreateMediaUploadInput) (domain.MediaUpload, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.MediaUpload{}, err
	}
	if !validUploadInput(input) {
		if input.ByteSize > domain.MaxPersonPhotoBytes {
			return domain.MediaUpload{}, domain.ErrMediaTooLarge
		}
		return domain.MediaUpload{}, domain.ErrInvalidMediaInput
	}
	if input.Purpose == domain.MediaPurposeAvatarSourcePreparation {
		confirmed, err := s.repository.IsSelfAdultConfirmed(ctx, user.ID)
		if err != nil {
			return domain.MediaUpload{}, err
		}
		if !confirmed {
			return domain.MediaUpload{}, domain.ErrConsentRequired
		}
	}
	media, err := s.repository.CreateMedia(ctx, user.ID, input, s.now().UTC())
	if err != nil {
		return domain.MediaUpload{}, err
	}
	signed, err := s.objects.SignUpload(ctx, media)
	if err != nil {
		return domain.MediaUpload{}, domain.ErrMediaUnavailable
	}
	return domain.MediaUpload{Media: media, Method: signed.Method, URL: signed.URL, Headers: signed.Headers, ExpiresAt: signed.ExpiresAt}, nil
}

func (s *MediaService) CompleteMediaUpload(ctx context.Context, token, mediaID string, input domain.CompleteMediaUploadInput) (domain.MediaAsset, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	if mediaID == "" || input.VersionID == "" || len(input.VersionID) > 160 {
		return domain.MediaAsset{}, domain.ErrInvalidMediaInput
	}
	media, err := s.repository.GetMedia(ctx, user.ID, mediaID)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	fact, err := s.objects.HeadVersion(ctx, media, input.VersionID)
	if err != nil {
		return domain.MediaAsset{}, domain.ErrMediaConflict
	}
	if fact.ContentType != media.ContentType || fact.ByteSize != media.ByteSize || fact.SHA256 != media.SHA256 {
		return domain.MediaAsset{}, domain.ErrMediaConflict
	}
	return s.repository.CompleteMedia(ctx, user.ID, mediaID, input.VersionID, fact, s.now().UTC())
}

func (s *MediaService) GetMedia(ctx context.Context, token, mediaID string) (domain.MediaAsset, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	if mediaID == "" {
		return domain.MediaAsset{}, domain.ErrInvalidMediaInput
	}
	return s.repository.GetMedia(ctx, user.ID, mediaID)
}

func (s *MediaService) DeleteMedia(ctx context.Context, token, mediaID string) (domain.DeletionRequest, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.DeletionRequest{}, err
	}
	if mediaID == "" {
		return domain.DeletionRequest{}, domain.ErrInvalidMediaInput
	}
	return s.repository.DeleteMedia(ctx, user.ID, mediaID, s.now().UTC())
}

func (s *MediaService) GetDeletionRequest(ctx context.Context, token, requestID string) (domain.DeletionRequest, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.DeletionRequest{}, err
	}
	if requestID == "" {
		return domain.DeletionRequest{}, domain.ErrInvalidMediaInput
	}
	return s.repository.GetDeletionRequest(ctx, user.ID, requestID)
}

func validConsentInput(input domain.CreateConsentInput) bool {
	return input.Purpose == domain.MediaPurposeAvatarSourcePreparation &&
		input.Category == domain.MediaCategoryPersonPhoto &&
		input.PolicyVersion == domain.CurrentMediaPolicyVersion &&
		input.ActivelyAgreed && !input.TrainingAllowed
}

func validUploadInput(input domain.CreateMediaUploadInput) bool {
	if input.ContentType != domain.MediaContentTypeJPEG || input.ByteSize <= 0 || input.ByteSize > domain.MaxPersonPhotoBytes || !sha256Pattern.MatchString(input.SHA256) {
		return false
	}
	switch input.Purpose {
	case domain.MediaPurposeAvatarSourcePreparation:
		return input.ConsentID != ""
	case domain.MediaPurposeDiaryImage:
		return input.ConsentID == ""
	default:
		return false
	}
}

package service

import (
	"context"
	"regexp"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type MediaRepository interface {
	CreateConsent(context.Context, string, model.CreateConsentInput, time.Time) (model.ConsentRecord, error)
	GetConsent(context.Context, string, string) (model.ConsentRecord, error)
	WithdrawConsent(context.Context, string, string, time.Time) (model.ConsentRecord, error)
	IsSelfAdultConfirmed(context.Context, string) (bool, error)
	CreateMedia(context.Context, string, model.CreateMediaUploadInput, time.Time) (model.MediaAsset, error)
	GetMedia(context.Context, string, string) (model.MediaAsset, error)
	CompleteMedia(context.Context, string, string, string, model.ObjectFact, time.Time) (model.MediaAsset, error)
	DeleteMedia(context.Context, string, string, time.Time) (model.DeletionRequest, error)
	GetDeletionRequest(context.Context, string, string) (model.DeletionRequest, error)
}

type MediaObjectStore interface {
	SignUpload(context.Context, model.MediaAsset) (model.SignedUpload, error)
	HeadVersion(context.Context, model.MediaAsset, string) (model.ObjectFact, error)
}

type MediaService struct {
	authenticator PrivacyAuthenticator
	repository    MediaRepository
	objects       MediaObjectStore
	now           func() time.Time
}

func NewMediaService(authenticator PrivacyAuthenticator, repository MediaRepository, objects MediaObjectStore) (*MediaService, error) {
	if authenticator == nil || repository == nil || objects == nil {
		return nil, model.ErrMediaUnavailable
	}
	return &MediaService{authenticator: authenticator, repository: repository, objects: objects, now: time.Now}, nil
}

func (s *MediaService) CreateConsent(ctx context.Context, token string, input model.CreateConsentInput) (model.ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.ConsentRecord{}, err
	}
	if !validConsentInput(input) {
		return model.ConsentRecord{}, model.ErrInvalidMediaInput
	}
	return s.repository.CreateConsent(ctx, user.ID, input, s.now().UTC())
}

func (s *MediaService) GetConsent(ctx context.Context, token, consentID string) (model.ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.ConsentRecord{}, err
	}
	if consentID == "" {
		return model.ConsentRecord{}, model.ErrInvalidMediaInput
	}
	return s.repository.GetConsent(ctx, user.ID, consentID)
}

func (s *MediaService) WithdrawConsent(ctx context.Context, token, consentID string) (model.ConsentRecord, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.ConsentRecord{}, err
	}
	if consentID == "" {
		return model.ConsentRecord{}, model.ErrInvalidMediaInput
	}
	return s.repository.WithdrawConsent(ctx, user.ID, consentID, s.now().UTC())
}

func (s *MediaService) CreateMediaUpload(ctx context.Context, token string, input model.CreateMediaUploadInput) (model.MediaUpload, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.MediaUpload{}, err
	}
	if !validUploadInput(input) {
		if input.ByteSize > model.MaxPersonPhotoBytes {
			return model.MediaUpload{}, model.ErrMediaTooLarge
		}
		return model.MediaUpload{}, model.ErrInvalidMediaInput
	}
	confirmed, err := s.repository.IsSelfAdultConfirmed(ctx, user.ID)
	if err != nil {
		return model.MediaUpload{}, err
	}
	if !confirmed {
		return model.MediaUpload{}, model.ErrConsentRequired
	}
	media, err := s.repository.CreateMedia(ctx, user.ID, input, s.now().UTC())
	if err != nil {
		return model.MediaUpload{}, err
	}
	signed, err := s.objects.SignUpload(ctx, media)
	if err != nil {
		return model.MediaUpload{}, model.ErrMediaUnavailable
	}
	return model.MediaUpload{Media: media, Method: signed.Method, URL: signed.URL, Headers: signed.Headers, ExpiresAt: signed.ExpiresAt}, nil
}

func (s *MediaService) CompleteMediaUpload(ctx context.Context, token, mediaID string, input model.CompleteMediaUploadInput) (model.MediaAsset, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.MediaAsset{}, err
	}
	if mediaID == "" || input.VersionID == "" || len(input.VersionID) > 160 {
		return model.MediaAsset{}, model.ErrInvalidMediaInput
	}
	media, err := s.repository.GetMedia(ctx, user.ID, mediaID)
	if err != nil {
		return model.MediaAsset{}, err
	}
	fact, err := s.objects.HeadVersion(ctx, media, input.VersionID)
	if err != nil {
		return model.MediaAsset{}, model.ErrMediaConflict
	}
	if fact.ContentType != media.ContentType || fact.ByteSize != media.ByteSize || fact.SHA256 != media.SHA256 {
		return model.MediaAsset{}, model.ErrMediaConflict
	}
	return s.repository.CompleteMedia(ctx, user.ID, mediaID, input.VersionID, fact, s.now().UTC())
}

func (s *MediaService) GetMedia(ctx context.Context, token, mediaID string) (model.MediaAsset, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.MediaAsset{}, err
	}
	if mediaID == "" {
		return model.MediaAsset{}, model.ErrInvalidMediaInput
	}
	return s.repository.GetMedia(ctx, user.ID, mediaID)
}

func (s *MediaService) DeleteMedia(ctx context.Context, token, mediaID string) (model.DeletionRequest, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.DeletionRequest{}, err
	}
	if mediaID == "" {
		return model.DeletionRequest{}, model.ErrInvalidMediaInput
	}
	return s.repository.DeleteMedia(ctx, user.ID, mediaID, s.now().UTC())
}

func (s *MediaService) GetDeletionRequest(ctx context.Context, token, requestID string) (model.DeletionRequest, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.DeletionRequest{}, err
	}
	if requestID == "" {
		return model.DeletionRequest{}, model.ErrInvalidMediaInput
	}
	return s.repository.GetDeletionRequest(ctx, user.ID, requestID)
}

func validConsentInput(input model.CreateConsentInput) bool {
	return input.Purpose == model.MediaPurposeAvatarSourcePreparation &&
		input.Category == model.MediaCategoryPersonPhoto &&
		input.PolicyVersion == model.CurrentMediaPolicyVersion &&
		input.ActivelyAgreed && !input.TrainingAllowed
}

func validUploadInput(input model.CreateMediaUploadInput) bool {
	return input.ConsentID != "" && input.Purpose == model.MediaPurposeAvatarSourcePreparation &&
		input.ContentType == model.MediaContentTypeJPEG && input.ByteSize > 0 &&
		input.ByteSize <= model.MaxPersonPhotoBytes && sha256Pattern.MatchString(input.SHA256)
}

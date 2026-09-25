package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/danielgtaylor/huma/v2"
)

type MediaService interface {
	CreateConsent(context.Context, string, mediaapp.CreateConsentInput) (mediaapp.ConsentRecord, error)
	GetConsent(context.Context, string, string) (mediaapp.ConsentRecord, error)
	WithdrawConsent(context.Context, string, string) (mediaapp.ConsentRecord, error)
	CreateMediaUpload(context.Context, string, mediaapp.CreateMediaUploadInput) (mediaapp.MediaUpload, error)
	CompleteMediaUpload(context.Context, string, string, mediaapp.CompleteMediaUploadInput) (mediaapp.MediaAsset, error)
	GetMedia(context.Context, string, string) (mediaapp.MediaAsset, error)
	DeleteMedia(context.Context, string, string) (mediaapp.DeletionRequest, error)
	GetDeletionRequest(context.Context, string, string) (mediaapp.DeletionRequest, error)
}

type MediaHandler struct {
	service      MediaService
	secureCookie bool
}

func NewMediaHandler(service MediaService, secureCookie bool) *MediaHandler {
	return &MediaHandler{service: service, secureCookie: secureCookie}
}

func (h *MediaHandler) createConsent(ctx context.Context, input *createConsentInput) (*consentOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	r := input.Body
	if r.Processor != mediaapp.MediaProcessorThen || r.Region != mediaapp.MediaRegionLocalDevelopment || r.MaxRetentionHours != 24 {
		return nil, newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	consent, err := h.service.CreateConsent(ctx, input.Session, mediaapp.CreateConsentInput{Purpose: r.Purpose, Category: r.Category, PolicyVersion: r.PolicyVersion, ActivelyAgreed: r.ActivelyAgreed, TrainingAllowed: r.TrainingAllowed})
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &consentOutput{RequestID: requestID(ctx), Body: consentResponse(consent)}, nil
}

func (h *MediaHandler) getConsent(ctx context.Context, input *consentResourceInput) (*consentOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	consent, err := h.service.GetConsent(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &consentOutput{RequestID: requestID(ctx), Body: consentResponse(consent)}, nil
}

func (h *MediaHandler) withdrawConsent(ctx context.Context, input *consentResourceInput) (*consentOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	consent, err := h.service.WithdrawConsent(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &consentOutput{RequestID: requestID(ctx), Body: consentResponse(consent)}, nil
}

func (h *MediaHandler) createUpload(ctx context.Context, input *createMediaUploadInput) (*mediaUploadOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	r := input.Body
	consentID := ""
	if r.ConsentID != nil {
		consentID = *r.ConsentID
	}
	upload, err := h.service.CreateMediaUpload(ctx, input.Session, mediaapp.CreateMediaUploadInput{ConsentID: consentID, Purpose: r.Purpose, Category: r.Category, ContentType: r.ContentType, ByteSize: r.ByteSize, SHA256: r.SHA256})
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &mediaUploadOutput{RequestID: requestID(ctx), Body: MediaUploadResponse{Media: mediaResponse(upload.Media), Method: upload.Method, URL: upload.URL, Headers: upload.Headers, ExpiresAt: upload.ExpiresAt}}, nil
}

func (h *MediaHandler) completeUpload(ctx context.Context, input *completeMediaUploadInput) (*mediaOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	media, err := h.service.CompleteMediaUpload(ctx, input.Session, input.ID, mediaapp.CompleteMediaUploadInput{VersionID: input.Body.VersionID})
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &mediaOutput{RequestID: requestID(ctx), Body: mediaResponse(media)}, nil
}

func (h *MediaHandler) getMedia(ctx context.Context, input *mediaResourceInput) (*mediaOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	media, err := h.service.GetMedia(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &mediaOutput{RequestID: requestID(ctx), Body: mediaResponse(media)}, nil
}

func (h *MediaHandler) deleteMedia(ctx context.Context, input *mediaResourceInput) (*deletionOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	deletion, err := h.service.DeleteMedia(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &deletionOutput{RequestID: requestID(ctx), Body: deletionResponse(deletion)}, nil
}

func (h *MediaHandler) getDeletion(ctx context.Context, input *deletionResourceInput) (*deletionOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	deletion, err := h.service.GetDeletionRequest(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &deletionOutput{RequestID: requestID(ctx), Body: deletionResponse(deletion)}, nil
}

func (h *MediaHandler) available(ctx context.Context, session string) error {
	if h == nil || h.service == nil {
		return newErrorResponse(http.StatusServiceUnavailable, requestID(ctx))
	}
	if session == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}

func (h *MediaHandler) mediaError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	case errors.Is(err, mediaapp.ErrInvalidMediaInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, mediaapp.ErrMediaTooLarge):
		return newErrorResponse(http.StatusRequestEntityTooLarge, requestID(ctx))
	case errors.Is(err, mediaapp.ErrMediaNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, mediaapp.ErrConsentRequired), errors.Is(err, mediaapp.ErrMediaConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Request conflicts with current media state."
		return response
	default:
		return huma.ErrorWithHeaders(newErrorResponse(http.StatusServiceUnavailable, requestID(ctx)), http.Header{"Retry-After": []string{"1"}})
	}
}

func consentResponse(c mediaapp.ConsentRecord) ConsentResponse {
	return ConsentResponse{ID: c.ID, Purpose: c.Purpose, Category: c.Category, Processor: c.Processor, Region: c.Region, PolicyVersion: c.PolicyVersion, MaxRetentionHours: c.MaxRetentionHours, TrainingAllowed: c.TrainingAllowed, Status: c.Status, AgreedAt: c.AgreedAt, WithdrawnAt: c.WithdrawnAt}
}

func mediaResponse(m mediaapp.MediaAsset) MediaResponse {
	var consentID *string
	if m.ConsentID != "" {
		value := m.ConsentID
		consentID = &value
	}
	return MediaResponse{ID: m.ID, ConsentID: consentID, Purpose: m.Purpose, Category: m.Category, ContentType: m.ContentType, ByteSize: m.ByteSize, Status: m.Status, Reason: m.StableReason, PixelWidth: m.PixelWidth, PixelHeight: m.PixelHeight, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func deletionResponse(d mediaapp.DeletionRequest) DeletionRequestResponse {
	return DeletionRequestResponse{ID: d.ID, MediaID: d.MediaID, Status: d.Status, ReadRevokedAt: d.ReadRevokedAt, CompletedAt: d.CompletedAt, BackupExpiresAt: d.BackupExpiresAt, Error: d.StableError, Attempts: d.Attempts, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

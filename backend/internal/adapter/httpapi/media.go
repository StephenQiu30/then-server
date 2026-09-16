package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"github.com/danielgtaylor/huma/v2"
)

type MediaService interface {
	CreateConsent(context.Context, string, domain.CreateConsentInput) (domain.ConsentRecord, error)
	GetConsent(context.Context, string, string) (domain.ConsentRecord, error)
	WithdrawConsent(context.Context, string, string) (domain.ConsentRecord, error)
	CreateMediaUpload(context.Context, string, domain.CreateMediaUploadInput) (domain.MediaUpload, error)
	CompleteMediaUpload(context.Context, string, string, domain.CompleteMediaUploadInput) (domain.MediaAsset, error)
	GetMedia(context.Context, string, string) (domain.MediaAsset, error)
	DeleteMedia(context.Context, string, string) (domain.DeletionRequest, error)
	GetDeletionRequest(context.Context, string, string) (domain.DeletionRequest, error)
}

type MediaHandler struct {
	service      MediaService
	secureCookie bool
}

func NewMediaHandler(service MediaService, secureCookie bool) *MediaHandler {
	return &MediaHandler{service: service, secureCookie: secureCookie}
}

type CreateConsentRequest struct {
	Purpose           string `json:"purpose" enum:"avatar_source_preparation" example:"avatar_source_preparation"`
	Category          string `json:"category" enum:"person_photo" example:"person_photo"`
	Processor         string `json:"processor" enum:"then" example:"then"`
	Region            string `json:"region" enum:"local-development" example:"local-development"`
	PolicyVersion     string `json:"policy_version" enum:"person-photo-v1" example:"person-photo-v1"`
	MaxRetentionHours int    `json:"max_retention_hours" enum:"24" example:"24"`
	ActivelyAgreed    bool   `json:"actively_agreed" enum:"true" example:"true"`
	TrainingAllowed   bool   `json:"training_allowed" enum:"false" example:"false"`
}

type ConsentResponse struct {
	ID                string               `json:"id" format:"uuid"`
	Purpose           string               `json:"purpose" enum:"avatar_source_preparation"`
	Category          string               `json:"category" enum:"person_photo"`
	Processor         string               `json:"processor" enum:"then"`
	Region            string               `json:"region" enum:"local-development"`
	PolicyVersion     string               `json:"policy_version" enum:"person-photo-v1"`
	MaxRetentionHours int                  `json:"max_retention_hours" enum:"24"`
	TrainingAllowed   bool                 `json:"training_allowed" enum:"false"`
	Status            domain.ConsentStatus `json:"status" enum:"active,withdrawn"`
	AgreedAt          time.Time            `json:"agreed_at" format:"date-time"`
	WithdrawnAt       *time.Time           `json:"withdrawn_at,omitempty" format:"date-time"`
}

type CreateMediaUploadRequest struct {
	ConsentID   *string `json:"consent_id,omitempty" format:"uuid"`
	Purpose     string  `json:"purpose" enum:"avatar_source_preparation,diary_image"`
	ContentType string  `json:"content_type" enum:"image/jpeg"`
	ByteSize    int64   `json:"byte_size" minimum:"1"`
	SHA256      string  `json:"sha256" pattern:"^[a-f0-9]{64}$"`
}

type MediaResponse struct {
	ID          string             `json:"id" format:"uuid"`
	ConsentID   *string            `json:"consent_id,omitempty" format:"uuid"`
	Purpose     string             `json:"purpose" enum:"avatar_source_preparation,diary_image"`
	Category    string             `json:"category" enum:"person_photo,ordinary_image"`
	ContentType string             `json:"content_type" enum:"image/jpeg"`
	ByteSize    int64              `json:"byte_size" minimum:"1"`
	Status      domain.MediaStatus `json:"status" enum:"pending_upload,uploaded,checking,ready,rejected,deleting,deleted"`
	Reason      string             `json:"reason,omitempty" maxLength:"80"`
	PixelWidth  int                `json:"pixel_width,omitempty" minimum:"0"`
	PixelHeight int                `json:"pixel_height,omitempty" minimum:"0"`
	CreatedAt   time.Time          `json:"created_at" format:"date-time"`
	UpdatedAt   time.Time          `json:"updated_at" format:"date-time"`
}

type MediaUploadResponse struct {
	Media     MediaResponse     `json:"media"`
	Method    string            `json:"method" enum:"PUT"`
	URL       string            `json:"url" format:"uri"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at" format:"date-time"`
}

type CompleteMediaUploadRequest struct {
	VersionID string `json:"version_id" minLength:"1" maxLength:"160"`
}

type DeletionRequestResponse struct {
	ID              string                `json:"id" format:"uuid"`
	MediaID         string                `json:"media_id" format:"uuid"`
	Status          domain.DeletionStatus `json:"status" enum:"pending,running,complete,failed"`
	ReadRevokedAt   time.Time             `json:"read_revoked_at" format:"date-time"`
	CompletedAt     *time.Time            `json:"completed_at,omitempty" format:"date-time"`
	BackupExpiresAt time.Time             `json:"backup_expires_at" format:"date-time"`
	Error           string                `json:"error,omitempty" maxLength:"80"`
	Attempts        int                   `json:"attempts" minimum:"0"`
	CreatedAt       time.Time             `json:"created_at" format:"date-time"`
	UpdatedAt       time.Time             `json:"updated_at" format:"date-time"`
}

type createConsentInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateConsentRequest
}
type mediaResourceInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"media_id" format:"uuid"`
}
type consentResourceInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"consent_id" format:"uuid"`
}
type deletionResourceInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"request_id" format:"uuid"`
}
type createMediaUploadInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateMediaUploadRequest
}
type completeMediaUploadInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"media_id" format:"uuid"`
	Body    CompleteMediaUploadRequest
}

type consentOutput struct {
	RequestID string          `header:"X-Request-ID"`
	Body      ConsentResponse `json:"body"`
}
type mediaOutput struct {
	RequestID string        `header:"X-Request-ID"`
	Body      MediaResponse `json:"body"`
}
type mediaUploadOutput struct {
	RequestID string              `header:"X-Request-ID"`
	Body      MediaUploadResponse `json:"body"`
}
type deletionOutput struct {
	RequestID string                  `header:"X-Request-ID"`
	Body      DeletionRequestResponse `json:"body"`
}

func registerMediaOperations(api huma.API, handler *MediaHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createConsent", Method: http.MethodPost, Path: "/consents", Tags: []string{"Private media"}, Summary: "同意本人照片的固定本地开发用途", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024, Errors: mediaErrors(http.StatusBadRequest)}), handler.createConsent)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getConsent", Method: http.MethodGet, Path: "/consents/{consent_id}", Tags: []string{"Private media"}, Summary: "获取本人同意状态", Errors: mediaErrors(http.StatusNotFound)}), handler.getConsent)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "withdrawConsent", Method: http.MethodPost, Path: "/consents/{consent_id}/withdraw", Tags: []string{"Private media"}, Summary: "撤回本人照片同意", Errors: mediaErrors(http.StatusNotFound)}), handler.withdrawConsent)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createMediaUpload", Method: http.MethodPost, Path: "/media/uploads", Tags: []string{"Private media"}, Summary: "创建一次 JPEG 私有直传意图", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024, Errors: mediaErrors(http.StatusBadRequest, http.StatusConflict, http.StatusRequestEntityTooLarge)}), handler.createUpload)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "completeMediaUpload", Method: http.MethodPost, Path: "/media/{media_id}/complete", Tags: []string{"Private media"}, Summary: "固定已上传对象版本", MaxBodyBytes: 4 * 1024, Errors: mediaErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.completeUpload)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getMedia", Method: http.MethodGet, Path: "/media/{media_id}", Tags: []string{"Private media"}, Summary: "获取本人媒体处理状态", Errors: mediaErrors(http.StatusNotFound)}), handler.getMedia)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteMedia", Method: http.MethodDelete, Path: "/media/{media_id}", Tags: []string{"Private media"}, Summary: "立即撤销读取并请求删除媒体", DefaultStatus: http.StatusAccepted, Errors: mediaErrors(http.StatusNotFound, http.StatusConflict)}), handler.deleteMedia)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getDeletionRequest", Method: http.MethodGet, Path: "/deletion-requests/{request_id}", Tags: []string{"Private media"}, Summary: "获取本人媒体删除进度", Errors: mediaErrors(http.StatusNotFound)}), handler.getDeletion)
}

func mediaErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusServiceUnavailable, http.StatusInternalServerError}, statuses...)
}

func (h *MediaHandler) createConsent(ctx context.Context, input *createConsentInput) (*consentOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	r := input.Body
	if r.Processor != domain.MediaProcessorThen || r.Region != domain.MediaRegionLocalDevelopment || r.MaxRetentionHours != 24 {
		return nil, newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	consent, err := h.service.CreateConsent(ctx, input.Session, domain.CreateConsentInput{Purpose: r.Purpose, Category: r.Category, PolicyVersion: r.PolicyVersion, ActivelyAgreed: r.ActivelyAgreed, TrainingAllowed: r.TrainingAllowed})
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
	upload, err := h.service.CreateMediaUpload(ctx, input.Session, domain.CreateMediaUploadInput{ConsentID: consentID, Purpose: r.Purpose, ContentType: r.ContentType, ByteSize: r.ByteSize, SHA256: r.SHA256})
	if err != nil {
		return nil, h.mediaError(ctx, err)
	}
	return &mediaUploadOutput{RequestID: requestID(ctx), Body: MediaUploadResponse{Media: mediaResponse(upload.Media), Method: upload.Method, URL: upload.URL, Headers: upload.Headers, ExpiresAt: upload.ExpiresAt}}, nil
}

func (h *MediaHandler) completeUpload(ctx context.Context, input *completeMediaUploadInput) (*mediaOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	media, err := h.service.CompleteMediaUpload(ctx, input.Session, input.ID, domain.CompleteMediaUploadInput{VersionID: input.Body.VersionID})
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
	case errors.Is(err, domain.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	case errors.Is(err, domain.ErrInvalidMediaInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, domain.ErrMediaTooLarge):
		return newErrorResponse(http.StatusRequestEntityTooLarge, requestID(ctx))
	case errors.Is(err, domain.ErrMediaNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, domain.ErrConsentRequired), errors.Is(err, domain.ErrMediaConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Request conflicts with current media state."
		return response
	default:
		return huma.ErrorWithHeaders(newErrorResponse(http.StatusServiceUnavailable, requestID(ctx)), http.Header{"Retry-After": []string{"1"}})
	}
}

func consentResponse(c domain.ConsentRecord) ConsentResponse {
	return ConsentResponse{ID: c.ID, Purpose: c.Purpose, Category: c.Category, Processor: c.Processor, Region: c.Region, PolicyVersion: c.PolicyVersion, MaxRetentionHours: c.MaxRetentionHours, TrainingAllowed: c.TrainingAllowed, Status: c.Status, AgreedAt: c.AgreedAt, WithdrawnAt: c.WithdrawnAt}
}
func mediaResponse(m domain.MediaAsset) MediaResponse {
	var consentID *string
	if m.ConsentID != "" {
		value := m.ConsentID
		consentID = &value
	}
	return MediaResponse{ID: m.ID, ConsentID: consentID, Purpose: m.Purpose, Category: m.Category, ContentType: m.ContentType, ByteSize: m.ByteSize, Status: m.Status, Reason: m.StableReason, PixelWidth: m.PixelWidth, PixelHeight: m.PixelHeight, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func deletionResponse(d domain.DeletionRequest) DeletionRequestResponse {
	return DeletionRequestResponse{ID: d.ID, MediaID: d.MediaID, Status: d.Status, ReadRevokedAt: d.ReadRevokedAt, CompletedAt: d.CompletedAt, BackupExpiresAt: d.BackupExpiresAt, Error: d.StableError, Attempts: d.Attempts, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

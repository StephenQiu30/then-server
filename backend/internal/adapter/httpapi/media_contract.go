package httpapi

import (
	"time"

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
)

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
	ID                string                 `json:"id" format:"uuid"`
	Purpose           string                 `json:"purpose" enum:"avatar_source_preparation"`
	Category          string                 `json:"category" enum:"person_photo"`
	Processor         string                 `json:"processor" enum:"then"`
	Region            string                 `json:"region" enum:"local-development"`
	PolicyVersion     string                 `json:"policy_version" enum:"person-photo-v1"`
	MaxRetentionHours int                    `json:"max_retention_hours" enum:"24"`
	TrainingAllowed   bool                   `json:"training_allowed" enum:"false"`
	Status            mediaapp.ConsentStatus `json:"status" enum:"active,withdrawn"`
	AgreedAt          time.Time              `json:"agreed_at" format:"date-time"`
	WithdrawnAt       *time.Time             `json:"withdrawn_at,omitempty" format:"date-time"`
}

type CreateMediaUploadRequest struct {
	ConsentID   *string `json:"consent_id,omitempty" format:"uuid"`
	Purpose     string  `json:"purpose" enum:"avatar_source_preparation,diary_image,community_publish,profile_avatar"`
	ContentType string  `json:"content_type" enum:"image/jpeg"`
	ByteSize    int64   `json:"byte_size" minimum:"1"`
	SHA256      string  `json:"sha256" pattern:"^[a-f0-9]{64}$"`
}

type MediaResponse struct {
	ID          string               `json:"id" format:"uuid"`
	ConsentID   *string              `json:"consent_id,omitempty" format:"uuid"`
	Purpose     string               `json:"purpose" enum:"avatar_source_preparation,diary_image,community_publish,profile_avatar"`
	Category    string               `json:"category" enum:"person_photo,ordinary_image"`
	ContentType string               `json:"content_type" enum:"image/jpeg"`
	ByteSize    int64                `json:"byte_size" minimum:"1"`
	Status      mediaapp.MediaStatus `json:"status" enum:"pending_upload,uploaded,checking,ready,rejected,deleting,deleted"`
	Reason      string               `json:"reason,omitempty" maxLength:"80"`
	PixelWidth  int                  `json:"pixel_width,omitempty" minimum:"0"`
	PixelHeight int                  `json:"pixel_height,omitempty" minimum:"0"`
	CreatedAt   time.Time            `json:"created_at" format:"date-time"`
	UpdatedAt   time.Time            `json:"updated_at" format:"date-time"`
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
	ID              string                  `json:"id" format:"uuid"`
	MediaID         string                  `json:"media_id" format:"uuid"`
	Status          mediaapp.DeletionStatus `json:"status" enum:"pending,running,complete,failed"`
	ReadRevokedAt   time.Time               `json:"read_revoked_at" format:"date-time"`
	CompletedAt     *time.Time              `json:"completed_at,omitempty" format:"date-time"`
	BackupExpiresAt time.Time               `json:"backup_expires_at" format:"date-time"`
	Error           string                  `json:"error,omitempty" maxLength:"80"`
	Attempts        int                     `json:"attempts" minimum:"0"`
	CreatedAt       time.Time               `json:"created_at" format:"date-time"`
	UpdatedAt       time.Time               `json:"updated_at" format:"date-time"`
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

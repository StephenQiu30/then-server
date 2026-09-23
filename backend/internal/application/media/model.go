package media

import (
	"errors"
	"time"
)

const (
	MediaPurposeAvatarSourcePreparation       = "avatar_source_preparation"
	MediaPurposeDiaryImage                    = "diary_image"
	MediaPurposeCommunityPublish              = "community_publish"
	MediaPurposeProfileAvatar                 = "profile_avatar"
	MediaCategoryPersonPhoto                  = "person_photo"
	MediaCategoryOrdinaryImage                = "ordinary_image"
	MediaProcessorThen                        = "then"
	MediaRegionLocalDevelopment               = "local-development"
	MediaContentTypeJPEG                      = "image/jpeg"
	CurrentMediaPolicyVersion                 = "person-photo-v1"
	MaxPersonPhotoBytes                 int64 = 12 * 1024 * 1024
	MaxPersonPhotoPixels                int64 = 24_000_000
	UploadIntentLifetime                      = 10 * time.Minute
	UnfinishedUploadLifetime                  = 24 * time.Hour
)

var (
	ErrInvalidMediaInput = errors.New("invalid media input")
	ErrConsentRequired   = errors.New("active consent required")
	ErrMediaNotFound     = errors.New("media resource not found")
	ErrMediaConflict     = errors.New("media state conflicts with request")
	ErrMediaTooLarge     = errors.New("media payload too large")
	ErrMediaUnavailable  = errors.New("media service unavailable")
)

type ConsentStatus string

const (
	ConsentActive    ConsentStatus = "active"
	ConsentWithdrawn ConsentStatus = "withdrawn"
)

type ConsentRecord struct {
	ID                string
	OwnerID           string
	Purpose           string
	Category          string
	Processor         string
	Region            string
	PolicyVersion     string
	MaxRetentionHours int
	TrainingAllowed   bool
	Status            ConsentStatus
	AgreedAt          time.Time
	WithdrawnAt       *time.Time
}

type CreateConsentInput struct {
	Purpose         string
	Category        string
	PolicyVersion   string
	ActivelyAgreed  bool
	TrainingAllowed bool
}

type MediaStatus string

const (
	MediaPendingUpload MediaStatus = "pending_upload"
	MediaUploaded      MediaStatus = "uploaded"
	MediaChecking      MediaStatus = "checking"
	MediaReady         MediaStatus = "ready"
	MediaRejected      MediaStatus = "rejected"
	MediaDeleting      MediaStatus = "deleting"
	MediaDeleted       MediaStatus = "deleted"
)

func (status MediaStatus) CanTransitionTo(next MediaStatus) bool {
	switch status {
	case MediaPendingUpload:
		return next == MediaUploaded || next == MediaDeleting
	case MediaUploaded:
		return next == MediaChecking || next == MediaDeleting
	case MediaChecking:
		return next == MediaReady || next == MediaRejected || next == MediaDeleting
	case MediaReady, MediaRejected:
		return next == MediaDeleting
	case MediaDeleting:
		return next == MediaDeleted
	default:
		return false
	}
}

type CreateMediaUploadInput struct {
	ConsentID   string
	Purpose     string
	ContentType string
	ByteSize    int64
	SHA256      string
}

type MediaUpload struct {
	Media     MediaAsset
	Method    string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

type SignedUpload struct {
	Method    string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

type CompleteMediaUploadInput struct {
	VersionID string
}

type ObjectFact struct {
	ContentType string
	ByteSize    int64
	SHA256      string
}

type MediaAsset struct {
	ID              string
	OwnerID         string
	ConsentID       string
	Purpose         string
	Category        string
	ContentType     string
	ByteSize        int64
	SHA256          string
	RawObjectKey    string
	ObjectVersionID string
	Status          MediaStatus
	StableReason    string
	PixelWidth      int
	PixelHeight     int
	UploadExpiresAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	SourceDeletedAt *time.Time
}

type DeletionStatus string

const (
	DeletionPending  DeletionStatus = "pending"
	DeletionRunning  DeletionStatus = "running"
	DeletionComplete DeletionStatus = "complete"
	DeletionFailed   DeletionStatus = "failed"
)

type DeletionRequest struct {
	ID              string
	OwnerID         string
	MediaID         string
	Status          DeletionStatus
	ReadRevokedAt   time.Time
	CompletedAt     *time.Time
	BackupExpiresAt time.Time
	StableError     string
	Attempts        int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type OutboxEvent struct {
	ID          string
	EventType   string
	AggregateID string
	CreatedAt   time.Time
}

type MediaDerivation struct {
	ObjectKey       string
	ObjectVersionID string
}

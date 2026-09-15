package repository

import (
	"context"
	"errors"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MediaRepository struct{ database *gorm.DB }

func NewMediaRepository(database *gorm.DB) *MediaRepository {
	return &MediaRepository{database: database}
}

type consentRecord struct {
	ID                string             `gorm:"column:id;type:uuid;primaryKey"`
	OwnerID           string             `gorm:"column:owner_id;type:uuid;not null;index:consent_records_owner_idx;uniqueIndex:consent_records_active_unique,where:withdrawn_at IS NULL"`
	Purpose           string             `gorm:"column:purpose;type:text;not null;uniqueIndex:consent_records_active_unique,where:withdrawn_at IS NULL;check:consent_records_purpose_check,purpose = 'avatar_source_preparation'"`
	Category          string             `gorm:"column:category;type:text;not null;uniqueIndex:consent_records_active_unique,where:withdrawn_at IS NULL;check:consent_records_category_check,category = 'person_photo'"`
	Processor         string             `gorm:"column:processor;type:text;not null;uniqueIndex:consent_records_active_unique,where:withdrawn_at IS NULL;check:consent_records_processor_check,processor = 'then'"`
	Region            string             `gorm:"column:region;type:text;not null;uniqueIndex:consent_records_active_unique,where:withdrawn_at IS NULL;check:consent_records_region_check,region = 'local-development'"`
	PolicyVersion     string             `gorm:"column:policy_version;type:text;not null;uniqueIndex:consent_records_active_unique,where:withdrawn_at IS NULL;check:consent_records_policy_check,policy_version = 'person-photo-v1'"`
	MaxRetentionHours int                `gorm:"column:max_retention_hours;not null;check:consent_records_retention_check,max_retention_hours = 24"`
	TrainingAllowed   bool               `gorm:"column:training_allowed;not null;uniqueIndex:consent_records_active_unique,where:withdrawn_at IS NULL;check:consent_records_training_check,training_allowed = false"`
	AgreedAt          time.Time          `gorm:"column:agreed_at;type:timestamptz;not null"`
	WithdrawnAt       *time.Time         `gorm:"column:withdrawn_at;type:timestamptz;check:consent_records_withdrawn_check,withdrawn_at IS NULL OR withdrawn_at >= agreed_at"`
	Media             []mediaAssetRecord `gorm:"foreignKey:ConsentID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (consentRecord) TableName() string { return "consent_records" }

type mediaAssetRecord struct {
	ID              string                  `gorm:"column:id;type:uuid;primaryKey"`
	OwnerID         string                  `gorm:"column:owner_id;type:uuid;not null;index:media_assets_owner_idx"`
	ConsentID       string                  `gorm:"column:consent_id;type:uuid;not null;index:media_assets_consent_idx"`
	Purpose         string                  `gorm:"column:purpose;type:text;not null;check:media_assets_purpose_check,purpose = 'avatar_source_preparation'"`
	Category        string                  `gorm:"column:category;type:text;not null;check:media_assets_category_check,category = 'person_photo'"`
	ContentType     string                  `gorm:"column:content_type;type:text;not null;check:media_assets_content_type_check,content_type = 'image/jpeg'"`
	ByteSize        int64                   `gorm:"column:byte_size;not null;check:media_assets_byte_size_check,byte_size BETWEEN 1 AND 12582912"`
	SHA256          string                  `gorm:"column:sha256;type:char(64);not null;check:media_assets_sha256_check,char_length(sha256) = 64 AND sha256 = lower(sha256)"`
	RawObjectKey    string                  `gorm:"column:raw_object_key;type:text;not null;uniqueIndex:media_assets_raw_key_unique"`
	ObjectVersionID string                  `gorm:"column:object_version_id;type:text"`
	Status          string                  `gorm:"column:status;type:text;not null;index:media_assets_status_idx;check:media_assets_status_check,status IN ('pending_upload','uploaded','checking','ready','rejected','deleting','deleted')"`
	StableReason    string                  `gorm:"column:stable_reason;type:text;not null;default:''"`
	PixelWidth      int                     `gorm:"column:pixel_width;not null;default:0;check:media_assets_pixel_width_check,pixel_width >= 0"`
	PixelHeight     int                     `gorm:"column:pixel_height;not null;default:0;check:media_assets_pixel_height_check,pixel_height >= 0"`
	UploadExpiresAt time.Time               `gorm:"column:upload_expires_at;type:timestamptz;not null;index:media_assets_upload_expiry_idx"`
	CreatedAt       time.Time               `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time               `gorm:"column:updated_at;type:timestamptz;not null"`
	SourceDeletedAt *time.Time              `gorm:"column:source_deleted_at;type:timestamptz;index:media_assets_source_cleanup_idx"`
	Derivations     []mediaDerivationRecord `gorm:"foreignKey:MediaID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	DeletionRequest *deletionRequestRecord  `gorm:"foreignKey:MediaID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (mediaAssetRecord) TableName() string { return "media_assets" }

type mediaDerivationRecord struct {
	ID              string    `gorm:"column:id;type:uuid;primaryKey"`
	MediaID         string    `gorm:"column:media_id;type:uuid;not null;index:media_derivations_media_idx"`
	ObjectKey       string    `gorm:"column:object_key;type:text;not null;uniqueIndex:media_derivations_object_unique"`
	ObjectVersionID string    `gorm:"column:object_version_id;type:text;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;type:timestamptz;not null"`
}

func (mediaDerivationRecord) TableName() string { return "media_derivations" }

type deletionRequestRecord struct {
	ID              string     `gorm:"column:id;type:uuid;primaryKey"`
	OwnerID         string     `gorm:"column:owner_id;type:uuid;not null;index:deletion_requests_owner_idx"`
	MediaID         string     `gorm:"column:media_id;type:uuid;not null;uniqueIndex:deletion_requests_media_unique"`
	Status          string     `gorm:"column:status;type:text;not null;index:deletion_requests_status_idx;check:deletion_requests_status_check,status IN ('pending','running','complete','failed')"`
	ReadRevokedAt   time.Time  `gorm:"column:read_revoked_at;type:timestamptz;not null"`
	CompletedAt     *time.Time `gorm:"column:completed_at;type:timestamptz"`
	BackupExpiresAt time.Time  `gorm:"column:backup_expires_at;type:timestamptz;not null"`
	StableError     string     `gorm:"column:stable_error;type:text;not null;default:''"`
	Attempts        int        `gorm:"column:attempts;not null;default:0;check:deletion_requests_attempts_check,attempts >= 0"`
	NextAttemptAt   time.Time  `gorm:"column:next_attempt_at;type:timestamptz;not null;index:deletion_requests_next_attempt_idx"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (deletionRequestRecord) TableName() string { return "deletion_requests" }

type outboxEventRecord struct {
	ID          string     `gorm:"column:id;type:uuid;primaryKey"`
	EventType   string     `gorm:"column:event_type;type:text;not null;index:outbox_events_pending_idx,priority:2;check:outbox_events_type_check,event_type IN ('media.uploaded','media.deletion_requested')"`
	AggregateID string     `gorm:"column:aggregate_id;type:uuid;not null"`
	Payload     []byte     `gorm:"column:payload;type:jsonb;not null"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	PublishedAt *time.Time `gorm:"column:published_at;type:timestamptz;index:outbox_events_pending_idx,priority:1"`
}

func (outboxEventRecord) TableName() string { return "outbox_events" }

type inboxReceiptRecord struct {
	EventID     string    `gorm:"column:event_id;type:uuid;primaryKey"`
	HandlerName string    `gorm:"column:handler_name;type:text;primaryKey"`
	ProcessedAt time.Time `gorm:"column:processed_at;type:timestamptz;not null"`
}

func (inboxReceiptRecord) TableName() string { return "inbox_receipts" }

func (r *MediaRepository) CreateConsent(ctx context.Context, ownerID string, input model.CreateConsentInput, at time.Time) (model.ConsentRecord, error) {
	var result consentRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("owner_id = ? AND purpose = ? AND category = ? AND processor = ? AND region = ? AND policy_version = ? AND training_allowed = false AND withdrawn_at IS NULL", ownerID, input.Purpose, input.Category, model.MediaProcessorThen, model.MediaRegionLocalDevelopment, input.PolicyVersion).First(&result).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result = consentRecord{ID: uuid.NewString(), OwnerID: ownerID, Purpose: input.Purpose, Category: input.Category, Processor: model.MediaProcessorThen, Region: model.MediaRegionLocalDevelopment, PolicyVersion: input.PolicyVersion, MaxRetentionHours: 24, TrainingAllowed: false, AgreedAt: at}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&result)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			return nil
		}
		return tx.Where("owner_id = ? AND purpose = ? AND category = ? AND processor = ? AND region = ? AND policy_version = ? AND training_allowed = false AND withdrawn_at IS NULL", ownerID, input.Purpose, input.Category, model.MediaProcessorThen, model.MediaRegionLocalDevelopment, input.PolicyVersion).First(&result).Error
	})
	if err != nil {
		return model.ConsentRecord{}, model.ErrMediaUnavailable
	}
	return consentFromRecord(result), nil
}

func (r *MediaRepository) GetConsent(ctx context.Context, ownerID, consentID string) (model.ConsentRecord, error) {
	var record consentRecord
	if err := r.database.WithContext(ctx).Where("id = ? AND owner_id = ?", consentID, ownerID).First(&record).Error; err != nil {
		return model.ConsentRecord{}, mediaLookupError(err)
	}
	return consentFromRecord(record), nil
}

func (r *MediaRepository) WithdrawConsent(ctx context.Context, ownerID, consentID string, at time.Time) (model.ConsentRecord, error) {
	var record consentRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", consentID, ownerID).First(&record).Error; err != nil {
			return err
		}
		if record.WithdrawnAt == nil {
			if err := tx.Model(&record).Update("withdrawn_at", at).Error; err != nil {
				return err
			}
			record.WithdrawnAt = &at
		}
		return nil
	})
	if err != nil {
		return model.ConsentRecord{}, mediaLookupError(err)
	}
	return consentFromRecord(record), nil
}

func (r *MediaRepository) IsSelfAdultConfirmed(ctx context.Context, ownerID string) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&selfAdultDeclarationRecord{}).Where("user_id = ? AND policy_version = ? AND withdrawn_at IS NULL", ownerID, model.CurrentSelfAdultPolicyVersion).Count(&count).Error
	if err != nil {
		return false, model.ErrMediaUnavailable
	}
	return count == 1, nil
}

func (r *MediaRepository) CreateMedia(ctx context.Context, ownerID string, input model.CreateMediaUploadInput, at time.Time) (model.MediaAsset, error) {
	mediaID := uuid.NewString()
	record := mediaAssetRecord{ID: mediaID, OwnerID: ownerID, ConsentID: input.ConsentID, Purpose: input.Purpose, Category: model.MediaCategoryPersonPhoto, ContentType: input.ContentType, ByteSize: input.ByteSize, SHA256: input.SHA256, RawObjectKey: "owners/" + ownerID + "/media/" + mediaID + "/source.jpg", Status: string(model.MediaPendingUpload), UploadExpiresAt: at.Add(model.UploadIntentLifetime), CreatedAt: at, UpdatedAt: at}
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", ownerID).First(&user).Error; err != nil {
			return err
		}
		var consent consentRecord
		if err := tx.Where("id = ? AND owner_id = ? AND purpose = ? AND category = ? AND policy_version = ? AND withdrawn_at IS NULL", input.ConsentID, ownerID, model.MediaPurposeAvatarSourcePreparation, model.MediaCategoryPersonPhoto, model.CurrentMediaPolicyVersion).First(&consent).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.MediaAsset{}, model.ErrConsentRequired
	}
	if err != nil {
		return model.MediaAsset{}, model.ErrMediaUnavailable
	}
	return mediaFromRecord(record), nil
}

func (r *MediaRepository) GetMedia(ctx context.Context, ownerID, mediaID string) (model.MediaAsset, error) {
	var record mediaAssetRecord
	if err := r.database.WithContext(ctx).Where("id = ? AND owner_id = ?", mediaID, ownerID).First(&record).Error; err != nil {
		return model.MediaAsset{}, mediaLookupError(err)
	}
	return mediaFromRecord(record), nil
}

func (r *MediaRepository) CompleteMedia(ctx context.Context, ownerID, mediaID, versionID string, _ model.ObjectFact, at time.Time) (model.MediaAsset, error) {
	var record mediaAssetRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", mediaID, ownerID).First(&record).Error; err != nil {
			return err
		}
		if record.Status == string(model.MediaUploaded) && record.ObjectVersionID == versionID {
			return nil
		}
		if record.Status != string(model.MediaPendingUpload) || record.ObjectVersionID != "" || !at.Before(record.UploadExpiresAt) {
			return model.ErrMediaConflict
		}
		record.Status, record.ObjectVersionID, record.UpdatedAt = string(model.MediaUploaded), versionID, at
		if err := tx.Model(&record).Updates(map[string]any{"status": record.Status, "object_version_id": versionID, "updated_at": at}).Error; err != nil {
			return err
		}
		return tx.Create(&outboxEventRecord{ID: uuid.NewString(), EventType: "media.uploaded", AggregateID: mediaID, Payload: []byte(`{"media_id":"` + mediaID + `"}`), CreatedAt: at}).Error
	})
	if err != nil {
		if errors.Is(err, model.ErrMediaConflict) {
			return model.MediaAsset{}, err
		}
		return model.MediaAsset{}, mediaLookupError(err)
	}
	return mediaFromRecord(record), nil
}

func (r *MediaRepository) DeleteMedia(ctx context.Context, ownerID, mediaID string, at time.Time) (model.DeletionRequest, error) {
	var request deletionRequestRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var media mediaAssetRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", mediaID, ownerID).First(&media).Error; err != nil {
			return err
		}
		if err := tx.Where("media_id = ? AND owner_id = ?", mediaID, ownerID).First(&request).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if media.Status == string(model.MediaDeleted) {
			return model.ErrMediaConflict
		}
		request = deletionRequestRecord{ID: uuid.NewString(), OwnerID: ownerID, MediaID: mediaID, Status: string(model.DeletionPending), ReadRevokedAt: at, BackupExpiresAt: at, NextAttemptAt: at, CreatedAt: at, UpdatedAt: at}
		if err := tx.Model(&media).Updates(map[string]any{"status": string(model.MediaDeleting), "updated_at": at}).Error; err != nil {
			return err
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		return tx.Create(&outboxEventRecord{ID: uuid.NewString(), EventType: "media.deletion_requested", AggregateID: mediaID, Payload: []byte(`{"media_id":"` + mediaID + `"}`), CreatedAt: at}).Error
	})
	if err != nil {
		if errors.Is(err, model.ErrMediaConflict) {
			return model.DeletionRequest{}, err
		}
		return model.DeletionRequest{}, mediaLookupError(err)
	}
	return deletionFromRecord(request), nil
}

func (r *MediaRepository) GetDeletionRequest(ctx context.Context, ownerID, requestID string) (model.DeletionRequest, error) {
	var record deletionRequestRecord
	if err := r.database.WithContext(ctx).Where("id = ? AND owner_id = ?", requestID, ownerID).First(&record).Error; err != nil {
		return model.DeletionRequest{}, mediaLookupError(err)
	}
	return deletionFromRecord(record), nil
}

func (r *MediaRepository) PendingOutbox(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, model.ErrMediaUnavailable
	}
	var records []outboxEventRecord
	if err := r.database.WithContext(ctx).Where("published_at IS NULL").Order("created_at ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, model.ErrMediaUnavailable
	}
	events := make([]model.OutboxEvent, 0, len(records))
	for _, record := range records {
		events = append(events, model.OutboxEvent{ID: record.ID, EventType: record.EventType, AggregateID: record.AggregateID, CreatedAt: record.CreatedAt})
	}
	return events, nil
}

func (r *MediaRepository) MarkOutboxPublished(ctx context.Context, eventID string, at time.Time) error {
	result := r.database.WithContext(ctx).Model(&outboxEventRecord{}).Where("id = ? AND published_at IS NULL", eventID).Update("published_at", at)
	if result.Error != nil {
		return model.ErrMediaUnavailable
	}
	return nil
}

func (r *MediaRepository) BeginMediaCheck(ctx context.Context, mediaID string, at time.Time) (model.MediaAsset, bool, error) {
	var record mediaAssetRecord
	process := false
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", mediaID).First(&record).Error; err != nil {
			return err
		}
		switch model.MediaStatus(record.Status) {
		case model.MediaUploaded:
			var consent consentRecord
			if err := tx.Where("id = ?", record.ConsentID).First(&consent).Error; err != nil {
				return err
			}
			if consent.WithdrawnAt != nil {
				record.Status, record.StableReason, record.UpdatedAt = string(model.MediaRejected), "consent_withdrawn", at
				return tx.Model(&record).Updates(map[string]any{"status": record.Status, "stable_reason": record.StableReason, "updated_at": at}).Error
			}
			record.Status, record.UpdatedAt, process = string(model.MediaChecking), at, true
			return tx.Model(&record).Updates(map[string]any{"status": record.Status, "updated_at": at}).Error
		case model.MediaChecking:
			process = true
			return nil
		default:
			return nil
		}
	})
	if err != nil {
		return model.MediaAsset{}, false, mediaLookupError(err)
	}
	return mediaFromRecord(record), process, nil
}

func (r *MediaRepository) CompleteMediaCheck(ctx context.Context, eventID, mediaID string, derivation *model.MediaDerivation, width, height int, status model.MediaStatus, reason string, at time.Time) error {
	if status != model.MediaReady && status != model.MediaRejected {
		return model.ErrMediaConflict
	}
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt inboxReceiptRecord
		if err := tx.Where("event_id = ? AND handler_name = ?", eventID, "media-check").First(&receipt).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result := tx.Model(&mediaAssetRecord{}).Where("id = ? AND status = ?", mediaID, string(model.MediaChecking)).Updates(map[string]any{"status": string(status), "stable_reason": reason, "pixel_width": width, "pixel_height": height, "updated_at": at})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrMediaConflict
		}
		if derivation != nil {
			if err := tx.Create(&mediaDerivationRecord{ID: uuid.NewString(), MediaID: mediaID, ObjectKey: derivation.ObjectKey, ObjectVersionID: derivation.ObjectVersionID, CreatedAt: at}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&inboxReceiptRecord{EventID: eventID, HandlerName: "media-check", ProcessedAt: at}).Error
	})
	if err != nil {
		if errors.Is(err, model.ErrMediaConflict) {
			return err
		}
		return model.ErrMediaUnavailable
	}
	return nil
}

func (r *MediaRepository) BeginDeletion(ctx context.Context, mediaID string, at time.Time) (model.MediaAsset, []model.MediaDerivation, model.DeletionRequest, bool, error) {
	var media mediaAssetRecord
	var request deletionRequestRecord
	var records []mediaDerivationRecord
	process := false
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", mediaID).First(&media).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("media_id = ?", mediaID).First(&request).Error; err != nil {
			return err
		}
		if request.Status == string(model.DeletionComplete) {
			return nil
		}
		process = true
		request.Status, request.Attempts, request.UpdatedAt = string(model.DeletionRunning), request.Attempts+1, at
		if err := tx.Model(&request).Updates(map[string]any{"status": request.Status, "attempts": request.Attempts, "updated_at": at}).Error; err != nil {
			return err
		}
		return tx.Where("media_id = ?", mediaID).Find(&records).Error
	})
	if err != nil {
		return model.MediaAsset{}, nil, model.DeletionRequest{}, false, mediaLookupError(err)
	}
	derivations := make([]model.MediaDerivation, 0, len(records))
	for _, record := range records {
		derivations = append(derivations, model.MediaDerivation{ObjectKey: record.ObjectKey, ObjectVersionID: record.ObjectVersionID})
	}
	return mediaFromRecord(media), derivations, deletionFromRecord(request), process, nil
}

func (r *MediaRepository) CompleteDeletion(ctx context.Context, eventID, mediaID string, at time.Time) error {
	return r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt inboxReceiptRecord
		if err := tx.Where("event_id = ? AND handler_name = ?", eventID, "media-delete").First(&receipt).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Where("media_id = ?", mediaID).Delete(&mediaDerivationRecord{}).Error; err != nil {
			return err
		}
		result := tx.Model(&mediaAssetRecord{}).Where("id = ? AND status = ?", mediaID, string(model.MediaDeleting)).Updates(map[string]any{"status": string(model.MediaDeleted), "raw_object_key": "deleted/" + mediaID, "object_version_id": "", "sha256": "0000000000000000000000000000000000000000000000000000000000000000", "stable_reason": "deleted", "updated_at": at})
		if result.Error != nil || result.RowsAffected != 1 {
			return model.ErrMediaConflict
		}
		result = tx.Model(&deletionRequestRecord{}).Where("media_id = ?", mediaID).Updates(map[string]any{"status": string(model.DeletionComplete), "completed_at": at, "stable_error": "", "updated_at": at})
		if result.Error != nil || result.RowsAffected != 1 {
			return model.ErrMediaConflict
		}
		return tx.Create(&inboxReceiptRecord{EventID: eventID, HandlerName: "media-delete", ProcessedAt: at}).Error
	})
}

func (r *MediaRepository) SourceCleanupCandidates(ctx context.Context, at time.Time, limit int) ([]model.MediaAsset, error) {
	if limit < 1 || limit > 100 {
		return nil, model.ErrMediaUnavailable
	}
	var records []mediaAssetRecord
	err := r.database.WithContext(ctx).
		Where("source_deleted_at IS NULL AND ((status = ? AND created_at <= ?) OR status IN ? OR (status = ? AND updated_at <= ?))", string(model.MediaPendingUpload), at.Add(-model.UnfinishedUploadLifetime), []string{string(model.MediaReady), string(model.MediaRejected)}, string(model.MediaDeleted), at.Add(-time.Minute)).
		Order("updated_at ASC").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, model.ErrMediaUnavailable
	}
	result := make([]model.MediaAsset, 0, len(records))
	for _, record := range records {
		result = append(result, mediaFromRecord(record))
	}
	return result, nil
}

func (r *MediaRepository) MarkSourceDeleted(ctx context.Context, mediaID string, at time.Time) error {
	updates := map[string]any{"source_deleted_at": at, "updated_at": at}
	var record mediaAssetRecord
	if err := r.database.WithContext(ctx).Where("id = ?", mediaID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return mediaLookupError(err)
	}
	if model.MediaStatus(record.Status) == model.MediaPendingUpload {
		updates["status"] = string(model.MediaRejected)
		updates["stable_reason"] = "upload_expired"
	}
	if err := r.database.WithContext(ctx).Model(&mediaAssetRecord{}).Where("id = ? AND status = ? AND source_deleted_at IS NULL", mediaID, record.Status).Updates(updates).Error; err != nil {
		return model.ErrMediaUnavailable
	}
	return nil
}

func mediaLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrMediaNotFound
	}
	return model.ErrMediaUnavailable
}

func consentFromRecord(record consentRecord) model.ConsentRecord {
	status := model.ConsentActive
	if record.WithdrawnAt != nil {
		status = model.ConsentWithdrawn
	}
	return model.ConsentRecord{ID: record.ID, OwnerID: record.OwnerID, Purpose: record.Purpose, Category: record.Category, Processor: record.Processor, Region: record.Region, PolicyVersion: record.PolicyVersion, MaxRetentionHours: record.MaxRetentionHours, TrainingAllowed: record.TrainingAllowed, Status: status, AgreedAt: record.AgreedAt, WithdrawnAt: record.WithdrawnAt}
}

func mediaFromRecord(record mediaAssetRecord) model.MediaAsset {
	return model.MediaAsset{ID: record.ID, OwnerID: record.OwnerID, ConsentID: record.ConsentID, Purpose: record.Purpose, Category: record.Category, ContentType: record.ContentType, ByteSize: record.ByteSize, SHA256: record.SHA256, RawObjectKey: record.RawObjectKey, ObjectVersionID: record.ObjectVersionID, Status: model.MediaStatus(record.Status), StableReason: record.StableReason, PixelWidth: record.PixelWidth, PixelHeight: record.PixelHeight, UploadExpiresAt: record.UploadExpiresAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, SourceDeletedAt: record.SourceDeletedAt}
}

func deletionFromRecord(record deletionRequestRecord) model.DeletionRequest {
	return model.DeletionRequest{ID: record.ID, OwnerID: record.OwnerID, MediaID: record.MediaID, Status: model.DeletionStatus(record.Status), ReadRevokedAt: record.ReadRevokedAt, CompletedAt: record.CompletedAt, BackupExpiresAt: record.BackupExpiresAt, StableError: record.StableError, Attempts: record.Attempts, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
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
	ConsentID       *string                 `gorm:"column:consent_id;type:uuid;index:media_assets_consent_idx"`
	Purpose         string                  `gorm:"column:purpose;type:text;not null;check:media_assets_purpose_check,purpose IN ('avatar_source_preparation','diary_image');check:media_assets_purpose_category_check,(purpose = 'avatar_source_preparation' AND category = 'person_photo' AND consent_id IS NOT NULL) OR (purpose = 'diary_image' AND category = 'ordinary_image' AND consent_id IS NULL)"`
	Category        string                  `gorm:"column:category;type:text;not null;check:media_assets_category_check,category IN ('person_photo','ordinary_image')"`
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

func (r *MediaRepository) CreateConsent(ctx context.Context, ownerID string, input domain.CreateConsentInput, at time.Time) (domain.ConsentRecord, error) {
	var result consentRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("owner_id = ? AND purpose = ? AND category = ? AND processor = ? AND region = ? AND policy_version = ? AND training_allowed = false AND withdrawn_at IS NULL", ownerID, input.Purpose, input.Category, domain.MediaProcessorThen, domain.MediaRegionLocalDevelopment, input.PolicyVersion).First(&result).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result = consentRecord{ID: uuid.NewString(), OwnerID: ownerID, Purpose: input.Purpose, Category: input.Category, Processor: domain.MediaProcessorThen, Region: domain.MediaRegionLocalDevelopment, PolicyVersion: input.PolicyVersion, MaxRetentionHours: 24, TrainingAllowed: false, AgreedAt: at}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&result)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			return nil
		}
		return tx.Where("owner_id = ? AND purpose = ? AND category = ? AND processor = ? AND region = ? AND policy_version = ? AND training_allowed = false AND withdrawn_at IS NULL", ownerID, input.Purpose, input.Category, domain.MediaProcessorThen, domain.MediaRegionLocalDevelopment, input.PolicyVersion).First(&result).Error
	})
	if err != nil {
		return domain.ConsentRecord{}, domain.ErrMediaUnavailable
	}
	return consentFromRecord(result), nil
}

func (r *MediaRepository) GetConsent(ctx context.Context, ownerID, consentID string) (domain.ConsentRecord, error) {
	var record consentRecord
	if err := r.database.WithContext(ctx).Where("id = ? AND owner_id = ?", consentID, ownerID).First(&record).Error; err != nil {
		return domain.ConsentRecord{}, mediaLookupError(err)
	}
	return consentFromRecord(record), nil
}

func (r *MediaRepository) WithdrawConsent(ctx context.Context, ownerID, consentID string, at time.Time) (domain.ConsentRecord, error) {
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
		return domain.ConsentRecord{}, mediaLookupError(err)
	}
	return consentFromRecord(record), nil
}

func (r *MediaRepository) IsSelfAdultConfirmed(ctx context.Context, ownerID string) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&selfAdultDeclarationRecord{}).Where("user_id = ? AND policy_version = ? AND withdrawn_at IS NULL", ownerID, domain.CurrentSelfAdultPolicyVersion).Count(&count).Error
	if err != nil {
		return false, domain.ErrMediaUnavailable
	}
	return count == 1, nil
}

func (r *MediaRepository) CreateMedia(ctx context.Context, ownerID string, input domain.CreateMediaUploadInput, at time.Time) (domain.MediaAsset, error) {
	mediaID := uuid.NewString()
	category := domain.MediaCategoryOrdinaryImage
	var consentID *string
	if input.Purpose == domain.MediaPurposeAvatarSourcePreparation {
		category = domain.MediaCategoryPersonPhoto
		consentID = &input.ConsentID
	}
	record := mediaAssetRecord{ID: mediaID, OwnerID: ownerID, ConsentID: consentID, Purpose: input.Purpose, Category: category, ContentType: input.ContentType, ByteSize: input.ByteSize, SHA256: input.SHA256, RawObjectKey: "owners/" + ownerID + "/media/" + mediaID + "/source.jpg", Status: string(domain.MediaPendingUpload), UploadExpiresAt: at.Add(domain.UploadIntentLifetime), CreatedAt: at, UpdatedAt: at}
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", ownerID).First(&user).Error; err != nil {
			return err
		}
		if input.Purpose == domain.MediaPurposeAvatarSourcePreparation {
			var consent consentRecord
			if err := tx.Where("id = ? AND owner_id = ? AND purpose = ? AND category = ? AND policy_version = ? AND withdrawn_at IS NULL", input.ConsentID, ownerID, domain.MediaPurposeAvatarSourcePreparation, domain.MediaCategoryPersonPhoto, domain.CurrentMediaPolicyVersion).First(&consent).Error; err != nil {
				return err
			}
		}
		return tx.Create(&record).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.MediaAsset{}, domain.ErrConsentRequired
	}
	if err != nil {
		return domain.MediaAsset{}, domain.ErrMediaUnavailable
	}
	return mediaFromRecord(record), nil
}

func (r *MediaRepository) GetMedia(ctx context.Context, ownerID, mediaID string) (domain.MediaAsset, error) {
	var record mediaAssetRecord
	if err := r.database.WithContext(ctx).Where("id = ? AND owner_id = ?", mediaID, ownerID).First(&record).Error; err != nil {
		return domain.MediaAsset{}, mediaLookupError(err)
	}
	return mediaFromRecord(record), nil
}

func (r *MediaRepository) CompleteMedia(ctx context.Context, ownerID, mediaID, versionID string, _ domain.ObjectFact, at time.Time) (domain.MediaAsset, error) {
	var record mediaAssetRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", mediaID, ownerID).First(&record).Error; err != nil {
			return err
		}
		if record.Status == string(domain.MediaUploaded) && record.ObjectVersionID == versionID {
			return nil
		}
		if record.Status != string(domain.MediaPendingUpload) || record.ObjectVersionID != "" || !at.Before(record.UploadExpiresAt) {
			return domain.ErrMediaConflict
		}
		record.Status, record.ObjectVersionID, record.UpdatedAt = string(domain.MediaUploaded), versionID, at
		if err := tx.Model(&record).Updates(map[string]any{"status": record.Status, "object_version_id": versionID, "updated_at": at}).Error; err != nil {
			return err
		}
		return tx.Create(&outboxEventRecord{ID: uuid.NewString(), EventType: "media.uploaded", AggregateID: mediaID, Payload: []byte(`{"media_id":"` + mediaID + `"}`), CreatedAt: at}).Error
	})
	if err != nil {
		if errors.Is(err, domain.ErrMediaConflict) {
			return domain.MediaAsset{}, err
		}
		return domain.MediaAsset{}, mediaLookupError(err)
	}
	return mediaFromRecord(record), nil
}

func (r *MediaRepository) DeleteMedia(ctx context.Context, ownerID, mediaID string, at time.Time) (domain.DeletionRequest, error) {
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
		if media.Status == string(domain.MediaDeleted) {
			return domain.ErrMediaConflict
		}
		if err := unlinkMediaFromDiaries(tx, ownerID, mediaID, at); err != nil {
			return err
		}
		request = deletionRequestRecord{ID: uuid.NewString(), OwnerID: ownerID, MediaID: mediaID, Status: string(domain.DeletionPending), ReadRevokedAt: at, BackupExpiresAt: at, NextAttemptAt: at, CreatedAt: at, UpdatedAt: at}
		if err := tx.Model(&media).Updates(map[string]any{"status": string(domain.MediaDeleting), "updated_at": at}).Error; err != nil {
			return err
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		return tx.Create(&outboxEventRecord{ID: uuid.NewString(), EventType: "media.deletion_requested", AggregateID: mediaID, Payload: []byte(`{"media_id":"` + mediaID + `"}`), CreatedAt: at}).Error
	})
	if err != nil {
		if errors.Is(err, domain.ErrMediaConflict) {
			return domain.DeletionRequest{}, err
		}
		return domain.DeletionRequest{}, mediaLookupError(err)
	}
	return deletionFromRecord(request), nil
}

func unlinkMediaFromDiaries(tx *gorm.DB, ownerID, mediaID string, at time.Time) error {
	var links []diaryEntryMediaRecord
	if err := tx.Where("owner_id = ? AND media_id = ?", ownerID, mediaID).Find(&links).Error; err != nil {
		return err
	}
	for _, link := range links {
		var entry diaryEntryRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, link.EntryID).First(&entry).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&diaryEntryMediaRecord{}).Where("owner_id = ? AND entry_id = ?", ownerID, link.EntryID).Count(&count).Error; err != nil {
			return err
		}
		if entry.Body == nil && count <= 1 {
			return domain.ErrMediaConflict
		}
		if err := tx.Where("owner_id = ? AND entry_id = ? AND media_id = ?", ownerID, link.EntryID, mediaID).Delete(&diaryEntryMediaRecord{}).Error; err != nil {
			return err
		}
		var remaining []diaryEntryMediaRecord
		if err := tx.Where("owner_id = ? AND entry_id = ?", ownerID, link.EntryID).Order("ordinal ASC").Find(&remaining).Error; err != nil {
			return err
		}
		for ordinal, record := range remaining {
			if record.Ordinal == ordinal {
				continue
			}
			if err := tx.Model(&diaryEntryMediaRecord{}).Where("owner_id = ? AND entry_id = ? AND ordinal = ?", ownerID, link.EntryID, record.Ordinal).Update("ordinal", ordinal).Error; err != nil {
				return err
			}
		}
		updatedAt := at
		if updatedAt.Before(entry.UpdatedAt) {
			updatedAt = entry.UpdatedAt
		}
		if err := tx.Model(&diaryEntryRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, link.EntryID, entry.Revision).Updates(map[string]any{"revision": entry.Revision + 1, "updated_at": updatedAt}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *MediaRepository) GetDeletionRequest(ctx context.Context, ownerID, requestID string) (domain.DeletionRequest, error) {
	var record deletionRequestRecord
	if err := r.database.WithContext(ctx).Where("id = ? AND owner_id = ?", requestID, ownerID).First(&record).Error; err != nil {
		return domain.DeletionRequest{}, mediaLookupError(err)
	}
	return deletionFromRecord(record), nil
}

func (r *MediaRepository) PendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, domain.ErrMediaUnavailable
	}
	var records []outboxEventRecord
	if err := r.database.WithContext(ctx).Where("published_at IS NULL").Order("created_at ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, domain.ErrMediaUnavailable
	}
	events := make([]domain.OutboxEvent, 0, len(records))
	for _, record := range records {
		events = append(events, domain.OutboxEvent{ID: record.ID, EventType: record.EventType, AggregateID: record.AggregateID, CreatedAt: record.CreatedAt})
	}
	return events, nil
}

func (r *MediaRepository) MarkOutboxPublished(ctx context.Context, eventID string, at time.Time) error {
	result := r.database.WithContext(ctx).Model(&outboxEventRecord{}).Where("id = ? AND published_at IS NULL", eventID).Update("published_at", at)
	if result.Error != nil {
		return domain.ErrMediaUnavailable
	}
	return nil
}

func (r *MediaRepository) BeginMediaCheck(ctx context.Context, mediaID string, at time.Time) (domain.MediaAsset, bool, error) {
	var record mediaAssetRecord
	process := false
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", mediaID).First(&record).Error; err != nil {
			return err
		}
		switch domain.MediaStatus(record.Status) {
		case domain.MediaUploaded:
			if record.Purpose == domain.MediaPurposeAvatarSourcePreparation {
				if record.ConsentID == nil {
					return domain.ErrMediaConflict
				}
				var consent consentRecord
				if err := tx.Where("id = ?", *record.ConsentID).First(&consent).Error; err != nil {
					return err
				}
				if consent.WithdrawnAt != nil {
					record.Status, record.StableReason, record.UpdatedAt = string(domain.MediaRejected), "consent_withdrawn", at
					return tx.Model(&record).Updates(map[string]any{"status": record.Status, "stable_reason": record.StableReason, "updated_at": at}).Error
				}
			}
			record.Status, record.UpdatedAt, process = string(domain.MediaChecking), at, true
			return tx.Model(&record).Updates(map[string]any{"status": record.Status, "updated_at": at}).Error
		case domain.MediaChecking:
			process = true
			return nil
		default:
			return nil
		}
	})
	if err != nil {
		return domain.MediaAsset{}, false, mediaLookupError(err)
	}
	return mediaFromRecord(record), process, nil
}

func (r *MediaRepository) CompleteMediaCheck(ctx context.Context, eventID, mediaID string, derivation *domain.MediaDerivation, width, height int, status domain.MediaStatus, reason string, at time.Time) error {
	if status != domain.MediaReady && status != domain.MediaRejected {
		return domain.ErrMediaConflict
	}
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt inboxReceiptRecord
		if err := tx.Where("event_id = ? AND handler_name = ?", eventID, "media-check").First(&receipt).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result := tx.Model(&mediaAssetRecord{}).Where("id = ? AND status = ?", mediaID, string(domain.MediaChecking)).Updates(map[string]any{"status": string(status), "stable_reason": reason, "pixel_width": width, "pixel_height": height, "updated_at": at})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrMediaConflict
		}
		if derivation != nil {
			if err := tx.Create(&mediaDerivationRecord{ID: uuid.NewString(), MediaID: mediaID, ObjectKey: derivation.ObjectKey, ObjectVersionID: derivation.ObjectVersionID, CreatedAt: at}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&inboxReceiptRecord{EventID: eventID, HandlerName: "media-check", ProcessedAt: at}).Error
	})
	if err != nil {
		if errors.Is(err, domain.ErrMediaConflict) {
			return err
		}
		return domain.ErrMediaUnavailable
	}
	return nil
}

func (r *MediaRepository) BeginDeletion(ctx context.Context, mediaID string, at time.Time) (domain.MediaAsset, []domain.MediaDerivation, domain.DeletionRequest, bool, error) {
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
		if request.Status == string(domain.DeletionComplete) {
			return nil
		}
		process = true
		request.Status, request.Attempts, request.UpdatedAt = string(domain.DeletionRunning), request.Attempts+1, at
		if err := tx.Model(&request).Updates(map[string]any{"status": request.Status, "attempts": request.Attempts, "updated_at": at}).Error; err != nil {
			return err
		}
		return tx.Where("media_id = ?", mediaID).Find(&records).Error
	})
	if err != nil {
		return domain.MediaAsset{}, nil, domain.DeletionRequest{}, false, mediaLookupError(err)
	}
	derivations := make([]domain.MediaDerivation, 0, len(records))
	for _, record := range records {
		derivations = append(derivations, domain.MediaDerivation{ObjectKey: record.ObjectKey, ObjectVersionID: record.ObjectVersionID})
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
		result := tx.Model(&mediaAssetRecord{}).Where("id = ? AND status = ?", mediaID, string(domain.MediaDeleting)).Updates(map[string]any{"status": string(domain.MediaDeleted), "raw_object_key": "deleted/" + mediaID, "object_version_id": "", "sha256": "0000000000000000000000000000000000000000000000000000000000000000", "stable_reason": "deleted", "updated_at": at})
		if result.Error != nil || result.RowsAffected != 1 {
			return domain.ErrMediaConflict
		}
		result = tx.Model(&deletionRequestRecord{}).Where("media_id = ?", mediaID).Updates(map[string]any{"status": string(domain.DeletionComplete), "completed_at": at, "stable_error": "", "updated_at": at})
		if result.Error != nil || result.RowsAffected != 1 {
			return domain.ErrMediaConflict
		}
		return tx.Create(&inboxReceiptRecord{EventID: eventID, HandlerName: "media-delete", ProcessedAt: at}).Error
	})
}

func (r *MediaRepository) SourceCleanupCandidates(ctx context.Context, at time.Time, limit int) ([]domain.MediaAsset, error) {
	if limit < 1 || limit > 100 {
		return nil, domain.ErrMediaUnavailable
	}
	var records []mediaAssetRecord
	err := r.database.WithContext(ctx).
		Where("source_deleted_at IS NULL AND ((status = ? AND created_at <= ?) OR status IN ? OR (status = ? AND updated_at <= ?))", string(domain.MediaPendingUpload), at.Add(-domain.UnfinishedUploadLifetime), []string{string(domain.MediaReady), string(domain.MediaRejected)}, string(domain.MediaDeleted), at.Add(-time.Minute)).
		Order("updated_at ASC").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, domain.ErrMediaUnavailable
	}
	result := make([]domain.MediaAsset, 0, len(records))
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
	if domain.MediaStatus(record.Status) == domain.MediaPendingUpload {
		updates["status"] = string(domain.MediaRejected)
		updates["stable_reason"] = "upload_expired"
	}
	if err := r.database.WithContext(ctx).Model(&mediaAssetRecord{}).Where("id = ? AND status = ? AND source_deleted_at IS NULL", mediaID, record.Status).Updates(updates).Error; err != nil {
		return domain.ErrMediaUnavailable
	}
	return nil
}

func mediaLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrMediaNotFound
	}
	return domain.ErrMediaUnavailable
}

func consentFromRecord(record consentRecord) domain.ConsentRecord {
	status := domain.ConsentActive
	if record.WithdrawnAt != nil {
		status = domain.ConsentWithdrawn
	}
	return domain.ConsentRecord{ID: record.ID, OwnerID: record.OwnerID, Purpose: record.Purpose, Category: record.Category, Processor: record.Processor, Region: record.Region, PolicyVersion: record.PolicyVersion, MaxRetentionHours: record.MaxRetentionHours, TrainingAllowed: record.TrainingAllowed, Status: status, AgreedAt: record.AgreedAt, WithdrawnAt: record.WithdrawnAt}
}

func mediaFromRecord(record mediaAssetRecord) domain.MediaAsset {
	consentID := ""
	if record.ConsentID != nil {
		consentID = *record.ConsentID
	}
	return domain.MediaAsset{ID: record.ID, OwnerID: record.OwnerID, ConsentID: consentID, Purpose: record.Purpose, Category: record.Category, ContentType: record.ContentType, ByteSize: record.ByteSize, SHA256: record.SHA256, RawObjectKey: record.RawObjectKey, ObjectVersionID: record.ObjectVersionID, Status: domain.MediaStatus(record.Status), StableReason: record.StableReason, PixelWidth: record.PixelWidth, PixelHeight: record.PixelHeight, UploadExpiresAt: record.UploadExpiresAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, SourceDeletedAt: record.SourceDeletedAt}
}

func deletionFromRecord(record deletionRequestRecord) domain.DeletionRequest {
	return domain.DeletionRequest{ID: record.ID, OwnerID: record.OwnerID, MediaID: record.MediaID, Status: domain.DeletionStatus(record.Status), ReadRevokedAt: record.ReadRevokedAt, CompletedAt: record.CompletedAt, BackupExpiresAt: record.BackupExpiresAt, StableError: record.StableError, Attempts: record.Attempts, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

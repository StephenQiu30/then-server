package postgres

import (
	"context"

	"gorm.io/gorm"
)

// Migrate keeps the development database schema aligned with the GORM records.
func Migrate(ctx context.Context, database *gorm.DB) error {
	if err := database.WithContext(ctx).AutoMigrate(
		&userRecord{},
		&userProfileRecord{},
		&wardrobeItemRecord{},
		&wardrobeItemDeletionRecord{},
		&syncPositionRecord{},
		&syncChangeRecord{},
		&outfitPlanRecord{},
		&outfitPlanItemRecord{},
		&outfitPlanDeletionRecord{},
		&wearEventRecord{},
		&wearEventItemRecord{},
		&wearFeedbackRecord{},
		&wearFeedbackMutationRecord{},
		&wearEventDeletionRecord{},
		&credentialRecord{},
		&sessionRecord{},
		&mailChallengeRecord{},
		&accountDeletionRequestRecord{},
		&selfAdultDeclarationRecord{},
		&consentRecord{},
		&mediaAssetRecord{},
		&mediaDerivationRecord{},
		&deletionRequestRecord{},
		&outboxEventRecord{},
		&inboxReceiptRecord{},
		&diaryEntryRecord{},
		&diaryEntryMediaRecord{},
		&diaryEntryDeletionRecord{},
		&postRecord{},
		&postRevisionRecord{},
		&postRevisionMediaRecord{},
		&postRevisionTagRecord{},
		&postLikeRecord{},
		&postBookmarkRecord{},
		&userFollowRecord{},
		&userBlockRecord{},
		&commentRecord{},
		&contentReportRecord{},
		&moderationActionRecord{},
		&moderationAppealRecord{},
		&notificationRecord{},
		&dataExportRecord{},
		&generationJobRecord{},
		&generationQuotaReservationRecord{},
		&generationOutputRecord{},
		&generationCleanupRequestRecord{},
		&generationSubmissionReconciliationRecord{},
	); err != nil {
		return err
	}
	// AutoMigrate does not alter an existing CHECK constraint when a new
	// provider-neutral outbox event type or cleanup scope is added.
	if database.Dialector.Name() == "postgres" {
		for _, statement := range []string{
			"ALTER TABLE consent_records DROP CONSTRAINT IF EXISTS consent_records_purpose_check",
			"ALTER TABLE consent_records ADD CONSTRAINT consent_records_purpose_check CHECK (purpose IN ('avatar_source_preparation','generation_input'))",
			"ALTER TABLE consent_records DROP CONSTRAINT IF EXISTS consent_records_category_check",
			"ALTER TABLE consent_records ADD CONSTRAINT consent_records_category_check CHECK (category IN ('person_photo','ordinary_image'))",
			"ALTER TABLE consent_records DROP CONSTRAINT IF EXISTS consent_records_policy_check",
			"ALTER TABLE consent_records ADD CONSTRAINT consent_records_policy_check CHECK (policy_version IN ('person-photo-v1','generation-input-v1'))",
			"ALTER TABLE consent_records DROP CONSTRAINT IF EXISTS consent_records_purpose_policy_check",
			"ALTER TABLE consent_records ADD CONSTRAINT consent_records_purpose_policy_check CHECK ((purpose = 'avatar_source_preparation' AND category = 'person_photo' AND policy_version = 'person-photo-v1') OR (purpose = 'generation_input' AND category IN ('person_photo','ordinary_image') AND policy_version = 'generation-input-v1'))",
			"ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_purpose_check",
			"ALTER TABLE media_assets ADD CONSTRAINT media_assets_purpose_check CHECK (purpose IN ('avatar_source_preparation','generation_input','diary_image','community_publish','profile_avatar'))",
			"ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_purpose_category_check",
			"ALTER TABLE media_assets ADD CONSTRAINT media_assets_purpose_category_check CHECK ((purpose = 'avatar_source_preparation' AND category = 'person_photo' AND consent_id IS NOT NULL) OR (purpose = 'generation_input' AND category IN ('person_photo','ordinary_image') AND consent_id IS NOT NULL) OR (purpose IN ('diary_image','community_publish','profile_avatar') AND category = 'ordinary_image' AND consent_id IS NULL))",
		} {
			if err := database.WithContext(ctx).Exec(statement).Error; err != nil {
				return err
			}
		}
		if err := database.WithContext(ctx).Exec("ALTER TABLE outbox_events DROP CONSTRAINT IF EXISTS outbox_events_type_check").Error; err != nil {
			return err
		}
		if err := database.WithContext(ctx).Exec("ALTER TABLE outbox_events ADD CONSTRAINT outbox_events_type_check CHECK (event_type IN ('media.uploaded','media.deletion_requested','community.notification_requested','generation.task_requested'))").Error; err != nil {
			return err
		}
		if err := database.WithContext(ctx).Exec("ALTER TABLE generation_cleanup_requests DROP CONSTRAINT IF EXISTS generation_cleanup_scope_check").Error; err != nil {
			return err
		}
		if err := database.WithContext(ctx).Exec("ALTER TABLE generation_cleanup_requests ADD CONSTRAINT generation_cleanup_scope_check CHECK (scope IN ('task','source','account','orphan_output'))").Error; err != nil {
			return err
		}
		if err := database.WithContext(ctx).Exec("ALTER TABLE generation_cleanup_requests ALTER COLUMN access_revoked_at DROP NOT NULL").Error; err != nil {
			return err
		}
		if err := database.WithContext(ctx).Exec("ALTER TABLE generation_cleanup_requests DROP CONSTRAINT IF EXISTS generation_cleanup_access_revocation_check").Error; err != nil {
			return err
		}
		if err := database.WithContext(ctx).Exec("ALTER TABLE generation_cleanup_requests ADD CONSTRAINT generation_cleanup_access_revocation_check CHECK ((scope = 'orphan_output' AND access_revoked_at IS NULL) OR (scope <> 'orphan_output' AND access_revoked_at IS NOT NULL))").Error; err != nil {
			return err
		}
	}
	return nil
}

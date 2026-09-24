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

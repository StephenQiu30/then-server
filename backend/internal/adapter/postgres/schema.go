package postgres

import (
	"context"

	"gorm.io/gorm"
)

// Migrate keeps the development database schema aligned with the GORM records.
func Migrate(ctx context.Context, database *gorm.DB) error {
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// GORM does not refresh an existing named CHECK when its record tag changes.
		// Replace changed checks inside the same transaction as AutoMigrate.
		for _, name := range []string{"media_assets_purpose_check", "media_assets_purpose_category_check", "media_assets_content_type_check"} {
			if tx.Migrator().HasConstraint(&mediaAssetRecord{}, name) {
				if err := tx.Migrator().DropConstraint(&mediaAssetRecord{}, name); err != nil {
					return err
				}
			}
		}
		if err := tx.AutoMigrate(
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
		return nil
	})
}

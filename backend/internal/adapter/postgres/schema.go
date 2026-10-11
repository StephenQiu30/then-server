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
	return nil
}

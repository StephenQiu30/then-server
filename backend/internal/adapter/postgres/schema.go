package postgres

import (
	"context"

	"gorm.io/gorm"
)

// Migrate keeps the development database schema aligned with the GORM records.
func Migrate(ctx context.Context, database *gorm.DB) error {
	return database.WithContext(ctx).AutoMigrate(
		&userRecord{},
		&userProfileRecord{},
		&wardrobeItemRecord{},
		&outfitPlanRecord{},
		&outfitPlanItemRecord{},
		&outfitPlanDeletionRecord{},
		&wearEventRecord{},
		&wearEventItemRecord{},
		&wearEventDeletionRecord{},
		&credentialRecord{},
		&sessionRecord{},
		&selfAdultDeclarationRecord{},
		&consentRecord{},
		&mediaAssetRecord{},
		&mediaDerivationRecord{},
		&deletionRequestRecord{},
		&outboxEventRecord{},
		&inboxReceiptRecord{},
	)
}

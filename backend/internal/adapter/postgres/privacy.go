package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PrivacyRepository struct{ database *gorm.DB }

func NewPrivacyRepository(database *gorm.DB) *PrivacyRepository {
	return &PrivacyRepository{database: database}
}

type selfAdultDeclarationRecord struct {
	UserID        string     `gorm:"column:user_id;type:uuid;primaryKey"`
	PolicyVersion string     `gorm:"column:policy_version;type:text;primaryKey;check:self_adult_declarations_policy_check,policy_version = btrim(policy_version) AND char_length(policy_version) BETWEEN 1 AND 80"`
	ConfirmedAt   time.Time  `gorm:"column:confirmed_at;type:timestamptz;not null"`
	WithdrawnAt   *time.Time `gorm:"column:withdrawn_at;type:timestamptz;check:self_adult_declarations_withdrawn_check,withdrawn_at IS NULL OR withdrawn_at >= confirmed_at"`
}

func (selfAdultDeclarationRecord) TableName() string { return "self_adult_declarations" }

func (r *PrivacyRepository) GetSelfAdultDeclaration(ctx context.Context, userID, version string) (domain.SelfAdultDeclaration, error) {
	var record selfAdultDeclarationRecord
	err := r.database.WithContext(ctx).Where("user_id = ? AND policy_version = ?", userID, version).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return emptySelfAdultDeclaration(version), nil
	}
	if err != nil {
		return domain.SelfAdultDeclaration{}, domain.ErrPrivacyUnavailable
	}
	return selfAdultDeclarationFromRecord(record), nil
}

func (r *PrivacyRepository) ConfirmSelfAdultDeclaration(ctx context.Context, userID, version string, at time.Time) (domain.SelfAdultDeclaration, error) {
	var result domain.SelfAdultDeclaration
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, found, err := lockSelfAdultDeclaration(ctx, tx, userID, version)
		if err != nil {
			return err
		}
		if !found {
			record = selfAdultDeclarationRecord{UserID: userID, PolicyVersion: version, ConfirmedAt: at}
			created := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected == 0 {
				record, found, err = lockSelfAdultDeclaration(ctx, tx, userID, version)
				if err != nil || !found {
					return err
				}
			}
		}
		if record.WithdrawnAt != nil {
			if err := tx.WithContext(ctx).Model(&record).Updates(map[string]any{"confirmed_at": at, "withdrawn_at": nil}).Error; err != nil {
				return err
			}
			record.ConfirmedAt, record.WithdrawnAt = at, nil
		}
		result = selfAdultDeclarationFromRecord(record)
		return nil
	})
	if err != nil {
		return domain.SelfAdultDeclaration{}, domain.ErrPrivacyUnavailable
	}
	return result, nil
}

func (r *PrivacyRepository) WithdrawSelfAdultDeclaration(ctx context.Context, userID, version string, at time.Time) (domain.SelfAdultDeclaration, error) {
	var result domain.SelfAdultDeclaration
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, found, err := lockSelfAdultDeclaration(ctx, tx, userID, version)
		if err != nil {
			return err
		}
		if !found {
			result = emptySelfAdultDeclaration(version)
			return nil
		}
		if record.WithdrawnAt == nil {
			if err := tx.WithContext(ctx).Model(&record).Update("withdrawn_at", at).Error; err != nil {
				return err
			}
			record.WithdrawnAt = &at
		}
		result = selfAdultDeclarationFromRecord(record)
		return nil
	})
	if err != nil {
		return domain.SelfAdultDeclaration{}, domain.ErrPrivacyUnavailable
	}
	return result, nil
}

func lockSelfAdultDeclaration(ctx context.Context, database *gorm.DB, userID, version string) (selfAdultDeclarationRecord, bool, error) {
	var record selfAdultDeclarationRecord
	err := database.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND policy_version = ?", userID, version).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return selfAdultDeclarationRecord{}, false, nil
	}
	return record, err == nil, err
}

func emptySelfAdultDeclaration(version string) domain.SelfAdultDeclaration {
	return domain.SelfAdultDeclaration{PolicyVersion: version}
}

func selfAdultDeclarationFromRecord(record selfAdultDeclarationRecord) domain.SelfAdultDeclaration {
	confirmedAt := record.ConfirmedAt
	return domain.SelfAdultDeclaration{
		PolicyVersion: record.PolicyVersion,
		Confirmed:     record.WithdrawnAt == nil,
		ConfirmedAt:   &confirmedAt,
		WithdrawnAt:   record.WithdrawnAt,
	}
}

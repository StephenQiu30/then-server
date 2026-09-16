package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userProfileRecord struct {
	UserID    string    `gorm:"column:user_id;type:uuid;primaryKey"`
	Handle    string    `gorm:"column:handle;type:text;not null;uniqueIndex:user_profiles_handle_unique;check:user_profiles_handle_check,handle ~ '^[a-z0-9_]{3,30}$'"`
	Bio       *string   `gorm:"column:bio;type:text;check:user_profiles_bio_check,bio IS NULL OR (bio = btrim(bio) AND char_length(bio) BETWEEN 1 AND 300)"`
	Revision  int       `gorm:"column:revision;not null;check:user_profiles_revision_check,revision >= 1"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;check:user_profiles_timestamps_check,updated_at >= created_at"`
}

func (userProfileRecord) TableName() string { return "user_profiles" }

type profileRow struct {
	Handle      string    `gorm:"column:handle"`
	DisplayName string    `gorm:"column:display_name"`
	Bio         *string   `gorm:"column:bio"`
	Revision    int       `gorm:"column:revision"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (r *AccountRepository) FindProfileByUserID(ctx context.Context, userID string) (domain.PublicProfile, error) {
	return findProfile(ctx, r.database, "p.user_id = ?", userID)
}

func (r *AccountRepository) FindProfileByHandle(ctx context.Context, handle string) (domain.PublicProfile, error) {
	return findProfile(ctx, r.database, "p.handle = ?", handle)
}

func (r *AccountRepository) PutProfile(ctx context.Context, userID string, input domain.PutProfileInput, at time.Time) (domain.PublicProfile, error) {
	var result domain.PublicProfile
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "display_name", "status").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.Status != string(domain.AccountActive) {
			return domain.ErrAuthentication
		}

		var current userProfileRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&current).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if input.ExpectedRevision != 0 {
				return domain.ErrProfileConflict
			}
			current = userProfileRecord{
				UserID: userID, Handle: input.Handle, Bio: input.Bio, Revision: 1,
				CreatedAt: at, UpdatedAt: at,
			}
			if err := tx.Create(&current).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if input.ExpectedRevision != current.Revision {
				return domain.ErrProfileConflict
			}
			update := tx.Model(&userProfileRecord{}).
				Where("user_id = ? AND revision = ?", userID, current.Revision).
				Updates(map[string]any{
					"handle": input.Handle, "bio": input.Bio,
					"revision": current.Revision + 1, "updated_at": at,
				})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return domain.ErrProfileConflict
			}
		}
		profile, err := findProfile(ctx, tx, "p.user_id = ?", userID)
		if err != nil {
			return err
		}
		result = profile
		return nil
	})
	if err != nil {
		return domain.PublicProfile{}, mapDatabaseError(err)
	}
	return result, nil
}

func findProfile(ctx context.Context, database *gorm.DB, predicate string, value string) (domain.PublicProfile, error) {
	query := `
		SELECT p.handle, u.display_name, p.bio, p.revision, p.created_at, p.updated_at
		FROM user_profiles AS p
		JOIN users AS u ON u.id = p.user_id
		WHERE ` + predicate + ` AND u.status = 'active'
		LIMIT 1`
	rows, err := gorm.G[profileRow](database).Raw(query, value).Find(ctx)
	if err != nil {
		return domain.PublicProfile{}, domain.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return domain.PublicProfile{}, domain.ErrProfileNotFound
	}
	row := rows[0]
	return domain.PublicProfile{
		Handle: row.Handle, DisplayName: row.DisplayName, Bio: row.Bio,
		Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

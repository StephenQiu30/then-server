package postgres

import (
	"context"
	"errors"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userProfileRecord struct {
	UserID        string            `gorm:"column:user_id;type:uuid;primaryKey"`
	Handle        string            `gorm:"column:handle;type:text;not null;uniqueIndex:user_profiles_handle_unique;check:user_profiles_handle_check,handle ~ '^[a-z0-9_]{3,30}$'"`
	Bio           *string           `gorm:"column:bio;type:text;check:user_profiles_bio_check,bio IS NULL OR (bio = btrim(bio) AND char_length(bio) BETWEEN 1 AND 300)"`
	AvatarMediaID *string           `gorm:"column:avatar_media_id;type:uuid;index:user_profiles_avatar_media_idx"`
	AvatarMedia   *mediaAssetRecord `gorm:"foreignKey:AvatarMediaID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:SET NULL"`
	Revision      int               `gorm:"column:revision;not null;check:user_profiles_revision_check,revision >= 1"`
	CreatedAt     time.Time         `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt     time.Time         `gorm:"column:updated_at;type:timestamptz;not null;check:user_profiles_timestamps_check,updated_at >= created_at"`
}

func (userProfileRecord) TableName() string { return "user_profiles" }

type profileRow struct {
	Handle      string    `gorm:"column:handle"`
	DisplayName string    `gorm:"column:display_name"`
	Bio         *string   `gorm:"column:bio"`
	HasAvatar   bool      `gorm:"column:has_avatar"`
	Revision    int       `gorm:"column:revision"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (r *AccountRepository) FindProfileByUserID(ctx context.Context, userID string) (accountapp.PublicProfile, error) {
	return findProfile(ctx, r.database, "p.user_id = ?", userID)
}

func (r *AccountRepository) FindProfileByHandle(ctx context.Context, handle string) (accountapp.PublicProfile, error) {
	return findProfile(ctx, r.database, "p.handle = ?", handle)
}

func (r *AccountRepository) PutProfile(ctx context.Context, userID string, input accountapp.PutProfileInput, at time.Time) (accountapp.PublicProfile, error) {
	var result accountapp.PublicProfile
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "display_name", "status").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.Status != string(accountapp.AccountActive) {
			return accountapp.ErrAuthentication
		}

		var current userProfileRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&current).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if input.ExpectedRevision != 0 {
				return accountapp.ErrProfileConflict
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
				return accountapp.ErrProfileConflict
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
				return accountapp.ErrProfileConflict
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
		return accountapp.PublicProfile{}, mapDatabaseError(err)
	}
	return result, nil
}

func findProfile(ctx context.Context, database *gorm.DB, predicate string, value string) (accountapp.PublicProfile, error) {
	query := `
		SELECT p.handle, u.display_name, p.bio, (m.id IS NOT NULL) AS has_avatar, p.revision, p.created_at, p.updated_at
		FROM user_profiles AS p
		JOIN users AS u ON u.id = p.user_id
		LEFT JOIN media_assets AS m ON m.id = p.avatar_media_id AND m.owner_id = p.user_id AND m.purpose = 'profile_avatar' AND m.status = 'ready'
		WHERE ` + predicate + ` AND u.status = 'active'
		LIMIT 1`
	rows, err := gorm.G[profileRow](database).Raw(query, value).Find(ctx)
	if err != nil {
		return accountapp.PublicProfile{}, accountapp.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return accountapp.PublicProfile{}, accountapp.ErrProfileNotFound
	}
	row := rows[0]
	return accountapp.PublicProfile{
		Handle: row.Handle, DisplayName: row.DisplayName, Bio: row.Bio, HasAvatar: row.HasAvatar,
		Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *AccountRepository) PutProfileAvatar(ctx context.Context, userID, mediaID string, expectedRevision int, at time.Time) (accountapp.PublicProfile, error) {
	return r.changeProfileAvatar(ctx, userID, &mediaID, expectedRevision, at)
}

func (r *AccountRepository) DeleteProfileAvatar(ctx context.Context, userID string, expectedRevision int, at time.Time) (accountapp.PublicProfile, error) {
	return r.changeProfileAvatar(ctx, userID, nil, expectedRevision, at)
}

func (r *AccountRepository) changeProfileAvatar(ctx context.Context, userID string, mediaID *string, expectedRevision int, at time.Time) (accountapp.PublicProfile, error) {
	var result accountapp.PublicProfile
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.Status != string(accountapp.AccountActive) {
			return accountapp.ErrAuthentication
		}
		var profile userProfileRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&profile).Error; err != nil {
			return err
		}
		if profile.Revision != expectedRevision {
			return accountapp.ErrProfileConflict
		}
		if mediaID != nil {
			var media mediaAssetRecord
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ? AND owner_id = ? AND purpose = ? AND category = ? AND status = ?", *mediaID, userID, mediaapp.MediaPurposeProfileAvatar, mediaapp.MediaCategoryOrdinaryImage, string(mediaapp.MediaReady)).First(&media).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return accountapp.ErrProfileNotFound
				}
				return err
			}
			var derivation mediaDerivationRecord
			if err := tx.Where("media_id = ?", *mediaID).First(&derivation).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return accountapp.ErrProfileNotFound
				}
				return err
			}
		}
		if (profile.AvatarMediaID == nil && mediaID == nil) || (profile.AvatarMediaID != nil && mediaID != nil && *profile.AvatarMediaID == *mediaID) {
			var err error
			result, err = findProfile(ctx, tx, "p.user_id = ?", userID)
			return err
		}
		oldID := profile.AvatarMediaID
		if err := tx.Model(&profile).Updates(map[string]any{"avatar_media_id": mediaID, "revision": profile.Revision + 1, "updated_at": at}).Error; err != nil {
			return err
		}
		if oldID != nil {
			if _, err := deleteMediaInTx(tx, userID, *oldID, at); err != nil {
				return err
			}
		}
		var err error
		result, err = findProfile(ctx, tx, "p.user_id = ?", userID)
		return err
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return accountapp.PublicProfile{}, accountapp.ErrProfileNotFound
	}
	if err != nil {
		return accountapp.PublicProfile{}, mapDatabaseError(err)
	}
	return result, nil
}

func (r *AccountRepository) FindProfileAvatar(ctx context.Context, handle string) (accountapp.ProfileAvatarReference, error) {
	type avatarRow struct {
		ObjectKey       string `gorm:"column:object_key"`
		ObjectVersionID string `gorm:"column:object_version_id"`
	}
	rows, err := gorm.G[avatarRow](r.database).Raw(`
		SELECT d.object_key, d.object_version_id
		FROM user_profiles p
		JOIN users u ON u.id = p.user_id AND u.status = 'active'
		JOIN media_assets m ON m.id = p.avatar_media_id AND m.owner_id = p.user_id AND m.purpose = 'profile_avatar' AND m.category = 'ordinary_image' AND m.status = 'ready'
		JOIN media_derivations d ON d.media_id = m.id
		WHERE p.handle = ?
		ORDER BY d.created_at DESC
		LIMIT 1`, handle).Find(ctx)
	if err != nil {
		return accountapp.ProfileAvatarReference{}, accountapp.ErrAccountUnavailable
	}
	if len(rows) != 1 || rows[0].ObjectKey == "" || rows[0].ObjectVersionID == "" {
		return accountapp.ProfileAvatarReference{}, accountapp.ErrProfileNotFound
	}
	return accountapp.ProfileAvatarReference{ObjectKey: rows[0].ObjectKey, ObjectVersionID: rows[0].ObjectVersionID}, nil
}

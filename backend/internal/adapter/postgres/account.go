// Package repository owns PostgreSQL persistence for business aggregates.
package postgres

import (
	"context"
	"errors"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AccountRepository struct{ database *gorm.DB }

func NewAccountRepository(database *gorm.DB) *AccountRepository {
	return &AccountRepository{database: database}
}

type userRecord struct {
	ID              string                       `gorm:"column:id;type:uuid;primaryKey"`
	DisplayName     string                       `gorm:"column:display_name;type:text;not null;check:users_display_name_check,display_name = btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 80"`
	Status          string                       `gorm:"column:status;type:text;not null;default:'active';check:users_status_check,status IN ('active','suspended','deleting')"`
	Role            string                       `gorm:"column:role;type:text;not null;default:'user';check:users_role_check,role IN ('user','moderator','admin')"`
	EmailVerifiedAt *time.Time                   `gorm:"column:email_verified_at;type:timestamptz"`
	Revision        int                          `gorm:"column:revision;not null;default:1;check:users_revision_check,revision >= 1"`
	CreatedAt       time.Time                    `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time                    `gorm:"column:updated_at;type:timestamptz;not null;check:users_timestamps_check,updated_at >= created_at"`
	Profile         *userProfileRecord           `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Credentials     []credentialRecord           `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Sessions        []sessionRecord              `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	MailChallenges  []mailChallengeRecord        `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Declarations    []selfAdultDeclarationRecord `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Consents        []consentRecord              `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Media           []mediaAssetRecord           `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Deletions       []deletionRequestRecord      `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Wardrobe        []wardrobeItemRecord         `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	OutfitPlans     []outfitPlanRecord           `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	OutfitDeletes   []outfitPlanDeletionRecord   `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	WearEvents      []wearEventRecord            `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	WearDeletes     []wearEventDeletionRecord    `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	DiaryEntries    []diaryEntryRecord           `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	DiaryDeletes    []diaryEntryDeletionRecord   `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (userRecord) TableName() string { return "users" }

type credentialRecord struct {
	UserID       string `gorm:"column:user_id;type:uuid;primaryKey"`
	Email        string `gorm:"column:email;type:text;not null;uniqueIndex:user_credentials_email_unique;check:user_credentials_email_check,email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254"`
	PasswordHash string `gorm:"column:password_hash;type:text;not null;check:user_credentials_password_hash_check,char_length(password_hash) BETWEEN 59 AND 72 AND password_hash LIKE '$2%'"`
}

func (credentialRecord) TableName() string { return "user_credentials" }

type sessionRecord struct {
	ID        string    `gorm:"column:id;type:uuid;primaryKey"`
	UserID    string    `gorm:"column:user_id;type:uuid;not null;index:user_sessions_user_id_idx"`
	TokenHash []byte    `gorm:"column:token_hash;type:bytea;not null;uniqueIndex:user_sessions_token_hash_unique;check:user_sessions_token_hash_check,octet_length(token_hash) = 32"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamptz;not null;index:user_sessions_expires_at_idx;check:user_sessions_expiry_check,expires_at > created_at"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null"`
}

func (sessionRecord) TableName() string { return "user_sessions" }

type accountDeletionRequestRecord struct {
	ID               string     `gorm:"column:id;type:uuid;primaryKey"`
	UserID           string     `gorm:"column:user_id;type:uuid;not null;uniqueIndex:account_deletion_requests_user_unique"`
	Status           string     `gorm:"column:status;type:text;not null;index:account_deletion_requests_status_idx;check:account_deletion_requests_status_check,status IN ('pending','complete')"`
	MediaCount       int        `gorm:"column:media_count;not null;check:account_deletion_requests_media_count_check,media_count >= 0"`
	RetryObserved    bool       `gorm:"column:retry_observed;not null;default:false"`
	ReceiptHash      []byte     `gorm:"column:receipt_hash;type:bytea;check:account_deletion_requests_receipt_hash_check,receipt_hash IS NULL OR octet_length(receipt_hash) = 32"`
	ReceiptExpiresAt *time.Time `gorm:"column:receipt_expires_at;type:timestamptz;index:account_deletion_requests_receipt_expiry_idx;check:account_deletion_requests_receipt_expiry_check,receipt_expires_at IS NULL OR receipt_expires_at > requested_at"`
	ReceiptRevokedAt *time.Time `gorm:"column:receipt_revoked_at;type:timestamptz"`
	RequestedAt      time.Time  `gorm:"column:requested_at;type:timestamptz;not null"`
	CompletedAt      *time.Time `gorm:"column:completed_at;type:timestamptz;check:account_deletion_requests_completion_check,(status = 'pending' AND completed_at IS NULL) OR (status = 'complete' AND completed_at IS NOT NULL)"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;type:timestamptz;not null;check:account_deletion_requests_timestamps_check,updated_at >= requested_at"`
}

func (accountDeletionRequestRecord) TableName() string { return "account_deletion_requests" }

type accountRow struct {
	ID              string     `gorm:"column:id"`
	Email           string     `gorm:"column:email"`
	DisplayName     string     `gorm:"column:display_name"`
	Status          string     `gorm:"column:status"`
	Role            string     `gorm:"column:role"`
	EmailVerifiedAt *time.Time `gorm:"column:email_verified_at"`
	Revision        int        `gorm:"column:revision"`
	PasswordHash    string     `gorm:"column:password_hash"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (r *AccountRepository) CreateAccount(ctx context.Context, user accountapp.User, passwordHash string, session accountapp.Session) (accountapp.User, error) {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := gorm.G[userRecord](tx).Create(ctx, &userRecord{
			ID: user.ID, DisplayName: user.DisplayName, Status: string(user.Status), Role: string(user.Role),
			Revision: user.Revision, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
		}); err != nil {
			return err
		}
		if err := gorm.G[credentialRecord](tx).Create(ctx, &credentialRecord{
			UserID: user.ID, Email: user.Email, PasswordHash: passwordHash,
		}); err != nil {
			return err
		}
		return gorm.G[sessionRecord](tx).Create(ctx, &sessionRecord{
			ID: session.ID, UserID: session.UserID, TokenHash: session.TokenHash,
			ExpiresAt: session.ExpiresAt, CreatedAt: session.CreatedAt,
		})
	})
	if err != nil {
		return accountapp.User{}, mapDatabaseError(err)
	}
	return user, nil
}

func (r *AccountRepository) FindCredentialByEmail(ctx context.Context, email string) (accountapp.Credential, error) {
	rows, err := gorm.G[accountRow](r.database).Raw(`
		SELECT u.id, c.email, u.display_name, u.status, u.role, u.email_verified_at, u.revision,
		       c.password_hash, u.created_at, u.updated_at
		FROM users AS u
		JOIN user_credentials AS c ON c.user_id = u.id
		WHERE c.email = ? AND u.status = 'active'
		LIMIT 1`, email).Find(ctx)
	if err != nil {
		return accountapp.Credential{}, accountapp.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return accountapp.Credential{}, accountapp.ErrAuthentication
	}
	return credentialFromRow(rows[0]), nil
}

func (r *AccountRepository) CreateSession(ctx context.Context, session accountapp.Session) error {
	err := gorm.G[sessionRecord](r.database).Create(ctx, &sessionRecord{
		ID: session.ID, UserID: session.UserID, TokenHash: session.TokenHash,
		ExpiresAt: session.ExpiresAt, CreatedAt: session.CreatedAt,
	})
	return mapDatabaseError(err)
}

func (r *AccountRepository) FindUserBySession(ctx context.Context, tokenHash []byte, now time.Time) (accountapp.User, error) {
	rows, err := gorm.G[accountRow](r.database).Raw(`
		SELECT u.id, c.email, u.display_name, u.status, u.role, u.email_verified_at, u.revision,
		       '' AS password_hash, u.created_at, u.updated_at
		FROM user_sessions AS s
		JOIN users AS u ON u.id = s.user_id
		JOIN user_credentials AS c ON c.user_id = u.id
		WHERE s.token_hash = ? AND s.expires_at > ? AND u.status = 'active'
		LIMIT 1`, tokenHash, now).Find(ctx)
	if err != nil {
		return accountapp.User{}, accountapp.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return accountapp.User{}, accountapp.ErrAuthentication
	}
	return userFromRow(rows[0]), nil
}

func (r *AccountRepository) UpdateUser(ctx context.Context, userID string, expectedRevision int, email, displayName *string, updatedAt time.Time) (accountapp.User, error) {
	var result accountapp.User
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&current).Error; err != nil {
			return err
		}
		if current.Status != string(accountapp.AccountActive) {
			return accountapp.ErrAuthentication
		}
		if current.Revision != expectedRevision {
			return accountapp.ErrAccountConflict
		}
		if email != nil {
			var credential credentialRecord
			if err := tx.Where("user_id = ?", userID).Take(&credential).Error; err != nil {
				return err
			}
			rows, err := gorm.G[credentialRecord](tx).Where("user_id = ?", userID).Update(ctx, "email", *email)
			if err != nil {
				return err
			}
			if rows != 1 {
				return accountapp.ErrAuthentication
			}
			if credential.Email != *email {
				if err := tx.Model(&userRecord{}).Where("id = ?", userID).Update("email_verified_at", nil).Error; err != nil {
					return err
				}
				if err := tx.Model(&mailChallengeRecord{}).Where("user_id = ? AND consumed_at IS NULL AND revoked_at IS NULL", userID).
					Updates(map[string]any{"revoked_at": updatedAt, "lease_until": nil, "updated_at": updatedAt}).Error; err != nil {
					return err
				}
			}
		}
		updates := map[string]any{"updated_at": updatedAt, "revision": current.Revision + 1}
		if displayName != nil {
			updates["display_name"] = *displayName
		}
		resultUpdate := tx.Model(&userRecord{}).Where("id = ? AND revision = ?", userID, current.Revision).Updates(updates)
		if resultUpdate.Error != nil {
			return resultUpdate.Error
		}
		if resultUpdate.RowsAffected != 1 {
			return accountapp.ErrAccountConflict
		}
		account, err := findAccount(ctx, tx, userID)
		if err != nil {
			return err
		}
		result = account
		return nil
	})
	if err != nil {
		return accountapp.User{}, mapDatabaseError(err)
	}
	return result, nil
}

func (r *AccountRepository) DeleteSession(ctx context.Context, tokenHash []byte) error {
	rows, err := gorm.G[sessionRecord](r.database).Where("token_hash = ?", tokenHash).Delete(ctx)
	if err != nil {
		return accountapp.ErrAccountUnavailable
	}
	if rows != 1 {
		return accountapp.ErrAuthentication
	}
	return nil
}

func (r *AccountRepository) BeginAccountDeletion(ctx context.Context, userID string, receiptHash []byte, at, expiresAt time.Time) (accountapp.AccountDeletionRequest, error) {
	if len(receiptHash) != 32 || !expiresAt.After(at) {
		return accountapp.AccountDeletionRequest{}, accountapp.ErrAccountUnavailable
	}
	var request accountDeletionRequestRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.Status == string(accountapp.AccountDeleting) {
			return accountapp.ErrAuthentication
		}
		if user.Status != string(accountapp.AccountActive) {
			return accountapp.ErrAuthentication
		}
		if err := tx.Model(&dataExportRecord{}).Where("owner_id = ? AND status NOT IN ?", userID, []string{"revoked", "expired"}).Updates(map[string]any{"status": "revoked", "revoked_at": at}).Error; err != nil {
			return err
		}
		if err := tx.Model(&postRecord{}).Where("source_diary_owner_id = ?", userID).Updates(map[string]any{"source_diary_owner_id": nil, "source_diary_id": nil}).Error; err != nil {
			return err
		}
		if err := tx.Model(&postRecord{}).Where("owner_id = ? AND state <> ?", userID, "deleted").Updates(map[string]any{
			"state": "deleted", "draft_version": nil, "pending_version": nil, "published_version": nil,
			"source_diary_owner_id": nil, "source_diary_id": nil, "published_at": nil, "withdrawn_at": at,
			"deleted_at": at, "updated_at": at,
		}).Error; err != nil {
			return err
		}
		var mediaRecords []mediaAssetRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND status <> ?", userID, string(mediaapp.MediaDeleted)).Order("id ASC").Find(&mediaRecords).Error; err != nil {
			return err
		}
		request = accountDeletionRequestRecord{ID: uuid.NewString(), UserID: userID, Status: string(accountapp.AccountDeletionPending), MediaCount: len(mediaRecords), ReceiptHash: receiptHash, ReceiptExpiresAt: &expiresAt, RequestedAt: at, UpdatedAt: at}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		userUpdate := tx.Model(&userRecord{}).Where("id = ? AND status = ?", userID, string(accountapp.AccountActive)).Updates(map[string]any{"status": string(accountapp.AccountDeleting), "revision": user.Revision + 1, "updated_at": at})
		if userUpdate.Error != nil {
			return userUpdate.Error
		}
		if userUpdate.RowsAffected != 1 {
			return accountapp.ErrAccountConflict
		}
		if err := tx.Where("user_id = ?", userID).Delete(&sessionRecord{}).Error; err != nil {
			return err
		}
		for index := range mediaRecords {
			media := &mediaRecords[index]
			var existing deletionRequestRecord
			if err := tx.Where("media_id = ?", media.ID).First(&existing).Error; err == nil {
				linked := tx.Model(&deletionRequestRecord{}).Where("id = ? AND owner_id = ?", existing.ID, userID).Update("account_deletion_id", request.ID)
				if linked.Error != nil || linked.RowsAffected != 1 {
					return accountapp.ErrAccountUnavailable
				}
				if media.Status != string(mediaapp.MediaDeleting) {
					if err := tx.Model(&mediaAssetRecord{}).Where("id = ?", media.ID).Updates(map[string]any{"status": string(mediaapp.MediaDeleting), "updated_at": at}).Error; err != nil {
						return err
					}
				}
				continue
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			deletion := deletionRequestRecord{ID: uuid.NewString(), OwnerID: userID, AccountDeletionID: &request.ID, MediaID: media.ID, Status: string(mediaapp.DeletionPending), ReadRevokedAt: at, BackupExpiresAt: at, NextAttemptAt: at, CreatedAt: at, UpdatedAt: at}
			if err := tx.Model(&mediaAssetRecord{}).Where("id = ?", media.ID).Updates(map[string]any{"status": string(mediaapp.MediaDeleting), "updated_at": at}).Error; err != nil {
				return err
			}
			if err := tx.Create(&deletion).Error; err != nil {
				return err
			}
			if err := tx.Create(&outboxEventRecord{ID: uuid.NewString(), EventType: "media.deletion_requested", AggregateID: media.ID, Payload: []byte(`{"media_id":"` + media.ID + `"}`), CreatedAt: at}).Error; err != nil {
				return err
			}
		}
		if len(mediaRecords) == 0 {
			if err := finalizeAccountDeletionIfReady(ctx, tx, userID, at); err != nil {
				return err
			}
			request.Status, request.CompletedAt = string(accountapp.AccountDeletionComplete), &at
		}
		return nil
	})
	if err != nil {
		return accountapp.AccountDeletionRequest{}, mapDatabaseError(err)
	}
	return accountDeletionFromRecord(request), nil
}

func finalizeAccountDeletionIfReady(ctx context.Context, tx *gorm.DB, userID string, at time.Time) error {
	var request accountDeletionRequestRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND status = ?", userID, string(accountapp.AccountDeletionPending)).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var remaining int64
	if err := tx.Model(&mediaAssetRecord{}).Where("owner_id = ? AND status <> ?", userID, string(mediaapp.MediaDeleted)).Count(&remaining).Error; err != nil {
		return err
	}
	if remaining != 0 {
		return nil
	}
	var retried int64
	if err := tx.Model(&deletionRequestRecord{}).Where("account_deletion_id = ? AND attempts > 1", request.ID).Count(&retried).Error; err != nil {
		return err
	}
	rows, err := gorm.G[userRecord](tx).Where("id = ? AND status = ?", userID, string(accountapp.AccountDeleting)).Delete(ctx)
	if err != nil {
		return err
	}
	if rows != 1 {
		return accountapp.ErrAuthentication
	}
	result := tx.Model(&accountDeletionRequestRecord{}).Where("id = ? AND status = ?", request.ID, string(accountapp.AccountDeletionPending)).Updates(map[string]any{"status": string(accountapp.AccountDeletionComplete), "completed_at": at, "updated_at": at, "retry_observed": retried > 0})
	if result.Error != nil || result.RowsAffected != 1 {
		return accountapp.ErrAccountUnavailable
	}
	return nil
}

func accountDeletionFromRecord(record accountDeletionRequestRecord) accountapp.AccountDeletionRequest {
	phase := "media_cleanup"
	remaining := record.MediaCount
	if record.Status == string(accountapp.AccountDeletionComplete) {
		phase, remaining = "complete", 0
	}
	request := accountapp.AccountDeletionRequest{ID: record.ID, Status: accountapp.AccountDeletionStatus(record.Status), MediaCount: record.MediaCount, RemainingMediaCount: remaining, RetryObserved: record.RetryObserved, AccessClosed: true, Phase: phase, RequestedAt: record.RequestedAt, UpdatedAt: record.UpdatedAt, CompletedAt: record.CompletedAt}
	if record.ReceiptExpiresAt != nil {
		request.ReceiptExpiresAt = *record.ReceiptExpiresAt
	}
	return request
}

func findAccount(ctx context.Context, database *gorm.DB, userID string) (accountapp.User, error) {
	rows, err := gorm.G[accountRow](database).Raw(`
		SELECT u.id, c.email, u.display_name, u.status, u.role, u.email_verified_at, u.revision,
		       '' AS password_hash, u.created_at, u.updated_at
		FROM users AS u
		JOIN user_credentials AS c ON c.user_id = u.id
		WHERE u.id = ? AND u.status = 'active'
		LIMIT 1`, userID).Find(ctx)
	if err != nil {
		return accountapp.User{}, accountapp.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return accountapp.User{}, accountapp.ErrAuthentication
	}
	return userFromRow(rows[0]), nil
}

func credentialFromRow(row accountRow) accountapp.Credential {
	return accountapp.Credential{User: userFromRow(row), PasswordHash: row.PasswordHash}
}

func userFromRow(row accountRow) accountapp.User {
	return accountapp.User{
		ID: row.ID, Email: row.Email, EmailVerified: row.EmailVerifiedAt != nil, DisplayName: row.DisplayName,
		Status: accountapp.AccountStatus(row.Status), Role: accountapp.AccountRole(row.Role), Revision: row.Revision,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func mapDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	for _, domainError := range []error{
		accountapp.ErrAuthentication, accountapp.ErrAccountConflict, accountapp.ErrProfileNotFound,
		accountapp.ErrProfileConflict, accountapp.ErrHandleConflict, accountapp.ErrInvalidChallenge,
	} {
		if errors.Is(err, domainError) {
			return domainError
		}
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "user_credentials_email_unique" {
		return accountapp.ErrEmailConflict
	}
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "user_profiles_handle_unique" {
		return accountapp.ErrHandleConflict
	}
	return accountapp.ErrAccountUnavailable
}

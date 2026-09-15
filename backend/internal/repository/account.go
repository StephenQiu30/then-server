// Package repository owns PostgreSQL persistence for business aggregates.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AccountRepository struct{ database *gorm.DB }

func NewAccountRepository(database *gorm.DB) *AccountRepository {
	return &AccountRepository{database: database}
}

type userRecord struct {
	ID           string                       `gorm:"column:id;type:uuid;primaryKey"`
	DisplayName  string                       `gorm:"column:display_name;type:text;not null;check:users_display_name_check,display_name = btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 80"`
	CreatedAt    time.Time                    `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt    time.Time                    `gorm:"column:updated_at;type:timestamptz;not null;check:users_timestamps_check,updated_at >= created_at"`
	Credentials  []credentialRecord           `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Sessions     []sessionRecord              `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Declarations []selfAdultDeclarationRecord `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Consents     []consentRecord              `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Media        []mediaAssetRecord           `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Deletions    []deletionRequestRecord      `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
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

type accountRow struct {
	ID           string    `gorm:"column:id"`
	Email        string    `gorm:"column:email"`
	DisplayName  string    `gorm:"column:display_name"`
	PasswordHash string    `gorm:"column:password_hash"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (r *AccountRepository) CreateAccount(ctx context.Context, user model.User, passwordHash string, session model.Session) (model.User, error) {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := gorm.G[userRecord](tx).Create(ctx, &userRecord{
			ID: user.ID, DisplayName: user.DisplayName, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
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
		return model.User{}, mapDatabaseError(err)
	}
	return user, nil
}

func (r *AccountRepository) FindCredentialByEmail(ctx context.Context, email string) (model.Credential, error) {
	rows, err := gorm.G[accountRow](r.database).Raw(`
		SELECT u.id, c.email, u.display_name, c.password_hash, u.created_at, u.updated_at
		FROM users AS u
		JOIN user_credentials AS c ON c.user_id = u.id
		WHERE c.email = ?
		LIMIT 1`, email).Find(ctx)
	if err != nil {
		return model.Credential{}, model.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return model.Credential{}, model.ErrAuthentication
	}
	return credentialFromRow(rows[0]), nil
}

func (r *AccountRepository) CreateSession(ctx context.Context, session model.Session) error {
	err := gorm.G[sessionRecord](r.database).Create(ctx, &sessionRecord{
		ID: session.ID, UserID: session.UserID, TokenHash: session.TokenHash,
		ExpiresAt: session.ExpiresAt, CreatedAt: session.CreatedAt,
	})
	return mapDatabaseError(err)
}

func (r *AccountRepository) FindUserBySession(ctx context.Context, tokenHash []byte, now time.Time) (model.User, error) {
	rows, err := gorm.G[accountRow](r.database).Raw(`
		SELECT u.id, c.email, u.display_name, '' AS password_hash, u.created_at, u.updated_at
		FROM user_sessions AS s
		JOIN users AS u ON u.id = s.user_id
		JOIN user_credentials AS c ON c.user_id = u.id
		WHERE s.token_hash = ? AND s.expires_at > ?
		LIMIT 1`, tokenHash, now).Find(ctx)
	if err != nil {
		return model.User{}, model.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return model.User{}, model.ErrAuthentication
	}
	return userFromRow(rows[0]), nil
}

func (r *AccountRepository) UpdateUser(ctx context.Context, userID string, email, displayName *string, updatedAt time.Time) (model.User, error) {
	var result model.User
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if email != nil {
			rows, err := gorm.G[credentialRecord](tx).Where("user_id = ?", userID).Update(ctx, "email", *email)
			if err != nil {
				return err
			}
			if rows != 1 {
				return model.ErrAuthentication
			}
		}
		if displayName != nil {
			rows, err := gorm.G[userRecord](tx).Where("id = ?", userID).Update(ctx, "display_name", *displayName)
			if err != nil {
				return err
			}
			if rows != 1 {
				return model.ErrAuthentication
			}
		}
		rows, err := gorm.G[userRecord](tx).Where("id = ?", userID).Update(ctx, "updated_at", updatedAt)
		if err != nil {
			return err
		}
		if rows != 1 {
			return model.ErrAuthentication
		}
		account, err := findAccount(ctx, tx, userID)
		if err != nil {
			return err
		}
		result = account
		return nil
	})
	if err != nil {
		return model.User{}, mapDatabaseError(err)
	}
	return result, nil
}

func (r *AccountRepository) DeleteSession(ctx context.Context, tokenHash []byte) error {
	rows, err := gorm.G[sessionRecord](r.database).Where("token_hash = ?", tokenHash).Delete(ctx)
	if err != nil {
		return model.ErrAccountUnavailable
	}
	if rows != 1 {
		return model.ErrAuthentication
	}
	return nil
}

func (r *AccountRepository) DeleteUser(ctx context.Context, userID string) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var activeMedia int64
		if err := tx.Model(&mediaAssetRecord{}).Where("owner_id = ? AND status <> ?", userID, string(model.MediaDeleted)).Count(&activeMedia).Error; err != nil {
			return err
		}
		if activeMedia != 0 {
			return model.ErrAccountMediaConflict
		}
		if err := tx.Exec("DELETE FROM inbox_receipts WHERE event_id IN (SELECT id FROM outbox_events WHERE aggregate_id IN (SELECT id FROM media_assets WHERE owner_id = ?))", userID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM outbox_events WHERE aggregate_id IN (SELECT id FROM media_assets WHERE owner_id = ?)", userID).Error; err != nil {
			return err
		}
		rows, err := gorm.G[userRecord](tx).Where("id = ?", userID).Delete(ctx)
		if err != nil {
			return err
		}
		if rows != 1 {
			return model.ErrAuthentication
		}
		return nil
	})
	return mapDatabaseError(err)
}

func findAccount(ctx context.Context, database *gorm.DB, userID string) (model.User, error) {
	rows, err := gorm.G[accountRow](database).Raw(`
		SELECT u.id, c.email, u.display_name, '' AS password_hash, u.created_at, u.updated_at
		FROM users AS u
		JOIN user_credentials AS c ON c.user_id = u.id
		WHERE u.id = ?
		LIMIT 1`, userID).Find(ctx)
	if err != nil {
		return model.User{}, model.ErrAccountUnavailable
	}
	if len(rows) != 1 {
		return model.User{}, model.ErrAuthentication
	}
	return userFromRow(rows[0]), nil
}

func credentialFromRow(row accountRow) model.Credential {
	return model.Credential{User: userFromRow(row), PasswordHash: row.PasswordHash}
}

func userFromRow(row accountRow) model.User {
	return model.User{
		ID: row.ID, Email: row.Email, DisplayName: row.DisplayName,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func mapDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, model.ErrAuthentication) {
		return model.ErrAuthentication
	}
	if errors.Is(err, model.ErrAccountMediaConflict) {
		return model.ErrAccountMediaConflict
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "user_credentials_email_unique" {
		return model.ErrEmailConflict
	}
	return model.ErrAccountUnavailable
}

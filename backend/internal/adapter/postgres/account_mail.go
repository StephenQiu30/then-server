package postgres

import (
	"context"
	"errors"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mailChallengeRecord struct {
	ID            string     `gorm:"column:id;type:uuid;primaryKey"`
	UserID        string     `gorm:"column:user_id;type:uuid;not null;index:account_mail_challenges_user_idx"`
	Email         string     `gorm:"column:email;type:text;not null;check:account_mail_challenges_email_check,email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254"`
	Purpose       string     `gorm:"column:purpose;type:text;not null;check:account_mail_challenges_purpose_check,purpose IN ('verify_email','reset_password')"`
	ExpiresAt     time.Time  `gorm:"column:expires_at;type:timestamptz;not null;index:account_mail_challenges_due_idx;check:account_mail_challenges_expiry_check,expires_at > created_at"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at;type:timestamptz;not null;index:account_mail_challenges_due_idx"`
	LeaseUntil    *time.Time `gorm:"column:lease_until;type:timestamptz"`
	Attempts      int        `gorm:"column:attempts;not null;default:0;check:account_mail_challenges_attempts_check,attempts >= 0"`
	SentAt        *time.Time `gorm:"column:sent_at;type:timestamptz"`
	ConsumedAt    *time.Time `gorm:"column:consumed_at;type:timestamptz"`
	RevokedAt     *time.Time `gorm:"column:revoked_at;type:timestamptz"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (mailChallengeRecord) TableName() string { return "account_mail_challenges" }

func (r *AccountRepository) CreateVerificationChallenge(ctx context.Context, challenge accountapp.MailChallenge, now time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = 'active'", challenge.UserID).Take(&user).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return accountapp.ErrAuthentication
		} else if err != nil {
			return err
		}
		var credential credentialRecord
		if err := tx.Where("user_id = ?", user.ID).Take(&credential).Error; err != nil {
			return err
		}
		if credential.Email != challenge.Email || user.EmailVerifiedAt != nil {
			return nil
		}
		if err := revokeChallenges(tx, user.ID, string(accountapp.VerifyEmail), now); err != nil {
			return err
		}
		return tx.Create(newMailChallengeRecord(challenge, now)).Error
	})
	return mapDatabaseError(err)
}

func (r *AccountRepository) CreateResetChallenge(ctx context.Context, challenge accountapp.MailChallenge, now time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var credential credentialRecord
		if err := tx.Where("email = ?", challenge.Email).Take(&credential).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = 'active'", credential.UserID).Take(&user).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND email = ?", user.ID, challenge.Email).Take(&credential).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		if err := revokeChallenges(tx, user.ID, string(accountapp.ResetPassword), now); err != nil {
			return err
		}
		challenge.UserID = user.ID
		return tx.Create(newMailChallengeRecord(challenge, now)).Error
	})
	return mapDatabaseError(err)
}

func newMailChallengeRecord(challenge accountapp.MailChallenge, now time.Time) *mailChallengeRecord {
	return &mailChallengeRecord{
		ID: challenge.ID, UserID: challenge.UserID, Email: challenge.Email, Purpose: string(challenge.Purpose),
		ExpiresAt: challenge.ExpiresAt, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	}
}

func revokeChallenges(tx *gorm.DB, userID, purpose string, now time.Time) error {
	return tx.Model(&mailChallengeRecord{}).Where("user_id = ? AND purpose = ? AND consumed_at IS NULL AND revoked_at IS NULL", userID, purpose).
		Updates(map[string]any{"revoked_at": now, "lease_until": nil, "updated_at": now}).Error
}

func (r *AccountRepository) FindChallenge(ctx context.Context, id string) (accountapp.MailChallenge, error) {
	var record mailChallengeRecord
	if err := r.database.WithContext(ctx).Where("id = ?", id).Take(&record).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return accountapp.MailChallenge{}, accountapp.ErrInvalidChallenge
	} else if err != nil {
		return accountapp.MailChallenge{}, accountapp.ErrAccountUnavailable
	}
	return mailChallengeFromRecord(record), nil
}

func (r *AccountRepository) ConfirmVerification(ctx context.Context, id, userID, email string, now time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = 'active'", userID).Take(&user).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return accountapp.ErrInvalidChallenge
		} else if err != nil {
			return err
		}
		var record mailChallengeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&record).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return accountapp.ErrInvalidChallenge
		} else if err != nil {
			return err
		}
		if !usableChallenge(record, userID, email, string(accountapp.VerifyEmail), now) {
			return accountapp.ErrInvalidChallenge
		}
		var credential credentialRecord
		if err := tx.Where("user_id = ?", userID).Take(&credential).Error; err != nil || credential.Email != email || user.EmailVerifiedAt != nil {
			return accountapp.ErrInvalidChallenge
		}
		if err := tx.Model(&userRecord{}).Where("id = ?", userID).Updates(map[string]any{"email_verified_at": now, "revision": user.Revision + 1, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&mailChallengeRecord{}).Where("id = ?", id).Updates(map[string]any{"consumed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&mailChallengeRecord{}).Where("user_id = ? AND purpose = ? AND id <> ? AND consumed_at IS NULL AND revoked_at IS NULL", userID, string(accountapp.VerifyEmail), id).
			Updates(map[string]any{"revoked_at": now, "lease_until": nil, "updated_at": now}).Error
	})
	return mapDatabaseError(err)
}

func (r *AccountRepository) ConfirmReset(ctx context.Context, id, email, passwordHash string, now time.Time) error {
	challenge, err := r.FindChallenge(ctx, id)
	if err != nil {
		return err
	}
	err = r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = 'active'", challenge.UserID).Take(&user).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return accountapp.ErrInvalidChallenge
		} else if err != nil {
			return err
		}
		var record mailChallengeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&record).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return accountapp.ErrInvalidChallenge
		} else if err != nil {
			return err
		}
		if !usableChallenge(record, user.ID, email, string(accountapp.ResetPassword), now) {
			return accountapp.ErrInvalidChallenge
		}
		result := tx.Model(&credentialRecord{}).Where("user_id = ? AND email = ?", user.ID, email).Update("password_hash", passwordHash)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return accountapp.ErrInvalidChallenge
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&sessionRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&userRecord{}).Where("id = ?", user.ID).Updates(map[string]any{"revision": user.Revision + 1, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&mailChallengeRecord{}).Where("id = ?", id).Updates(map[string]any{"consumed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&mailChallengeRecord{}).Where("user_id = ? AND purpose = ? AND id <> ? AND consumed_at IS NULL AND revoked_at IS NULL", user.ID, string(accountapp.ResetPassword), id).
			Updates(map[string]any{"revoked_at": now, "lease_until": nil, "updated_at": now}).Error
	})
	return mapDatabaseError(err)
}

func usableChallenge(record mailChallengeRecord, userID, email, purpose string, now time.Time) bool {
	return record.UserID == userID && record.Email == email && record.Purpose == purpose && record.ExpiresAt.After(now) && record.SentAt != nil && record.ConsumedAt == nil && record.RevokedAt == nil
}

func (r *AccountRepository) ClaimDueChallenges(ctx context.Context, now time.Time, limit int) ([]accountapp.MailChallenge, error) {
	var records []mailChallengeRecord
	err := r.database.WithContext(ctx).Raw(`
		WITH due AS (
			SELECT m.id FROM account_mail_challenges AS m
			JOIN users AS u ON u.id = m.user_id
			JOIN user_credentials AS c ON c.user_id = u.id
			WHERE m.sent_at IS NULL AND m.consumed_at IS NULL AND m.revoked_at IS NULL
			  AND m.expires_at > ? AND m.next_attempt_at <= ?
			  AND (m.lease_until IS NULL OR m.lease_until <= ?)
			  AND u.status = 'active' AND c.email = m.email
			ORDER BY m.next_attempt_at, m.id
			FOR UPDATE OF m SKIP LOCKED LIMIT ?
		)
		UPDATE account_mail_challenges AS m
		SET lease_until = ?, attempts = m.attempts + 1, updated_at = ?
		FROM due WHERE m.id = due.id
		RETURNING m.*`, now, now, now, limit, now.Add(30*time.Second), now).Scan(&records).Error
	if err != nil {
		return nil, accountapp.ErrAccountUnavailable
	}
	challenges := make([]accountapp.MailChallenge, 0, len(records))
	for _, record := range records {
		challenges = append(challenges, mailChallengeFromRecord(record))
	}
	return challenges, nil
}

func mailChallengeFromRecord(record mailChallengeRecord) accountapp.MailChallenge {
	challenge := accountapp.MailChallenge{ID: record.ID, UserID: record.UserID, Email: record.Email, Purpose: accountapp.ChallengePurpose(record.Purpose), ExpiresAt: record.ExpiresAt, Attempts: record.Attempts}
	if record.LeaseUntil != nil {
		challenge.LeaseUntil = *record.LeaseUntil
	}
	return challenge
}

func (r *AccountRepository) MarkChallengeSent(ctx context.Context, id string, lease, at time.Time) error {
	result := r.database.WithContext(ctx).Model(&mailChallengeRecord{}).Where("id = ? AND lease_until = ? AND sent_at IS NULL AND revoked_at IS NULL", id, lease).
		Updates(map[string]any{"sent_at": at, "lease_until": nil, "updated_at": at})
	if result.Error != nil {
		return accountapp.ErrAccountUnavailable
	}
	return nil
}

func (r *AccountRepository) RetryChallenge(ctx context.Context, id string, lease, at, next time.Time) error {
	result := r.database.WithContext(ctx).Model(&mailChallengeRecord{}).Where("id = ? AND lease_until = ? AND sent_at IS NULL AND revoked_at IS NULL", id, lease).
		Updates(map[string]any{"next_attempt_at": next, "lease_until": nil, "updated_at": at})
	if result.Error != nil {
		return accountapp.ErrAccountUnavailable
	}
	return nil
}

func (r *AccountRepository) PurgeOldChallenges(ctx context.Context, before time.Time) error {
	return r.database.WithContext(ctx).Where("expires_at < ?", before).Delete(&mailChallengeRecord{}).Error
}

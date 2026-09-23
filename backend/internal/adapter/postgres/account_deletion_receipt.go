package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"gorm.io/gorm"
)

func (r *AccountRepository) GetDeletionReceipt(ctx context.Context, id string, hash []byte, at time.Time) (accountapp.AccountDeletionRequest, error) {
	var request accountapp.AccountDeletionRequest
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record accountDeletionRequestRecord
		if err := tx.Where("id = ? AND receipt_hash = ? AND receipt_revoked_at IS NULL AND receipt_expires_at > ?", id, hash, at).Take(&record).Error; err != nil {
			return err
		}
		request = accountDeletionFromRecord(record)
		if record.Status != string(accountapp.AccountDeletionPending) {
			return nil
		}
		var progress struct {
			Remaining     int       `gorm:"column:remaining"`
			RetryObserved bool      `gorm:"column:retry_observed"`
			UpdatedAt     time.Time `gorm:"column:updated_at"`
		}
		if err := tx.Raw(`SELECT count(*) FILTER (WHERE status <> 'complete') AS remaining,
			COALESCE(bool_or(attempts > 1), false) AS retry_observed,
			COALESCE(max(updated_at), ?) AS updated_at
			FROM deletion_requests WHERE account_deletion_id = ?`, record.RequestedAt, record.ID).Scan(&progress).Error; err != nil {
			return err
		}
		request.RemainingMediaCount = progress.Remaining
		request.RetryObserved = progress.RetryObserved
		if progress.UpdatedAt.After(request.UpdatedAt) {
			request.UpdatedAt = progress.UpdatedAt
		}
		if progress.RetryObserved {
			request.Phase = "media_cleanup_retry_observed"
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return accountapp.AccountDeletionRequest{}, accountapp.ErrDeletionReceiptNotFound
	}
	if err != nil {
		return accountapp.AccountDeletionRequest{}, accountapp.ErrAccountUnavailable
	}
	return request, nil
}

func (r *AccountRepository) RevokeDeletionReceipt(ctx context.Context, id string, hash []byte, at time.Time) error {
	result := r.database.WithContext(ctx).Model(&accountDeletionRequestRecord{}).
		Where("id = ? AND receipt_hash = ? AND receipt_revoked_at IS NULL AND receipt_expires_at > ?", id, hash, at).
		Updates(map[string]any{"receipt_hash": nil, "receipt_revoked_at": at})
	if result.Error != nil {
		return accountapp.ErrAccountUnavailable
	}
	if result.RowsAffected != 1 {
		return accountapp.ErrDeletionReceiptNotFound
	}
	return nil
}

func (r *AccountRepository) PurgeDeletionReceipts(ctx context.Context, at time.Time) error {
	return r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&accountDeletionRequestRecord{}).
			Where("status = ? AND receipt_expires_at <= ? AND receipt_hash IS NOT NULL", string(accountapp.AccountDeletionPending), at).
			Updates(map[string]any{"receipt_hash": nil, "receipt_revoked_at": at}).Error; err != nil {
			return err
		}
		return tx.Where("status = ? AND ((receipt_expires_at IS NOT NULL AND receipt_expires_at <= ?) OR (receipt_expires_at IS NULL AND requested_at <= ?))", string(accountapp.AccountDeletionComplete), at, at.Add(-7*24*time.Hour)).Delete(&accountDeletionRequestRecord{}).Error
	})
}

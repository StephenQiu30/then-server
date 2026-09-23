package postgres

import (
	"context"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type sessionViewRow struct {
	ID        string    `gorm:"column:id"`
	CreatedAt time.Time `gorm:"column:created_at"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	Current   bool      `gorm:"column:current"`
}

func (r *AccountRepository) ListSessions(ctx context.Context, userID string, tokenHash []byte, now time.Time, limit, offset int) (accountapp.SessionPage, error) {
	var rows []sessionViewRow
	err := r.database.WithContext(ctx).Raw(`
		SELECT s.id, s.created_at, s.expires_at, s.token_hash = ? AS current
		FROM user_sessions AS s
		WHERE s.user_id = ? AND s.expires_at > ?
		  AND EXISTS (SELECT 1 FROM user_sessions AS caller JOIN users AS u ON u.id = caller.user_id
		              WHERE caller.token_hash = ? AND caller.expires_at > ? AND caller.user_id = s.user_id AND u.status = 'active')
		ORDER BY s.created_at DESC, s.id DESC LIMIT ? OFFSET ?`, tokenHash, userID, now, tokenHash, now, limit+1, offset).Scan(&rows).Error
	if err != nil {
		return accountapp.SessionPage{}, accountapp.ErrAccountUnavailable
	}
	page := accountapp.SessionPage{Items: make([]accountapp.SessionView, 0, min(len(rows), limit))}
	for i, row := range rows {
		if i == limit {
			next := offset + limit
			page.NextOffset = &next
			break
		}
		page.Items = append(page.Items, accountapp.SessionView{ID: row.ID, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Current: row.Current})
	}
	return page, nil
}

func (r *AccountRepository) RevokeSession(ctx context.Context, tokenHash []byte, sessionID string, now time.Time) (bool, error) {
	var rows []struct {
		Current bool `gorm:"column:current"`
	}
	err := r.database.WithContext(ctx).Raw(`
		WITH caller AS (
			SELECT s.user_id FROM user_sessions AS s JOIN users AS u ON u.id = s.user_id
			WHERE s.token_hash = ? AND s.expires_at > ? AND u.status = 'active'
		), deleted AS (
			DELETE FROM user_sessions AS target USING caller
			WHERE target.id = ? AND target.user_id = caller.user_id AND target.expires_at > ?
			RETURNING target.token_hash
		)
		SELECT token_hash = ? AS current FROM deleted`, tokenHash, now, sessionID, now, tokenHash).Scan(&rows).Error
	if err != nil {
		return false, accountapp.ErrAccountUnavailable
	}
	if len(rows) == 1 {
		return rows[0].Current, nil
	}
	if _, err := r.FindUserBySession(ctx, tokenHash, now); err != nil {
		return false, err
	}
	return false, accountapp.ErrSessionNotFound
}

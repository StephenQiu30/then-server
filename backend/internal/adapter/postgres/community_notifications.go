package postgres

import (
	"context"
	"errors"
	"time"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *CommunityRepository) ListNotifications(ctx context.Context, recipientID string, limit int, afterID *string) (communityapp.NotificationPage, error) {
	query := r.database.WithContext(ctx).Where("recipient_id = ? AND available_at IS NOT NULL", recipientID).
		Where(`kind IN ('post_reviewed','comment_reviewed','appeal_resolved') OR
			(kind = 'followed' AND actor_id IS NOT NULL AND NOT EXISTS (
				SELECT 1 FROM user_blocks ub WHERE (ub.blocker_id = ? AND ub.blocked_id = notifications.actor_id) OR (ub.blocker_id = notifications.actor_id AND ub.blocked_id = ?)
			)) OR
			(kind IN ('comment_published','reply_published') AND post_id IS NOT NULL AND comment_id IS NOT NULL
				AND EXISTS (SELECT 1 FROM posts p WHERE p.id = notifications.post_id AND p.state = 'published')
				AND EXISTS (SELECT 1 FROM comments c WHERE c.id = notifications.comment_id AND c.state = 'published')
				AND NOT EXISTS (SELECT 1 FROM user_blocks ub JOIN posts p ON p.id = notifications.post_id WHERE (ub.blocker_id = ? AND ub.blocked_id = p.owner_id) OR (ub.blocker_id = p.owner_id AND ub.blocked_id = ?))
			)`, recipientID, recipientID, recipientID, recipientID)
	if afterID != nil {
		var cursor notificationRecord
		if err := query.Session(&gorm.Session{}).Where("id = ?", *afterID).First(&cursor).Error; err != nil {
			return communityapp.NotificationPage{}, communityapp.ErrNotificationNotFound
		}
		query = query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []notificationRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.NotificationPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.NotificationPage{Notifications: make([]communityapp.Notification, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			cursor := records[index-1].ID
			page.NextAfterID = &cursor
			break
		}
		notification, err := notificationFromRecord(r.database.WithContext(ctx), record)
		if err != nil {
			return communityapp.NotificationPage{}, mapCommunityError(err)
		}
		page.Notifications = append(page.Notifications, notification)
	}
	return page, nil
}

func (r *CommunityRepository) MarkNotificationRead(ctx context.Context, recipientID, notificationID string, at time.Time) (communityapp.Notification, error) {
	var record notificationRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND recipient_id = ? AND available_at IS NOT NULL", notificationID, recipientID).First(&record).Error; err != nil {
			return communityapp.ErrNotificationNotFound
		}
		if record.ReadAt == nil {
			if err := tx.Model(&record).Update("read_at", at).Error; err != nil {
				return err
			}
			record.ReadAt = &at
		}
		return nil
	})
	if err != nil {
		return communityapp.Notification{}, mapCommunityError(err)
	}
	return notificationFromRecord(r.database.WithContext(ctx), record)
}

func (r *CommunityRepository) CreateAppeal(ctx context.Context, appellantID, appealID, actionID, reason string, at time.Time) (communityapp.ModerationAppeal, error) {
	var result moderationAppealRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing moderationAppealRecord
		if err := tx.Where("id = ?", appealID).First(&existing).Error; err == nil {
			if existing.AppellantID != appellantID || existing.ActionID != actionID || existing.Reason != reason {
				return communityapp.ErrPostConflict
			}
			result = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err := tx.Raw(`SELECT COUNT(*) FROM moderation_actions ma
			LEFT JOIN posts p ON p.id = ma.post_id
			LEFT JOIN comments c ON c.id = ma.comment_id
			WHERE ma.id = ? AND ma.action IN ('post_remove','post_reject','comment_remove','comment_reject')
			  AND (ma.subject_user_id = ? OR p.owner_id = ? OR c.author_id = ?)`, actionID, appellantID, appellantID, appellantID).Scan(&count).Error; err != nil || count != 1 {
			return communityapp.ErrCommunityForbidden
		}
		if err := tx.Where("appellant_id = ? AND action_id = ? AND status = 'open'", appellantID, actionID).First(&result).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result = moderationAppealRecord{ID: appealID, AppellantID: appellantID, ActionID: actionID, Reason: reason, Status: string(communityapp.AppealOpen), CreatedAt: at}
		return tx.Create(&result).Error
	})
	if err != nil {
		return communityapp.ModerationAppeal{}, mapCommunityError(err)
	}
	return appealFromRecord(r.database.WithContext(ctx), result), nil
}

func (r *CommunityRepository) ListOwnAppeals(ctx context.Context, appellantID string, limit int, afterID *string) (communityapp.ModerationAppealPage, error) {
	query := r.database.WithContext(ctx).Where("appellant_id = ?", appellantID)
	query, err := appealCursor(query, afterID)
	if err != nil {
		return communityapp.ModerationAppealPage{}, err
	}
	var records []moderationAppealRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.ModerationAppealPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.ModerationAppealPage{Appeals: make([]communityapp.ModerationAppeal, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			cursor := records[index-1].ID
			page.NextAfterID = &cursor
			break
		}
		page.Appeals = append(page.Appeals, appealFromRecord(r.database.WithContext(ctx), record))
	}
	return page, nil
}

func (r *CommunityRepository) ListAppeals(ctx context.Context, limit int, afterID *string, status *communityapp.AppealStatus) (communityapp.AdminModerationAppealPage, error) {
	query := r.database.WithContext(ctx).Model(&moderationAppealRecord{})
	if status != nil {
		query = query.Where("status = ?", string(*status))
	}
	query, err := appealCursor(query, afterID)
	if err != nil {
		return communityapp.AdminModerationAppealPage{}, err
	}
	var records []moderationAppealRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.AdminModerationAppealPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.AdminModerationAppealPage{Appeals: make([]communityapp.AdminModerationAppeal, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			cursor := records[index-1].ID
			page.NextAfterID = &cursor
			break
		}
		page.Appeals = append(page.Appeals, communityapp.AdminModerationAppeal{ModerationAppeal: appealFromRecord(r.database.WithContext(ctx), record), AppellantID: record.AppellantID})
	}
	return page, nil
}

func (r *CommunityRepository) ResolveAppeal(ctx context.Context, actorID, appealID string, status communityapp.AppealStatus, reason string, at time.Time) (communityapp.ModerationAppeal, error) {
	var result moderationAppealRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", appealID).First(&result).Error; err != nil {
			return communityapp.ErrAppealNotFound
		}
		if result.Status != string(communityapp.AppealOpen) {
			if result.Status == string(status) && result.ResolutionCode != nil && *result.ResolutionCode == reason {
				return nil
			}
			return communityapp.ErrPostConflict
		}
		if err := tx.Model(&result).Updates(map[string]any{"status": string(status), "resolution_code": reason, "resolved_by": actorID, "resolved_at": at}).Error; err != nil {
			return err
		}
		result.Status, result.ResolutionCode, result.ResolvedBy, result.ResolvedAt = string(status), &reason, &actorID, &at
		if err := tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, SubjectUserID: &result.AppellantID, Action: "appeal_" + string(status), ReasonCode: reason, CreatedAt: at}).Error; err != nil {
			return err
		}
		return enqueueNotification(tx, result.AppellantID, &actorID, communityapp.NotificationAppealResolved, nil, nil, at)
	})
	if err != nil {
		return communityapp.ModerationAppeal{}, mapCommunityError(err)
	}
	return appealFromRecord(r.database.WithContext(ctx), result), nil
}

func notificationFromRecord(database *gorm.DB, record notificationRecord) (communityapp.Notification, error) {
	var handle *string
	if record.ActorID != nil {
		var profile userProfileRecord
		if err := database.Select("handle").Where("user_id = ?", *record.ActorID).First(&profile).Error; err == nil {
			handle = &profile.Handle
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return communityapp.Notification{}, err
		}
	}
	return communityapp.Notification{ID: record.ID, Kind: communityapp.NotificationKind(record.Kind), ActorHandle: handle, PostID: record.PostID, CommentID: record.CommentID, ReadAt: record.ReadAt, CreatedAt: record.CreatedAt}, nil
}

func appealFromRecord(database *gorm.DB, record moderationAppealRecord) communityapp.ModerationAppeal {
	var action moderationActionRecord
	_ = database.Select("action").Where("id = ?", record.ActionID).First(&action).Error
	return communityapp.ModerationAppeal{ID: record.ID, ActionID: record.ActionID, Action: action.Action, Reason: record.Reason, Status: communityapp.AppealStatus(record.Status), ResolutionCode: record.ResolutionCode, CreatedAt: record.CreatedAt, ResolvedAt: record.ResolvedAt}
}

func appealCursor(query *gorm.DB, afterID *string) (*gorm.DB, error) {
	if afterID == nil {
		return query, nil
	}
	var cursor moderationAppealRecord
	if err := query.Session(&gorm.Session{}).Select("id", "created_at").Where("id = ?", *afterID).First(&cursor).Error; err != nil {
		return nil, communityapp.ErrAppealNotFound
	}
	return query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID), nil
}

func sameString(first, second *string) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}

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

func (r *CommunityRepository) CreateComment(ctx context.Context, authorID, commentID, postID string, input communityapp.CreateCommentInput, at time.Time) (communityapp.Comment, error) {
	var result communityapp.Comment
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing commentRecord
		if err := tx.Where("id = ?", commentID).First(&existing).Error; err == nil {
			if existing.AuthorID != authorID || existing.PostID != postID || existing.Body == nil || *existing.Body != input.Body || !sameString(existing.ParentID, input.ParentID) {
				return communityapp.ErrPostConflict
			}
			loaded, loadErr := loadComment(tx, commentID)
			result = loaded
			return loadErr
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := lockVisiblePost(tx, postID, authorID); err != nil {
			return err
		}
		if input.ParentID != nil {
			var parent commentRecord
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND post_id = ? AND parent_id IS NULL AND state = 'published'", *input.ParentID, postID).First(&parent).Error; err != nil {
				return communityapp.ErrCommentNotFound
			}
		}
		body := input.Body
		record := commentRecord{ID: commentID, PostID: postID, AuthorID: authorID, ParentID: input.ParentID, Body: &body, State: string(communityapp.CommentPending), Revision: 1, CreatedAt: at, UpdatedAt: at}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		loaded, loadErr := loadComment(tx, commentID)
		result = loaded
		return loadErr
	})
	return result, mapCommunityError(err)
}

func (r *CommunityRepository) ListComments(ctx context.Context, postID string, viewerID, parentID *string, limit int, afterID *string) (communityapp.CommentPage, error) {
	viewer := stringValue(viewerID)
	if _, err := loadPublicPostForViewer(ctx, r.database, postID, viewer); err != nil {
		return communityapp.CommentPage{}, mapCommunityError(err)
	}
	query := r.database.WithContext(ctx).Table("comments c").Joins("JOIN users cu ON cu.id = c.author_id AND cu.status = 'active'").Where("c.post_id = ?", postID)
	if parentID == nil {
		query = query.Where("c.parent_id IS NULL")
	} else {
		query = query.Where("c.parent_id = ?", *parentID)
	}
	if viewer == "" {
		query = query.Where("c.state IN ?", []string{string(communityapp.CommentPublished), string(communityapp.CommentDeleted), string(communityapp.CommentRemoved)})
	} else {
		query = query.Where("(c.state IN ? OR (c.state = ? AND c.author_id = ?))", []string{string(communityapp.CommentPublished), string(communityapp.CommentDeleted), string(communityapp.CommentRemoved)}, string(communityapp.CommentPending), viewer).
			Where(`NOT EXISTS (SELECT 1 FROM user_blocks ub WHERE (ub.blocker_id = ? AND ub.blocked_id = c.author_id) OR (ub.blocker_id = c.author_id AND ub.blocked_id = ?))`, viewer, viewer)
	}
	if afterID != nil {
		var cursor commentRecord
		cursorQuery := query.Session(&gorm.Session{}).Where("c.id = ?", *afterID).Select("c.*").Scan(&cursor)
		if cursorQuery.Error != nil || cursor.ID == "" {
			return communityapp.CommentPage{}, communityapp.ErrCommentNotFound
		}
		query = query.Where("(c.created_at, c.id) > (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []commentRecord
	if err := query.Select("c.*").Order("c.created_at ASC, c.id ASC").Limit(limit + 1).Scan(&records).Error; err != nil {
		return communityapp.CommentPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.CommentPage{Comments: make([]communityapp.Comment, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			cursor := records[index-1].ID
			page.NextAfterID = &cursor
			break
		}
		comment, err := loadComment(r.database.WithContext(ctx), record.ID)
		if err != nil {
			return communityapp.CommentPage{}, mapCommunityError(err)
		}
		page.Comments = append(page.Comments, comment)
	}
	return page, nil
}

func (r *CommunityRepository) ListReplies(ctx context.Context, commentID string, viewerID *string, limit int, afterID *string) (communityapp.CommentPage, error) {
	var parent commentRecord
	if err := r.database.WithContext(ctx).Where("id = ? AND parent_id IS NULL AND state IN ?", commentID, []string{string(communityapp.CommentPublished), string(communityapp.CommentDeleted), string(communityapp.CommentRemoved)}).First(&parent).Error; err != nil {
		return communityapp.CommentPage{}, communityapp.ErrCommentNotFound
	}
	return r.ListComments(ctx, parent.PostID, viewerID, &commentID, limit, afterID)
}

func (r *CommunityRepository) DeleteComment(ctx context.Context, authorID, commentID string, expectedRevision int, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record commentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND author_id = ?", commentID, authorID).First(&record).Error; err != nil {
			return communityapp.ErrCommentNotFound
		}
		if record.State == string(communityapp.CommentDeleted) {
			return nil
		}
		if record.Revision != expectedRevision || record.State == string(communityapp.CommentRemoved) {
			return communityapp.ErrPostConflict
		}
		return tx.Model(&commentRecord{}).Where("id = ? AND revision = ?", commentID, expectedRevision).Updates(map[string]any{"body": nil, "state": string(communityapp.CommentDeleted), "revision": expectedRevision + 1, "updated_at": at}).Error
	})
	return mapCommunityError(err)
}

func (r *CommunityRepository) ListCommentModeration(ctx context.Context, limit int, afterID *string) (communityapp.CommentModerationPage, error) {
	query := r.database.WithContext(ctx).Model(&commentRecord{}).Where("state = ?", string(communityapp.CommentPending))
	if afterID != nil {
		var cursor commentRecord
		if err := query.Session(&gorm.Session{}).Where("id = ?", *afterID).First(&cursor).Error; err != nil {
			return communityapp.CommentModerationPage{}, communityapp.ErrCommentNotFound
		}
		query = query.Where("(created_at, id) > (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []commentRecord
	if err := query.Order("created_at ASC, id ASC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.CommentModerationPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.CommentModerationPage{Comments: make([]communityapp.CommentModerationCandidate, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			cursor := records[index-1].ID
			page.NextAfterID = &cursor
			break
		}
		comment, err := loadComment(r.database.WithContext(ctx), record.ID)
		if err != nil {
			return communityapp.CommentModerationPage{}, mapCommunityError(err)
		}
		post, err := loadPublicPostForModeration(r.database.WithContext(ctx), record.PostID)
		if err != nil {
			return communityapp.CommentModerationPage{}, mapCommunityError(err)
		}
		page.Comments = append(page.Comments, communityapp.CommentModerationCandidate{Comment: comment, PostAuthorHandle: post.AuthorHandle, PostTitle: post.Title})
	}
	return page, nil
}

func (r *CommunityRepository) DecideComment(ctx context.Context, actorID, commentID string, input communityapp.DecideCommentInput, at time.Time) (communityapp.Comment, error) {
	var result communityapp.Comment
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record commentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", commentID).First(&record).Error; err != nil {
			return communityapp.ErrCommentNotFound
		}
		destination := communityapp.CommentPublished
		if input.Decision == communityapp.ModerationReject {
			destination = communityapp.CommentRejected
		}
		if record.State != string(communityapp.CommentPending) || record.Revision != input.ExpectedRevision {
			if record.State == string(destination) && record.ReasonCode != nil && *record.ReasonCode == input.ReasonCode {
				loaded, err := loadComment(tx, commentID)
				result = loaded
				return err
			}
			return communityapp.ErrPostConflict
		}
		if destination == communityapp.CommentPublished {
			if err := lockVisiblePost(tx, record.PostID, record.AuthorID); err != nil {
				return communityapp.ErrPostConflict
			}
			var active int64
			if err := tx.Model(&userRecord{}).Where("id = ? AND status = 'active'", record.AuthorID).Count(&active).Error; err != nil || active != 1 {
				return communityapp.ErrPostConflict
			}
		}
		if err := tx.Model(&commentRecord{}).Where("id = ? AND revision = ?", commentID, input.ExpectedRevision).Updates(map[string]any{"state": string(destination), "revision": input.ExpectedRevision + 1, "reason_code": input.ReasonCode, "reviewed_at": at, "updated_at": at}).Error; err != nil {
			return err
		}
		if err := tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, PostID: &record.PostID, CommentID: &record.ID, Action: "comment_" + string(input.Decision), ReasonCode: input.ReasonCode, CreatedAt: at}).Error; err != nil {
			return err
		}
		if err := enqueueNotification(tx, record.AuthorID, &actorID, communityapp.NotificationCommentReviewed, &record.PostID, &record.ID, at); err != nil {
			return err
		}
		if destination == communityapp.CommentPublished {
			if err := enqueuePublishedCommentNotification(tx, record, at); err != nil {
				return err
			}
		}
		loaded, loadErr := loadComment(tx, commentID)
		result = loaded
		return loadErr
	})
	return result, mapCommunityError(err)
}

func (r *CommunityRepository) RemoveComment(ctx context.Context, actorID, commentID string, expectedRevision int, reason string, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record commentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", commentID).First(&record).Error; err != nil {
			return communityapp.ErrCommentNotFound
		}
		if record.State != string(communityapp.CommentPublished) || record.Revision != expectedRevision {
			return communityapp.ErrPostConflict
		}
		if err := tx.Model(&commentRecord{}).Where("id = ? AND revision = ?", commentID, expectedRevision).Updates(map[string]any{"body": nil, "state": string(communityapp.CommentRemoved), "revision": expectedRevision + 1, "reason_code": reason, "reviewed_at": at, "updated_at": at}).Error; err != nil {
			return err
		}
		return tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, PostID: &record.PostID, CommentID: &record.ID, Action: "comment_remove", ReasonCode: reason, CreatedAt: at}).Error
	})
	return mapCommunityError(err)
}

func loadComment(database *gorm.DB, commentID string) (communityapp.Comment, error) {
	var record commentRecord
	if err := database.Where("id = ?", commentID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return communityapp.Comment{}, communityapp.ErrCommentNotFound
		}
		return communityapp.Comment{}, err
	}
	var author struct {
		Handle      string
		DisplayName string
	}
	result := database.Table("users u").Select("up.handle, u.display_name").Joins("JOIN user_profiles up ON up.user_id = u.id").Where("u.id = ?", record.AuthorID).Scan(&author)
	if result.Error != nil || result.RowsAffected != 1 {
		return communityapp.Comment{}, communityapp.ErrCommentNotFound
	}
	return communityapp.Comment{ID: record.ID, PostID: record.PostID, ParentID: record.ParentID, AuthorHandle: author.Handle, AuthorDisplayName: author.DisplayName, Body: record.Body, State: communityapp.CommentState(record.State), Revision: record.Revision, ReasonCode: record.ReasonCode, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, nil
}

func loadPublicPostForModeration(database *gorm.DB, postID string) (communityapp.PublicPost, error) {
	var row struct {
		Handle string
		Title  *string
	}
	result := database.Raw(`SELECT up.handle, pr.title FROM posts p JOIN user_profiles up ON up.user_id = p.owner_id JOIN post_revisions pr ON pr.post_id = p.id AND pr.version = p.published_version WHERE p.id = ?`, postID).Scan(&row)
	if result.Error != nil || result.RowsAffected != 1 {
		return communityapp.PublicPost{}, communityapp.ErrPostNotFound
	}
	return communityapp.PublicPost{AuthorHandle: row.Handle, Title: row.Title}, nil
}

func enqueuePublishedCommentNotification(tx *gorm.DB, record commentRecord, at time.Time) error {
	var recipient string
	kind := communityapp.NotificationCommentPublished
	if record.ParentID != nil {
		var parent commentRecord
		if err := tx.Where("id = ?", *record.ParentID).First(&parent).Error; err != nil {
			return err
		}
		recipient = parent.AuthorID
		kind = communityapp.NotificationReplyPublished
	} else {
		var post postRecord
		if err := tx.Where("id = ?", record.PostID).First(&post).Error; err != nil {
			return err
		}
		recipient = post.OwnerID
	}
	if recipient == record.AuthorID {
		return nil
	}
	return enqueueNotification(tx, recipient, &record.AuthorID, kind, &record.PostID, &record.ID, at)
}

func enqueueNotification(tx *gorm.DB, recipientID string, actorID *string, kind communityapp.NotificationKind, postID, commentID *string, at time.Time) error {
	if actorID != nil && *actorID == recipientID && kind != communityapp.NotificationPostReviewed && kind != communityapp.NotificationCommentReviewed && kind != communityapp.NotificationAppealResolved {
		return nil
	}
	eventID := uuid.NewString()
	notificationID := uuid.NewString()
	notification := notificationRecord{ID: notificationID, RecipientID: recipientID, ActorID: actorID, EventID: eventID, Kind: string(kind), PostID: postID, CommentID: commentID, CreatedAt: at}
	if err := tx.Create(&notification).Error; err != nil {
		return err
	}
	return tx.Create(&outboxEventRecord{ID: eventID, EventType: "community.notification_requested", AggregateID: notificationID, Payload: []byte(`{}`), CreatedAt: at}).Error
}

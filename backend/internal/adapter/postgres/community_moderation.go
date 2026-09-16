package postgres

import (
	"context"
	"errors"
	"reflect"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *CommunityRepository) ListModerationCandidates(ctx context.Context, limit int, afterID *string) (communityapp.ModerationCandidatePage, error) {
	query := r.database.WithContext(ctx).Table("posts p").
		Select("p.id").
		Joins("JOIN post_revisions pr ON pr.post_id = p.id AND pr.version = p.pending_version").
		Where("p.pending_version IS NOT NULL AND pr.review_state = ?", string(communityapp.ReviewPending))
	if afterID != nil {
		var cursor postRevisionRecord
		if err := r.database.WithContext(ctx).Where("post_id = ? AND review_state = ?", *afterID, string(communityapp.ReviewPending)).Order("submitted_at DESC").First(&cursor).Error; err != nil {
			return communityapp.ModerationCandidatePage{}, communityapp.ErrPostNotFound
		}
		query = query.Where("(pr.submitted_at, p.id) < (?, ?)", cursor.SubmittedAt, *afterID)
	}
	var ids []string
	if err := query.Order("pr.submitted_at DESC, p.id DESC").Limit(limit+1).Pluck("p.id", &ids).Error; err != nil {
		return communityapp.ModerationCandidatePage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.ModerationCandidatePage{Candidates: make([]communityapp.ModerationCandidate, 0, min(limit, len(ids)))}
	for index, id := range ids {
		if index == limit {
			continuation := ids[index-1]
			page.NextAfterID = &continuation
			break
		}
		var record postRecord
		if err := r.database.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil || record.PendingVersion == nil {
			return communityapp.ModerationCandidatePage{}, communityapp.ErrCommunityUnavailable
		}
		candidate, err := loadModerationCandidate(ctx, r.database, id, *record.PendingVersion)
		if err != nil {
			return communityapp.ModerationCandidatePage{}, mapCommunityError(err)
		}
		page.Candidates = append(page.Candidates, candidate)
	}
	return page, nil
}

func (r *CommunityRepository) GetModerationCandidate(ctx context.Context, postID string, version int) (communityapp.ModerationCandidate, error) {
	candidate, err := loadModerationCandidate(ctx, r.database, postID, version)
	return candidate, mapCommunityError(err)
}

func (r *CommunityRepository) GetModerationImage(ctx context.Context, postID string, version, ordinal int) (communityapp.MediaObjectReference, error) {
	var media postRevisionMediaRecord
	err := r.database.WithContext(ctx).Raw(`
		SELECT prm.* FROM post_revision_media prm
		JOIN posts p ON p.id = prm.post_id AND p.pending_version = prm.version
		JOIN post_revisions pr ON pr.post_id = prm.post_id AND pr.version = prm.version
		WHERE prm.post_id = ? AND prm.version = ? AND prm.ordinal = ? AND pr.review_state = 'pending'
		LIMIT 1`, postID, version, ordinal).Scan(&media).Error
	if err != nil || media.ObjectKey == "" || media.ObjectVersionID == "" {
		return communityapp.MediaObjectReference{}, communityapp.ErrPostNotFound
	}
	return communityapp.MediaObjectReference{ObjectKey: media.ObjectKey, ObjectVersionID: media.ObjectVersionID}, nil
}

func (r *CommunityRepository) DecidePost(ctx context.Context, actorID, postID string, input communityapp.DecidePostInput, at time.Time) (communityapp.Post, error) {
	var result communityapp.Post
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record postRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", postID).First(&record).Error; err != nil {
			return err
		}
		var revision postRevisionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("post_id = ? AND version = ?", postID, input.Version).First(&revision).Error; err != nil {
			return err
		}
		if revision.ReviewedAt != nil {
			expected := communityapp.ReviewApproved
			if input.Decision == communityapp.ModerationReject {
				expected = communityapp.ReviewRejected
			}
			if revision.ReviewState == string(expected) && revision.ReasonCode != nil && *revision.ReasonCode == input.ReasonCode {
				loaded, loadErr := loadOwnPost(ctx, tx, record.OwnerID, postID)
				result = loaded
				return loadErr
			}
			return communityapp.ErrPostConflict
		}
		if record.Revision != input.ExpectedRevision || record.PendingVersion == nil || *record.PendingVersion != input.Version || record.ReviewRound != input.ReviewRound || revision.ReviewState != string(communityapp.ReviewPending) || revision.ReviewRound != input.ReviewRound {
			return communityapp.ErrPostConflict
		}
		reviewState := communityapp.ReviewApproved
		state := communityapp.PostPublished
		updates := map[string]any{"revision": record.Revision + 1, "pending_version": nil, "updated_at": at}
		if input.Decision == communityapp.ModerationApprove {
			updates["state"] = string(state)
			updates["published_version"] = input.Version
			updates["published_at"] = at
			updates["withdrawn_at"] = nil
		} else {
			reviewState = communityapp.ReviewRejected
			updates["draft_version"] = input.Version
			if record.PublishedVersion == nil {
				updates["state"] = string(communityapp.PostDraft)
			} else {
				updates["state"] = string(communityapp.PostPublished)
			}
		}
		if err := tx.Model(&postRevisionRecord{}).Where("post_id = ? AND version = ? AND review_state = ?", postID, input.Version, string(communityapp.ReviewPending)).Updates(map[string]any{"review_state": string(reviewState), "reason_code": input.ReasonCode, "reviewed_at": at}).Error; err != nil {
			return err
		}
		update := tx.Model(&postRecord{}).Where("id = ? AND revision = ?", postID, input.ExpectedRevision).Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return communityapp.ErrPostConflict
		}
		postVersion := input.Version
		if err := tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, PostID: &postID, PostVersion: &postVersion, Action: "post_" + string(input.Decision), ReasonCode: input.ReasonCode, CreatedAt: at}).Error; err != nil {
			return err
		}
		if err := enqueueNotification(tx, record.OwnerID, &actorID, communityapp.NotificationPostReviewed, &postID, nil, at); err != nil {
			return err
		}
		loaded, loadErr := loadOwnPost(ctx, tx, record.OwnerID, postID)
		result = loaded
		return loadErr
	})
	return result, mapCommunityError(err)
}

func (r *CommunityRepository) RemovePost(ctx context.Context, actorID, postID string, expectedRevision int, reason string, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record postRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", postID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.State != string(communityapp.PostPublished) || record.PublishedVersion == nil {
			return communityapp.ErrPostConflict
		}
		if record.PendingVersion != nil {
			if err := tx.Model(&postRevisionRecord{}).Where("post_id = ? AND version = ? AND review_state = ?", postID, *record.PendingVersion, string(communityapp.ReviewPending)).Updates(map[string]any{"review_state": string(communityapp.ReviewCancelled), "reviewed_at": at}).Error; err != nil {
				return err
			}
		}
		updates := map[string]any{"state": string(communityapp.PostRemoved), "revision": record.Revision + 1, "pending_version": nil, "withdrawn_at": at, "updated_at": at}
		if record.PublishedVersion == nil && record.DraftVersion == nil && record.PendingVersion != nil {
			updates["draft_version"] = *record.PendingVersion
		}
		if err := tx.Model(&postRecord{}).Where("id = ? AND revision = ?", postID, expectedRevision).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, PostID: &postID, Action: "post_remove", ReasonCode: reason, CreatedAt: at}).Error
	})
	return mapCommunityError(err)
}

func (r *CommunityRepository) CreateReport(ctx context.Context, reporterID, reportID, postID string, input communityapp.CreateReportInput, at time.Time) (communityapp.ContentReport, error) {
	var result contentReportRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing contentReportRecord
		err := tx.Where("id = ?", reportID).First(&existing).Error
		if err == nil {
			if existing.ReporterID == reporterID && existing.PostID == postID && existing.TargetType == string(input.TargetType) && reflect.DeepEqual(existing.CommentID, input.CommentID) && existing.ReasonCode == input.ReasonCode && reflect.DeepEqual(existing.Detail, input.Detail) {
				result = existing
				return nil
			}
			return communityapp.ErrPostConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var post postRecord
		if err := tx.Raw(`SELECT p.* FROM posts p JOIN users u ON u.id = p.owner_id AND u.status = 'active' WHERE p.id = ? AND p.state = 'published'`, postID).Scan(&post).Error; err != nil || post.ID == "" {
			return communityapp.ErrPostNotFound
		}
		if input.TargetType == communityapp.ReportTargetComment {
			var comment commentRecord
			if err := tx.Where("id = ? AND post_id = ? AND state = ?", *input.CommentID, postID, string(communityapp.CommentPublished)).First(&comment).Error; err != nil {
				return communityapp.ErrCommentNotFound
			}
		}
		var open contentReportRecord
		if err := tx.Where("reporter_id = ? AND post_id = ? AND target_type = ? AND comment_id IS NOT DISTINCT FROM ? AND status = 'open'", reporterID, postID, string(input.TargetType), input.CommentID).First(&open).Error; err == nil {
			result = open
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result = contentReportRecord{ID: reportID, ReporterID: reporterID, PostID: postID, TargetType: string(input.TargetType), CommentID: input.CommentID, ReasonCode: input.ReasonCode, Detail: input.Detail, Status: string(communityapp.ReportOpen), CreatedAt: at}
		return tx.Create(&result).Error
	})
	if err != nil {
		return communityapp.ContentReport{}, mapCommunityError(err)
	}
	return reportFromRecord(result), nil
}

func (r *CommunityRepository) ListOwnReports(ctx context.Context, reporterID string, limit int, afterID *string) (communityapp.ContentReportPage, error) {
	query := r.database.WithContext(ctx).Where("reporter_id = ?", reporterID)
	query, err := applyCreatedCursor(query, r.database.WithContext(ctx), "content_reports", reporterID, "reporter_id", afterID)
	if err != nil {
		return communityapp.ContentReportPage{}, err
	}
	var records []contentReportRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.ContentReportPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.ContentReportPage{Reports: make([]communityapp.ContentReport, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			id := records[index-1].ID
			page.NextAfterID = &id
			break
		}
		page.Reports = append(page.Reports, reportFromRecord(record))
	}
	return page, nil
}

func (r *CommunityRepository) ListReports(ctx context.Context, limit int, afterID *string, status *communityapp.ReportStatus) (communityapp.AdminReportPage, error) {
	query := r.database.WithContext(ctx)
	if status != nil {
		query = query.Where("status = ?", string(*status))
	}
	if afterID != nil {
		var cursor contentReportRecord
		if err := r.database.WithContext(ctx).Select("id", "created_at").Where("id = ?", *afterID).First(&cursor).Error; err != nil {
			return communityapp.AdminReportPage{}, communityapp.ErrReportNotFound
		}
		query = query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []contentReportRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.AdminReportPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.AdminReportPage{Reports: make([]communityapp.AdminReport, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			id := records[index-1].ID
			page.NextAfterID = &id
			break
		}
		page.Reports = append(page.Reports, communityapp.AdminReport{ContentReport: reportFromRecord(record), ReporterID: record.ReporterID})
	}
	return page, nil
}

func (r *CommunityRepository) ResolveReport(ctx context.Context, actorID, reportID string, status communityapp.ReportStatus, reason string, at time.Time) (communityapp.ContentReport, error) {
	var result contentReportRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", reportID).First(&result).Error; err != nil {
			return err
		}
		if result.Status != string(communityapp.ReportOpen) {
			if result.Status == string(status) && result.ResolutionCode != nil && *result.ResolutionCode == reason {
				return nil
			}
			return communityapp.ErrPostConflict
		}
		if err := tx.Model(&result).Updates(map[string]any{"status": string(status), "resolution_code": reason, "resolved_at": at}).Error; err != nil {
			return err
		}
		result.Status, result.ResolutionCode, result.ResolvedAt = string(status), &reason, &at
		return tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, PostID: &result.PostID, CommentID: result.CommentID, ReportID: &reportID, Action: "report_" + string(status), ReasonCode: reason, CreatedAt: at}).Error
	})
	if err != nil {
		return communityapp.ContentReport{}, mapCommunityError(err)
	}
	return reportFromRecord(result), nil
}

func (r *CommunityRepository) ListModerationActions(ctx context.Context, limit int, afterID *string) (communityapp.ModerationActionPage, error) {
	query := r.database.WithContext(ctx)
	if afterID != nil {
		var cursor moderationActionRecord
		if err := r.database.WithContext(ctx).Select("id", "created_at").Where("id = ?", *afterID).First(&cursor).Error; err != nil {
			return communityapp.ModerationActionPage{}, communityapp.ErrPostNotFound
		}
		query = query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []moderationActionRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.ModerationActionPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.ModerationActionPage{Actions: make([]communityapp.ModerationAction, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			id := records[index-1].ID
			page.NextAfterID = &id
			break
		}
		page.Actions = append(page.Actions, communityapp.ModerationAction{ID: record.ID, ActorID: record.ActorID, PostID: record.PostID, PostVersion: record.PostVersion, CommentID: record.CommentID, ReportID: record.ReportID, SubjectUserID: record.SubjectUserID, Action: record.Action, ReasonCode: record.ReasonCode, CreatedAt: record.CreatedAt})
	}
	return page, nil
}

func (r *CommunityRepository) ListUsers(ctx context.Context, limit int, afterID *string) (communityapp.AdminUserPage, error) {
	query := r.database.WithContext(ctx)
	if afterID != nil {
		var cursor userRecord
		if err := r.database.WithContext(ctx).Select("id", "created_at").Where("id = ?", *afterID).First(&cursor).Error; err != nil {
			return communityapp.AdminUserPage{}, communityapp.ErrPostNotFound
		}
		query = query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []userRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.AdminUserPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.AdminUserPage{Users: make([]communityapp.AdminUser, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			id := records[index-1].ID
			page.NextAfterID = &id
			break
		}
		page.Users = append(page.Users, communityapp.AdminUser{ID: record.ID, DisplayName: record.DisplayName, Status: accountapp.AccountStatus(record.Status), Role: accountapp.AccountRole(record.Role), Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
	}
	return page, nil
}

func (r *CommunityRepository) SetUserStatus(ctx context.Context, actorID, userID string, expectedRevision int, status accountapp.AccountStatus, reason string, at time.Time) (communityapp.AdminUser, error) {
	var result userRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&result).Error; err != nil {
			return err
		}
		if result.Revision != expectedRevision {
			return communityapp.ErrPostConflict
		}
		if result.Status == string(status) {
			return nil
		}
		nextRevision := result.Revision + 1
		if err := tx.Model(&userRecord{}).Where("id = ? AND revision = ?", userID, expectedRevision).Updates(map[string]any{"status": string(status), "revision": nextRevision, "updated_at": at}).Error; err != nil {
			return err
		}
		result.Status, result.Revision, result.UpdatedAt = string(status), nextRevision, at
		if status == accountapp.AccountSuspended {
			if err := tx.Where("user_id = ?", userID).Delete(&sessionRecord{}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, SubjectUserID: &userID, Action: "user_" + string(status), ReasonCode: reason, CreatedAt: at}).Error
	})
	if err != nil {
		return communityapp.AdminUser{}, mapCommunityError(err)
	}
	return communityapp.AdminUser{ID: result.ID, DisplayName: result.DisplayName, Status: accountapp.AccountStatus(result.Status), Role: accountapp.AccountRole(result.Role), Revision: result.Revision, CreatedAt: result.CreatedAt, UpdatedAt: result.UpdatedAt}, nil
}

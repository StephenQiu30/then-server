package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *CommunityRepository) GetPublicPostForViewer(ctx context.Context, postID string, viewerID *string) (communityapp.PublicPost, error) {
	post, err := loadPublicPostForViewer(ctx, r.database, postID, stringValue(viewerID))
	return post, mapCommunityError(err)
}

func (r *CommunityRepository) GetPublicPostImageForViewer(ctx context.Context, postID string, ordinal int, viewerID *string) (communityapp.MediaObjectReference, error) {
	if _, err := loadPublicPostForViewer(ctx, r.database, postID, stringValue(viewerID)); err != nil {
		return communityapp.MediaObjectReference{}, mapCommunityError(err)
	}
	var media postRevisionMediaRecord
	err := r.database.WithContext(ctx).Raw(`
		SELECT prm.* FROM post_revision_media prm
		JOIN posts p ON p.id = prm.post_id AND p.published_version = prm.version
		WHERE p.id = ? AND p.state = 'published' AND prm.ordinal = ?
		LIMIT 1`, postID, ordinal).Scan(&media).Error
	if err != nil || media.ObjectKey == "" || media.ObjectVersionID == "" {
		return communityapp.MediaObjectReference{}, communityapp.ErrPostNotFound
	}
	return communityapp.MediaObjectReference{ObjectKey: media.ObjectKey, ObjectVersionID: media.ObjectVersionID}, nil
}

func (r *CommunityRepository) ListFeed(ctx context.Context, feedType communityapp.FeedType, viewerID *string, limit int, afterID *string) (communityapp.PublicPostPage, error) {
	viewer := stringValue(viewerID)
	query := publicPostIDsQuery(r.database.WithContext(ctx), viewer)
	if feedType == communityapp.FeedFollowing {
		query = query.Where("EXISTS (SELECT 1 FROM user_follows uf WHERE uf.follower_id = ? AND uf.followee_id = p.owner_id)", viewer)
	}
	return r.publicPostPage(ctx, query, viewer, limit, afterID, "p.published_at DESC, p.id DESC")
}

func (r *CommunityRepository) SearchPosts(ctx context.Context, viewerID, queryText, tag *string, limit int, afterID *string) (communityapp.PublicPostPage, error) {
	viewer := stringValue(viewerID)
	query := publicPostIDsQuery(r.database.WithContext(ctx), viewer).Joins("JOIN post_revisions pr ON pr.post_id = p.id AND pr.version = p.published_version")
	if queryText != nil {
		pattern := "%" + escapeLike(*queryText) + "%"
		query = query.Where("COALESCE(pr.title, '') ILIKE ? ESCAPE '\\' OR COALESCE(pr.body, '') ILIKE ? ESCAPE '\\'", pattern, pattern)
	}
	if tag != nil {
		query = query.Where("EXISTS (SELECT 1 FROM post_revision_tags prt WHERE prt.post_id = p.id AND prt.version = p.published_version AND prt.tag = ?)", *tag)
	}
	return r.publicPostPage(ctx, query, viewer, limit, afterID, "p.published_at DESC, p.id DESC")
}

func (r *CommunityRepository) ListProfilePosts(ctx context.Context, handle string, viewerID *string, limit int, afterID *string) (communityapp.PublicPostPage, error) {
	viewer := stringValue(viewerID)
	query := publicPostIDsQuery(r.database.WithContext(ctx), viewer).Where("up.handle = ?", handle)
	return r.publicPostPage(ctx, query, viewer, limit, afterID, "p.published_at DESC, p.id DESC")
}

func (r *CommunityRepository) publicPostPage(ctx context.Context, query *gorm.DB, viewer string, limit int, afterID *string, order string) (communityapp.PublicPostPage, error) {
	if afterID != nil {
		var cursor postRecord
		if err := query.Session(&gorm.Session{}).Select("p.id", "p.published_at").Where("p.id = ?", *afterID).Scan(&cursor).Error; err != nil || cursor.ID == "" || cursor.PublishedAt == nil {
			return communityapp.PublicPostPage{}, communityapp.ErrPostNotFound
		}
		query = query.Where("(p.published_at, p.id) < (?, ?)", *cursor.PublishedAt, cursor.ID)
	}
	var ids []string
	if err := query.Select("p.id").Order(order).Limit(limit+1).Pluck("p.id", &ids).Error; err != nil {
		return communityapp.PublicPostPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.PublicPostPage{Posts: make([]communityapp.PublicPost, 0, min(limit, len(ids)))}
	for index, id := range ids {
		if index == limit {
			cursor := ids[index-1]
			page.NextAfterID = &cursor
			break
		}
		post, err := loadPublicPostForViewer(ctx, r.database, id, viewer)
		if err != nil {
			return communityapp.PublicPostPage{}, mapCommunityError(err)
		}
		page.Posts = append(page.Posts, post)
	}
	return page, nil
}

func publicPostIDsQuery(database *gorm.DB, viewer string) *gorm.DB {
	query := database.Table("posts p").
		Joins("JOIN users u ON u.id = p.owner_id AND u.status = 'active'").
		Joins("JOIN user_profiles up ON up.user_id = p.owner_id").
		Where("p.state = 'published' AND p.published_version IS NOT NULL")
	if viewer != "" {
		query = query.Where(`NOT EXISTS (
			SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id = ? AND ub.blocked_id = p.owner_id)
			   OR (ub.blocker_id = p.owner_id AND ub.blocked_id = ?)
		)`, viewer, viewer)
	}
	return query
}

func loadPublicPostForViewer(ctx context.Context, database *gorm.DB, postID, viewer string) (communityapp.PublicPost, error) {
	var viewerUUID any
	if viewer != "" {
		viewerUUID = viewer
	}
	var row struct {
		PostID           string
		OwnerID          string
		Handle           string
		DisplayName      string
		Title            *string
		Body             *string
		PublishedVersion int
		PublishedAt      time.Time
		LikeCount        int64
		CommentCount     int64
		ViewerLiked      bool
		ViewerBookmarked bool
		FollowingAuthor  bool
	}
	result := database.WithContext(ctx).Raw(`
		SELECT p.id AS post_id, p.owner_id, up.handle, u.display_name, pr.title, pr.body,
		       p.published_version, p.published_at,
		       (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id) AS like_count,
		       (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.state IN ('published','deleted','removed')) AS comment_count,
		       (?::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = ?::uuid)) AS viewer_liked,
		       (?::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM post_bookmarks pb WHERE pb.post_id = p.id AND pb.user_id = ?::uuid)) AS viewer_bookmarked,
		       (?::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM user_follows uf WHERE uf.followee_id = p.owner_id AND uf.follower_id = ?::uuid)) AS following_author
		FROM posts p
		JOIN users u ON u.id = p.owner_id AND u.status = 'active'
		JOIN user_profiles up ON up.user_id = p.owner_id
		JOIN post_revisions pr ON pr.post_id = p.id AND pr.version = p.published_version
		WHERE p.id = ? AND p.state = 'published' AND p.published_version IS NOT NULL
		  AND (?::uuid IS NULL OR NOT EXISTS (
		      SELECT 1 FROM user_blocks ub
		      WHERE (ub.blocker_id = ?::uuid AND ub.blocked_id = p.owner_id)
		         OR (ub.blocker_id = p.owner_id AND ub.blocked_id = ?::uuid)
		  ))
		LIMIT 1`, viewerUUID, viewerUUID, viewerUUID, viewerUUID, viewerUUID, viewerUUID, postID, viewerUUID, viewerUUID, viewerUUID).Scan(&row)
	if result.Error != nil {
		return communityapp.PublicPost{}, result.Error
	}
	if result.RowsAffected != 1 {
		return communityapp.PublicPost{}, communityapp.ErrPostNotFound
	}
	media, tags, err := loadRevisionCollections(database.WithContext(ctx), postID, row.PublishedVersion)
	if err != nil {
		return communityapp.PublicPost{}, err
	}
	return communityapp.PublicPost{ID: row.PostID, AuthorHandle: row.Handle, AuthorDisplayName: row.DisplayName, Title: row.Title, Body: row.Body, Tags: tags, ImageCount: len(media), LikeCount: row.LikeCount, CommentCount: row.CommentCount, ViewerLiked: row.ViewerLiked, ViewerBookmarked: row.ViewerBookmarked, FollowingAuthor: row.FollowingAuthor, PublishedVersion: row.PublishedVersion, PublishedAt: row.PublishedAt}, nil
}

func (r *CommunityRepository) SetPostLike(ctx context.Context, userID, postID string, enabled bool, at time.Time) error {
	return r.setPostRelation(ctx, "post_likes", userID, postID, enabled, at)
}

func (r *CommunityRepository) SetPostBookmark(ctx context.Context, userID, postID string, enabled bool, at time.Time) error {
	return r.setPostRelation(ctx, "post_bookmarks", userID, postID, enabled, at)
}

func (r *CommunityRepository) setPostRelation(ctx context.Context, table, userID, postID string, enabled bool, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !enabled {
			return tx.Exec("DELETE FROM "+table+" WHERE user_id = ? AND post_id = ?", userID, postID).Error
		}
		if err := lockVisiblePost(tx, postID, userID); err != nil {
			return err
		}
		record := map[string]any{"user_id": userID, "post_id": postID, "created_at": at}
		return tx.Table(table).Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error
	})
	return mapCommunityError(err)
}

func (r *CommunityRepository) ListBookmarks(ctx context.Context, userID string, limit int, afterID *string) (communityapp.PublicPostPage, error) {
	query := publicPostIDsQuery(r.database.WithContext(ctx), userID).Joins("JOIN post_bookmarks pb ON pb.post_id = p.id AND pb.user_id = ?", userID)
	if afterID != nil {
		var cursor postBookmarkRecord
		if err := r.database.WithContext(ctx).Where("user_id = ? AND post_id = ?", userID, *afterID).First(&cursor).Error; err != nil {
			return communityapp.PublicPostPage{}, communityapp.ErrPostNotFound
		}
		query = query.Where("(pb.created_at, pb.post_id) < (?, ?)", cursor.CreatedAt, cursor.PostID)
	}
	var ids []string
	if err := query.Select("p.id").Order("pb.created_at DESC, pb.post_id DESC").Limit(limit+1).Pluck("p.id", &ids).Error; err != nil {
		return communityapp.PublicPostPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.PublicPostPage{Posts: make([]communityapp.PublicPost, 0, min(limit, len(ids)))}
	for index, id := range ids {
		if index == limit {
			cursor := ids[index-1]
			page.NextAfterID = &cursor
			break
		}
		post, err := loadPublicPostForViewer(ctx, r.database, id, userID)
		if err != nil {
			return communityapp.PublicPostPage{}, mapCommunityError(err)
		}
		page.Posts = append(page.Posts, post)
	}
	return page, nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *CommunityRepository) SetFollow(ctx context.Context, followerID, handle string, enabled bool, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target struct{ UserID string }
		if err := tx.Table("user_profiles up").Select("up.user_id").Joins("JOIN users u ON u.id = up.user_id AND u.status = 'active'").Where("up.handle = ?", handle).Scan(&target).Error; err != nil || target.UserID == "" {
			return communityapp.ErrPostNotFound
		}
		if target.UserID == followerID {
			return communityapp.ErrPostConflict
		}
		if err := lockUsers(tx, followerID, target.UserID); err != nil {
			return err
		}
		if !enabled {
			return tx.Where("follower_id = ? AND followee_id = ?", followerID, target.UserID).Delete(&userFollowRecord{}).Error
		}
		blocked, err := blockedBetween(tx, followerID, target.UserID)
		if err != nil {
			return err
		}
		if blocked {
			return communityapp.ErrPostConflict
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&userFollowRecord{FollowerID: followerID, FolloweeID: target.UserID, CreatedAt: at})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			return enqueueNotification(tx, target.UserID, &followerID, communityapp.NotificationFollowed, nil, nil, at)
		}
		return nil
	})
	return mapCommunityError(err)
}

func (r *CommunityRepository) ListProfileRelationships(ctx context.Context, handle, direction string, viewerID *string, limit int, afterID *string) (communityapp.PublicProfilePage, error) {
	viewer := stringValue(viewerID)
	var owner struct{ UserID string }
	if err := r.database.WithContext(ctx).Table("user_profiles up").Select("up.user_id").Joins("JOIN users u ON u.id = up.user_id AND u.status = 'active'").Where("up.handle = ?", handle).Scan(&owner).Error; err != nil || owner.UserID == "" {
		return communityapp.PublicProfilePage{}, communityapp.ErrPostNotFound
	}
	query := r.database.WithContext(ctx).Table("user_follows uf")
	if direction == "followers" {
		query = query.Select("uf.follower_id AS user_id", "uf.created_at").Where("uf.followee_id = ?", owner.UserID)
	} else {
		query = query.Select("uf.followee_id AS user_id", "uf.created_at").Where("uf.follower_id = ?", owner.UserID)
	}
	if viewer != "" {
		identifier := "uf.follower_id"
		if direction == "following" {
			identifier = "uf.followee_id"
		}
		query = query.Where("NOT EXISTS (SELECT 1 FROM user_blocks ub WHERE (ub.blocker_id = ? AND ub.blocked_id = "+identifier+") OR (ub.blocker_id = "+identifier+" AND ub.blocked_id = ?))", viewer, viewer)
	}
	if afterID != nil {
		cursorQuery := r.database.WithContext(ctx).Model(&userFollowRecord{})
		if direction == "followers" {
			cursorQuery = cursorQuery.Where("followee_id = ? AND follower_id = ?", owner.UserID, *afterID)
		} else {
			cursorQuery = cursorQuery.Where("follower_id = ? AND followee_id = ?", owner.UserID, *afterID)
		}
		var cursor userFollowRecord
		if err := cursorQuery.First(&cursor).Error; err != nil {
			return communityapp.PublicProfilePage{}, communityapp.ErrPostNotFound
		}
		identifier := "uf.follower_id"
		if direction == "following" {
			identifier = "uf.followee_id"
		}
		query = query.Where("(uf.created_at, "+identifier+") < (?, ?)", cursor.CreatedAt, *afterID)
	}
	var rows []struct {
		UserID    string
		CreatedAt time.Time
	}
	identifier := "uf.follower_id"
	if direction == "following" {
		identifier = "uf.followee_id"
	}
	if err := query.Order("uf.created_at DESC, " + identifier + " DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return communityapp.PublicProfilePage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.PublicProfilePage{Profiles: make([]communityapp.PublicProfileSummary, 0, min(limit, len(rows)))}
	for index, row := range rows {
		if index == limit {
			cursor := rows[index-1].UserID
			page.NextAfterID = &cursor
			break
		}
		profile, err := loadPublicProfileSummary(r.database.WithContext(ctx), row.UserID, viewer)
		if err != nil {
			return communityapp.PublicProfilePage{}, mapCommunityError(err)
		}
		page.Profiles = append(page.Profiles, profile)
	}
	return page, nil
}

func (r *CommunityRepository) SetBlock(ctx context.Context, blockerID, blockedID string, enabled bool, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUsers(tx, blockerID, blockedID); err != nil {
			return err
		}
		if !enabled {
			return tx.Where("blocker_id = ? AND blocked_id = ?", blockerID, blockedID).Delete(&userBlockRecord{}).Error
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&userBlockRecord{BlockerID: blockerID, BlockedID: blockedID, CreatedAt: at}).Error; err != nil {
			return err
		}
		return tx.Where("(follower_id = ? AND followee_id = ?) OR (follower_id = ? AND followee_id = ?)", blockerID, blockedID, blockedID, blockerID).Delete(&userFollowRecord{}).Error
	})
	return mapCommunityError(err)
}

func (r *CommunityRepository) ListBlocks(ctx context.Context, blockerID string, limit int, afterID *string) (communityapp.BlockedUserPage, error) {
	query := r.database.WithContext(ctx).Table("user_blocks ub").
		Select("ub.blocked_id AS user_id", "up.handle", "u.display_name", "ub.created_at AS blocked_at").
		Joins("JOIN users u ON u.id = ub.blocked_id").
		Joins("JOIN user_profiles up ON up.user_id = ub.blocked_id").
		Where("ub.blocker_id = ?", blockerID)
	if afterID != nil {
		var cursor userBlockRecord
		if err := r.database.WithContext(ctx).Where("blocker_id = ? AND blocked_id = ?", blockerID, *afterID).First(&cursor).Error; err != nil {
			return communityapp.BlockedUserPage{}, communityapp.ErrPostNotFound
		}
		query = query.Where("(ub.created_at, ub.blocked_id) < (?, ?)", cursor.CreatedAt, cursor.BlockedID)
	}
	var rows []struct {
		UserID      string
		Handle      string
		DisplayName string
		BlockedAt   time.Time
	}
	if err := query.Order("ub.created_at DESC, ub.blocked_id DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return communityapp.BlockedUserPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.BlockedUserPage{Users: make([]communityapp.BlockedUser, 0, min(limit, len(rows)))}
	for index, row := range rows {
		if index == limit {
			cursor := rows[index-1].UserID
			page.NextAfterID = &cursor
			break
		}
		page.Users = append(page.Users, communityapp.BlockedUser{UserID: row.UserID, Handle: row.Handle, DisplayName: row.DisplayName, BlockedAt: row.BlockedAt})
	}
	return page, nil
}

func lockUsers(tx *gorm.DB, userIDs ...string) error {
	if len(userIDs) != 2 || userIDs[0] == userIDs[1] {
		return communityapp.ErrPostConflict
	}
	ids := append([]string(nil), userIDs...)
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	var records []userRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ? AND status = 'active'", ids).Order("id ASC").Find(&records).Error; err != nil {
		return err
	}
	if len(records) != 2 {
		return communityapp.ErrPostNotFound
	}
	return nil
}

func blockedBetween(tx *gorm.DB, firstID, secondID string) (bool, error) {
	var count int64
	if err := tx.Model(&userBlockRecord{}).Where("(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)", firstID, secondID, secondID, firstID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func lockVisiblePost(tx *gorm.DB, postID, viewerID string) error {
	var post postRecord
	result := tx.Raw(`
		SELECT p.* FROM posts p
		JOIN users u ON u.id = p.owner_id AND u.status = 'active'
		WHERE p.id = ? AND p.state = 'published'
		  AND NOT EXISTS (
			SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id = ? AND ub.blocked_id = p.owner_id)
			   OR (ub.blocker_id = p.owner_id AND ub.blocked_id = ?)
		  )
		FOR UPDATE`, postID, viewerID, viewerID).Scan(&post)
	if result.Error != nil {
		return result.Error
	}
	if post.ID == "" {
		return communityapp.ErrPostNotFound
	}
	return nil
}

func loadPublicProfileSummary(database *gorm.DB, userID, viewer string) (communityapp.PublicProfileSummary, error) {
	var viewerUUID any
	if viewer != "" {
		viewerUUID = viewer
	}
	var row struct {
		Handle         string
		DisplayName    string
		Bio            *string
		Following      bool
		FollowerCount  int64
		FollowingCount int64
	}
	result := database.Raw(`
		SELECT up.handle, u.display_name, up.bio,
		       (?::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM user_follows uf WHERE uf.follower_id = ?::uuid AND uf.followee_id = u.id)) AS following,
		       (SELECT COUNT(*) FROM user_follows uf WHERE uf.followee_id = u.id) AS follower_count,
		       (SELECT COUNT(*) FROM user_follows uf WHERE uf.follower_id = u.id) AS following_count
		FROM users u JOIN user_profiles up ON up.user_id = u.id
		WHERE u.id = ? AND u.status = 'active' LIMIT 1`, viewerUUID, viewerUUID, userID).Scan(&row)
	if result.Error != nil || result.RowsAffected != 1 {
		return communityapp.PublicProfileSummary{}, communityapp.ErrPostNotFound
	}
	return communityapp.PublicProfileSummary{Handle: row.Handle, DisplayName: row.DisplayName, Bio: row.Bio, Following: row.Following, FollowerCount: row.FollowerCount, FollowingCount: row.FollowingCount}, nil
}

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

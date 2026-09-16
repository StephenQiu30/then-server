package postgres

import (
	"context"
	"strings"
	"time"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

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

package postgres

import (
	"context"
	"time"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

package postgres

import (
	"context"
	"errors"
	"time"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *CommunityRepository) CreatePost(ctx context.Context, ownerID, postID string, input communityapp.PostContentInput, at time.Time) (communityapp.Post, error) {
	var result communityapp.Post
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing postRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", postID).First(&existing).Error
		if err == nil {
			if existing.OwnerID != ownerID || existing.State == string(communityapp.PostDeleted) {
				return communityapp.ErrPostConflict
			}
			current, loadErr := loadRevision(ctx, tx, existing.ID, dereferenceVersion(existing.DraftVersion, existing.PendingVersion, existing.PublishedVersion))
			if loadErr != nil || existing.Revision != 1 || !postInputEqual(input, existing.SourceDiaryID, current) {
				return communityapp.ErrPostConflict
			}
			result, loadErr = loadOwnPost(ctx, tx, ownerID, postID)
			return loadErr
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := validatePostReferences(tx, ownerID, input); err != nil {
			return err
		}
		version := 1
		var sourceOwnerID *string
		if input.SourceDiaryID != nil {
			sourceOwnerID = &ownerID
		}
		record := postRecord{ID: postID, OwnerID: ownerID, State: string(communityapp.PostDraft), Revision: 1, DraftVersion: &version, SourceDiaryOwnerID: sourceOwnerID, SourceDiaryID: input.SourceDiaryID, CreatedAt: at, UpdatedAt: at}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := createPostRevision(tx, postID, version, input, communityapp.ReviewDraft, 0, at); err != nil {
			return err
		}
		loaded, err := loadOwnPost(ctx, tx, ownerID, postID)
		result = loaded
		return err
	})
	return result, mapCommunityError(err)
}

func (r *CommunityRepository) ListOwnPosts(ctx context.Context, ownerID string, limit int, afterID *string, state *communityapp.PostState) (communityapp.PostPage, error) {
	query := r.database.WithContext(ctx).Where("owner_id = ? AND state <> ?", ownerID, string(communityapp.PostDeleted))
	if state != nil {
		query = query.Where("state = ?", string(*state))
	}
	if afterID != nil {
		var cursor postRecord
		if err := r.database.WithContext(ctx).Select("id", "created_at").Where("id = ? AND owner_id = ?", *afterID, ownerID).First(&cursor).Error; err != nil {
			return communityapp.PostPage{}, communityapp.ErrPostNotFound
		}
		query = query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []postRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return communityapp.PostPage{}, communityapp.ErrCommunityUnavailable
	}
	page := communityapp.PostPage{Posts: make([]communityapp.Post, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			id := records[index-1].ID
			page.NextAfterID = &id
			break
		}
		post, err := loadOwnPost(ctx, r.database, ownerID, record.ID)
		if err != nil {
			return communityapp.PostPage{}, mapCommunityError(err)
		}
		page.Posts = append(page.Posts, post)
	}
	return page, nil
}

func (r *CommunityRepository) GetOwnPost(ctx context.Context, ownerID, postID string) (communityapp.Post, error) {
	post, err := loadOwnPost(ctx, r.database, ownerID, postID)
	return post, mapCommunityError(err)
}

func (r *CommunityRepository) UpdatePost(ctx context.Context, ownerID, postID string, expectedRevision int, input communityapp.PostContentInput, at time.Time) (communityapp.Post, error) {
	var result communityapp.Post
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := lockOwnedPost(tx, ownerID, postID)
		if err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.PendingVersion != nil || record.State == string(communityapp.PostDeleted) || record.State == string(communityapp.PostRemoved) {
			return communityapp.ErrPostConflict
		}
		if err := validatePostReferences(tx, ownerID, input); err != nil {
			return err
		}
		version, err := nextPostVersion(tx, postID)
		if err != nil {
			return err
		}
		if err := createPostRevision(tx, postID, version, input, communityapp.ReviewDraft, 0, at); err != nil {
			return err
		}
		state := communityapp.PostDraft
		if record.PublishedVersion != nil {
			state = communityapp.PostPublished
		}
		var sourceOwnerID *string
		if input.SourceDiaryID != nil {
			sourceOwnerID = &ownerID
		}
		updates := map[string]any{"state": string(state), "revision": record.Revision + 1, "draft_version": version, "source_diary_owner_id": sourceOwnerID, "source_diary_id": input.SourceDiaryID, "updated_at": at, "withdrawn_at": nil}
		if err := tx.Model(&postRecord{}).Where("id = ? AND revision = ?", postID, expectedRevision).Updates(updates).Error; err != nil {
			return err
		}
		result, err = loadOwnPost(ctx, tx, ownerID, postID)
		return err
	})
	return result, mapCommunityError(err)
}

func (r *CommunityRepository) SubmitPost(ctx context.Context, ownerID, postID string, expectedRevision int, at time.Time) (communityapp.Post, error) {
	var result communityapp.Post
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := lockOwnedPost(tx, ownerID, postID)
		if err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.DraftVersion == nil || record.PendingVersion != nil || record.State == string(communityapp.PostDeleted) || record.State == string(communityapp.PostRemoved) {
			return communityapp.ErrPostConflict
		}
		var profile userProfileRecord
		if err := tx.Where("user_id = ?", ownerID).First(&profile).Error; err != nil {
			return communityapp.ErrCommunityForbidden
		}
		if err := freezeRevisionMedia(tx, ownerID, postID, *record.DraftVersion); err != nil {
			return err
		}
		round := record.ReviewRound + 1
		if err := tx.Model(&postRevisionRecord{}).Where("post_id = ? AND version = ? AND review_state IN ?", postID, *record.DraftVersion, []string{string(communityapp.ReviewDraft), string(communityapp.ReviewRejected)}).Updates(map[string]any{"review_state": string(communityapp.ReviewPending), "review_round": round, "reason_code": nil, "submitted_at": at, "reviewed_at": nil}).Error; err != nil {
			return err
		}
		state := communityapp.PostPending
		if record.PublishedVersion != nil {
			state = communityapp.PostPublished
		}
		updates := map[string]any{"state": string(state), "revision": record.Revision + 1, "draft_version": nil, "pending_version": *record.DraftVersion, "review_round": round, "updated_at": at}
		resultUpdate := tx.Model(&postRecord{}).Where("id = ? AND revision = ?", postID, expectedRevision).Updates(updates)
		if resultUpdate.Error != nil {
			return resultUpdate.Error
		}
		if resultUpdate.RowsAffected != 1 {
			return communityapp.ErrPostConflict
		}
		result, err = loadOwnPost(ctx, tx, ownerID, postID)
		return err
	})
	return result, mapCommunityError(err)
}

func (r *CommunityRepository) WithdrawPost(ctx context.Context, ownerID, postID string, expectedRevision int, at time.Time) (communityapp.Post, error) {
	var result communityapp.Post
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := lockOwnedPost(tx, ownerID, postID)
		if err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.State == string(communityapp.PostDeleted) || record.State == string(communityapp.PostRemoved) || (record.PendingVersion == nil && record.PublishedVersion == nil) {
			return communityapp.ErrPostConflict
		}
		sourceVersion := dereferenceVersion(record.PendingVersion, record.DraftVersion, record.PublishedVersion)
		if sourceVersion < 1 {
			return communityapp.ErrPostConflict
		}
		if record.PendingVersion != nil {
			if err := tx.Model(&postRevisionRecord{}).Where("post_id = ? AND version = ? AND review_state = ?", postID, *record.PendingVersion, string(communityapp.ReviewPending)).Updates(map[string]any{"review_state": string(communityapp.ReviewCancelled), "reviewed_at": at}).Error; err != nil {
				return err
			}
		}
		content, err := loadRevision(ctx, tx, postID, sourceVersion)
		if err != nil {
			return err
		}
		version, err := nextPostVersion(tx, postID)
		if err != nil {
			return err
		}
		input := communityapp.PostContentInput{Title: content.Title, Body: content.Body, SourceDiaryID: record.SourceDiaryID, MediaIDs: content.MediaIDs, Tags: content.Tags}
		if err := createPostRevision(tx, postID, version, input, communityapp.ReviewDraft, 0, at); err != nil {
			return err
		}
		updates := map[string]any{"state": string(communityapp.PostWithdrawn), "revision": record.Revision + 1, "draft_version": version, "pending_version": nil, "published_version": nil, "published_at": nil, "withdrawn_at": at, "updated_at": at}
		if err := tx.Model(&postRecord{}).Where("id = ? AND revision = ?", postID, expectedRevision).Updates(updates).Error; err != nil {
			return err
		}
		result, err = loadOwnPost(ctx, tx, ownerID, postID)
		return err
	})
	return result, mapCommunityError(err)
}

func (r *CommunityRepository) DeletePost(ctx context.Context, ownerID, postID string, expectedRevision int, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := lockOwnedPost(tx, ownerID, postID)
		if err != nil {
			return err
		}
		if record.State == string(communityapp.PostDeleted) {
			return nil
		}
		if record.Revision != expectedRevision {
			return communityapp.ErrPostConflict
		}
		if err := tx.Where("post_id = ?", postID).Delete(&postRevisionRecord{}).Error; err != nil {
			return err
		}
		updates := map[string]any{"state": string(communityapp.PostDeleted), "revision": record.Revision + 1, "draft_version": nil, "pending_version": nil, "published_version": nil, "source_diary_owner_id": nil, "source_diary_id": nil, "published_at": nil, "withdrawn_at": at, "deleted_at": at, "updated_at": at}
		return tx.Model(&postRecord{}).Where("id = ? AND revision = ?", postID, expectedRevision).Updates(updates).Error
	})
	return mapCommunityError(err)
}

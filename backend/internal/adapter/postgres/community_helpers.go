package postgres

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"time"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func lockOwnedPost(tx *gorm.DB, ownerID, postID string) (postRecord, error) {
	var record postRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", postID, ownerID).First(&record).Error; err != nil {
		return postRecord{}, err
	}
	return record, nil
}

func validatePostReferences(tx *gorm.DB, ownerID string, input communityapp.PostContentInput) error {
	if input.SourceDiaryID != nil {
		var count int64
		if err := tx.Model(&diaryEntryRecord{}).Where("id = ? AND owner_id = ?", *input.SourceDiaryID, ownerID).Count(&count).Error; err != nil || count != 1 {
			return communityapp.ErrPostNotFound
		}
	}
	if len(input.MediaIDs) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&mediaAssetRecord{}).Where("id IN ? AND owner_id = ? AND purpose = ? AND category = ? AND status NOT IN ?", input.MediaIDs, ownerID, mediaapp.MediaPurposeCommunityPublish, mediaapp.MediaCategoryOrdinaryImage, []string{string(mediaapp.MediaDeleting), string(mediaapp.MediaDeleted)}).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != len(input.MediaIDs) {
		return communityapp.ErrPostNotFound
	}
	return nil
}

func createPostRevision(tx *gorm.DB, postID string, version int, input communityapp.PostContentInput, state communityapp.ReviewState, round int, at time.Time) error {
	revision := postRevisionRecord{PostID: postID, Version: version, Title: input.Title, Body: input.Body, ReviewState: string(state), ReviewRound: round, CreatedAt: at}
	if err := tx.Create(&revision).Error; err != nil {
		return err
	}
	for ordinal, mediaID := range input.MediaIDs {
		if err := tx.Create(&postRevisionMediaRecord{PostID: postID, Version: version, Ordinal: ordinal, MediaID: mediaID}).Error; err != nil {
			return err
		}
	}
	for _, tag := range input.Tags {
		if err := tx.Create(&postRevisionTagRecord{PostID: postID, Version: version, Tag: tag}).Error; err != nil {
			return err
		}
	}
	return nil
}

func freezeRevisionMedia(tx *gorm.DB, ownerID, postID string, version int) error {
	var links []postRevisionMediaRecord
	if err := tx.Where("post_id = ? AND version = ?", postID, version).Order("ordinal ASC").Find(&links).Error; err != nil {
		return err
	}
	for _, link := range links {
		var derivation mediaDerivationRecord
		err := tx.Table("media_derivations md").Select("md.*").Joins("JOIN media_assets ma ON ma.id = md.media_id").Where("md.media_id = ? AND ma.owner_id = ? AND ma.purpose = ? AND ma.category = ? AND ma.status = ?", link.MediaID, ownerID, mediaapp.MediaPurposeCommunityPublish, mediaapp.MediaCategoryOrdinaryImage, string(mediaapp.MediaReady)).Order("md.created_at DESC").First(&derivation).Error
		if err != nil || derivation.ObjectKey == "" || derivation.ObjectVersionID == "" {
			return communityapp.ErrPostConflict
		}
		if err := tx.Model(&postRevisionMediaRecord{}).Where("post_id = ? AND version = ? AND ordinal = ?", postID, version, link.Ordinal).Updates(map[string]any{"object_key": derivation.ObjectKey, "object_version_id": derivation.ObjectVersionID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func nextPostVersion(tx *gorm.DB, postID string) (int, error) {
	var maximum int
	if err := tx.Model(&postRevisionRecord{}).Where("post_id = ?", postID).Select("COALESCE(MAX(version), 0)").Scan(&maximum).Error; err != nil {
		return 0, err
	}
	return maximum + 1, nil
}

func loadOwnPost(ctx context.Context, database *gorm.DB, ownerID, postID string) (communityapp.Post, error) {
	var record postRecord
	if err := database.WithContext(ctx).Where("id = ? AND owner_id = ? AND state <> ?", postID, ownerID, string(communityapp.PostDeleted)).First(&record).Error; err != nil {
		return communityapp.Post{}, err
	}
	currentVersion := dereferenceVersion(record.DraftVersion, record.PendingVersion, record.PublishedVersion)
	current, err := loadRevision(ctx, database, postID, currentVersion)
	if err != nil {
		return communityapp.Post{}, err
	}
	var published *communityapp.PostRevision
	if record.PublishedVersion != nil {
		value, err := loadRevision(ctx, database, postID, *record.PublishedVersion)
		if err != nil {
			return communityapp.Post{}, err
		}
		published = &value
	}
	return communityapp.Post{ID: record.ID, State: communityapp.PostState(record.State), Revision: record.Revision, DraftVersion: cloneInt(record.DraftVersion), PendingVersion: cloneInt(record.PendingVersion), PublishedVersion: cloneInt(record.PublishedVersion), ReviewRound: record.ReviewRound, SourceDiaryID: cloneString(record.SourceDiaryID), Current: current, Published: published, PublishedAt: record.PublishedAt, WithdrawnAt: record.WithdrawnAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, nil
}

func loadRevision(ctx context.Context, database *gorm.DB, postID string, version int) (communityapp.PostRevision, error) {
	if version < 1 {
		return communityapp.PostRevision{}, communityapp.ErrPostNotFound
	}
	var record postRevisionRecord
	if err := database.WithContext(ctx).Where("post_id = ? AND version = ?", postID, version).First(&record).Error; err != nil {
		return communityapp.PostRevision{}, err
	}
	media, tags, err := loadRevisionCollections(database.WithContext(ctx), postID, version)
	if err != nil {
		return communityapp.PostRevision{}, err
	}
	return communityapp.PostRevision{Version: record.Version, Title: record.Title, Body: record.Body, MediaIDs: media, Tags: tags, ReviewState: communityapp.ReviewState(record.ReviewState), ReviewRound: record.ReviewRound, ReasonCode: record.ReasonCode, SubmittedAt: record.SubmittedAt, ReviewedAt: record.ReviewedAt, CreatedAt: record.CreatedAt}, nil
}

func loadRevisionCollections(database *gorm.DB, postID string, version int) ([]string, []string, error) {
	var mediaRecords []postRevisionMediaRecord
	if err := database.Where("post_id = ? AND version = ?", postID, version).Order("ordinal ASC").Find(&mediaRecords).Error; err != nil {
		return nil, nil, err
	}
	media := make([]string, 0, len(mediaRecords))
	for _, record := range mediaRecords {
		media = append(media, record.MediaID)
	}
	var tagRecords []postRevisionTagRecord
	if err := database.Where("post_id = ? AND version = ?", postID, version).Order("tag ASC").Find(&tagRecords).Error; err != nil {
		return nil, nil, err
	}
	tags := make([]string, 0, len(tagRecords))
	for _, record := range tagRecords {
		tags = append(tags, record.Tag)
	}
	return media, tags, nil
}

func loadModerationCandidate(ctx context.Context, database *gorm.DB, postID string, version int) (communityapp.ModerationCandidate, error) {
	var row struct {
		PostID      string
		Revision    int
		Version     int
		ReviewRound int
		Handle      string
		DisplayName string
		Title       *string
		Body        *string
		SubmittedAt *time.Time
	}
	result := database.WithContext(ctx).Raw(`
		SELECT p.id AS post_id, p.revision, pr.version, pr.review_round, up.handle,
		       u.display_name, pr.title, pr.body, pr.submitted_at
		FROM posts p
		JOIN users u ON u.id = p.owner_id
		JOIN user_profiles up ON up.user_id = p.owner_id
		JOIN post_revisions pr ON pr.post_id = p.id AND pr.version = p.pending_version
		WHERE p.id = ? AND pr.version = ? AND pr.review_state = 'pending'
		LIMIT 1`, postID, version).Scan(&row)
	if result.Error != nil || result.RowsAffected != 1 || row.SubmittedAt == nil {
		return communityapp.ModerationCandidate{}, communityapp.ErrPostNotFound
	}
	media, tags, err := loadRevisionCollections(database.WithContext(ctx), postID, version)
	if err != nil {
		return communityapp.ModerationCandidate{}, err
	}
	return communityapp.ModerationCandidate{PostID: row.PostID, PostRevision: row.Revision, Version: row.Version, ReviewRound: row.ReviewRound, AuthorHandle: row.Handle, AuthorDisplayName: row.DisplayName, Title: row.Title, Body: row.Body, Tags: tags, ImageCount: len(media), SubmittedAt: *row.SubmittedAt}, nil
}

func postInputEqual(input communityapp.PostContentInput, sourceDiaryID *string, revision communityapp.PostRevision) bool {
	return reflect.DeepEqual(input.Title, revision.Title) && reflect.DeepEqual(input.Body, revision.Body) && reflect.DeepEqual(input.SourceDiaryID, sourceDiaryID) && slices.Equal(input.MediaIDs, revision.MediaIDs) && slices.Equal(input.Tags, revision.Tags)
}

func dereferenceVersion(versions ...*int) int {
	for _, version := range versions {
		if version != nil {
			return *version
		}
	}
	return 0
}

func reportFromRecord(record contentReportRecord) communityapp.ContentReport {
	return communityapp.ContentReport{ID: record.ID, PostID: record.PostID, TargetType: communityapp.ReportTargetType(record.TargetType), CommentID: record.CommentID, ReasonCode: record.ReasonCode, Detail: record.Detail, Status: communityapp.ReportStatus(record.Status), ResolutionCode: record.ResolutionCode, CreatedAt: record.CreatedAt, ResolvedAt: record.ResolvedAt}
}

func applyCreatedCursor(query, database *gorm.DB, table, ownerID, ownerColumn string, afterID *string) (*gorm.DB, error) {
	if afterID == nil {
		return query, nil
	}
	var cursor struct {
		ID        string
		CreatedAt time.Time
	}
	lookup := database.Table(table).Select("id", "created_at").Where("id = ?", *afterID)
	if ownerColumn != "" {
		lookup = lookup.Where(ownerColumn+" = ?", ownerID)
	}
	if err := lookup.First(&cursor).Error; err != nil {
		return nil, communityapp.ErrReportNotFound
	}
	return query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID), nil
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func mapCommunityError(err error) error {
	if err == nil {
		return nil
	}
	for _, domainError := range []error{communityapp.ErrPostNotFound, communityapp.ErrPostConflict, communityapp.ErrCommentNotFound, communityapp.ErrAppealNotFound, communityapp.ErrNotificationNotFound, communityapp.ErrCommunityForbidden, communityapp.ErrReportNotFound} {
		if errors.Is(err, domainError) {
			return domainError
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return communityapp.ErrPostNotFound
	}
	return communityapp.ErrCommunityUnavailable
}

package postgres

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommunityRepository struct{ database *gorm.DB }

func NewCommunityRepository(database *gorm.DB) *CommunityRepository {
	return &CommunityRepository{database: database}
}

type postRecord struct {
	ID                 string               `gorm:"column:id;type:uuid;primaryKey"`
	OwnerID            string               `gorm:"column:owner_id;type:uuid;not null;index:posts_owner_created_idx,priority:1;index:posts_public_idx,priority:2"`
	State              string               `gorm:"column:state;type:text;not null;index:posts_public_idx,priority:1;check:posts_state_check,state IN ('draft','pending','published','withdrawn','removed','deleted')"`
	Revision           int                  `gorm:"column:revision;not null;check:posts_revision_check,revision >= 1"`
	DraftVersion       *int                 `gorm:"column:draft_version;check:posts_draft_version_check,draft_version IS NULL OR draft_version >= 1"`
	PendingVersion     *int                 `gorm:"column:pending_version;check:posts_pending_version_check,pending_version IS NULL OR pending_version >= 1"`
	PublishedVersion   *int                 `gorm:"column:published_version;check:posts_published_version_check,published_version IS NULL OR published_version >= 1"`
	ReviewRound        int                  `gorm:"column:review_round;not null;default:0;check:posts_review_round_check,review_round >= 0"`
	SourceDiaryOwnerID *string              `gorm:"column:source_diary_owner_id;type:uuid;index:posts_source_diary_idx,priority:1;check:posts_source_diary_pair_check,(source_diary_owner_id IS NULL) = (source_diary_id IS NULL)"`
	SourceDiaryID      *string              `gorm:"column:source_diary_id;type:uuid;index:posts_source_diary_idx,priority:2"`
	PublishedAt        *time.Time           `gorm:"column:published_at;type:timestamptz;index:posts_public_idx,priority:3,sort:desc"`
	WithdrawnAt        *time.Time           `gorm:"column:withdrawn_at;type:timestamptz"`
	DeletedAt          *time.Time           `gorm:"column:deleted_at;type:timestamptz"`
	CreatedAt          time.Time            `gorm:"column:created_at;type:timestamptz;not null;index:posts_owner_created_idx,priority:2,sort:desc"`
	UpdatedAt          time.Time            `gorm:"column:updated_at;type:timestamptz;not null;check:posts_timestamps_check,updated_at >= created_at"`
	Revisions          []postRevisionRecord `gorm:"foreignKey:PostID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Owner              userRecord           `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	SourceDiary        *diaryEntryRecord    `gorm:"foreignKey:SourceDiaryOwnerID,SourceDiaryID;references:OwnerID,ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (postRecord) TableName() string { return "posts" }

type postRevisionRecord struct {
	PostID      string                    `gorm:"column:post_id;type:uuid;primaryKey"`
	Version     int                       `gorm:"column:version;primaryKey;check:post_revisions_version_check,version >= 1"`
	Title       *string                   `gorm:"column:title;type:text;check:post_revisions_title_check,title IS NULL OR (title = btrim(title) AND char_length(title) BETWEEN 1 AND 80)"`
	Body        *string                   `gorm:"column:body;type:text;check:post_revisions_body_check,body IS NULL OR (body = btrim(body) AND char_length(body) BETWEEN 1 AND 3000)"`
	ReviewState string                    `gorm:"column:review_state;type:text;not null;index:post_revisions_review_queue_idx,priority:1;check:post_revisions_review_state_check,review_state IN ('draft','pending','approved','rejected','cancelled')"`
	ReviewRound int                       `gorm:"column:review_round;not null;default:0;index:post_revisions_review_queue_idx,priority:2;check:post_revisions_review_round_check,review_round >= 0"`
	ReasonCode  *string                   `gorm:"column:reason_code;type:text"`
	SubmittedAt *time.Time                `gorm:"column:submitted_at;type:timestamptz;index:post_revisions_review_queue_idx,priority:3"`
	ReviewedAt  *time.Time                `gorm:"column:reviewed_at;type:timestamptz"`
	CreatedAt   time.Time                 `gorm:"column:created_at;type:timestamptz;not null"`
	Media       []postRevisionMediaRecord `gorm:"foreignKey:PostID,Version;references:PostID,Version;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Tags        []postRevisionTagRecord   `gorm:"foreignKey:PostID,Version;references:PostID,Version;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (postRevisionRecord) TableName() string { return "post_revisions" }

type postRevisionMediaRecord struct {
	PostID          string           `gorm:"column:post_id;type:uuid;primaryKey"`
	Version         int              `gorm:"column:version;primaryKey"`
	Ordinal         int              `gorm:"column:ordinal;primaryKey;check:post_revision_media_ordinal_check,ordinal BETWEEN 0 AND 8"`
	MediaID         string           `gorm:"column:media_id;type:uuid;not null;index:post_revision_media_media_idx"`
	ObjectKey       string           `gorm:"column:object_key;type:text;not null;default:''"`
	ObjectVersionID string           `gorm:"column:object_version_id;type:text;not null;default:''"`
	Media           mediaAssetRecord `gorm:"foreignKey:MediaID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (postRevisionMediaRecord) TableName() string { return "post_revision_media" }

type postRevisionTagRecord struct {
	PostID  string `gorm:"column:post_id;type:uuid;primaryKey"`
	Version int    `gorm:"column:version;primaryKey"`
	Tag     string `gorm:"column:tag;type:text;primaryKey;check:post_revision_tags_tag_check,tag = lower(btrim(tag)) AND char_length(tag) BETWEEN 1 AND 20"`
}

func (postRevisionTagRecord) TableName() string { return "post_revision_tags" }

type contentReportRecord struct {
	ID             string     `gorm:"column:id;type:uuid;primaryKey"`
	ReporterID     string     `gorm:"column:reporter_id;type:uuid;not null;index:content_reports_reporter_idx;uniqueIndex:content_reports_open_unique,priority:1,where:status = 'open'"`
	PostID         string     `gorm:"column:post_id;type:uuid;not null;index:content_reports_post_idx;uniqueIndex:content_reports_open_unique,priority:2,where:status = 'open'"`
	ReasonCode     string     `gorm:"column:reason_code;type:text;not null"`
	Detail         *string    `gorm:"column:detail;type:text;check:content_reports_detail_check,detail IS NULL OR char_length(detail) BETWEEN 1 AND 500"`
	Status         string     `gorm:"column:status;type:text;not null;index:content_reports_status_idx;check:content_reports_status_check,status IN ('open','resolved','dismissed')"`
	ResolutionCode *string    `gorm:"column:resolution_code;type:text"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:timestamptz;not null;index:content_reports_created_idx,sort:desc"`
	ResolvedAt     *time.Time `gorm:"column:resolved_at;type:timestamptz"`
	Reporter       userRecord `gorm:"foreignKey:ReporterID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Post           postRecord `gorm:"foreignKey:PostID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (contentReportRecord) TableName() string { return "content_reports" }

type moderationActionRecord struct {
	ID            string    `gorm:"column:id;type:uuid;primaryKey"`
	ActorID       string    `gorm:"column:actor_id;type:uuid;not null;index:moderation_actions_actor_idx"`
	PostID        *string   `gorm:"column:post_id;type:uuid;index:moderation_actions_post_idx"`
	PostVersion   *int      `gorm:"column:post_version"`
	ReportID      *string   `gorm:"column:report_id;type:uuid;index:moderation_actions_report_idx"`
	SubjectUserID *string   `gorm:"column:subject_user_id;type:uuid;index:moderation_actions_subject_idx"`
	Action        string    `gorm:"column:action;type:text;not null"`
	ReasonCode    string    `gorm:"column:reason_code;type:text;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;not null;index:moderation_actions_created_idx,sort:desc"`
}

func (moderationActionRecord) TableName() string { return "moderation_actions" }

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

func (r *CommunityRepository) GetPublicPost(ctx context.Context, postID string) (communityapp.PublicPost, error) {
	var row struct {
		PostID           string
		Handle           string
		DisplayName      string
		Title            *string
		Body             *string
		PublishedVersion int
		PublishedAt      time.Time
	}
	result := r.database.WithContext(ctx).Raw(`
		SELECT p.id AS post_id, up.handle, u.display_name, pr.title, pr.body,
		       p.published_version, p.published_at
		FROM posts p
		JOIN users u ON u.id = p.owner_id AND u.status = 'active'
		JOIN user_profiles up ON up.user_id = p.owner_id
		JOIN post_revisions pr ON pr.post_id = p.id AND pr.version = p.published_version
		WHERE p.id = ? AND p.state = 'published' AND p.published_version IS NOT NULL
		LIMIT 1`, postID).Scan(&row)
	if result.Error != nil {
		return communityapp.PublicPost{}, communityapp.ErrCommunityUnavailable
	}
	if result.RowsAffected != 1 {
		return communityapp.PublicPost{}, communityapp.ErrPostNotFound
	}
	media, tags, err := loadRevisionCollections(r.database.WithContext(ctx), postID, row.PublishedVersion)
	if err != nil {
		return communityapp.PublicPost{}, communityapp.ErrCommunityUnavailable
	}
	return communityapp.PublicPost{ID: row.PostID, AuthorHandle: row.Handle, AuthorDisplayName: row.DisplayName, Title: row.Title, Body: row.Body, Tags: tags, ImageCount: len(media), PublishedVersion: row.PublishedVersion, PublishedAt: row.PublishedAt}, nil
}

func (r *CommunityRepository) GetPublicPostImage(ctx context.Context, postID string, ordinal int) (communityapp.MediaObjectReference, error) {
	var media postRevisionMediaRecord
	err := r.database.WithContext(ctx).Raw(`
		SELECT prm.* FROM post_revision_media prm
		JOIN posts p ON p.id = prm.post_id AND p.published_version = prm.version
		JOIN users u ON u.id = p.owner_id AND u.status = 'active'
		WHERE p.id = ? AND p.state = 'published' AND prm.ordinal = ?
		LIMIT 1`, postID, ordinal).Scan(&media).Error
	if err != nil || media.ObjectKey == "" || media.ObjectVersionID == "" {
		return communityapp.MediaObjectReference{}, communityapp.ErrPostNotFound
	}
	return communityapp.MediaObjectReference{ObjectKey: media.ObjectKey, ObjectVersionID: media.ObjectVersionID}, nil
}

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
			if existing.ReporterID == reporterID && existing.PostID == postID && existing.ReasonCode == input.ReasonCode && reflect.DeepEqual(existing.Detail, input.Detail) {
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
		var open contentReportRecord
		if err := tx.Where("reporter_id = ? AND post_id = ? AND status = 'open'", reporterID, postID).First(&open).Error; err == nil {
			result = open
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result = contentReportRecord{ID: reportID, ReporterID: reporterID, PostID: postID, ReasonCode: input.ReasonCode, Detail: input.Detail, Status: string(communityapp.ReportOpen), CreatedAt: at}
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
		return tx.Create(&moderationActionRecord{ID: uuid.NewString(), ActorID: actorID, PostID: &result.PostID, ReportID: &reportID, Action: "report_" + string(status), ReasonCode: reason, CreatedAt: at}).Error
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
		page.Actions = append(page.Actions, communityapp.ModerationAction{ID: record.ID, ActorID: record.ActorID, PostID: record.PostID, PostVersion: record.PostVersion, ReportID: record.ReportID, SubjectUserID: record.SubjectUserID, Action: record.Action, ReasonCode: record.ReasonCode, CreatedAt: record.CreatedAt})
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
	return communityapp.ContentReport{ID: record.ID, PostID: record.PostID, ReasonCode: record.ReasonCode, Detail: record.Detail, Status: communityapp.ReportStatus(record.Status), ResolutionCode: record.ResolutionCode, CreatedAt: record.CreatedAt, ResolvedAt: record.ResolvedAt}
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
	for _, domainError := range []error{communityapp.ErrPostNotFound, communityapp.ErrPostConflict, communityapp.ErrCommunityForbidden, communityapp.ErrReportNotFound} {
		if errors.Is(err, domainError) {
			return domainError
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return communityapp.ErrPostNotFound
	}
	return communityapp.ErrCommunityUnavailable
}

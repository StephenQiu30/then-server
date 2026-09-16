package postgres

import (
	"time"

	"gorm.io/gorm"
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
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	ReporterID     string         `gorm:"column:reporter_id;type:uuid;not null;index:content_reports_reporter_idx;uniqueIndex:content_reports_open_post_unique,priority:1,where:status = 'open' AND target_type = 'post';uniqueIndex:content_reports_open_comment_unique,priority:1,where:status = 'open' AND target_type = 'comment'"`
	PostID         string         `gorm:"column:post_id;type:uuid;not null;index:content_reports_post_idx;uniqueIndex:content_reports_open_post_unique,priority:2,where:status = 'open' AND target_type = 'post'"`
	TargetType     string         `gorm:"column:target_type;type:text;not null;default:'post';check:content_reports_target_type_check,target_type IN ('post','comment')"`
	CommentID      *string        `gorm:"column:comment_id;type:uuid;uniqueIndex:content_reports_open_comment_unique,priority:2,where:status = 'open' AND target_type = 'comment';check:content_reports_target_pair_check,(target_type = 'post' AND comment_id IS NULL) OR (target_type = 'comment' AND comment_id IS NOT NULL)"`
	ReasonCode     string         `gorm:"column:reason_code;type:text;not null"`
	Detail         *string        `gorm:"column:detail;type:text;check:content_reports_detail_check,detail IS NULL OR char_length(detail) BETWEEN 1 AND 500"`
	Status         string         `gorm:"column:status;type:text;not null;index:content_reports_status_idx;check:content_reports_status_check,status IN ('open','resolved','dismissed')"`
	ResolutionCode *string        `gorm:"column:resolution_code;type:text"`
	CreatedAt      time.Time      `gorm:"column:created_at;type:timestamptz;not null;index:content_reports_created_idx,sort:desc"`
	ResolvedAt     *time.Time     `gorm:"column:resolved_at;type:timestamptz"`
	Reporter       userRecord     `gorm:"foreignKey:ReporterID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Post           postRecord     `gorm:"foreignKey:PostID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Comment        *commentRecord `gorm:"foreignKey:CommentID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (contentReportRecord) TableName() string { return "content_reports" }

type moderationActionRecord struct {
	ID            string    `gorm:"column:id;type:uuid;primaryKey"`
	ActorID       string    `gorm:"column:actor_id;type:uuid;not null;index:moderation_actions_actor_idx"`
	PostID        *string   `gorm:"column:post_id;type:uuid;index:moderation_actions_post_idx"`
	PostVersion   *int      `gorm:"column:post_version"`
	CommentID     *string   `gorm:"column:comment_id;type:uuid;index:moderation_actions_comment_idx"`
	ReportID      *string   `gorm:"column:report_id;type:uuid;index:moderation_actions_report_idx"`
	SubjectUserID *string   `gorm:"column:subject_user_id;type:uuid;index:moderation_actions_subject_idx"`
	Action        string    `gorm:"column:action;type:text;not null"`
	ReasonCode    string    `gorm:"column:reason_code;type:text;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;not null;index:moderation_actions_created_idx,sort:desc"`
}

func (moderationActionRecord) TableName() string { return "moderation_actions" }

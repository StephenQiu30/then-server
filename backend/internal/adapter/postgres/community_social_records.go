package postgres

import "time"

type postLikeRecord struct {
	UserID    string     `gorm:"column:user_id;type:uuid;primaryKey;index:post_likes_post_idx,priority:2"`
	PostID    string     `gorm:"column:post_id;type:uuid;primaryKey;index:post_likes_post_idx,priority:1"`
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	User      userRecord `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Post      postRecord `gorm:"foreignKey:PostID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (postLikeRecord) TableName() string { return "post_likes" }

type postBookmarkRecord struct {
	UserID    string     `gorm:"column:user_id;type:uuid;primaryKey;index:post_bookmarks_user_created_idx,priority:1"`
	PostID    string     `gorm:"column:post_id;type:uuid;primaryKey"`
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamptz;not null;index:post_bookmarks_user_created_idx,priority:2,sort:desc"`
	User      userRecord `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Post      postRecord `gorm:"foreignKey:PostID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (postBookmarkRecord) TableName() string { return "post_bookmarks" }

type userFollowRecord struct {
	FollowerID string     `gorm:"column:follower_id;type:uuid;primaryKey;index:user_follows_follower_created_idx,priority:1;check:user_follows_distinct_check,follower_id <> followee_id"`
	FolloweeID string     `gorm:"column:followee_id;type:uuid;primaryKey;index:user_follows_followee_created_idx,priority:1"`
	CreatedAt  time.Time  `gorm:"column:created_at;type:timestamptz;not null;index:user_follows_follower_created_idx,priority:2,sort:desc;index:user_follows_followee_created_idx,priority:2,sort:desc"`
	Follower   userRecord `gorm:"foreignKey:FollowerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Followee   userRecord `gorm:"foreignKey:FolloweeID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (userFollowRecord) TableName() string { return "user_follows" }

type userBlockRecord struct {
	BlockerID string     `gorm:"column:blocker_id;type:uuid;primaryKey;index:user_blocks_blocker_created_idx,priority:1;check:user_blocks_distinct_check,blocker_id <> blocked_id"`
	BlockedID string     `gorm:"column:blocked_id;type:uuid;primaryKey;index:user_blocks_blocked_idx"`
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamptz;not null;index:user_blocks_blocker_created_idx,priority:2,sort:desc"`
	Blocker   userRecord `gorm:"foreignKey:BlockerID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Blocked   userRecord `gorm:"foreignKey:BlockedID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (userBlockRecord) TableName() string { return "user_blocks" }

type commentRecord struct {
	ID         string         `gorm:"column:id;type:uuid;primaryKey"`
	PostID     string         `gorm:"column:post_id;type:uuid;not null;index:comments_root_idx,priority:1;index:comments_pending_idx,priority:2"`
	AuthorID   string         `gorm:"column:author_id;type:uuid;not null;index:comments_author_idx"`
	ParentID   *string        `gorm:"column:parent_id;type:uuid;index:comments_parent_idx"`
	Body       *string        `gorm:"column:body;type:text;check:comments_body_check,body IS NULL OR (body = btrim(body) AND char_length(body) BETWEEN 1 AND 500)"`
	State      string         `gorm:"column:state;type:text;not null;index:comments_root_idx,priority:2;index:comments_pending_idx,priority:1;check:comments_state_check,state IN ('pending','published','rejected','deleted','removed')"`
	Revision   int            `gorm:"column:revision;not null;check:comments_revision_check,revision >= 1"`
	ReasonCode *string        `gorm:"column:reason_code;type:text"`
	ReviewedAt *time.Time     `gorm:"column:reviewed_at;type:timestamptz"`
	CreatedAt  time.Time      `gorm:"column:created_at;type:timestamptz;not null;index:comments_root_idx,priority:3;index:comments_pending_idx,priority:3"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;type:timestamptz;not null;check:comments_timestamps_check,updated_at >= created_at"`
	Post       postRecord     `gorm:"foreignKey:PostID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Author     userRecord     `gorm:"foreignKey:AuthorID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Parent     *commentRecord `gorm:"foreignKey:ParentID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (commentRecord) TableName() string { return "comments" }

type moderationAppealRecord struct {
	ID             string                 `gorm:"column:id;type:uuid;primaryKey"`
	AppellantID    string                 `gorm:"column:appellant_id;type:uuid;not null;index:moderation_appeals_appellant_created_idx,priority:1;uniqueIndex:moderation_appeals_open_unique,priority:1,where:status = 'open'"`
	ActionID       string                 `gorm:"column:action_id;type:uuid;not null;uniqueIndex:moderation_appeals_open_unique,priority:2,where:status = 'open'"`
	Reason         string                 `gorm:"column:reason;type:text;not null;check:moderation_appeals_reason_check,reason = btrim(reason) AND char_length(reason) BETWEEN 1 AND 500"`
	Status         string                 `gorm:"column:status;type:text;not null;index:moderation_appeals_status_idx;check:moderation_appeals_status_check,status IN ('open','upheld','reversed')"`
	ResolutionCode *string                `gorm:"column:resolution_code;type:text"`
	ResolvedBy     *string                `gorm:"column:resolved_by;type:uuid"`
	CreatedAt      time.Time              `gorm:"column:created_at;type:timestamptz;not null;index:moderation_appeals_appellant_created_idx,priority:2,sort:desc"`
	ResolvedAt     *time.Time             `gorm:"column:resolved_at;type:timestamptz"`
	Appellant      userRecord             `gorm:"foreignKey:AppellantID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Action         moderationActionRecord `gorm:"foreignKey:ActionID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	Resolver       *userRecord            `gorm:"foreignKey:ResolvedBy;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:SET NULL"`
}

func (moderationAppealRecord) TableName() string { return "moderation_appeals" }

type notificationRecord struct {
	ID          string         `gorm:"column:id;type:uuid;primaryKey"`
	RecipientID string         `gorm:"column:recipient_id;type:uuid;not null;uniqueIndex:notifications_event_unique,priority:1;index:notifications_recipient_created_idx,priority:1"`
	ActorID     *string        `gorm:"column:actor_id;type:uuid"`
	EventID     string         `gorm:"column:event_id;type:uuid;not null;uniqueIndex:notifications_event_unique,priority:2"`
	Kind        string         `gorm:"column:kind;type:text;not null;uniqueIndex:notifications_event_unique,priority:3;check:notifications_kind_check,kind IN ('comment_published','reply_published','followed','post_reviewed','comment_reviewed','appeal_resolved')"`
	PostID      *string        `gorm:"column:post_id;type:uuid"`
	CommentID   *string        `gorm:"column:comment_id;type:uuid"`
	AvailableAt *time.Time     `gorm:"column:available_at;type:timestamptz;index:notifications_recipient_created_idx,priority:2"`
	ReadAt      *time.Time     `gorm:"column:read_at;type:timestamptz"`
	CreatedAt   time.Time      `gorm:"column:created_at;type:timestamptz;not null;index:notifications_recipient_created_idx,priority:3,sort:desc"`
	Recipient   userRecord     `gorm:"foreignKey:RecipientID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Actor       *userRecord    `gorm:"foreignKey:ActorID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:SET NULL"`
	Post        *postRecord    `gorm:"foreignKey:PostID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:SET NULL"`
	Comment     *commentRecord `gorm:"foreignKey:CommentID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:SET NULL"`
}

func (notificationRecord) TableName() string { return "notifications" }

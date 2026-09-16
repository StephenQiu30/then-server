package community

import "time"

type FeedType string

const (
	FeedDiscover  FeedType = "discover"
	FeedFollowing FeedType = "following"
)

type PublicPostPage struct {
	Posts       []PublicPost
	NextAfterID *string
}

type PublicProfileSummary struct {
	Handle         string
	DisplayName    string
	Bio            *string
	Following      bool
	FollowerCount  int64
	FollowingCount int64
}

type PublicProfilePage struct {
	Profiles    []PublicProfileSummary
	NextAfterID *string
}

type BlockedUser struct {
	UserID      string
	Handle      string
	DisplayName string
	BlockedAt   time.Time
}

type BlockedUserPage struct {
	Users       []BlockedUser
	NextAfterID *string
}

type CommentState string

const (
	CommentPending   CommentState = "pending"
	CommentPublished CommentState = "published"
	CommentRejected  CommentState = "rejected"
	CommentDeleted   CommentState = "deleted"
	CommentRemoved   CommentState = "removed"
)

type Comment struct {
	ID                string
	PostID            string
	ParentID          *string
	AuthorHandle      string
	AuthorDisplayName string
	Body              *string
	State             CommentState
	Revision          int
	ReasonCode        *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CommentPage struct {
	Comments    []Comment
	NextAfterID *string
}

type CreateCommentInput struct {
	Body     string
	ParentID *string
}

type CommentModerationCandidate struct {
	Comment
	PostAuthorHandle string
	PostTitle        *string
}

type CommentModerationPage struct {
	Comments    []CommentModerationCandidate
	NextAfterID *string
}

type DecideCommentInput struct {
	ExpectedRevision int
	Decision         ModerationDecision
	ReasonCode       string
}

type ReportTargetType string

const (
	ReportTargetPost    ReportTargetType = "post"
	ReportTargetComment ReportTargetType = "comment"
)

type NotificationKind string

const (
	NotificationCommentPublished NotificationKind = "comment_published"
	NotificationReplyPublished   NotificationKind = "reply_published"
	NotificationFollowed         NotificationKind = "followed"
	NotificationPostReviewed     NotificationKind = "post_reviewed"
	NotificationCommentReviewed  NotificationKind = "comment_reviewed"
	NotificationAppealResolved   NotificationKind = "appeal_resolved"
)

type Notification struct {
	ID          string
	Kind        NotificationKind
	ActorHandle *string
	PostID      *string
	CommentID   *string
	ReadAt      *time.Time
	CreatedAt   time.Time
}

type NotificationPage struct {
	Notifications []Notification
	NextAfterID   *string
}

type AppealStatus string

const (
	AppealOpen     AppealStatus = "open"
	AppealUpheld   AppealStatus = "upheld"
	AppealReversed AppealStatus = "reversed"
)

type ModerationAppeal struct {
	ID             string
	ActionID       string
	Action         string
	Reason         string
	Status         AppealStatus
	ResolutionCode *string
	CreatedAt      time.Time
	ResolvedAt     *time.Time
}

type ModerationAppealPage struct {
	Appeals     []ModerationAppeal
	NextAfterID *string
}

type AdminModerationAppeal struct {
	ModerationAppeal
	AppellantID string
}

type AdminModerationAppealPage struct {
	Appeals     []AdminModerationAppeal
	NextAfterID *string
}

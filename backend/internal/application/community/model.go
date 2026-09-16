package community

import (
	"errors"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

var (
	ErrInvalidCommunityInput = errors.New("invalid community input")
	ErrPostNotFound          = errors.New("post not found")
	ErrPostConflict          = errors.New("post revision or state conflict")
	ErrCommentNotFound       = errors.New("comment not found")
	ErrAppealNotFound        = errors.New("moderation appeal not found")
	ErrNotificationNotFound  = errors.New("notification not found")
	ErrCommunityForbidden    = errors.New("community operation forbidden")
	ErrReportNotFound        = errors.New("report not found")
	ErrCommunityUnavailable  = errors.New("community service unavailable")
)

type PostState string

const (
	PostDraft     PostState = "draft"
	PostPending   PostState = "pending"
	PostPublished PostState = "published"
	PostWithdrawn PostState = "withdrawn"
	PostRemoved   PostState = "removed"
	PostDeleted   PostState = "deleted"
)

type ReviewState string

const (
	ReviewDraft     ReviewState = "draft"
	ReviewPending   ReviewState = "pending"
	ReviewApproved  ReviewState = "approved"
	ReviewRejected  ReviewState = "rejected"
	ReviewCancelled ReviewState = "cancelled"
)

type PostContentInput struct {
	Title         *string
	Body          *string
	SourceDiaryID *string
	MediaIDs      []string
	Tags          []string
}

type PostRevision struct {
	Version     int
	Title       *string
	Body        *string
	MediaIDs    []string
	Tags        []string
	ReviewState ReviewState
	ReviewRound int
	ReasonCode  *string
	SubmittedAt *time.Time
	ReviewedAt  *time.Time
	CreatedAt   time.Time
}

type Post struct {
	ID               string
	State            PostState
	Revision         int
	DraftVersion     *int
	PendingVersion   *int
	PublishedVersion *int
	ReviewRound      int
	SourceDiaryID    *string
	Current          PostRevision
	Published        *PostRevision
	PublishedAt      *time.Time
	WithdrawnAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PostPage struct {
	Posts       []Post
	NextAfterID *string
}

type PublicPost struct {
	ID                string
	AuthorHandle      string
	AuthorDisplayName string
	Title             *string
	Body              *string
	Tags              []string
	ImageCount        int
	LikeCount         int64
	CommentCount      int64
	ViewerLiked       bool
	ViewerBookmarked  bool
	FollowingAuthor   bool
	PublishedVersion  int
	PublishedAt       time.Time
}

type ModerationCandidate struct {
	PostID            string
	PostRevision      int
	Version           int
	ReviewRound       int
	AuthorHandle      string
	AuthorDisplayName string
	Title             *string
	Body              *string
	Tags              []string
	ImageCount        int
	SubmittedAt       time.Time
}

type ModerationCandidatePage struct {
	Candidates  []ModerationCandidate
	NextAfterID *string
}

type ModerationDecision string

const (
	ModerationApprove ModerationDecision = "approve"
	ModerationReject  ModerationDecision = "reject"
)

type DecidePostInput struct {
	Version          int
	ReviewRound      int
	ExpectedRevision int
	Decision         ModerationDecision
	ReasonCode       string
}

type MediaObjectReference struct {
	ObjectKey       string
	ObjectVersionID string
}

type ReportStatus string

const (
	ReportOpen      ReportStatus = "open"
	ReportResolved  ReportStatus = "resolved"
	ReportDismissed ReportStatus = "dismissed"
)

type CreateReportInput struct {
	TargetType ReportTargetType
	CommentID  *string
	ReasonCode string
	Detail     *string
}

type ContentReport struct {
	ID             string
	PostID         string
	TargetType     ReportTargetType
	CommentID      *string
	ReasonCode     string
	Detail         *string
	Status         ReportStatus
	ResolutionCode *string
	CreatedAt      time.Time
	ResolvedAt     *time.Time
}

type ContentReportPage struct {
	Reports     []ContentReport
	NextAfterID *string
}

type AdminReport struct {
	ContentReport
	ReporterID string
}

type AdminReportPage struct {
	Reports     []AdminReport
	NextAfterID *string
}

type ModerationAction struct {
	ID            string
	ActorID       string
	PostID        *string
	PostVersion   *int
	CommentID     *string
	ReportID      *string
	SubjectUserID *string
	Action        string
	ReasonCode    string
	CreatedAt     time.Time
}

type ModerationActionPage struct {
	Actions     []ModerationAction
	NextAfterID *string
}

type AdminUser struct {
	ID          string
	DisplayName string
	Status      accountapp.AccountStatus
	Role        accountapp.AccountRole
	Revision    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AdminUserPage struct {
	Users       []AdminUser
	NextAfterID *string
}

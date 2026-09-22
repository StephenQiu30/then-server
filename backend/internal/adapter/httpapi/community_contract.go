package httpapi

import (
	"time"
)

type PostContentRequest struct {
	Title         *string  `json:"title,omitempty" maxLength:"80"`
	Body          *string  `json:"body,omitempty" maxLength:"3000"`
	SourceDiaryID *string  `json:"source_diary_id,omitempty" format:"uuid"`
	MediaIDs      []string `json:"media_ids" maxItems:"9"`
	Tags          []string `json:"tags" maxItems:"5"`
}

type CreatePostRequest struct {
	ID string `json:"id" format:"uuid"`
	PostContentRequest
}

type UpdatePostRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
	PostContentRequest
}

type PostRevisionRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
}

type DecidePostRequest struct {
	Version          int    `json:"version" minimum:"1"`
	ReviewRound      int    `json:"review_round" minimum:"1"`
	ExpectedRevision int    `json:"expected_revision" minimum:"1"`
	Decision         string `json:"decision" enum:"approve,reject"`
	ReasonCode       string `json:"reason_code" pattern:"^[a-z][a-z0-9_]{0,39}$"`
}

type RemovePostRequest struct {
	ExpectedRevision int    `json:"expected_revision" minimum:"1"`
	ReasonCode       string `json:"reason_code" pattern:"^[a-z][a-z0-9_]{0,39}$"`
}

type CreateReportRequest struct {
	ID         string  `json:"id" format:"uuid"`
	PostID     string  `json:"post_id" format:"uuid"`
	TargetType string  `json:"target_type" enum:"post,comment" default:"post"`
	CommentID  *string `json:"comment_id,omitempty" format:"uuid"`
	ReasonCode string  `json:"reason_code" enum:"spam,harassment,sexual,violence,misinformation,other"`
	Detail     *string `json:"detail,omitempty" maxLength:"500"`
}

type ResolveReportRequest struct {
	Status         string `json:"status" enum:"resolved,dismissed"`
	ResolutionCode string `json:"resolution_code" pattern:"^[a-z][a-z0-9_]{0,39}$"`
}

type SetUserStatusRequest struct {
	ExpectedRevision int    `json:"expected_revision" minimum:"1"`
	ReasonCode       string `json:"reason_code" pattern:"^[a-z][a-z0-9_]{0,39}$"`
}

type PostRevisionResponse struct {
	Version     int        `json:"version" minimum:"1"`
	Title       *string    `json:"title,omitempty" maxLength:"80"`
	Body        *string    `json:"body,omitempty" maxLength:"3000"`
	MediaIDs    []string   `json:"media_ids" maxItems:"9"`
	Tags        []string   `json:"tags" maxItems:"5"`
	ReviewState string     `json:"review_state" enum:"draft,pending,approved,rejected,cancelled"`
	ReviewRound int        `json:"review_round" minimum:"0"`
	ReasonCode  *string    `json:"reason_code,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty" format:"date-time"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty" format:"date-time"`
	CreatedAt   time.Time  `json:"created_at" format:"date-time"`
}

type PostResponse struct {
	ID               string                `json:"id" format:"uuid"`
	State            string                `json:"state" enum:"draft,pending,published,withdrawn,removed"`
	Revision         int                   `json:"revision" minimum:"1"`
	DraftVersion     *int                  `json:"draft_version,omitempty" minimum:"1"`
	PendingVersion   *int                  `json:"pending_version,omitempty" minimum:"1"`
	PublishedVersion *int                  `json:"published_version,omitempty" minimum:"1"`
	ReviewRound      int                   `json:"review_round" minimum:"0"`
	SourceDiaryID    *string               `json:"source_diary_id,omitempty" format:"uuid"`
	Current          PostRevisionResponse  `json:"current"`
	Published        *PostRevisionResponse `json:"published,omitempty"`
	PublishedAt      *time.Time            `json:"published_at,omitempty" format:"date-time"`
	WithdrawnAt      *time.Time            `json:"withdrawn_at,omitempty" format:"date-time"`
	CreatedAt        time.Time             `json:"created_at" format:"date-time"`
	UpdatedAt        time.Time             `json:"updated_at" format:"date-time"`
}

type PostPageResponse struct {
	Posts       []PostResponse `json:"posts" maxItems:"50"`
	NextAfterID *string        `json:"next_after_id,omitempty" format:"uuid"`
}

type PublicPostResponse struct {
	ID                string    `json:"id" format:"uuid"`
	AuthorHandle      string    `json:"author_handle"`
	AuthorDisplayName string    `json:"author_display_name"`
	Title             *string   `json:"title,omitempty" maxLength:"80"`
	Body              *string   `json:"body,omitempty" maxLength:"3000"`
	Tags              []string  `json:"tags" maxItems:"5"`
	ImageCount        int       `json:"image_count" minimum:"0" maximum:"9"`
	LikeCount         int64     `json:"like_count" minimum:"0"`
	CommentCount      int64     `json:"comment_count" minimum:"0"`
	ViewerLiked       bool      `json:"viewer_liked"`
	ViewerBookmarked  bool      `json:"viewer_bookmarked"`
	FollowingAuthor   bool      `json:"following_author"`
	PublishedAt       time.Time `json:"published_at" format:"date-time"`
}

type ModerationCandidateResponse struct {
	PostID            string    `json:"post_id" format:"uuid"`
	PostRevision      int       `json:"post_revision" minimum:"1"`
	Version           int       `json:"version" minimum:"1"`
	ReviewRound       int       `json:"review_round" minimum:"1"`
	AuthorHandle      string    `json:"author_handle"`
	AuthorDisplayName string    `json:"author_display_name"`
	Title             *string   `json:"title,omitempty" maxLength:"80"`
	Body              *string   `json:"body,omitempty" maxLength:"3000"`
	Tags              []string  `json:"tags" maxItems:"5"`
	ImageCount        int       `json:"image_count" minimum:"0" maximum:"9"`
	SubmittedAt       time.Time `json:"submitted_at" format:"date-time"`
}

type ModerationCandidatePageResponse struct {
	Candidates  []ModerationCandidateResponse `json:"candidates" maxItems:"50"`
	NextAfterID *string                       `json:"next_after_id,omitempty" format:"uuid"`
}

type ContentReportResponse struct {
	ID             string     `json:"id" format:"uuid"`
	PostID         string     `json:"post_id" format:"uuid"`
	TargetType     string     `json:"target_type" enum:"post,comment"`
	CommentID      *string    `json:"comment_id,omitempty" format:"uuid"`
	ReasonCode     string     `json:"reason_code"`
	Detail         *string    `json:"detail,omitempty" maxLength:"500"`
	Status         string     `json:"status" enum:"open,resolved,dismissed"`
	ResolutionCode *string    `json:"resolution_code,omitempty"`
	CreatedAt      time.Time  `json:"created_at" format:"date-time"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty" format:"date-time"`
}

type AdminReportResponse struct {
	ContentReportResponse
	ReporterID string `json:"reporter_id" format:"uuid"`
}

type ReportPageResponse struct {
	Reports     []ContentReportResponse `json:"reports" maxItems:"50"`
	NextAfterID *string                 `json:"next_after_id,omitempty" format:"uuid"`
}

type AdminReportPageResponse struct {
	Reports     []AdminReportResponse `json:"reports" maxItems:"50"`
	NextAfterID *string               `json:"next_after_id,omitempty" format:"uuid"`
}

type ModerationActionResponse struct {
	ID            string    `json:"id" format:"uuid"`
	ActorID       string    `json:"actor_id" format:"uuid"`
	PostID        *string   `json:"post_id,omitempty" format:"uuid"`
	PostVersion   *int      `json:"post_version,omitempty" minimum:"1"`
	CommentID     *string   `json:"comment_id,omitempty" format:"uuid"`
	ReportID      *string   `json:"report_id,omitempty" format:"uuid"`
	SubjectUserID *string   `json:"subject_user_id,omitempty" format:"uuid"`
	Action        string    `json:"action"`
	ReasonCode    string    `json:"reason_code"`
	CreatedAt     time.Time `json:"created_at" format:"date-time"`
}

type ModerationActionPageResponse struct {
	Actions     []ModerationActionResponse `json:"actions" maxItems:"50"`
	NextAfterID *string                    `json:"next_after_id,omitempty" format:"uuid"`
}

type AdminUserResponse struct {
	ID          string    `json:"id" format:"uuid"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status" enum:"active,suspended,deleting"`
	Role        string    `json:"role" enum:"user,moderator,admin"`
	Revision    int       `json:"revision" minimum:"1"`
	CreatedAt   time.Time `json:"created_at" format:"date-time"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time"`
}

type AdminUserPageResponse struct {
	Users       []AdminUserResponse `json:"users" maxItems:"50"`
	NextAfterID *string             `json:"next_after_id,omitempty" format:"uuid"`
}

type communitySessionInput struct {
	Session string `cookie:"then_session" hidden:"true"`
}
type postResourceInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
}
type ownPostInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
}
type createPostInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreatePostRequest
}
type updatePostInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
	Body    UpdatePostRequest
}
type mutatePostInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
	Body    PostRevisionRequest
}
type deletePostInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ID               string `path:"post_id" format:"uuid"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}
type listOwnPostsInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
	State   string `query:"state" required:"false" enum:"draft,pending,published,withdrawn,removed"`
}
type postImageInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
	Ordinal int    `path:"ordinal" minimum:"0" maximum:"8"`
}
type moderationListInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
}
type moderationCandidateInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
	Version int    `path:"version" minimum:"1"`
}
type moderationImageInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
	Version int    `path:"version" minimum:"1"`
	Ordinal int    `path:"ordinal" minimum:"0" maximum:"8"`
}
type decisionInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
	Body    DecidePostRequest
}
type removePostInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
	Body    RemovePostRequest
}
type createReportInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateReportRequest
}
type reportsInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
	Status  string `query:"status" required:"false" enum:"open,resolved,dismissed"`
}
type reportResourceInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"report_id" format:"uuid"`
	Body    ResolveReportRequest
}
type adminUserInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"user_id" format:"uuid"`
	Body    SetUserStatusRequest
}

type postOutput struct {
	RequestID string       `header:"X-Request-ID"`
	Body      PostResponse `json:"body"`
}
type postPageOutput struct {
	RequestID string           `header:"X-Request-ID"`
	Body      PostPageResponse `json:"body"`
}
type publicPostOutput struct {
	RequestID string             `header:"X-Request-ID"`
	Body      PublicPostResponse `json:"body"`
}
type candidateOutput struct {
	RequestID string                      `header:"X-Request-ID"`
	Body      ModerationCandidateResponse `json:"body"`
}
type candidatePageOutput struct {
	RequestID string                          `header:"X-Request-ID"`
	Body      ModerationCandidatePageResponse `json:"body"`
}
type reportOutput struct {
	RequestID string                `header:"X-Request-ID"`
	Body      ContentReportResponse `json:"body"`
}
type reportPageOutput struct {
	RequestID string             `header:"X-Request-ID"`
	Body      ReportPageResponse `json:"body"`
}
type adminReportPageOutput struct {
	RequestID string                  `header:"X-Request-ID"`
	Body      AdminReportPageResponse `json:"body"`
}
type actionPageOutput struct {
	RequestID string                       `header:"X-Request-ID"`
	Body      ModerationActionPageResponse `json:"body"`
}
type adminUserOutput struct {
	RequestID string            `header:"X-Request-ID"`
	Body      AdminUserResponse `json:"body"`
}
type adminUserPageOutput struct {
	RequestID string                `header:"X-Request-ID"`
	Body      AdminUserPageResponse `json:"body"`
}
type communityEmptyOutput struct {
	RequestID string `header:"X-Request-ID"`
}
type imageOutput struct {
	RequestID   string `header:"X-Request-ID"`
	ContentType string `header:"Content-Type"`
	Body        []byte
}

package httpapi

import (
	"time"
)

type PublicPostPageResponse struct {
	Posts       []PublicPostResponse `json:"posts" maxItems:"50"`
	NextAfterID *string              `json:"next_after_id,omitempty" format:"uuid"`
}

type PublicProfileSummaryResponse struct {
	Handle         string  `json:"handle"`
	DisplayName    string  `json:"display_name"`
	Bio            *string `json:"bio,omitempty"`
	Following      bool    `json:"following"`
	FollowerCount  int64   `json:"follower_count" minimum:"0"`
	FollowingCount int64   `json:"following_count" minimum:"0"`
}

type PublicProfilePageResponse struct {
	Profiles    []PublicProfileSummaryResponse `json:"profiles" maxItems:"50"`
	NextAfterID *string                        `json:"next_after_id,omitempty" format:"uuid"`
}

type BlockedUserResponse struct {
	UserID      string    `json:"user_id" format:"uuid"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	BlockedAt   time.Time `json:"blocked_at" format:"date-time"`
}

type BlockedUserPageResponse struct {
	Users       []BlockedUserResponse `json:"users" maxItems:"50"`
	NextAfterID *string               `json:"next_after_id,omitempty" format:"uuid"`
}

type CommentResponse struct {
	ID                string    `json:"id" format:"uuid"`
	PostID            string    `json:"post_id" format:"uuid"`
	ParentID          *string   `json:"parent_id,omitempty" format:"uuid"`
	AuthorHandle      string    `json:"author_handle"`
	AuthorDisplayName string    `json:"author_display_name"`
	Body              *string   `json:"body,omitempty" maxLength:"500"`
	State             string    `json:"state" enum:"pending,published,rejected,deleted,removed"`
	Revision          int       `json:"revision" minimum:"1"`
	ReasonCode        *string   `json:"reason_code,omitempty"`
	CreatedAt         time.Time `json:"created_at" format:"date-time"`
	UpdatedAt         time.Time `json:"updated_at" format:"date-time"`
}

type CommentPageResponse struct {
	Comments    []CommentResponse `json:"comments" maxItems:"50"`
	NextAfterID *string           `json:"next_after_id,omitempty" format:"uuid"`
}

type CommentModerationResponse struct {
	CommentResponse
	PostAuthorHandle string  `json:"post_author_handle"`
	PostTitle        *string `json:"post_title,omitempty"`
}

type CommentModerationPageResponse struct {
	Comments    []CommentModerationResponse `json:"comments" maxItems:"50"`
	NextAfterID *string                     `json:"next_after_id,omitempty" format:"uuid"`
}

type NotificationResponse struct {
	ID          string     `json:"id" format:"uuid"`
	Kind        string     `json:"kind" enum:"comment_published,reply_published,followed,post_reviewed,comment_reviewed,appeal_resolved"`
	ActorHandle *string    `json:"actor_handle,omitempty"`
	PostID      *string    `json:"post_id,omitempty" format:"uuid"`
	CommentID   *string    `json:"comment_id,omitempty" format:"uuid"`
	ReadAt      *time.Time `json:"read_at,omitempty" format:"date-time"`
	CreatedAt   time.Time  `json:"created_at" format:"date-time"`
}

type NotificationPageResponse struct {
	Notifications []NotificationResponse `json:"notifications" maxItems:"50"`
	NextAfterID   *string                `json:"next_after_id,omitempty" format:"uuid"`
}

type AppealResponse struct {
	ID             string     `json:"id" format:"uuid"`
	ActionID       string     `json:"action_id" format:"uuid"`
	Action         string     `json:"action"`
	Reason         string     `json:"reason" maxLength:"500"`
	Status         string     `json:"status" enum:"open,upheld,reversed"`
	ResolutionCode *string    `json:"resolution_code,omitempty"`
	CreatedAt      time.Time  `json:"created_at" format:"date-time"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty" format:"date-time"`
}

type AppealPageResponse struct {
	Appeals     []AppealResponse `json:"appeals" maxItems:"50"`
	NextAfterID *string          `json:"next_after_id,omitempty" format:"uuid"`
}

type AdminAppealResponse struct {
	AppealResponse
	AppellantID string `json:"appellant_id" format:"uuid"`
}

type AdminAppealPageResponse struct {
	Appeals     []AdminAppealResponse `json:"appeals" maxItems:"50"`
	NextAfterID *string               `json:"next_after_id,omitempty" format:"uuid"`
}

type CreateCommentRequest struct {
	ID       string  `json:"id" format:"uuid"`
	Body     string  `json:"body" minLength:"1" maxLength:"500"`
	ParentID *string `json:"parent_id,omitempty" format:"uuid"`
}
type DecideCommentRequest struct {
	ExpectedRevision int    `json:"expected_revision" minimum:"1"`
	Decision         string `json:"decision" enum:"approve,reject"`
	ReasonCode       string `json:"reason_code" pattern:"^[a-z][a-z0-9_]{0,39}$"`
}
type CreateAppealRequest struct {
	ID       string `json:"id" format:"uuid"`
	ActionID string `json:"action_id" format:"uuid"`
	Reason   string `json:"reason" minLength:"1" maxLength:"500"`
}
type ResolveAppealRequest struct {
	Status         string `json:"status" enum:"upheld,reversed"`
	ResolutionCode string `json:"resolution_code" pattern:"^[a-z][a-z0-9_]{0,39}$"`
}

type feedInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
	Type    string `query:"type" default:"discover" enum:"discover,following"`
}
type searchInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
	Query   string `query:"q" required:"false" minLength:"2" maxLength:"80"`
	Tag     string `query:"tag" required:"false" maxLength:"20"`
}
type profilePageInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
	Handle  string `path:"handle" pattern:"^[a-z0-9_]{3,30}$"`
}
type profileRelationshipInput struct {
	profilePageInput
	Direction string `path:"direction" enum:"followers,following"`
}
type relationInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"post_id" format:"uuid"`
}
type followInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Handle  string `path:"handle" pattern:"^[a-z0-9_]{3,30}$"`
}
type blockInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	UserID  string `path:"user_id" format:"uuid"`
}
type authenticatedPageInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
}
type commentsInput struct {
	Session  string `cookie:"then_session" hidden:"true"`
	Limit    int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID  string `query:"after_id" format:"uuid" required:"false"`
	PostID   string `path:"post_id" format:"uuid"`
	ParentID string `query:"parent_id" format:"uuid" required:"false"`
}
type repliesInput struct {
	Session   string `cookie:"then_session" hidden:"true"`
	Limit     int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID   string `query:"after_id" format:"uuid" required:"false"`
	CommentID string `path:"comment_id" format:"uuid"`
}
type createCommentInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	PostID  string `path:"post_id" format:"uuid"`
	Body    CreateCommentRequest
}
type commentResourceInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ID               string `path:"comment_id" format:"uuid"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}
type decideCommentInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"comment_id" format:"uuid"`
	Body    DecideCommentRequest
}
type removeCommentInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"comment_id" format:"uuid"`
	Body    RemovePostRequest
}
type notificationInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"notification_id" format:"uuid"`
}
type createAppealInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateAppealRequest
}
type adminAppealsInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
	Status  string `query:"status" required:"false" enum:"open,upheld,reversed"`
}
type resolveAppealInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"appeal_id" format:"uuid"`
	Body    ResolveAppealRequest
}

type publicPostPageOutput struct {
	RequestID string                 `header:"X-Request-ID"`
	Body      PublicPostPageResponse `json:"body"`
}
type publicProfilePageOutput struct {
	RequestID string                    `header:"X-Request-ID"`
	Body      PublicProfilePageResponse `json:"body"`
}
type blockedUserPageOutput struct {
	RequestID string                  `header:"X-Request-ID"`
	Body      BlockedUserPageResponse `json:"body"`
}
type commentOutput struct {
	RequestID string          `header:"X-Request-ID"`
	Body      CommentResponse `json:"body"`
}
type commentPageOutput struct {
	RequestID string              `header:"X-Request-ID"`
	Body      CommentPageResponse `json:"body"`
}
type commentModerationPageOutput struct {
	RequestID string                        `header:"X-Request-ID"`
	Body      CommentModerationPageResponse `json:"body"`
}
type notificationOutput struct {
	RequestID string               `header:"X-Request-ID"`
	Body      NotificationResponse `json:"body"`
}
type notificationPageOutput struct {
	RequestID string                   `header:"X-Request-ID"`
	Body      NotificationPageResponse `json:"body"`
}
type appealOutput struct {
	RequestID string         `header:"X-Request-ID"`
	Body      AppealResponse `json:"body"`
}
type appealPageOutput struct {
	RequestID string             `header:"X-Request-ID"`
	Body      AppealPageResponse `json:"body"`
}
type adminAppealPageOutput struct {
	RequestID string                  `header:"X-Request-ID"`
	Body      AdminAppealPageResponse `json:"body"`
}

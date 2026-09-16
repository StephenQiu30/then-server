package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

	"github.com/danielgtaylor/huma/v2"
)

type CommunityHTTPService interface {
	CreatePost(context.Context, string, string, communityapp.PostContentInput) (communityapp.Post, error)
	ListOwnPosts(context.Context, string, int, string, string) (communityapp.PostPage, error)
	GetOwnPost(context.Context, string, string) (communityapp.Post, error)
	UpdatePost(context.Context, string, string, int, communityapp.PostContentInput) (communityapp.Post, error)
	SubmitPost(context.Context, string, string, int) (communityapp.Post, error)
	WithdrawPost(context.Context, string, string, int) (communityapp.Post, error)
	DeletePost(context.Context, string, string, int) error
	GetPublicPostForViewer(context.Context, string, string) (communityapp.PublicPost, error)
	GetPublicPostImageForViewer(context.Context, string, string, int) (communityapp.MediaObjectReference, error)
	ListFeed(context.Context, string, string, int, string) (communityapp.PublicPostPage, error)
	SearchPosts(context.Context, string, string, string, int, string) (communityapp.PublicPostPage, error)
	ListProfilePosts(context.Context, string, string, int, string) (communityapp.PublicPostPage, error)
	SetPostLike(context.Context, string, string, bool) error
	SetPostBookmark(context.Context, string, string, bool) error
	ListBookmarks(context.Context, string, int, string) (communityapp.PublicPostPage, error)
	SetFollow(context.Context, string, string, bool) error
	ListProfileRelationships(context.Context, string, string, string, int, string) (communityapp.PublicProfilePage, error)
	SetBlock(context.Context, string, string, bool) error
	ListBlocks(context.Context, string, int, string) (communityapp.BlockedUserPage, error)
	CreateComment(context.Context, string, string, string, communityapp.CreateCommentInput) (communityapp.Comment, error)
	ListComments(context.Context, string, string, *string, int, string) (communityapp.CommentPage, error)
	ListReplies(context.Context, string, string, int, string) (communityapp.CommentPage, error)
	DeleteComment(context.Context, string, string, int) error
	ListCommentModeration(context.Context, string, int, string) (communityapp.CommentModerationPage, error)
	DecideComment(context.Context, string, string, communityapp.DecideCommentInput) (communityapp.Comment, error)
	RemoveComment(context.Context, string, string, int, string) error
	ListNotifications(context.Context, string, int, string) (communityapp.NotificationPage, error)
	MarkNotificationRead(context.Context, string, string) (communityapp.Notification, error)
	CreateAppeal(context.Context, string, string, string, string) (communityapp.ModerationAppeal, error)
	ListOwnAppeals(context.Context, string, int, string) (communityapp.ModerationAppealPage, error)
	ListAppeals(context.Context, string, int, string, string) (communityapp.AdminModerationAppealPage, error)
	ResolveAppeal(context.Context, string, string, string, string) (communityapp.ModerationAppeal, error)
	ListModerationCandidates(context.Context, string, int, string) (communityapp.ModerationCandidatePage, error)
	GetModerationCandidate(context.Context, string, string, int) (communityapp.ModerationCandidate, error)
	GetModerationImage(context.Context, string, string, int, int) (communityapp.MediaObjectReference, error)
	DecidePost(context.Context, string, string, communityapp.DecidePostInput) (communityapp.Post, error)
	RemovePost(context.Context, string, string, int, string) error
	CreateReport(context.Context, string, string, string, communityapp.CreateReportInput) (communityapp.ContentReport, error)
	ListOwnReports(context.Context, string, int, string) (communityapp.ContentReportPage, error)
	ListReports(context.Context, string, int, string, string) (communityapp.AdminReportPage, error)
	ResolveReport(context.Context, string, string, string, string) (communityapp.ContentReport, error)
	ListModerationActions(context.Context, string, int, string) (communityapp.ModerationActionPage, error)
	ListUsers(context.Context, string, int, string) (communityapp.AdminUserPage, error)
	SetUserStatus(context.Context, string, string, int, accountapp.AccountStatus, string) (communityapp.AdminUser, error)
}

type CommunityObjectStore interface {
	OpenDerivedVersion(context.Context, string, string) (io.ReadCloser, error)
}

type CommunityHandler struct {
	service      CommunityHTTPService
	objects      CommunityObjectStore
	secureCookie bool
}

func NewCommunityHandler(service CommunityHTTPService, objects CommunityObjectStore, secureCookie bool) *CommunityHandler {
	return &CommunityHandler{service: service, objects: objects, secureCookie: secureCookie}
}

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

func registerCommunityOperations(api huma.API, handler *CommunityHandler) {
	common := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError}
	admin := func(operation huma.Operation) huma.Operation { return authenticatedOperation(operation) }
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createPost", Method: http.MethodPost, Path: "/posts", Tags: []string{"Community posts"}, Summary: "创建本人私有帖子草稿", DefaultStatus: http.StatusCreated, MaxBodyBytes: 64 * 1024, Errors: common}), handler.createPost)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listOwnPosts", Method: http.MethodGet, Path: "/users/me/posts", Tags: []string{"Community posts"}, Summary: "分页读取本人全部帖子状态", Errors: common}), handler.listOwnPosts)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getOwnPost", Method: http.MethodGet, Path: "/users/me/posts/{post_id}", Tags: []string{"Community posts"}, Summary: "读取本人帖子及当前私有版本", Errors: common}), handler.getOwnPost)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updatePost", Method: http.MethodPut, Path: "/posts/{post_id}", Tags: []string{"Community posts"}, Summary: "创建新的私有帖子草稿版本", MaxBodyBytes: 64 * 1024, Errors: common}), handler.updatePost)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "submitPost", Method: http.MethodPost, Path: "/posts/{post_id}/submit", Tags: []string{"Community posts"}, Summary: "明确提交当前草稿进入人工审核", DefaultStatus: http.StatusAccepted, MaxBodyBytes: 4 * 1024, Errors: common}), handler.submitPost)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "withdrawPost", Method: http.MethodPost, Path: "/posts/{post_id}/withdraw", Tags: []string{"Community posts"}, Summary: "撤回公开或待审帖子", MaxBodyBytes: 4 * 1024, Errors: common}), handler.withdrawPost)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deletePost", Method: http.MethodDelete, Path: "/posts/{post_id}", Tags: []string{"Community posts"}, Summary: "删除帖子内容并保留最小墓碑", DefaultStatus: http.StatusNoContent, Errors: common}), handler.deletePost)
	huma.Register(api, huma.Operation{OperationID: "getPublicPost", Method: http.MethodGet, Path: "/posts/{post_id}", Tags: []string{"Public community"}, Summary: "匿名读取当前批准帖子版本", Errors: []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError}}, handler.getPublicPost)
	imageResponse := map[string]*huma.Response{"200": {Description: "净化后的固定 JPEG 版本", Content: map[string]*huma.MediaType{"image/jpeg": {Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"}}}}}
	huma.Register(api, huma.Operation{OperationID: "getPublicPostImage", Method: http.MethodGet, Path: "/posts/{post_id}/images/{ordinal}", Tags: []string{"Public community"}, Summary: "匿名读取当前批准版本图片", Responses: imageResponse, Errors: []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError}}, handler.getPublicImage)
	huma.Register(api, admin(huma.Operation{OperationID: "listPostModerationCandidates", Method: http.MethodGet, Path: "/admin/moderation/posts", Tags: []string{"Community moderation"}, Summary: "读取待审帖子队列", Errors: common}), handler.listCandidates)
	huma.Register(api, admin(huma.Operation{OperationID: "getPostModerationCandidate", Method: http.MethodGet, Path: "/admin/moderation/posts/{post_id}/revisions/{version}", Tags: []string{"Community moderation"}, Summary: "读取一个待审帖子版本", Errors: common}), handler.getCandidate)
	huma.Register(api, admin(huma.Operation{OperationID: "getPostModerationImage", Method: http.MethodGet, Path: "/admin/moderation/posts/{post_id}/revisions/{version}/images/{ordinal}", Tags: []string{"Community moderation"}, Summary: "读取待审版本净化图片", Responses: imageResponse, Errors: common}), handler.getModerationImage)
	huma.Register(api, admin(huma.Operation{OperationID: "decidePostModeration", Method: http.MethodPost, Path: "/admin/moderation/posts/{post_id}/decisions", Tags: []string{"Community moderation"}, Summary: "按版本、轮次和 revision 决定帖子审核", MaxBodyBytes: 8 * 1024, Errors: common}), handler.decidePost)
	huma.Register(api, admin(huma.Operation{OperationID: "removePublishedPost", Method: http.MethodPost, Path: "/admin/posts/{post_id}/remove", Tags: []string{"Community moderation"}, Summary: "独立下架帖子并记录治理动作", DefaultStatus: http.StatusNoContent, MaxBodyBytes: 8 * 1024, Errors: common}), handler.removePost)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createPostReport", Method: http.MethodPost, Path: "/reports", Tags: []string{"Community reports"}, Summary: "举报当前公开帖子", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024, Errors: common}), handler.createReport)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listOwnReports", Method: http.MethodGet, Path: "/users/me/reports", Tags: []string{"Community reports"}, Summary: "分页读取本人举报处理状态", Errors: common}), handler.listOwnReports)
	huma.Register(api, admin(huma.Operation{OperationID: "listCommunityReports", Method: http.MethodGet, Path: "/admin/reports", Tags: []string{"Community moderation"}, Summary: "运营分页读取举报", Errors: common}), handler.listReports)
	huma.Register(api, admin(huma.Operation{OperationID: "resolveCommunityReport", Method: http.MethodPost, Path: "/admin/reports/{report_id}/resolve", Tags: []string{"Community moderation"}, Summary: "解决或驳回举报，不自动下架", MaxBodyBytes: 8 * 1024, Errors: common}), handler.resolveReport)
	huma.Register(api, admin(huma.Operation{OperationID: "listModerationActions", Method: http.MethodGet, Path: "/admin/moderation-actions", Tags: []string{"Community moderation"}, Summary: "分页读取最小治理审计动作", Errors: common}), handler.listActions)
	huma.Register(api, admin(huma.Operation{OperationID: "listAdminUsers", Method: http.MethodGet, Path: "/admin/users", Tags: []string{"Account administration"}, Summary: "管理员分页读取账号状态", Errors: common}), handler.listUsers)
	huma.Register(api, admin(huma.Operation{OperationID: "suspendUser", Method: http.MethodPost, Path: "/admin/users/{user_id}/suspend", Tags: []string{"Account administration"}, Summary: "封禁账号并撤销全部会话", MaxBodyBytes: 8 * 1024, Errors: common}), handler.suspendUser)
	huma.Register(api, admin(huma.Operation{OperationID: "restoreUser", Method: http.MethodPost, Path: "/admin/users/{user_id}/restore", Tags: []string{"Account administration"}, Summary: "恢复账号，不改变帖子治理状态", MaxBodyBytes: 8 * 1024, Errors: common}), handler.restoreUser)
	registerCommunitySocialOperations(api, handler, common)
}

func (h *CommunityHandler) createPost(ctx context.Context, input *createPostInput) (*postOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	post, err := h.service.CreatePost(ctx, input.Session, input.Body.ID, postContent(input.Body.PostContentRequest))
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &postOutput{RequestID: requestID(ctx), Body: postResponse(post)}, nil
}

func (h *CommunityHandler) listOwnPosts(ctx context.Context, input *listOwnPostsInput) (*postPageOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	page, err := h.service.ListOwnPosts(ctx, input.Session, input.Limit, input.AfterID, input.State)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	posts := make([]PostResponse, 0, len(page.Posts))
	for _, post := range page.Posts {
		posts = append(posts, postResponse(post))
	}
	return &postPageOutput{RequestID: requestID(ctx), Body: PostPageResponse{Posts: posts, NextAfterID: page.NextAfterID}}, nil
}

func (h *CommunityHandler) getOwnPost(ctx context.Context, input *ownPostInput) (*postOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	post, err := h.service.GetOwnPost(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &postOutput{RequestID: requestID(ctx), Body: postResponse(post)}, nil
}

func (h *CommunityHandler) updatePost(ctx context.Context, input *updatePostInput) (*postOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	post, err := h.service.UpdatePost(ctx, input.Session, input.ID, input.Body.ExpectedRevision, postContent(input.Body.PostContentRequest))
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &postOutput{RequestID: requestID(ctx), Body: postResponse(post)}, nil
}

func (h *CommunityHandler) submitPost(ctx context.Context, input *mutatePostInput) (*postOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	post, err := h.service.SubmitPost(ctx, input.Session, input.ID, input.Body.ExpectedRevision)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &postOutput{RequestID: requestID(ctx), Body: postResponse(post)}, nil
}

func (h *CommunityHandler) withdrawPost(ctx context.Context, input *mutatePostInput) (*postOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	post, err := h.service.WithdrawPost(ctx, input.Session, input.ID, input.Body.ExpectedRevision)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &postOutput{RequestID: requestID(ctx), Body: postResponse(post)}, nil
}

func (h *CommunityHandler) deletePost(ctx context.Context, input *deletePostInput) (*communityEmptyOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	if err := h.service.DeletePost(ctx, input.Session, input.ID, input.ExpectedRevision); err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &communityEmptyOutput{RequestID: requestID(ctx)}, nil
}

func (h *CommunityHandler) getPublicPost(ctx context.Context, input *postResourceInput) (*publicPostOutput, error) {
	if err := h.available(ctx, "", true); err != nil {
		return nil, err
	}
	post, err := h.service.GetPublicPostForViewer(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &publicPostOutput{RequestID: requestID(ctx), Body: publicPostResponse(post)}, nil
}

func (h *CommunityHandler) getPublicImage(ctx context.Context, input *postImageInput) (*imageOutput, error) {
	if err := h.available(ctx, "", true); err != nil {
		return nil, err
	}
	reference, err := h.service.GetPublicPostImageForViewer(ctx, input.Session, input.ID, input.Ordinal)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return h.readImage(ctx, reference)
}

func (h *CommunityHandler) listCandidates(ctx context.Context, input *moderationListInput) (*candidatePageOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	page, err := h.service.ListModerationCandidates(ctx, input.Session, input.Limit, input.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	candidates := make([]ModerationCandidateResponse, 0, len(page.Candidates))
	for _, candidate := range page.Candidates {
		candidates = append(candidates, candidateResponse(candidate))
	}
	return &candidatePageOutput{RequestID: requestID(ctx), Body: ModerationCandidatePageResponse{Candidates: candidates, NextAfterID: page.NextAfterID}}, nil
}

func (h *CommunityHandler) getCandidate(ctx context.Context, input *moderationCandidateInput) (*candidateOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	candidate, err := h.service.GetModerationCandidate(ctx, input.Session, input.ID, input.Version)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &candidateOutput{RequestID: requestID(ctx), Body: candidateResponse(candidate)}, nil
}

func (h *CommunityHandler) getModerationImage(ctx context.Context, input *moderationImageInput) (*imageOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	reference, err := h.service.GetModerationImage(ctx, input.Session, input.ID, input.Version, input.Ordinal)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return h.readImage(ctx, reference)
}

func (h *CommunityHandler) decidePost(ctx context.Context, input *decisionInput) (*postOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	post, err := h.service.DecidePost(ctx, input.Session, input.ID, communityapp.DecidePostInput{Version: input.Body.Version, ReviewRound: input.Body.ReviewRound, ExpectedRevision: input.Body.ExpectedRevision, Decision: communityapp.ModerationDecision(input.Body.Decision), ReasonCode: input.Body.ReasonCode})
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &postOutput{RequestID: requestID(ctx), Body: postResponse(post)}, nil
}

func (h *CommunityHandler) removePost(ctx context.Context, input *removePostInput) (*communityEmptyOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	if err := h.service.RemovePost(ctx, input.Session, input.ID, input.Body.ExpectedRevision, input.Body.ReasonCode); err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &communityEmptyOutput{RequestID: requestID(ctx)}, nil
}

func (h *CommunityHandler) createReport(ctx context.Context, input *createReportInput) (*reportOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	report, err := h.service.CreateReport(ctx, input.Session, input.Body.ID, input.Body.PostID, communityapp.CreateReportInput{TargetType: communityapp.ReportTargetType(input.Body.TargetType), CommentID: input.Body.CommentID, ReasonCode: input.Body.ReasonCode, Detail: input.Body.Detail})
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &reportOutput{RequestID: requestID(ctx), Body: reportResponse(report)}, nil
}

func (h *CommunityHandler) listOwnReports(ctx context.Context, input *reportsInput) (*reportPageOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	page, err := h.service.ListOwnReports(ctx, input.Session, input.Limit, input.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	reports := make([]ContentReportResponse, 0, len(page.Reports))
	for _, report := range page.Reports {
		reports = append(reports, reportResponse(report))
	}
	return &reportPageOutput{RequestID: requestID(ctx), Body: ReportPageResponse{Reports: reports, NextAfterID: page.NextAfterID}}, nil
}

func (h *CommunityHandler) listReports(ctx context.Context, input *reportsInput) (*adminReportPageOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	page, err := h.service.ListReports(ctx, input.Session, input.Limit, input.AfterID, input.Status)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	reports := make([]AdminReportResponse, 0, len(page.Reports))
	for _, report := range page.Reports {
		reports = append(reports, AdminReportResponse{ContentReportResponse: reportResponse(report.ContentReport), ReporterID: report.ReporterID})
	}
	return &adminReportPageOutput{RequestID: requestID(ctx), Body: AdminReportPageResponse{Reports: reports, NextAfterID: page.NextAfterID}}, nil
}

func (h *CommunityHandler) resolveReport(ctx context.Context, input *reportResourceInput) (*reportOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	report, err := h.service.ResolveReport(ctx, input.Session, input.ID, input.Body.Status, input.Body.ResolutionCode)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &reportOutput{RequestID: requestID(ctx), Body: reportResponse(report)}, nil
}

func (h *CommunityHandler) listActions(ctx context.Context, input *moderationListInput) (*actionPageOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	page, err := h.service.ListModerationActions(ctx, input.Session, input.Limit, input.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	actions := make([]ModerationActionResponse, 0, len(page.Actions))
	for _, action := range page.Actions {
		actions = append(actions, ModerationActionResponse{ID: action.ID, ActorID: action.ActorID, PostID: action.PostID, PostVersion: action.PostVersion, CommentID: action.CommentID, ReportID: action.ReportID, SubjectUserID: action.SubjectUserID, Action: action.Action, ReasonCode: action.ReasonCode, CreatedAt: action.CreatedAt})
	}
	return &actionPageOutput{RequestID: requestID(ctx), Body: ModerationActionPageResponse{Actions: actions, NextAfterID: page.NextAfterID}}, nil
}

func (h *CommunityHandler) listUsers(ctx context.Context, input *moderationListInput) (*adminUserPageOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	page, err := h.service.ListUsers(ctx, input.Session, input.Limit, input.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	users := make([]AdminUserResponse, 0, len(page.Users))
	for _, user := range page.Users {
		users = append(users, adminUserResponse(user))
	}
	return &adminUserPageOutput{RequestID: requestID(ctx), Body: AdminUserPageResponse{Users: users, NextAfterID: page.NextAfterID}}, nil
}

func (h *CommunityHandler) suspendUser(ctx context.Context, input *adminUserInput) (*adminUserOutput, error) {
	return h.setUserStatus(ctx, input, accountapp.AccountSuspended)
}

func (h *CommunityHandler) restoreUser(ctx context.Context, input *adminUserInput) (*adminUserOutput, error) {
	return h.setUserStatus(ctx, input, accountapp.AccountActive)
}

func (h *CommunityHandler) setUserStatus(ctx context.Context, input *adminUserInput, status accountapp.AccountStatus) (*adminUserOutput, error) {
	if err := h.available(ctx, input.Session, false); err != nil {
		return nil, err
	}
	user, err := h.service.SetUserStatus(ctx, input.Session, input.ID, input.Body.ExpectedRevision, status, input.Body.ReasonCode)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &adminUserOutput{RequestID: requestID(ctx), Body: adminUserResponse(user)}, nil
}

func (h *CommunityHandler) readImage(ctx context.Context, reference communityapp.MediaObjectReference) (*imageOutput, error) {
	reader, err := h.objects.OpenDerivedVersion(ctx, reference.ObjectKey, reference.ObjectVersionID)
	if err != nil {
		return nil, newErrorResponse(http.StatusNotFound, requestID(ctx))
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 12*1024*1024+1))
	if err != nil || len(data) == 0 || len(data) > 12*1024*1024 {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	return &imageOutput{RequestID: requestID(ctx), ContentType: "image/jpeg", Body: data}, nil
}

func (h *CommunityHandler) available(ctx context.Context, session string, public bool) error {
	if h == nil || h.service == nil || h.objects == nil {
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if !public && session == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}

func (h *CommunityHandler) mapError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	case errors.Is(err, communityapp.ErrInvalidCommunityInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, communityapp.ErrPostNotFound), errors.Is(err, communityapp.ErrCommentNotFound), errors.Is(err, communityapp.ErrAppealNotFound), errors.Is(err, communityapp.ErrNotificationNotFound), errors.Is(err, communityapp.ErrReportNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, communityapp.ErrCommunityForbidden):
		response := newErrorResponse(http.StatusForbidden, requestID(ctx))
		response.Code, response.Message = "FORBIDDEN", "Operation is not allowed for this account."
		return response
	case errors.Is(err, communityapp.ErrPostConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Request conflicts with the current community state."
		return response
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func postContent(request PostContentRequest) communityapp.PostContentInput {
	return communityapp.PostContentInput{Title: request.Title, Body: request.Body, SourceDiaryID: request.SourceDiaryID, MediaIDs: append([]string(nil), request.MediaIDs...), Tags: append([]string(nil), request.Tags...)}
}

func postResponse(post communityapp.Post) PostResponse {
	var published *PostRevisionResponse
	if post.Published != nil {
		value := revisionResponse(*post.Published)
		published = &value
	}
	return PostResponse{ID: post.ID, State: string(post.State), Revision: post.Revision, DraftVersion: post.DraftVersion, PendingVersion: post.PendingVersion, PublishedVersion: post.PublishedVersion, ReviewRound: post.ReviewRound, SourceDiaryID: post.SourceDiaryID, Current: revisionResponse(post.Current), Published: published, PublishedAt: post.PublishedAt, WithdrawnAt: post.WithdrawnAt, CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt}
}

func revisionResponse(revision communityapp.PostRevision) PostRevisionResponse {
	return PostRevisionResponse{Version: revision.Version, Title: revision.Title, Body: revision.Body, MediaIDs: append([]string(nil), revision.MediaIDs...), Tags: append([]string(nil), revision.Tags...), ReviewState: string(revision.ReviewState), ReviewRound: revision.ReviewRound, ReasonCode: revision.ReasonCode, SubmittedAt: revision.SubmittedAt, ReviewedAt: revision.ReviewedAt, CreatedAt: revision.CreatedAt}
}

func publicPostResponse(post communityapp.PublicPost) PublicPostResponse {
	return PublicPostResponse{ID: post.ID, AuthorHandle: post.AuthorHandle, AuthorDisplayName: post.AuthorDisplayName, Title: post.Title, Body: post.Body, Tags: append([]string(nil), post.Tags...), ImageCount: post.ImageCount, LikeCount: post.LikeCount, CommentCount: post.CommentCount, ViewerLiked: post.ViewerLiked, ViewerBookmarked: post.ViewerBookmarked, FollowingAuthor: post.FollowingAuthor, PublishedAt: post.PublishedAt}
}

func candidateResponse(candidate communityapp.ModerationCandidate) ModerationCandidateResponse {
	return ModerationCandidateResponse{PostID: candidate.PostID, PostRevision: candidate.PostRevision, Version: candidate.Version, ReviewRound: candidate.ReviewRound, AuthorHandle: candidate.AuthorHandle, AuthorDisplayName: candidate.AuthorDisplayName, Title: candidate.Title, Body: candidate.Body, Tags: append([]string(nil), candidate.Tags...), ImageCount: candidate.ImageCount, SubmittedAt: candidate.SubmittedAt}
}

func reportResponse(report communityapp.ContentReport) ContentReportResponse {
	return ContentReportResponse{ID: report.ID, PostID: report.PostID, TargetType: string(report.TargetType), CommentID: report.CommentID, ReasonCode: report.ReasonCode, Detail: report.Detail, Status: string(report.Status), ResolutionCode: report.ResolutionCode, CreatedAt: report.CreatedAt, ResolvedAt: report.ResolvedAt}
}

func adminUserResponse(user communityapp.AdminUser) AdminUserResponse {
	return AdminUserResponse{ID: user.ID, DisplayName: user.DisplayName, Status: string(user.Status), Role: string(user.Role), Revision: user.Revision, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}

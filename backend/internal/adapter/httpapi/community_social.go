package httpapi

import (
	"context"
	"net/http"
	"time"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"

	"github.com/danielgtaylor/huma/v2"
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

func registerCommunitySocialOperations(api huma.API, h *CommunityHandler, common []int) {
	auth := func(operation huma.Operation) huma.Operation { return authenticatedOperation(operation) }
	reg := func(id, method, path, tag, summary string, operation huma.Operation, handler any) {
		operation.OperationID, operation.Method, operation.Path, operation.Tags, operation.Summary, operation.Errors = id, method, path, []string{tag}, summary, common
		switch fn := handler.(type) {
		case func(context.Context, *feedInput) (*publicPostPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *searchInput) (*publicPostPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *profilePageInput) (*publicPostPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *profileRelationshipInput) (*publicProfilePageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *profilePageInput) (*publicProfilePageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *relationInput) (*communityEmptyOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *authenticatedPageInput) (*publicPostPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *followInput) (*communityEmptyOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *blockInput) (*communityEmptyOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *authenticatedPageInput) (*blockedUserPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *createCommentInput) (*commentOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *commentsInput) (*commentPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *repliesInput) (*commentPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *commentResourceInput) (*communityEmptyOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *authenticatedPageInput) (*commentModerationPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *decideCommentInput) (*commentOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *removeCommentInput) (*communityEmptyOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *authenticatedPageInput) (*notificationPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *notificationInput) (*notificationOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *createAppealInput) (*appealOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *authenticatedPageInput) (*appealPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *adminAppealsInput) (*adminAppealPageOutput, error):
			huma.Register(api, operation, fn)
		case func(context.Context, *resolveAppealInput) (*appealOutput, error):
			huma.Register(api, operation, fn)
		}
	}
	reg("listCommunityFeed", http.MethodGet, "/feed", "Community discovery", "读取发现或关注时间流", huma.Operation{}, h.listFeed)
	reg("searchCommunityPosts", http.MethodGet, "/search/posts", "Community discovery", "按关键词或标签搜索公开帖子", huma.Operation{}, h.searchPosts)
	reg("listProfilePosts", http.MethodGet, "/profiles/{handle}/posts", "Community discovery", "读取公开主页帖子", huma.Operation{}, h.listProfilePosts)
	reg("listProfileFollowers", http.MethodGet, "/profiles/{handle}/followers", "Community relationships", "读取粉丝列表", huma.Operation{}, h.listProfileFollowers)
	reg("listProfileFollowing", http.MethodGet, "/profiles/{handle}/following", "Community relationships", "读取关注列表", huma.Operation{}, h.listProfileFollowing)
	reg("likePost", http.MethodPut, "/posts/{post_id}/like", "Community interactions", "点赞帖子", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.likePost)
	reg("unlikePost", http.MethodDelete, "/posts/{post_id}/like", "Community interactions", "取消点赞", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.unlikePost)
	reg("bookmarkPost", http.MethodPut, "/posts/{post_id}/bookmark", "Community interactions", "收藏帖子", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.bookmarkPost)
	reg("unbookmarkPost", http.MethodDelete, "/posts/{post_id}/bookmark", "Community interactions", "取消收藏", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.unbookmarkPost)
	reg("listBookmarks", http.MethodGet, "/users/me/bookmarks", "Community interactions", "读取本人收藏", auth(huma.Operation{}), h.listBookmarks)
	reg("followProfile", http.MethodPut, "/profiles/{handle}/follow", "Community relationships", "关注用户", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.followProfile)
	reg("unfollowProfile", http.MethodDelete, "/profiles/{handle}/follow", "Community relationships", "取消关注", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.unfollowProfile)
	reg("blockUser", http.MethodPut, "/users/me/blocks/{user_id}", "Community safety", "屏蔽用户并解除双向关注", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.blockUser)
	reg("unblockUser", http.MethodDelete, "/users/me/blocks/{user_id}", "Community safety", "取消屏蔽用户", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.unblockUser)
	reg("listBlockedUsers", http.MethodGet, "/users/me/blocks", "Community safety", "读取本人屏蔽列表", auth(huma.Operation{}), h.listBlocks)
	reg("createComment", http.MethodPost, "/posts/{post_id}/comments", "Community comments", "创建待审评论或回复", auth(huma.Operation{DefaultStatus: http.StatusAccepted, MaxBodyBytes: 8 * 1024}), h.createComment)
	reg("listComments", http.MethodGet, "/posts/{post_id}/comments", "Community comments", "读取公开评论或回复", huma.Operation{}, h.listComments)
	reg("listCommentReplies", http.MethodGet, "/comments/{comment_id}/replies", "Community comments", "读取一级回复", huma.Operation{}, h.listReplies)
	reg("deleteComment", http.MethodDelete, "/comments/{comment_id}", "Community comments", "删除本人评论并保留墓碑", auth(huma.Operation{DefaultStatus: http.StatusNoContent}), h.deleteComment)
	reg("listCommentModerationCandidates", http.MethodGet, "/admin/moderation/comments", "Community moderation", "读取待审评论", auth(huma.Operation{}), h.listCommentModeration)
	reg("decideCommentModeration", http.MethodPost, "/admin/moderation/comments/{comment_id}/decisions", "Community moderation", "审核评论", auth(huma.Operation{MaxBodyBytes: 8 * 1024}), h.decideComment)
	reg("removePublishedComment", http.MethodPost, "/admin/comments/{comment_id}/remove", "Community moderation", "下架已发布评论", auth(huma.Operation{DefaultStatus: http.StatusNoContent, MaxBodyBytes: 8 * 1024}), h.removeComment)
	reg("listNotifications", http.MethodGet, "/notifications", "Notifications", "读取已投递通知", auth(huma.Operation{}), h.listNotifications)
	reg("markNotificationRead", http.MethodPut, "/notifications/{notification_id}/read", "Notifications", "标记通知已读", auth(huma.Operation{}), h.markNotificationRead)
	reg("createModerationAppeal", http.MethodPost, "/moderation-appeals", "Community appeals", "申诉关联治理动作", auth(huma.Operation{DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024}), h.createAppeal)
	reg("listOwnModerationAppeals", http.MethodGet, "/users/me/moderation-appeals", "Community appeals", "读取本人申诉", auth(huma.Operation{}), h.listOwnAppeals)
	reg("listModerationAppeals", http.MethodGet, "/admin/moderation-appeals", "Community appeals", "管理员读取申诉", auth(huma.Operation{}), h.listAppeals)
	reg("resolveModerationAppeal", http.MethodPost, "/admin/moderation-appeals/{appeal_id}/resolve", "Community appeals", "管理员解决申诉", auth(huma.Operation{MaxBodyBytes: 8 * 1024}), h.resolveAppeal)
}

func (h *CommunityHandler) listFeed(ctx context.Context, in *feedInput) (*publicPostPageOutput, error) {
	p, e := h.service.ListFeed(ctx, in.Session, in.Type, in.Limit, in.AfterID)
	return h.publicPostPage(ctx, p, e)
}
func (h *CommunityHandler) searchPosts(ctx context.Context, in *searchInput) (*publicPostPageOutput, error) {
	p, e := h.service.SearchPosts(ctx, in.Session, in.Query, in.Tag, in.Limit, in.AfterID)
	return h.publicPostPage(ctx, p, e)
}
func (h *CommunityHandler) listProfilePosts(ctx context.Context, in *profilePageInput) (*publicPostPageOutput, error) {
	p, e := h.service.ListProfilePosts(ctx, in.Session, in.Handle, in.Limit, in.AfterID)
	return h.publicPostPage(ctx, p, e)
}
func (h *CommunityHandler) publicPostPage(ctx context.Context, page communityapp.PublicPostPage, err error) (*publicPostPageOutput, error) {
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]PublicPostResponse, 0, len(page.Posts))
	for _, item := range page.Posts {
		items = append(items, publicPostResponse(item))
	}
	return &publicPostPageOutput{RequestID: requestID(ctx), Body: PublicPostPageResponse{Posts: items, NextAfterID: page.NextAfterID}}, nil
}
func (h *CommunityHandler) listProfileRelationships(ctx context.Context, in *profileRelationshipInput) (*publicProfilePageOutput, error) {
	p, err := h.service.ListProfileRelationships(ctx, in.Session, in.Handle, in.Direction, in.Limit, in.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]PublicProfileSummaryResponse, 0, len(p.Profiles))
	for _, v := range p.Profiles {
		items = append(items, PublicProfileSummaryResponse{Handle: v.Handle, DisplayName: v.DisplayName, Bio: v.Bio, Following: v.Following, FollowerCount: v.FollowerCount, FollowingCount: v.FollowingCount})
	}
	return &publicProfilePageOutput{RequestID: requestID(ctx), Body: PublicProfilePageResponse{Profiles: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) listProfileFollowers(ctx context.Context, in *profilePageInput) (*publicProfilePageOutput, error) {
	return h.listProfileRelationships(ctx, &profileRelationshipInput{profilePageInput: *in, Direction: "followers"})
}
func (h *CommunityHandler) listProfileFollowing(ctx context.Context, in *profilePageInput) (*publicProfilePageOutput, error) {
	return h.listProfileRelationships(ctx, &profileRelationshipInput{profilePageInput: *in, Direction: "following"})
}
func (h *CommunityHandler) setPostRelation(ctx context.Context, in *relationInput, like, enabled bool) (*communityEmptyOutput, error) {
	var err error
	if like {
		err = h.service.SetPostLike(ctx, in.Session, in.ID, enabled)
	} else {
		err = h.service.SetPostBookmark(ctx, in.Session, in.ID, enabled)
	}
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &communityEmptyOutput{RequestID: requestID(ctx)}, nil
}
func (h *CommunityHandler) likePost(c context.Context, i *relationInput) (*communityEmptyOutput, error) {
	return h.setPostRelation(c, i, true, true)
}
func (h *CommunityHandler) unlikePost(c context.Context, i *relationInput) (*communityEmptyOutput, error) {
	return h.setPostRelation(c, i, true, false)
}
func (h *CommunityHandler) bookmarkPost(c context.Context, i *relationInput) (*communityEmptyOutput, error) {
	return h.setPostRelation(c, i, false, true)
}
func (h *CommunityHandler) unbookmarkPost(c context.Context, i *relationInput) (*communityEmptyOutput, error) {
	return h.setPostRelation(c, i, false, false)
}
func (h *CommunityHandler) listBookmarks(ctx context.Context, in *authenticatedPageInput) (*publicPostPageOutput, error) {
	p, e := h.service.ListBookmarks(ctx, in.Session, in.Limit, in.AfterID)
	return h.publicPostPage(ctx, p, e)
}
func (h *CommunityHandler) setFollow(ctx context.Context, in *followInput, enabled bool) (*communityEmptyOutput, error) {
	if err := h.service.SetFollow(ctx, in.Session, in.Handle, enabled); err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &communityEmptyOutput{RequestID: requestID(ctx)}, nil
}
func (h *CommunityHandler) followProfile(c context.Context, i *followInput) (*communityEmptyOutput, error) {
	return h.setFollow(c, i, true)
}
func (h *CommunityHandler) unfollowProfile(c context.Context, i *followInput) (*communityEmptyOutput, error) {
	return h.setFollow(c, i, false)
}
func (h *CommunityHandler) setBlock(ctx context.Context, in *blockInput, enabled bool) (*communityEmptyOutput, error) {
	if err := h.service.SetBlock(ctx, in.Session, in.UserID, enabled); err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &communityEmptyOutput{RequestID: requestID(ctx)}, nil
}
func (h *CommunityHandler) blockUser(c context.Context, i *blockInput) (*communityEmptyOutput, error) {
	return h.setBlock(c, i, true)
}
func (h *CommunityHandler) unblockUser(c context.Context, i *blockInput) (*communityEmptyOutput, error) {
	return h.setBlock(c, i, false)
}
func (h *CommunityHandler) listBlocks(ctx context.Context, in *authenticatedPageInput) (*blockedUserPageOutput, error) {
	p, err := h.service.ListBlocks(ctx, in.Session, in.Limit, in.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]BlockedUserResponse, 0, len(p.Users))
	for _, v := range p.Users {
		items = append(items, BlockedUserResponse{UserID: v.UserID, Handle: v.Handle, DisplayName: v.DisplayName, BlockedAt: v.BlockedAt})
	}
	return &blockedUserPageOutput{RequestID: requestID(ctx), Body: BlockedUserPageResponse{Users: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) createComment(ctx context.Context, in *createCommentInput) (*commentOutput, error) {
	v, err := h.service.CreateComment(ctx, in.Session, in.Body.ID, in.PostID, communityapp.CreateCommentInput{Body: in.Body.Body, ParentID: in.Body.ParentID})
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &commentOutput{RequestID: requestID(ctx), Body: commentResponse(v)}, nil
}
func (h *CommunityHandler) listComments(ctx context.Context, in *commentsInput) (*commentPageOutput, error) {
	var parentID *string
	if in.ParentID != "" {
		parentID = &in.ParentID
	}
	p, err := h.service.ListComments(ctx, in.Session, in.PostID, parentID, in.Limit, in.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]CommentResponse, 0, len(p.Comments))
	for _, v := range p.Comments {
		items = append(items, commentResponse(v))
	}
	return &commentPageOutput{RequestID: requestID(ctx), Body: CommentPageResponse{Comments: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) listReplies(ctx context.Context, in *repliesInput) (*commentPageOutput, error) {
	p, err := h.service.ListReplies(ctx, in.Session, in.CommentID, in.Limit, in.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]CommentResponse, 0, len(p.Comments))
	for _, v := range p.Comments {
		items = append(items, commentResponse(v))
	}
	return &commentPageOutput{RequestID: requestID(ctx), Body: CommentPageResponse{Comments: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) deleteComment(ctx context.Context, in *commentResourceInput) (*communityEmptyOutput, error) {
	if err := h.service.DeleteComment(ctx, in.Session, in.ID, in.ExpectedRevision); err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &communityEmptyOutput{RequestID: requestID(ctx)}, nil
}
func (h *CommunityHandler) listCommentModeration(ctx context.Context, in *authenticatedPageInput) (*commentModerationPageOutput, error) {
	p, err := h.service.ListCommentModeration(ctx, in.Session, in.Limit, in.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]CommentModerationResponse, 0, len(p.Comments))
	for _, v := range p.Comments {
		items = append(items, CommentModerationResponse{CommentResponse: commentResponse(v.Comment), PostAuthorHandle: v.PostAuthorHandle, PostTitle: v.PostTitle})
	}
	return &commentModerationPageOutput{RequestID: requestID(ctx), Body: CommentModerationPageResponse{Comments: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) decideComment(ctx context.Context, in *decideCommentInput) (*commentOutput, error) {
	v, err := h.service.DecideComment(ctx, in.Session, in.ID, communityapp.DecideCommentInput{ExpectedRevision: in.Body.ExpectedRevision, Decision: communityapp.ModerationDecision(in.Body.Decision), ReasonCode: in.Body.ReasonCode})
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &commentOutput{RequestID: requestID(ctx), Body: commentResponse(v)}, nil
}
func (h *CommunityHandler) removeComment(ctx context.Context, in *removeCommentInput) (*communityEmptyOutput, error) {
	if err := h.service.RemoveComment(ctx, in.Session, in.ID, in.Body.ExpectedRevision, in.Body.ReasonCode); err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &communityEmptyOutput{RequestID: requestID(ctx)}, nil
}
func (h *CommunityHandler) listNotifications(ctx context.Context, in *authenticatedPageInput) (*notificationPageOutput, error) {
	p, err := h.service.ListNotifications(ctx, in.Session, in.Limit, in.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]NotificationResponse, 0, len(p.Notifications))
	for _, v := range p.Notifications {
		items = append(items, notificationResponse(v))
	}
	return &notificationPageOutput{RequestID: requestID(ctx), Body: NotificationPageResponse{Notifications: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) markNotificationRead(ctx context.Context, in *notificationInput) (*notificationOutput, error) {
	v, err := h.service.MarkNotificationRead(ctx, in.Session, in.ID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &notificationOutput{RequestID: requestID(ctx), Body: notificationResponse(v)}, nil
}
func (h *CommunityHandler) createAppeal(ctx context.Context, in *createAppealInput) (*appealOutput, error) {
	v, err := h.service.CreateAppeal(ctx, in.Session, in.Body.ID, in.Body.ActionID, in.Body.Reason)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &appealOutput{RequestID: requestID(ctx), Body: appealResponse(v)}, nil
}
func (h *CommunityHandler) listOwnAppeals(ctx context.Context, in *authenticatedPageInput) (*appealPageOutput, error) {
	p, err := h.service.ListOwnAppeals(ctx, in.Session, in.Limit, in.AfterID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]AppealResponse, 0, len(p.Appeals))
	for _, v := range p.Appeals {
		items = append(items, appealResponse(v))
	}
	return &appealPageOutput{RequestID: requestID(ctx), Body: AppealPageResponse{Appeals: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) listAppeals(ctx context.Context, in *adminAppealsInput) (*adminAppealPageOutput, error) {
	p, err := h.service.ListAppeals(ctx, in.Session, in.Limit, in.AfterID, in.Status)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	items := make([]AdminAppealResponse, 0, len(p.Appeals))
	for _, v := range p.Appeals {
		items = append(items, AdminAppealResponse{AppealResponse: appealResponse(v.ModerationAppeal), AppellantID: v.AppellantID})
	}
	return &adminAppealPageOutput{RequestID: requestID(ctx), Body: AdminAppealPageResponse{Appeals: items, NextAfterID: p.NextAfterID}}, nil
}
func (h *CommunityHandler) resolveAppeal(ctx context.Context, in *resolveAppealInput) (*appealOutput, error) {
	v, err := h.service.ResolveAppeal(ctx, in.Session, in.ID, in.Body.Status, in.Body.ResolutionCode)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &appealOutput{RequestID: requestID(ctx), Body: appealResponse(v)}, nil
}

func commentResponse(v communityapp.Comment) CommentResponse {
	return CommentResponse{ID: v.ID, PostID: v.PostID, ParentID: v.ParentID, AuthorHandle: v.AuthorHandle, AuthorDisplayName: v.AuthorDisplayName, Body: v.Body, State: string(v.State), Revision: v.Revision, ReasonCode: v.ReasonCode, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func notificationResponse(v communityapp.Notification) NotificationResponse {
	return NotificationResponse{ID: v.ID, Kind: string(v.Kind), ActorHandle: v.ActorHandle, PostID: v.PostID, CommentID: v.CommentID, ReadAt: v.ReadAt, CreatedAt: v.CreatedAt}
}
func appealResponse(v communityapp.ModerationAppeal) AppealResponse {
	return AppealResponse{ID: v.ID, ActionID: v.ActionID, Action: v.Action, Reason: v.Reason, Status: string(v.Status), ResolutionCode: v.ResolutionCode, CreatedAt: v.CreatedAt, ResolvedAt: v.ResolvedAt}
}

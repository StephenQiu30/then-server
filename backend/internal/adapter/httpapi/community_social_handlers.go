package httpapi

import (
	"context"

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"
)

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

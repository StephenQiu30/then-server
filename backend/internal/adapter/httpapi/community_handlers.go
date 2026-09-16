package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"
)

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

package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

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

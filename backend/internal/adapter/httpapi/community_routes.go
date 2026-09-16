package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

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

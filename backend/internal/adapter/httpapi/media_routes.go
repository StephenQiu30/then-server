package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerMediaOperations(api huma.API, handler *MediaHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createConsent", Method: http.MethodPost, Path: "/consents", Tags: []string{"Private media"}, Summary: "同意本人照片的固定本地开发用途", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024, Errors: mediaErrors(http.StatusBadRequest)}), handler.createConsent)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getConsent", Method: http.MethodGet, Path: "/consents/{consent_id}", Tags: []string{"Private media"}, Summary: "获取本人同意状态", Errors: mediaErrors(http.StatusNotFound)}), handler.getConsent)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "withdrawConsent", Method: http.MethodPost, Path: "/consents/{consent_id}/withdraw", Tags: []string{"Private media"}, Summary: "撤回本人照片同意", Errors: mediaErrors(http.StatusNotFound)}), handler.withdrawConsent)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createMediaUpload", Method: http.MethodPost, Path: "/media/uploads", Tags: []string{"Private media"}, Summary: "创建一次 JPEG 私有直传意图", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024, Errors: mediaErrors(http.StatusBadRequest, http.StatusConflict, http.StatusRequestEntityTooLarge)}), handler.createUpload)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "completeMediaUpload", Method: http.MethodPost, Path: "/media/{media_id}/complete", Tags: []string{"Private media"}, Summary: "固定已上传对象版本", MaxBodyBytes: 4 * 1024, Errors: mediaErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.completeUpload)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getMedia", Method: http.MethodGet, Path: "/media/{media_id}", Tags: []string{"Private media"}, Summary: "获取本人媒体处理状态", Errors: mediaErrors(http.StatusNotFound)}), handler.getMedia)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteMedia", Method: http.MethodDelete, Path: "/media/{media_id}", Tags: []string{"Private media"}, Summary: "立即撤销读取并请求删除媒体", DefaultStatus: http.StatusAccepted, Errors: mediaErrors(http.StatusNotFound, http.StatusConflict)}), handler.deleteMedia)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getDeletionRequest", Method: http.MethodGet, Path: "/deletion-requests/{request_id}", Tags: []string{"Private media"}, Summary: "获取本人媒体删除进度", Errors: mediaErrors(http.StatusNotFound)}), handler.getDeletion)
}

func mediaErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusServiceUnavailable, http.StatusInternalServerError}, statuses...)
}

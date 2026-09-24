package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerGenerationOperations(api huma.API, handler *GenerationHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createGenerationJob", Method: http.MethodPost, Path: "/generation-jobs", Tags: []string{"Generation"}, Summary: "接纳一次 provider 无关的生成任务", DefaultStatus: http.StatusAccepted, MaxBodyBytes: 64 * 1024, Errors: generationErrors(http.StatusBadRequest, http.StatusConflict)}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listGenerationJobs", Method: http.MethodGet, Path: "/generation-jobs", Tags: []string{"Generation"}, Summary: "列出本人生成任务", Errors: generationErrors(http.StatusBadRequest)}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getGenerationJob", Method: http.MethodGet, Path: "/generation-jobs/{job_id}", Tags: []string{"Generation"}, Summary: "获取本人生成任务", Errors: generationErrors(http.StatusBadRequest, http.StatusNotFound)}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "cancelGenerationJob", Method: http.MethodPost, Path: "/generation-jobs/{job_id}/cancel", Tags: []string{"Generation"}, Summary: "请求取消本人生成任务", DefaultStatus: http.StatusAccepted, Errors: generationErrors(http.StatusBadRequest, http.StatusConflict, http.StatusNotFound)}), handler.cancel)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteGenerationJob", Method: http.MethodDelete, Path: "/generation-jobs/{job_id}", Tags: []string{"Generation"}, Summary: "撤销并清理本人生成任务", DefaultStatus: http.StatusAccepted, Errors: generationErrors(http.StatusBadRequest, http.StatusConflict, http.StatusNotFound)}), handler.delete)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listUnknownGenerationSubmissions", Method: http.MethodGet, Path: "/admin/generation/submission-reconciliations", Tags: []string{"Generation"}, Summary: "管理员列出待核对的生成提交", Errors: generationErrors(http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound)}), handler.listUnknown)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "reconcileUnknownGenerationSubmission", Method: http.MethodPost, Path: "/admin/generation-jobs/{job_id}/submission-reconciliation", Tags: []string{"Generation"}, Summary: "管理员依据证据核对未知提交结果", Errors: generationErrors(http.StatusBadRequest, http.StatusForbidden, http.StatusConflict, http.StatusNotFound)}), handler.reconcileUnknown)
}

func generationErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusServiceUnavailable, http.StatusInternalServerError}, statuses...)
}

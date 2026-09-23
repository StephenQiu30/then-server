package httpapi

import (
	"github.com/danielgtaylor/huma/v2"
	"net/http"
)

func registerFeedbackOperations(api huma.API, handler *FeedbackHandler) {
	errors := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError}
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWearFeedback", Method: http.MethodGet, Path: "/wear-events/{wear_event_id}/feedback", Tags: []string{"Wear Feedback"}, Summary: "读取本人实际穿着的明确反馈", Errors: errors}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "saveWearFeedback", Method: http.MethodPut, Path: "/wear-events/{wear_event_id}/feedback", Tags: []string{"Wear Feedback"}, Summary: "按 revision 保存或纠正明确反馈", MaxBodyBytes: 4 * 1024, Errors: errors}), handler.save)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteWearFeedback", Method: http.MethodDelete, Path: "/wear-events/{wear_event_id}/feedback", Tags: []string{"Wear Feedback"}, Summary: "按 revision 撤回反馈", DefaultStatus: http.StatusNoContent, Errors: errors}), handler.delete)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWearStatistics", Method: http.MethodGet, Path: "/statistics/wear", Tags: []string{"Wear Feedback"}, Summary: "从本人现存实际穿着实时计算区间统计", Errors: errors}), handler.statistics)
}

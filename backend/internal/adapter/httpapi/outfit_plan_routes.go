package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerOutfitPlanOperations(api huma.API, handler *OutfitPlanHandler) {
	errorsWithNotFound := outfitPlanErrors(http.StatusNotFound)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createOutfitPlan", Method: http.MethodPost, Path: "/outfit-plans", Tags: []string{"Outfit Plans"}, Summary: "创建本人真实衣物穿搭计划", DefaultStatus: http.StatusCreated, MaxBodyBytes: 16 * 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusConflict)}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listOutfitPlans", Method: http.MethodGet, Path: "/outfit-plans", Tags: []string{"Outfit Plans"}, Summary: "按本地日期稳定分页列出本人计划", Errors: errorsWithNotFound}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getOutfitPlan", Method: http.MethodGet, Path: "/outfit-plans/{plan_id}", Tags: []string{"Outfit Plans"}, Summary: "读取本人穿搭计划与服务端快照", Errors: errorsWithNotFound}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateOutfitPlan", Method: http.MethodPut, Path: "/outfit-plans/{plan_id}", Tags: []string{"Outfit Plans"}, Summary: "按 revision 完整更新 active 计划", MaxBodyBytes: 16 * 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "cancelOutfitPlan", Method: http.MethodPost, Path: "/outfit-plans/{plan_id}/cancel", Tags: []string{"Outfit Plans"}, Summary: "取消计划且不创建实际穿着", MaxBodyBytes: 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.cancel)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "markOutfitPlanNotWorn", Method: http.MethodPost, Path: "/outfit-plans/{plan_id}/not-worn", Tags: []string{"Outfit Plans"}, Summary: "确认计划最终未穿", MaxBodyBytes: 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.markNotWorn)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "restoreOutfitPlan", Method: http.MethodPost, Path: "/outfit-plans/{plan_id}/restore", Tags: []string{"Outfit Plans"}, Summary: "把未穿计划恢复为待确认", MaxBodyBytes: 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.restore)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteOutfitPlan", Method: http.MethodDelete, Path: "/outfit-plans/{plan_id}", Tags: []string{"Outfit Plans"}, Summary: "永久删除计划并阻止迟到复活", DefaultStatus: http.StatusNoContent, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.delete)
}

func outfitPlanErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusInternalServerError}, statuses...)
}

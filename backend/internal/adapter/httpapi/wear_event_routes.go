package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerWearEventOperations(api huma.API, handler *WearEventHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createWearEvent", Method: http.MethodPost, Path: "/wear-events", Tags: []string{"Wear Events"}, Summary: "记录本人实际穿着并生成服务端快照", DefaultStatus: http.StatusCreated, MaxBodyBytes: 20 * 1024, Errors: wearEventErrors(http.StatusBadRequest, http.StatusConflict)}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listWearEvents", Method: http.MethodGet, Path: "/wear-events", Tags: []string{"Wear Events"}, Summary: "按本地日期稳定分页列出本人实际穿着", Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound)}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWearEvent", Method: http.MethodGet, Path: "/wear-events/{wear_event_id}", Tags: []string{"Wear Events"}, Summary: "读取本人实际穿着与快照", Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound)}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateWearEvent", Method: http.MethodPut, Path: "/wear-events/{wear_event_id}", Tags: []string{"Wear Events"}, Summary: "按 revision 纠正实际穿着", MaxBodyBytes: 20 * 1024, Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteWearEvent", Method: http.MethodDelete, Path: "/wear-events/{wear_event_id}", Tags: []string{"Wear Events"}, Summary: "永久删除实际穿着并重算计划状态", DefaultStatus: http.StatusNoContent, Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.delete)
}

func wearEventErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusInternalServerError}, statuses...)
}

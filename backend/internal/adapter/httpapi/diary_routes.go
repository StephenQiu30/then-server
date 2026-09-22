package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerDiaryOperations(api huma.API, handler *DiaryHandler) {
	common := []int{http.StatusUnauthorized, http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError}
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createDiaryEntry", Method: http.MethodPost, Path: "/diary-entries", Tags: []string{"Diary"}, Summary: "创建本人私人穿搭日记", DefaultStatus: http.StatusCreated, MaxBodyBytes: 32 * 1024, Errors: common}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listDiaryEntries", Method: http.MethodGet, Path: "/diary-entries", Tags: []string{"Diary"}, Summary: "稳定分页列出本人日记", Errors: common}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getDiaryEntry", Method: http.MethodGet, Path: "/diary-entries/{entry_id}", Tags: []string{"Diary"}, Summary: "读取本人私人日记", Errors: common}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateDiaryEntry", Method: http.MethodPut, Path: "/diary-entries/{entry_id}", Tags: []string{"Diary"}, Summary: "按 revision 完整更新本人日记", MaxBodyBytes: 32 * 1024, Errors: common}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getDiaryEntryDeletionImpact", Method: http.MethodGet, Path: "/diary-entries/{entry_id}/deletion-impact", Tags: []string{"Diary"}, Summary: "查看日记删除影响", Errors: common}), handler.deletionImpact)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteDiaryEntry", Method: http.MethodDelete, Path: "/diary-entries/{entry_id}", Tags: []string{"Diary"}, Summary: "永久删除日记并保留独立事实", DefaultStatus: http.StatusNoContent, Errors: common}), handler.delete)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getCalendarMonth", Method: http.MethodGet, Path: "/calendar", Tags: []string{"Diary"}, Summary: "按原本地日期聚合计划、实际穿着和日记", Errors: common}), handler.calendar)
}

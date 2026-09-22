package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerWardrobeOperations(api huma.API, handler *WardrobeHandler) {
	errorsWithNotFound := wardrobeErrors(http.StatusNotFound)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createWardrobeItem", Method: http.MethodPost, Path: "/wardrobe/items", Tags: []string{"Wardrobe"}, Summary: "创建本人结构化衣物与确认属性", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024, Errors: wardrobeErrors(http.StatusBadRequest, http.StatusConflict)}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listWardrobeItems", Method: http.MethodGet, Path: "/wardrobe/items", Tags: []string{"Wardrobe"}, Summary: "分页列出本人结构化衣物", Errors: errorsWithNotFound}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWardrobeItem", Method: http.MethodGet, Path: "/wardrobe/items/{item_id}", Tags: []string{"Wardrobe"}, Summary: "读取本人结构化衣物", Errors: errorsWithNotFound}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWardrobeDeletionImpact", Method: http.MethodGet, Path: "/wardrobe/items/{item_id}/deletion-impact", Tags: []string{"Wardrobe"}, Summary: "读取衣物删除对计划的当前影响", Errors: errorsWithNotFound}), handler.deletionImpact)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateWardrobeItem", Method: http.MethodPut, Path: "/wardrobe/items/{item_id}", Tags: []string{"Wardrobe"}, Summary: "按 revision 修改本人结构化衣物", MaxBodyBytes: 8 * 1024, Errors: wardrobeErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteWardrobeItem", Method: http.MethodDelete, Path: "/wardrobe/items/{item_id}", Tags: []string{"Wardrobe"}, Summary: "按 revision 删除本人结构化衣物", DefaultStatus: http.StatusNoContent, Errors: wardrobeErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.delete)
}

func wardrobeErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusInternalServerError}, statuses...)
}

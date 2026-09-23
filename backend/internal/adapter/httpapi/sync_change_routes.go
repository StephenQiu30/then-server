package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerSyncOperations(api huma.API, handler *SyncHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "listSyncChanges", Method: http.MethodGet, Path: "/sync/changes", Tags: []string{"Sync"},
		Summary: "按本人提交顺序分页读取结构化变更", Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.list)
}

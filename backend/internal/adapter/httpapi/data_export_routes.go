package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerDataExportOperations(api huma.API, handler *DataExportHandler) {
	if handler == nil {
		handler = &DataExportHandler{}
	}
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "createDataExport", Method: http.MethodPost, Path: "/exports", Tags: []string{"Data export"},
		Summary: "请求本人结构化数据导出", DefaultStatus: http.StatusAccepted, MaxBodyBytes: 4096,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusServiceUnavailable},
	}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "getDataExport", Method: http.MethodGet, Path: "/exports/{id}", Tags: []string{"Data export"},
		Summary: "查看本人导出状态",
		Errors:  []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable},
	}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "downloadDataExport", Method: http.MethodGet, Path: "/exports/{id}/content", Tags: []string{"Data export"},
		Summary: "下载本人私有 ZIP",
		Errors:  []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusServiceUnavailable},
		Responses: map[string]*huma.Response{"200": {
			Description: "私有 ZIP",
			Content:     map[string]*huma.MediaType{"application/zip": {Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"}}},
		}},
	}), handler.download)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "revokeDataExport", Method: http.MethodDelete, Path: "/exports/{id}", Tags: []string{"Data export"},
		Summary: "撤销本人导出", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable},
	}), handler.revoke)
}

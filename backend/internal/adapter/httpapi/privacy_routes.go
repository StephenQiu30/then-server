package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerPrivacyOperations(api huma.API, handler *PrivacyHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "getSelfAdultDeclaration", Method: http.MethodGet, Path: "/privacy/self-adult-declaration", Tags: []string{"Privacy"},
		Summary: "获取当前本人成年声明状态", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.current)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "confirmSelfAdultDeclaration", Method: http.MethodPut, Path: "/privacy/self-adult-declaration", Tags: []string{"Privacy"},
		Summary: "确认当前本人成年声明", MaxBodyBytes: 4 * 1024, Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.confirm)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "withdrawSelfAdultDeclaration", Method: http.MethodDelete, Path: "/privacy/self-adult-declaration", Tags: []string{"Privacy"},
		Summary: "撤回当前本人成年声明", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.withdraw)
}

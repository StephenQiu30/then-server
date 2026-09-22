package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerProfileOperations(api huma.API, handler *AccountHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "getCurrentProfile", Method: http.MethodGet, Path: "/users/me/profile", Tags: []string{"Profile"},
		Summary: "获取本人公开资料", Errors: []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}), handler.currentProfile)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "putCurrentProfile", Method: http.MethodPut, Path: "/users/me/profile", Tags: []string{"Profile"},
		Summary: "创建或修改本人公开资料", MaxBodyBytes: 8 * 1024,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict, http.StatusInternalServerError},
	}), handler.putCurrentProfile)
	huma.Register(api, huma.Operation{
		OperationID: "getPublicProfile", Method: http.MethodGet, Path: "/profiles/{handle}", Tags: []string{"Profile"},
		Summary: "按唯一标识读取公开资料", Errors: []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, handler.publicProfile)
}

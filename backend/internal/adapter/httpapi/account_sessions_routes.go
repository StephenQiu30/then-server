package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerAccountSessionOperations(api huma.API, handler *AccountHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "listCurrentUserSessions", Method: http.MethodGet, Path: "/users/me/sessions", Tags: []string{"Account"},
		Summary: "分页列出本人有效会话", Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.listSessions)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "revokeCurrentUserSession", Method: http.MethodDelete, Path: "/users/me/sessions/{session_id}", Tags: []string{"Account"},
		Summary: "撤销本人指定会话", Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}), handler.revokeSession)
}

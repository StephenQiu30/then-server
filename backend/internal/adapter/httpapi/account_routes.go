package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerAccountOperations(api huma.API, handler *AccountHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "registerAccount", Method: http.MethodPost, Path: "/auth/registrations", Tags: []string{"Authentication"},
		Summary: "注册账户并建立会话", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024,
		Errors: []int{http.StatusBadRequest, http.StatusConflict, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}, handler.register)
	huma.Register(api, huma.Operation{
		OperationID: "createSession", Method: http.MethodPost, Path: "/auth/sessions", Tags: []string{"Authentication"},
		Summary: "使用邮箱密码登录", MaxBodyBytes: 8 * 1024,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}, handler.login)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "deleteSession", Method: http.MethodDelete, Path: "/auth/session", Tags: []string{"Authentication"},
		Summary: "退出当前会话", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.logout)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "getCurrentUser", Method: http.MethodGet, Path: "/users/me", Tags: []string{"Account"},
		Summary: "获取本人账户", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.current)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "updateCurrentUser", Method: http.MethodPatch, Path: "/users/me", Tags: []string{"Account"},
		Summary: "修改本人邮箱或显示名称", MaxBodyBytes: 8 * 1024,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict, http.StatusInternalServerError},
	}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "deleteCurrentUser", Method: http.MethodDelete, Path: "/users/me", Tags: []string{"Account"},
		Summary: "受理本人账户删除", Description: "立即撤销全部会话并关闭公开内容；对象存储清理完成后物理删除账户。", DefaultStatus: http.StatusAccepted,
		Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.deleteCurrent)
	registerProfileOperations(api, handler)
}

func authenticatedOperation(operation huma.Operation) huma.Operation {
	operation.Security = []map[string][]string{{"cookieAuth": {}}}
	return operation
}

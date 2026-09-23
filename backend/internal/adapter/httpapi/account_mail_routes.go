package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerAccountMailOperations(api huma.API, handler *AccountHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "requestEmailVerification", Method: http.MethodPost, Path: "/auth/email-verifications", Tags: []string{"Authentication"},
		Summary: "为当前邮箱申请验证邮件", DefaultStatus: http.StatusAccepted,
		Errors: []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}), handler.requestEmailVerification)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "confirmEmailVerification", Method: http.MethodPost, Path: "/auth/email-verifications/confirm", Tags: []string{"Authentication"},
		Summary: "确认当前邮箱的单次验证挑战", DefaultStatus: http.StatusNoContent, MaxBodyBytes: 1024,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}), handler.confirmEmailVerification)
	huma.Register(api, huma.Operation{
		OperationID: "requestPasswordReset", Method: http.MethodPost, Path: "/auth/password-resets", Tags: []string{"Authentication"},
		Summary: "申请密码找回邮件", DefaultStatus: http.StatusAccepted, MaxBodyBytes: 1024,
		Errors: []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}, handler.requestPasswordReset)
	huma.Register(api, huma.Operation{
		OperationID: "confirmPasswordReset", Method: http.MethodPost, Path: "/auth/password-resets/confirm", Tags: []string{"Authentication"},
		Summary: "用单次邮件挑战重置密码并撤销旧会话", DefaultStatus: http.StatusNoContent, MaxBodyBytes: 1024,
		Errors: []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}, handler.confirmPasswordReset)
}

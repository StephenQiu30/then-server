package transport

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/danielgtaylor/huma/v2"
)

type PrivacyService interface {
	CurrentSelfAdultDeclaration(context.Context, string) (model.SelfAdultDeclaration, error)
	ConfirmSelfAdultDeclaration(context.Context, string, model.ConfirmSelfAdultDeclarationInput) (model.SelfAdultDeclaration, error)
	WithdrawSelfAdultDeclaration(context.Context, string) (model.SelfAdultDeclaration, error)
}

type PrivacyHandler struct {
	service      PrivacyService
	secureCookie bool
}

func NewPrivacyHandler(service PrivacyService, secureCookie bool) *PrivacyHandler {
	return &PrivacyHandler{service: service, secureCookie: secureCookie}
}

type ConfirmSelfAdultDeclarationRequest struct {
	PolicyVersion        string `json:"policy_version" enum:"self-adult-v1" doc:"服务端当前本人成年声明版本" example:"self-adult-v1"`
	ConfirmsSelfAndAdult bool   `json:"confirms_self_and_adult" enum:"true" doc:"用户主动确认照片仅属于本人且已年满 18 周岁；必须为 true" example:"true"`
}

type SelfAdultDeclarationResponse struct {
	PolicyVersion string     `json:"policy_version" enum:"self-adult-v1" example:"self-adult-v1"`
	Confirmed     bool       `json:"confirmed" example:"true"`
	ConfirmedAt   *time.Time `json:"confirmed_at,omitempty" format:"date-time"`
	WithdrawnAt   *time.Time `json:"withdrawn_at,omitempty" format:"date-time"`
}

type confirmSelfAdultDeclarationInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    ConfirmSelfAdultDeclarationRequest
}

type selfAdultDeclarationOutput struct {
	RequestID string                       `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	Body      SelfAdultDeclarationResponse `json:"body"`
}

func registerPrivacyOperations(api huma.API, handler *PrivacyHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "getSelfAdultDeclaration", Method: http.MethodGet, Path: "/v1/privacy/self-adult-declaration", Tags: []string{"Privacy"},
		Summary: "获取当前本人成年声明状态", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.current)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "confirmSelfAdultDeclaration", Method: http.MethodPut, Path: "/v1/privacy/self-adult-declaration", Tags: []string{"Privacy"},
		Summary: "确认当前本人成年声明", MaxBodyBytes: 4 * 1024, Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.confirm)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "withdrawSelfAdultDeclaration", Method: http.MethodDelete, Path: "/v1/privacy/self-adult-declaration", Tags: []string{"Privacy"},
		Summary: "撤回当前本人成年声明", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.withdraw)
}

func (h *PrivacyHandler) current(ctx context.Context, input *authenticatedInput) (*selfAdultDeclarationOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	declaration, err := h.service.CurrentSelfAdultDeclaration(ctx, input.Session)
	if err != nil {
		return nil, privacyError(ctx, h.secureCookie, err)
	}
	return newSelfAdultDeclarationOutput(ctx, declaration), nil
}

func (h *PrivacyHandler) confirm(ctx context.Context, input *confirmSelfAdultDeclarationInput) (*selfAdultDeclarationOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	if input.Body.PolicyVersion != model.CurrentSelfAdultPolicyVersion || !input.Body.ConfirmsSelfAndAdult {
		return nil, newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	declaration, err := h.service.ConfirmSelfAdultDeclaration(ctx, input.Session, model.ConfirmSelfAdultDeclarationInput{
		PolicyVersion: input.Body.PolicyVersion, ConfirmsSelfAndAdult: input.Body.ConfirmsSelfAndAdult,
	})
	if err != nil {
		return nil, privacyError(ctx, h.secureCookie, err)
	}
	return newSelfAdultDeclarationOutput(ctx, declaration), nil
}

func (h *PrivacyHandler) withdraw(ctx context.Context, input *authenticatedInput) (*selfAdultDeclarationOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	declaration, err := h.service.WithdrawSelfAdultDeclaration(ctx, input.Session)
	if err != nil {
		return nil, privacyError(ctx, h.secureCookie, err)
	}
	return newSelfAdultDeclarationOutput(ctx, declaration), nil
}

func privacyError(ctx context.Context, secureCookie bool, err error) error {
	if errors.Is(err, model.ErrAuthentication) {
		return authenticatedSessionError(ctx, secureCookie)
	}
	if errors.Is(err, model.ErrInvalidPrivacyInput) {
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
}

func newSelfAdultDeclarationOutput(ctx context.Context, declaration model.SelfAdultDeclaration) *selfAdultDeclarationOutput {
	return &selfAdultDeclarationOutput{RequestID: requestID(ctx), Body: SelfAdultDeclarationResponse{
		PolicyVersion: declaration.PolicyVersion, Confirmed: declaration.Confirmed,
		ConfirmedAt: declaration.ConfirmedAt, WithdrawnAt: declaration.WithdrawnAt,
	}}
}

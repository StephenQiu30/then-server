package transport

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/danielgtaylor/huma/v2"
)

const sessionCookieName = "then_session"

type AccountService interface {
	Register(context.Context, model.RegisterAccountInput) (model.AuthenticatedUser, error)
	Login(context.Context, model.CreateSessionInput) (model.AuthenticatedUser, error)
	CurrentUser(context.Context, string) (model.User, error)
	UpdateCurrentUser(context.Context, string, model.UpdateCurrentUserInput) (model.User, error)
	Logout(context.Context, string) error
	DeleteCurrentUser(context.Context, string) error
}

type AccountHandler struct {
	service      AccountService
	secureCookie bool
}

func NewAccountHandler(service AccountService, secureCookie bool) *AccountHandler {
	return &AccountHandler{service: service, secureCookie: secureCookie}
}

type RegisterAccountRequest struct {
	Email       string `json:"email" format:"email" minLength:"3" maxLength:"254" doc:"登录邮箱" example:"developer@example.test"`
	DisplayName string `json:"display_name" minLength:"1" maxLength:"80" doc:"用户显示名称" example:"开发用户"`
	Password    string `json:"password" format:"password" doc:"服务端要求 12–72 个 UTF-8 字节" example:"example-password-2026"`
}

type CreateSessionRequest struct {
	Email    string `json:"email" format:"email" minLength:"3" maxLength:"254" doc:"登录邮箱" example:"developer@example.test"`
	Password string `json:"password" format:"password" doc:"服务端按 UTF-8 字节执行密码规则；登录失败统一返回 401" example:"example-password-2026"`
}

type UpdateCurrentUserRequest struct {
	Email       *string `json:"email,omitempty" format:"email" minLength:"3" maxLength:"254" doc:"新登录邮箱" example:"developer@example.test"`
	DisplayName *string `json:"display_name,omitempty" minLength:"1" maxLength:"80" doc:"新显示名称" example:"新显示名称"`
}

func (UpdateCurrentUserRequest) Schema(registry huma.Registry) *huma.Schema {
	type updateCurrentUserSchema UpdateCurrentUserRequest
	schema := huma.SchemaFromType(registry, reflect.TypeFor[updateCurrentUserSchema]())
	minimum := 1
	schema.MinProperties = &minimum
	return schema
}

type UserResponse struct {
	ID          string    `json:"id" format:"uuid" example:"018f1f74-a2d0-7c6d-9c17-4a0ea2400a11"`
	Email       string    `json:"email" format:"email" maxLength:"254" example:"developer@example.test"`
	DisplayName string    `json:"display_name" minLength:"1" maxLength:"80" example:"开发用户"`
	CreatedAt   time.Time `json:"created_at" format:"date-time" example:"2026-09-14T08:00:00Z"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time" example:"2026-09-14T08:00:00Z"`
}

type AuthenticatedUserResponse struct {
	User UserResponse `json:"user"`
}

type registerAccountInput struct{ Body RegisterAccountRequest }
type createSessionInput struct{ Body CreateSessionRequest }
type updateCurrentUserInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    UpdateCurrentUserRequest
}
type authenticatedInput struct {
	Session string `cookie:"then_session" hidden:"true"`
}

type authenticatedUserOutput struct {
	RequestID string                    `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	SetCookie http.Cookie               `header:"Set-Cookie" doc:"HttpOnly、SameSite=Strict 会话 Cookie"`
	Body      AuthenticatedUserResponse `json:"body"`
}
type userOutput struct {
	RequestID string       `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	Body      UserResponse `json:"body"`
}
type emptySessionOutput struct {
	RequestID string      `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	SetCookie http.Cookie `header:"Set-Cookie" doc:"清除当前会话 Cookie"`
}

func registerAccountOperations(api huma.API, handler *AccountHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "registerAccount", Method: http.MethodPost, Path: "/v1/auth/registrations", Tags: []string{"Authentication"},
		Summary: "注册账户并建立会话", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024,
		Errors: []int{http.StatusBadRequest, http.StatusConflict, http.StatusInternalServerError},
	}, handler.register)
	huma.Register(api, huma.Operation{
		OperationID: "createSession", Method: http.MethodPost, Path: "/v1/auth/sessions", Tags: []string{"Authentication"},
		Summary: "使用邮箱密码登录", MaxBodyBytes: 8 * 1024,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
	}, handler.login)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "deleteSession", Method: http.MethodDelete, Path: "/v1/auth/session", Tags: []string{"Authentication"},
		Summary: "退出当前会话", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.logout)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "getCurrentUser", Method: http.MethodGet, Path: "/v1/users/me", Tags: []string{"Account"},
		Summary: "获取本人账户", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.current)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "updateCurrentUser", Method: http.MethodPatch, Path: "/v1/users/me", Tags: []string{"Account"},
		Summary: "修改本人邮箱或显示名称", MaxBodyBytes: 8 * 1024,
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict, http.StatusInternalServerError},
	}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{
		OperationID: "deleteCurrentUser", Method: http.MethodDelete, Path: "/v1/users/me", Tags: []string{"Account"},
		Summary: "删除本人账户及全部会话", Errors: []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), handler.deleteCurrent)
}

func authenticatedOperation(operation huma.Operation) huma.Operation {
	operation.Security = []map[string][]string{{"cookieAuth": {}}}
	return operation
}

func (h *AccountHandler) register(ctx context.Context, input *registerAccountInput) (*authenticatedUserOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	result, err := h.service.Register(ctx, model.RegisterAccountInput{Email: input.Body.Email, DisplayName: input.Body.DisplayName, Password: input.Body.Password})
	if err != nil {
		return nil, accountError(ctx, err)
	}
	return &authenticatedUserOutput{RequestID: requestID(ctx), SetCookie: h.sessionCookie(result.Token, result.ExpiresAt), Body: AuthenticatedUserResponse{User: newUserResponse(result.User)}}, nil
}

func (h *AccountHandler) login(ctx context.Context, input *createSessionInput) (*authenticatedUserOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	result, err := h.service.Login(ctx, model.CreateSessionInput{Email: input.Body.Email, Password: input.Body.Password})
	if err != nil {
		return nil, accountError(ctx, err)
	}
	return &authenticatedUserOutput{RequestID: requestID(ctx), SetCookie: h.sessionCookie(result.Token, result.ExpiresAt), Body: AuthenticatedUserResponse{User: newUserResponse(result.User)}}, nil
}

func (h *AccountHandler) current(ctx context.Context, input *authenticatedInput) (*userOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	user, err := h.service.CurrentUser(ctx, input.Session)
	if err != nil {
		return nil, accountError(ctx, err)
	}
	return &userOutput{RequestID: requestID(ctx), Body: newUserResponse(user)}, nil
}

func (h *AccountHandler) update(ctx context.Context, input *updateCurrentUserInput) (*userOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	user, err := h.service.UpdateCurrentUser(ctx, input.Session, model.UpdateCurrentUserInput{Email: input.Body.Email, DisplayName: input.Body.DisplayName})
	if err != nil {
		return nil, accountError(ctx, err)
	}
	return &userOutput{RequestID: requestID(ctx), Body: newUserResponse(user)}, nil
}

func (h *AccountHandler) logout(ctx context.Context, input *authenticatedInput) (*emptySessionOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	if err := h.service.Logout(ctx, input.Session); err != nil {
		return nil, accountError(ctx, err)
	}
	return &emptySessionOutput{RequestID: requestID(ctx), SetCookie: h.expiredSessionCookie()}, nil
}

func (h *AccountHandler) deleteCurrent(ctx context.Context, input *authenticatedInput) (*emptySessionOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	if err := h.service.DeleteCurrentUser(ctx, input.Session); err != nil {
		return nil, accountError(ctx, err)
	}
	return &emptySessionOutput{RequestID: requestID(ctx), SetCookie: h.expiredSessionCookie()}, nil
}

func (h *AccountHandler) sessionCookie(token string, expiresAt time.Time) http.Cookie {
	return http.Cookie{Name: sessionCookieName, Value: token, Path: "/v1", Expires: expiresAt, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode}
}

func (h *AccountHandler) expiredSessionCookie() http.Cookie {
	return http.Cookie{Name: sessionCookieName, Path: "/v1", Expires: time.Unix(1, 0), MaxAge: -1, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode}
}

func accountError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, model.ErrInvalidAccountInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, model.ErrEmailConflict):
		return newErrorResponse(http.StatusConflict, requestID(ctx))
	case errors.Is(err, model.ErrAuthentication):
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func newUserResponse(user model.User) UserResponse {
	return UserResponse{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}

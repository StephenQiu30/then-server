package httpapi

import (
	"net/http"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

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
	Email            *string `json:"email,omitempty" format:"email" minLength:"3" maxLength:"254" doc:"新登录邮箱" example:"developer@example.test"`
	DisplayName      *string `json:"display_name,omitempty" minLength:"1" maxLength:"80" doc:"新显示名称" example:"新显示名称"`
	ExpectedRevision int     `json:"expected_revision" minimum:"1" doc:"从上次读取结果取得的账户版本；不匹配时返回 409" example:"1"`
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
	Status      string    `json:"status" enum:"active,suspended,deleting" example:"active"`
	Role        string    `json:"role" enum:"user,moderator,admin" example:"user"`
	Revision    int       `json:"revision" minimum:"1" example:"1"`
	CreatedAt   time.Time `json:"created_at" format:"date-time" example:"2026-09-14T08:00:00Z"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time" example:"2026-09-14T08:00:00Z"`
}

type AuthenticatedUserResponse struct {
	User UserResponse `json:"user"`
}

type AccountDeletionResponse struct {
	ID          string     `json:"id" format:"uuid"`
	Status      string     `json:"status" enum:"pending,complete"`
	MediaCount  int        `json:"media_count" minimum:"0"`
	RequestedAt time.Time  `json:"requested_at" format:"date-time"`
	CompletedAt *time.Time `json:"completed_at,omitempty" format:"date-time"`
}

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

type accountDeletionOutput struct {
	RequestID string                  `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	SetCookie http.Cookie             `header:"Set-Cookie" doc:"清除当前会话 Cookie"`
	Body      AccountDeletionResponse `json:"body"`
}

type registerAccountInput struct{ Body RegisterAccountRequest }

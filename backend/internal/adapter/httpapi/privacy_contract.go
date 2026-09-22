package httpapi

import (
	"time"
)

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

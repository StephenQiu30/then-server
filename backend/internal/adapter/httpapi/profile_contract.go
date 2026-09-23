package httpapi

import (
	"time"
)

type PutProfileRequest struct {
	Handle           string  `json:"handle" pattern:"^[A-Za-z0-9_]{3,30}$" doc:"公开主页唯一标识；保存时转为小写" example:"then_style"`
	Bio              *string `json:"bio,omitempty" maxLength:"300" doc:"公开简介；空白值保存为未设置" example:"记录日常穿搭与轻量生活。"`
	ExpectedRevision int     `json:"expected_revision" minimum:"0" doc:"首次创建传 0；后续修改传上次读取到的版本" example:"0"`
}

type PublicProfileResponse struct {
	Handle      string    `json:"handle" pattern:"^[a-z0-9_]{3,30}$" example:"then_style"`
	DisplayName string    `json:"display_name" minLength:"1" maxLength:"80" example:"于是用户"`
	Bio         *string   `json:"bio,omitempty" minLength:"1" maxLength:"300" example:"记录日常穿搭与轻量生活。"`
	AvatarURL   *string   `json:"avatar_url" doc:"当前有头像时指向同源净化图；无头像为 null"`
	Revision    int       `json:"revision" minimum:"1" example:"1"`
	CreatedAt   time.Time `json:"created_at" format:"date-time" example:"2026-09-16T08:00:00Z"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time" example:"2026-09-16T08:00:00Z"`
}

type PutProfileAvatarRequest struct {
	MediaID          string `json:"media_id" format:"uuid"`
	ExpectedRevision int    `json:"expected_revision" minimum:"1"`
}

type putProfileAvatarInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    PutProfileAvatarRequest
}

type deleteProfileAvatarInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}

type currentProfileInput struct {
	Session string `cookie:"then_session" hidden:"true"`
}

type putCurrentProfileInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    PutProfileRequest
}

type publicProfileInput struct {
	Handle string `path:"handle" pattern:"^[A-Za-z0-9_]{3,30}$" doc:"公开主页唯一标识" example:"then_style"`
}

type publicProfileOutput struct {
	RequestID string                `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	Body      PublicProfileResponse `json:"body"`
}

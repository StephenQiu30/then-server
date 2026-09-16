package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"

	"github.com/danielgtaylor/huma/v2"
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
	Revision    int       `json:"revision" minimum:"1" example:"1"`
	CreatedAt   time.Time `json:"created_at" format:"date-time" example:"2026-09-16T08:00:00Z"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time" example:"2026-09-16T08:00:00Z"`
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

func (h *AccountHandler) currentProfile(ctx context.Context, input *currentProfileInput) (*publicProfileOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	profile, err := h.service.CurrentProfile(ctx, input.Session)
	if err != nil {
		return nil, h.profileError(ctx, err)
	}
	return &publicProfileOutput{RequestID: requestID(ctx), Body: newPublicProfileResponse(profile)}, nil
}

func (h *AccountHandler) putCurrentProfile(ctx context.Context, input *putCurrentProfileInput) (*publicProfileOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	profile, err := h.service.PutCurrentProfile(ctx, input.Session, accountapp.PutProfileInput{
		Handle: input.Body.Handle, Bio: input.Body.Bio, ExpectedRevision: input.Body.ExpectedRevision,
	})
	if err != nil {
		return nil, h.profileError(ctx, err)
	}
	return &publicProfileOutput{RequestID: requestID(ctx), Body: newPublicProfileResponse(profile)}, nil
}

func (h *AccountHandler) publicProfile(ctx context.Context, input *publicProfileInput) (*publicProfileOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	profile, err := h.service.PublicProfile(ctx, input.Handle)
	if err != nil {
		return nil, profileError(ctx, err)
	}
	return &publicProfileOutput{RequestID: requestID(ctx), Body: newPublicProfileResponse(profile)}, nil
}

func (h *AccountHandler) profileError(ctx context.Context, err error) error {
	if errors.Is(err, accountapp.ErrAuthentication) {
		return authenticatedSessionError(ctx, h.secureCookie)
	}
	return profileError(ctx, err)
}

func profileError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountapp.ErrInvalidProfileInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, accountapp.ErrProfileNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, accountapp.ErrProfileConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "REVISION_CONFLICT", "Profile changed since it was read."
		return response
	case errors.Is(err, accountapp.ErrHandleConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "HANDLE_UNAVAILABLE", "Profile handle is unavailable."
		return response
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func newPublicProfileResponse(profile accountapp.PublicProfile) PublicProfileResponse {
	return PublicProfileResponse{
		Handle: profile.Handle, DisplayName: profile.DisplayName, Bio: profile.Bio,
		Revision: profile.Revision, CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt,
	}
}

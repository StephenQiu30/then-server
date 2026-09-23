package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

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

func (h *AccountHandler) putProfileAvatar(ctx context.Context, input *putProfileAvatarInput) (*publicProfileOutput, error) {
	if h == nil || h.service == nil || h.avatarObjects == nil {
		return nil, newErrorResponse(http.StatusServiceUnavailable, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	profile, err := h.service.PutProfileAvatar(ctx, input.Session, input.Body.MediaID, input.Body.ExpectedRevision)
	if err != nil {
		return nil, h.profileError(ctx, err)
	}
	return &publicProfileOutput{RequestID: requestID(ctx), Body: newPublicProfileResponse(profile)}, nil
}

func (h *AccountHandler) deleteProfileAvatar(ctx context.Context, input *deleteProfileAvatarInput) (*publicProfileOutput, error) {
	if h == nil || h.service == nil || h.avatarObjects == nil {
		return nil, newErrorResponse(http.StatusServiceUnavailable, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	profile, err := h.service.DeleteProfileAvatar(ctx, input.Session, input.ExpectedRevision)
	if err != nil {
		return nil, h.profileError(ctx, err)
	}
	return &publicProfileOutput{RequestID: requestID(ctx), Body: newPublicProfileResponse(profile)}, nil
}

func (h *AccountHandler) publicProfileAvatar(ctx context.Context, input *publicProfileInput) (*imageOutput, error) {
	if h == nil || h.service == nil || h.avatarObjects == nil {
		return nil, newErrorResponse(http.StatusServiceUnavailable, requestID(ctx))
	}
	reference, err := h.service.PublicProfileAvatar(ctx, input.Handle)
	if err != nil {
		return nil, profileError(ctx, err)
	}
	reader, err := h.avatarObjects.OpenDerivedVersion(ctx, reference.ObjectKey, reference.ObjectVersionID)
	if err != nil {
		return nil, newErrorResponse(http.StatusNotFound, requestID(ctx))
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 12*1024*1024+1))
	if err != nil || len(data) == 0 || len(data) > 12*1024*1024 {
		return nil, newErrorResponse(http.StatusServiceUnavailable, requestID(ctx))
	}
	current, err := h.service.PublicProfileAvatar(ctx, input.Handle)
	if err != nil || current != reference {
		return nil, newErrorResponse(http.StatusNotFound, requestID(ctx))
	}
	return &imageOutput{RequestID: requestID(ctx), ContentType: "image/jpeg", Body: data}, nil
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
	var avatarURL *string
	if profile.HasAvatar {
		url := "/profiles/" + profile.Handle + "/avatar"
		avatarURL = &url
	}
	return PublicProfileResponse{
		Handle: profile.Handle, DisplayName: profile.DisplayName, Bio: profile.Bio, AvatarURL: avatarURL,
		Revision: profile.Revision, CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt,
	}
}

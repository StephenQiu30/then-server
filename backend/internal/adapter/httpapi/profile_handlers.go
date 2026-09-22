package httpapi

import (
	"context"
	"errors"
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

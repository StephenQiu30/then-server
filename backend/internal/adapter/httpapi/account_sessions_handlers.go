package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

func (h *AccountHandler) listSessions(ctx context.Context, input *listSessionsInput) (*sessionPageOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, authenticatedSessionError(ctx, h.secureCookie)
	}
	page, err := h.service.ListSessions(ctx, input.Session, input.Limit, input.Offset)
	if err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	items := make([]SessionResponse, 0, len(page.Items))
	for _, session := range page.Items {
		items = append(items, SessionResponse{ID: session.ID, CreatedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt, Current: session.Current})
	}
	return &sessionPageOutput{RequestID: requestID(ctx), CacheControl: "no-store", Body: SessionPageResponse{Items: items, NextOffset: page.NextOffset}}, nil
}

func (h *AccountHandler) revokeSession(ctx context.Context, input *revokeSessionInput) (*revokedSessionOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, authenticatedSessionError(ctx, h.secureCookie)
	}
	current, err := h.service.RevokeSession(ctx, input.Session, input.ID)
	if errors.Is(err, accountapp.ErrSessionNotFound) {
		return nil, newErrorResponse(http.StatusNotFound, requestID(ctx))
	}
	if err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	output := &revokedSessionOutput{RequestID: requestID(ctx), CacheControl: "no-store"}
	if current {
		cookie := h.expiredSessionCookie()
		output.SetCookie = cookie.String()
	}
	return output, nil
}

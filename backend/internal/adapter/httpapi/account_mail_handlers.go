package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	mailRequestIPLimit    = 5
	mailRequestEmailLimit = 3
	mailConfirmIPLimit    = 10
	mailRequestWindow     = time.Hour
	mailConfirmWindow     = 15 * time.Minute
)

func (h *AccountHandler) requestEmailVerification(ctx context.Context, input *authenticatedInput) (*acceptedAccountOutput, error) {
	if h == nil || h.mail == nil {
		return nil, notReadyError(ctx)
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "email-verification-ip", mailRequestIPLimit, mailRequestWindow); err != nil {
		return nil, err
	}
	user, err := h.service.CurrentUser(ctx, input.Session)
	if err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	if err := h.enforceEmailRateLimit(ctx, "email-verification-email", user.Email); err != nil {
		return nil, err
	}
	if err := h.mail.RequestVerification(ctx, input.Session); err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	return &acceptedAccountOutput{RequestID: requestID(ctx)}, nil
}

func (h *AccountHandler) requestPasswordReset(ctx context.Context, input *requestPasswordResetInput) (*acceptedAccountOutput, error) {
	if h == nil || h.mail == nil {
		return nil, notReadyError(ctx)
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "password-reset-ip", mailRequestIPLimit, mailRequestWindow); err != nil {
		return nil, err
	}
	if err := h.enforceEmailRateLimit(ctx, "password-reset-email", input.Body.Email); err != nil {
		return nil, err
	}
	if err := h.mail.RequestPasswordReset(ctx, input.Body.Email); err != nil {
		return nil, accountError(ctx, err)
	}
	return &acceptedAccountOutput{RequestID: requestID(ctx)}, nil
}

func (h *AccountHandler) confirmEmailVerification(ctx context.Context, input *confirmMailChallengeInput) (*emptyAccountOutput, error) {
	if h == nil || h.mail == nil {
		return nil, notReadyError(ctx)
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "email-verification-confirm", mailConfirmIPLimit, mailConfirmWindow); err != nil {
		return nil, err
	}
	if err := h.mail.ConfirmVerification(ctx, input.Session, input.Body.Token); err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	return &emptyAccountOutput{RequestID: requestID(ctx)}, nil
}

func (h *AccountHandler) confirmPasswordReset(ctx context.Context, input *confirmPasswordResetInput) (*emptyAccountOutput, error) {
	if h == nil || h.mail == nil {
		return nil, notReadyError(ctx)
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "password-reset-confirm", mailConfirmIPLimit, mailConfirmWindow); err != nil {
		return nil, err
	}
	if err := h.mail.ConfirmPasswordReset(ctx, input.Body.Token, input.Body.NewPassword); err != nil {
		return nil, accountError(ctx, err)
	}
	return &emptyAccountOutput{RequestID: requestID(ctx)}, nil
}

func (h *AccountHandler) enforceEmailRateLimit(ctx context.Context, scope, rawEmail string) error {
	email := strings.ToLower(strings.TrimSpace(rawEmail))
	if email == "" {
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	allowed, retryAfter, err := h.limiter.Allow(ctx, scope, email, mailRequestEmailLimit, mailRequestWindow)
	if err != nil {
		return notReadyError(ctx)
	}
	if allowed {
		return nil
	}
	return rateLimitError(ctx, retryAfter)
}

package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	privacyapp "github.com/StephenQiu30/then-server/backend/internal/application/privacy"
)

type PrivacyService interface {
	CurrentSelfAdultDeclaration(context.Context, string) (privacyapp.SelfAdultDeclaration, error)
	ConfirmSelfAdultDeclaration(context.Context, string, privacyapp.ConfirmSelfAdultDeclarationInput) (privacyapp.SelfAdultDeclaration, error)
	WithdrawSelfAdultDeclaration(context.Context, string) (privacyapp.SelfAdultDeclaration, error)
}

type PrivacyHandler struct {
	service      PrivacyService
	secureCookie bool
}

func NewPrivacyHandler(service PrivacyService, secureCookie bool) *PrivacyHandler {
	return &PrivacyHandler{service: service, secureCookie: secureCookie}
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
	if input.Body.PolicyVersion != privacyapp.CurrentSelfAdultPolicyVersion || !input.Body.ConfirmsSelfAndAdult {
		return nil, newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	declaration, err := h.service.ConfirmSelfAdultDeclaration(ctx, input.Session, privacyapp.ConfirmSelfAdultDeclarationInput{
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
	if errors.Is(err, accountapp.ErrAuthentication) {
		return authenticatedSessionError(ctx, secureCookie)
	}
	if errors.Is(err, privacyapp.ErrInvalidPrivacyInput) {
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
}

func newSelfAdultDeclarationOutput(ctx context.Context, declaration privacyapp.SelfAdultDeclaration) *selfAdultDeclarationOutput {
	return &selfAdultDeclarationOutput{RequestID: requestID(ctx), Body: SelfAdultDeclarationResponse{
		PolicyVersion: declaration.PolicyVersion, Confirmed: declaration.Confirmed,
		ConfirmedAt: declaration.ConfirmedAt, WithdrawnAt: declaration.WithdrawnAt,
	}}
}

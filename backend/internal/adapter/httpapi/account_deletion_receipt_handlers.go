package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"github.com/danielgtaylor/huma/v2"
)

func (h *AccountHandler) getDeletionReceipt(ctx context.Context, input *accountDeletionReceiptInput) (*accountDeletionReceiptOutput, error) {
	if h == nil || h.service == nil || h.limiter == nil {
		return nil, notReadyError(ctx)
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "deletion_receipt", 30, time.Hour); err != nil {
		return nil, err
	}
	request, err := h.service.GetDeletionReceipt(ctx, input.ID, deletionReceiptToken(ctx))
	if err != nil {
		return nil, deletionReceiptError(ctx, err)
	}
	return &accountDeletionReceiptOutput{RequestID: requestID(ctx), CacheControl: "no-store", Body: AccountDeletionReceiptResponse{
		ID: request.ID, Status: string(request.Status), Phase: request.Phase, AccessClosed: request.AccessClosed,
		MediaCount: request.MediaCount, RemainingMediaCount: request.RemainingMediaCount, GenerationCount: request.GenerationCount, RemainingGenerationCount: request.RemainingGenerationCount, RetryObserved: request.RetryObserved,
		RequestedAt: request.RequestedAt, UpdatedAt: request.UpdatedAt, CompletedAt: request.CompletedAt, ReceiptExpiresAt: request.ReceiptExpiresAt,
	}}, nil
}

func (h *AccountHandler) revokeDeletionReceipt(ctx context.Context, input *accountDeletionReceiptInput) (*emptyAccountDeletionReceiptOutput, error) {
	if h == nil || h.service == nil || h.limiter == nil {
		return nil, notReadyError(ctx)
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "deletion_receipt", 30, time.Hour); err != nil {
		return nil, err
	}
	if err := h.service.RevokeDeletionReceipt(ctx, input.ID, deletionReceiptToken(ctx)); err != nil {
		return nil, deletionReceiptError(ctx, err)
	}
	return &emptyAccountDeletionReceiptOutput{RequestID: requestID(ctx), CacheControl: "no-store"}, nil
}

func deletionReceiptToken(ctx context.Context) string {
	request, ok := ctx.Value(httpRequestContextKey{}).(*http.Request)
	if !ok || request == nil {
		return ""
	}
	value := request.Header.Get("Authorization")
	if len(value) < 7 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	token := value[7:]
	if strings.ContainsAny(token, " \t\r\n") {
		return ""
	}
	return token
}

func deletionReceiptError(ctx context.Context, err error) error {
	if errors.Is(err, accountapp.ErrDeletionReceiptNotFound) {
		return huma.ErrorWithHeaders(newErrorResponse(http.StatusNotFound, requestID(ctx)), http.Header{"Cache-Control": []string{"no-store"}})
	}
	return huma.ErrorWithHeaders(notReadyError(ctx), http.Header{"Cache-Control": []string{"no-store"}})
}

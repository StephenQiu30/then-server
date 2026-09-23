package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	syncapp "github.com/StephenQiu30/then-server/backend/internal/application/syncchange"
)

type SyncHTTPService interface {
	List(context.Context, string, string, int) (syncapp.Page, error)
}

type SyncHandler struct {
	service      SyncHTTPService
	secureCookie bool
}

func NewSyncHandler(service SyncHTTPService, secureCookie bool) *SyncHandler {
	return &SyncHandler{service: service, secureCookie: secureCookie}
}

func (h *SyncHandler) list(ctx context.Context, input *syncChangesInput) (*syncChangesOutput, error) {
	if h == nil || h.service == nil {
		return nil, notReadyError(ctx)
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	page, err := h.service.List(ctx, input.Session, input.After, input.Limit)
	if err != nil {
		switch {
		case errors.Is(err, accountapp.ErrAuthentication):
			return nil, authenticatedSessionError(ctx, h.secureCookie)
		case errors.Is(err, syncapp.ErrInvalidCursor):
			return nil, newErrorResponse(http.StatusBadRequest, requestID(ctx))
		default:
			return nil, notReadyError(ctx)
		}
	}
	changes := make([]SyncChangeResponse, 0, len(page.Changes))
	for _, change := range page.Changes {
		changes = append(changes, SyncChangeResponse{Seq: change.Seq, Kind: change.Kind, EntityID: change.EntityID, Action: change.Action, Revision: change.Revision})
	}
	return &syncChangesOutput{RequestID: requestID(ctx), Body: SyncChangesResponse{Changes: changes, NextCursor: page.NextCursor, HasMore: page.HasMore}}, nil
}

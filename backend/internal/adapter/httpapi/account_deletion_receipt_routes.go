package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerAccountDeletionReceiptOperations(api huma.API, handler *AccountHandler) {
	security := []map[string][]string{{"deletionReceiptAuth": {}}}
	huma.Register(api, huma.Operation{
		OperationID: "getAccountDeletionReceipt", Method: http.MethodGet, Path: "/account-deletion-requests/{id}", Tags: []string{"Account"},
		Summary: "注销后查询删除回执", Security: security,
		Errors: []int{http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests, http.StatusServiceUnavailable},
	}, handler.getDeletionReceipt)
	huma.Register(api, huma.Operation{
		OperationID: "revokeAccountDeletionReceipt", Method: http.MethodDelete, Path: "/account-deletion-requests/{id}", Tags: []string{"Account"},
		Summary: "撤销删除回执查询权", Security: security, DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests, http.StatusServiceUnavailable},
	}, handler.revokeDeletionReceipt)
}

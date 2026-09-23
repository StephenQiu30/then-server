package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type requestIDContextKey struct{}

type ErrorResponse struct {
	status              int
	Code                string                       `json:"code" enum:"BAD_REQUEST,EMAIL_CONFLICT,CONFLICT,EXPORT_NOT_READY,PAYLOAD_TOO_LARGE,AUTHENTICATION_FAILED,FORBIDDEN,RATE_LIMITED,NOT_READY,NOT_FOUND,METHOD_NOT_ALLOWED,INTERNAL_ERROR" example:"AUTHENTICATION_FAILED"`
	Message             string                       `json:"message" minLength:"1" maxLength:"160" example:"Sign-in information is invalid."`
	RequestID           string                       `json:"request_id" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" example:"TESTREQUESTIDENTIFIER00000003"`
	Retryable           bool                         `json:"retryable" example:"false"`
	DuplicateCandidates []WearEventCandidateResponse `json:"duplicate_candidates,omitempty" maxItems:"50"`
}

func (e *ErrorResponse) Error() string { return e.Message }

func (e *ErrorResponse) GetStatus() int { return e.status }

type httpRequestContextKey struct{}

func newErrorResponse(status int, id string) *ErrorResponse {
	if status == http.StatusUnprocessableEntity || status == http.StatusRequestTimeout {
		status = http.StatusBadRequest
	}
	response := &ErrorResponse{status: status, RequestID: id}
	switch status {
	case http.StatusBadRequest:
		response.Code, response.Message = "BAD_REQUEST", "Request does not satisfy the API contract."
	case http.StatusUnauthorized:
		response.Code, response.Message = "AUTHENTICATION_FAILED", "Sign-in information is invalid."
	case http.StatusForbidden:
		response.Code, response.Message = "FORBIDDEN", "Operation is not allowed."
	case http.StatusConflict:
		response.Code, response.Message = "EMAIL_CONFLICT", "This email cannot be used."
	case http.StatusRequestEntityTooLarge:
		response.Code, response.Message = "PAYLOAD_TOO_LARGE", "Declared media size exceeds the allowed limit."
	case http.StatusTooManyRequests:
		response.Code, response.Message, response.Retryable = "RATE_LIMITED", "Too many authentication attempts.", true
	case http.StatusServiceUnavailable:
		response.Code, response.Message, response.Retryable = "NOT_READY", "Service is not ready.", true
	case http.StatusNotFound:
		response.Code, response.Message = "NOT_FOUND", "Resource not found."
	case http.StatusMethodNotAllowed:
		response.Code, response.Message = "METHOD_NOT_ALLOWED", "Method not allowed."
	default:
		response.status, response.Code, response.Message = http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error."
	}
	return response
}

func requestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

func respondError(c *gin.Context, status int, code, message string, retryable bool) {
	c.AbortWithStatusJSON(status, ErrorResponse{status: status, Code: code, Message: message, RequestID: c.GetString("request_id"), Retryable: retryable})
}

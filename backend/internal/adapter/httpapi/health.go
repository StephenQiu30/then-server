package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type DependencyProbe interface{ Probe(context.Context) error }

type LivenessResponse struct {
	Status    string `json:"status" enum:"live" example:"live"`
	RequestID string `json:"request_id" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" example:"TESTREQUESTIDENTIFIER00000001"`
}

type ReadinessResponse struct {
	Status    string `json:"status" enum:"ready" example:"ready"`
	RequestID string `json:"request_id" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" example:"TESTREQUESTIDENTIFIER00000002"`
}

type livenessOutput struct {
	RequestID string           `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	Body      LivenessResponse `json:"body"`
}

type readinessOutput struct {
	RequestID string            `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	Body      ReadinessResponse `json:"body"`
}

func registerHealthOperations(api huma.API, router *Router, probe DependencyProbe, timeout time.Duration) {
	huma.Register(api, huma.Operation{
		OperationID: "getLiveness", Method: http.MethodGet, Path: "/health/live", Tags: []string{"Health"},
		Summary: "检查 API 进程存活", Description: "仅供受限运维访问，不查询数据库，不代表云业务已启用。", Errors: []int{http.StatusBadRequest, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *struct{}) (*livenessOutput, error) {
		if err := rejectHealthPayload(ctx); err != nil {
			return nil, err
		}
		id := requestID(ctx)
		return &livenessOutput{RequestID: id, Body: LivenessResponse{Status: "live", RequestID: id}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getReadiness", Method: http.MethodGet, Path: "/health/ready", Tags: []string{"Health"},
		Summary: "检查 API 接纳就绪状态", Description: "在同一有界上下文中探测 PostgreSQL 与认证 Redis；退出或任一依赖故障时返回 503，不输出连接详情。", Errors: []int{http.StatusBadRequest, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *struct{}) (*readinessOutput, error) {
		if err := rejectHealthPayload(ctx); err != nil {
			return nil, err
		}
		if router.draining.Load() {
			return nil, notReadyError(ctx)
		}
		probeContext, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := probe.Probe(probeContext); err != nil || router.draining.Load() {
			return nil, notReadyError(ctx)
		}
		id := requestID(ctx)
		return &readinessOutput{RequestID: id, Body: ReadinessResponse{Status: "ready", RequestID: id}}, nil
	})
}

func rejectHealthPayload(ctx context.Context) error {
	if request, ok := ctx.Value(httpRequestContextKey{}).(*http.Request); ok && (request.URL.RawQuery != "" || request.ContentLength != 0 || len(request.TransferEncoding) != 0) {
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	return nil
}

func notReadyError(ctx context.Context) error {
	return huma.ErrorWithHeaders(newErrorResponse(http.StatusServiceUnavailable, requestID(ctx)), http.Header{"Retry-After": []string{"1"}})
}

// Package httpapi owns the HTTP contract and generates OpenAPI from the
// registered operations and annotated Go request/response types.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

type DependencyProbe interface{ Probe(context.Context) error }

type Router struct {
	engine   *gin.Engine
	draining atomic.Bool
}

type requestIDContextKey struct{}

type LivenessResponse struct {
	Status    string `json:"status" enum:"live" example:"live"`
	RequestID string `json:"request_id" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" example:"TESTREQUESTIDENTIFIER00000001"`
}

type ReadinessResponse struct {
	Status    string `json:"status" enum:"ready" example:"ready"`
	RequestID string `json:"request_id" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" example:"TESTREQUESTIDENTIFIER00000002"`
}

type ErrorResponse struct {
	status              int
	Code                string                       `json:"code" enum:"BAD_REQUEST,EMAIL_CONFLICT,CONFLICT,PAYLOAD_TOO_LARGE,AUTHENTICATION_FAILED,FORBIDDEN,RATE_LIMITED,NOT_READY,NOT_FOUND,METHOD_NOT_ALLOWED,INTERNAL_ERROR" example:"AUTHENTICATION_FAILED"`
	Message             string                       `json:"message" minLength:"1" maxLength:"160" example:"Sign-in information is invalid."`
	RequestID           string                       `json:"request_id" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" example:"TESTREQUESTIDENTIFIER00000003"`
	Retryable           bool                         `json:"retryable" example:"false"`
	DuplicateCandidates []WearEventCandidateResponse `json:"duplicate_candidates,omitempty" maxItems:"50"`
}

func (e *ErrorResponse) Error() string  { return e.Message }
func (e *ErrorResponse) GetStatus() int { return e.status }

type livenessOutput struct {
	RequestID string           `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	Body      LivenessResponse `json:"body"`
}

type readinessOutput struct {
	RequestID string            `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$" doc:"服务端生成的请求关联标识，不采纳客户端原始值"`
	Body      ReadinessResponse `json:"body"`
}

var configureHumaErrors sync.Once

func NewRouter(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, nil, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithDiary(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithCommunity(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, timeout, log, mediaHandlers...)
}

func newRouter(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	if probe == nil || log == nil || timeout <= 0 || timeout > 5*time.Second || (accounts != nil && (accounts.service == nil || accounts.limiter == nil)) || (privacy != nil && privacy.service == nil) || (wardrobe != nil && wardrobe.service == nil) || (outfits != nil && outfits.service == nil) || (wear != nil && wear.service == nil) || (diary != nil && diary.service == nil) || (community != nil && (community.service == nil || community.objects == nil)) {
		return nil, errors.New("invalid router dependencies")
	}
	engine, err := newEngine(log)
	if err != nil {
		return nil, err
	}
	router := &Router{engine: engine}
	var media *MediaHandler
	if len(mediaHandlers) > 0 {
		media = mediaHandlers[0]
	}
	api := registerAPI(engine, router, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, media, timeout)
	yamlDocument, jsonDocument, err := serializeOpenAPI(ctx, api.OpenAPI())
	if err != nil {
		return nil, err
	}
	if docsEnabled {
		if err := registerDocs(engine, yamlDocument, jsonDocument); err != nil {
			return nil, err
		}
	}
	engine.NoRoute(func(c *gin.Context) { respondError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", false) })
	engine.NoMethod(func(c *gin.Context) {
		respondError(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed.", false)
	})
	return router, nil
}

func newEngine(log *slog.Logger) (*gin.Engine, error) {
	engine := gin.New()
	engine.RedirectTrailingSlash = false
	engine.RedirectFixedPath = false
	engine.HandleMethodNotAllowed = true
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, errors.New("invalid proxy configuration")
	}
	engine.Use(func(c *gin.Context) {
		id := rand.Text()
		c.Set("request_id", id)
		requestContext := context.WithValue(c.Request.Context(), requestIDContextKey{}, id)
		requestContext = context.WithValue(requestContext, httpRequestContextKey{}, c.Request)
		c.Request = c.Request.WithContext(requestContext)
		c.Header("X-Request-ID", id)
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		started := time.Now()
		defer func() {
			if recover() != nil {
				if !c.Writer.Written() {
					respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", false)
				} else {
					c.Abort()
				}
			}
			route := c.FullPath()
			if route == "" {
				route = "unmatched"
			}
			log.InfoContext(c.Request.Context(), "http_request", "request_id", id, "route", route, "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
		}()
		c.Next()
	})
	return engine, nil
}

func registerAPI(engine *gin.Engine, router *Router, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, media *MediaHandler, timeout time.Duration) huma.API {
	configureHumaErrors.Do(func() {
		huma.NewError = func(status int, _ string, _ ...error) huma.StatusError {
			return newErrorResponse(status, "")
		}
		huma.NewErrorWithContext = func(ctx huma.Context, status int, _ string, _ ...error) huma.StatusError {
			return newErrorResponse(status, requestID(ctx.Context()))
		}
	})
	config := huma.DefaultConfig("于是 OOTD API", "0.17.0")
	config.OpenAPI.OpenAPI = "3.1.2"
	config.Info.Description = "“于是”OOTD 产品后端接口。OpenAPI 由 Go operation 与类型字段标签生成。"
	config.OpenAPIPath = ""
	config.DocsPath = ""
	config.SchemasPath = ""
	config.CreateHooks = nil
	config.RejectUnknownQueryParameters = true
	config.Servers = []*huma.Server{{URL: "/", Description: "Same-origin API"}}
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {Type: "apiKey", In: "cookie", Name: sessionCookieName, Description: "HttpOnly、SameSite=Strict 会话 Cookie"},
	}
	api := humagin.New(engine, config)
	registerHealthOperations(api, router, probe, timeout)
	registerAccountOperations(api, accounts)
	registerPrivacyOperations(api, privacy)
	registerWardrobeOperations(api, wardrobe)
	registerOutfitPlanOperations(api, outfits)
	registerWearEventOperations(api, wear)
	registerDiaryOperations(api, diary)
	registerCommunityOperations(api, community)
	registerMediaOperations(api, media)
	normalizeGeneratedOpenAPI(api.OpenAPI())
	return api
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

func normalizeGeneratedOpenAPI(spec *huma.OpenAPI) {
	requestIDHeader := &huma.Header{
		Description: "服务端生成的请求关联标识，不采纳客户端原始值",
		Schema:      &huma.Schema{Type: huma.TypeString, MinLength: integerPointer(26), MaxLength: integerPointer(64), Pattern: "^[A-Za-z0-9]+$"},
	}
	clearSessionHeader := &huma.Header{
		Description: "受保护端点拒绝无效、过期或已撤销会话时清除 HttpOnly 会话 Cookie",
		Schema:      &huma.Schema{Type: huma.TypeString},
	}
	for _, item := range spec.Paths {
		operations := []*huma.Operation{item.Get, item.Put, item.Post, item.Delete, item.Options, item.Head, item.Patch, item.Trace}
		for _, operation := range operations {
			if operation == nil {
				continue
			}
			// Huma uses 422 internally for validation. Our public error contract
			// deliberately maps every contract validation failure to 400.
			delete(operation.Responses, "422")
			for status, response := range operation.Responses {
				if status < "400" || response == nil {
					continue
				}
				if response.Headers == nil {
					response.Headers = map[string]*huma.Header{}
				}
				response.Headers["X-Request-ID"] = requestIDHeader
				if status == "401" && usesCookieAuthentication(operation) {
					response.Headers["Set-Cookie"] = clearSessionHeader
				}
				if status == "429" {
					response.Headers["Retry-After"] = &huma.Header{Schema: &huma.Schema{Type: huma.TypeString, Pattern: "^[1-9][0-9]*$"}}
				}
				if status == "503" {
					response.Headers["Retry-After"] = &huma.Header{Schema: &huma.Schema{Type: huma.TypeString, Pattern: "^[1-9][0-9]*$"}}
				}
			}
		}
	}
	if readiness := spec.Paths["/health/ready"].Get.Responses["503"]; readiness != nil {
		if readiness.Headers == nil {
			readiness.Headers = map[string]*huma.Header{}
		}
		readiness.Headers["Retry-After"] = &huma.Header{Schema: &huma.Schema{Type: huma.TypeString, Const: "1"}}
	}
}

func usesCookieAuthentication(operation *huma.Operation) bool {
	for _, requirement := range operation.Security {
		if _, ok := requirement["cookieAuth"]; ok {
			return true
		}
	}
	return false
}

func integerPointer(value int) *int { return &value }

func rejectHealthPayload(ctx context.Context) error {
	if request, ok := ctx.Value(httpRequestContextKey{}).(*http.Request); ok && (request.URL.RawQuery != "" || request.ContentLength != 0 || len(request.TransferEncoding) != 0) {
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	}
	return nil
}

type httpRequestContextKey struct{}

func notReadyError(ctx context.Context) error {
	return huma.ErrorWithHeaders(newErrorResponse(http.StatusServiceUnavailable, requestID(ctx)), http.Header{"Retry-After": []string{"1"}})
}

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

func serializeOpenAPI(ctx context.Context, spec *huma.OpenAPI) ([]byte, []byte, error) {
	yamlDocument, err := spec.YAML()
	if err != nil {
		return nil, nil, errors.New("OpenAPI YAML document cannot be generated")
	}
	jsonDocument, err := json.Marshal(spec)
	if err != nil {
		return nil, nil, errors.New("OpenAPI JSON document cannot be generated")
	}
	loader := openapi3.NewLoader()
	loader.Context = ctx
	document, err := loader.LoadFromData(jsonDocument)
	if err != nil {
		return nil, nil, errors.New("generated OpenAPI document cannot be loaded")
	}
	if err := document.Validate(ctx); err != nil {
		return nil, nil, errors.New("generated OpenAPI document is invalid")
	}
	return yamlDocument, jsonDocument, nil
}

// GeneratedOpenAPI returns the same generated contract that the runtime serves.
func GeneratedOpenAPI(ctx context.Context) ([]byte, []byte, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	api := registerAPI(engine, &Router{}, nil, nil, nil, nil, nil, nil, &DiaryHandler{}, nil, nil, time.Second)
	return serializeOpenAPI(ctx, api.OpenAPI())
}

func respondError(c *gin.Context, status int, code, message string, retryable bool) {
	c.AbortWithStatusJSON(status, ErrorResponse{status: status, Code: code, Message: message, RequestID: c.GetString("request_id"), Retryable: retryable})
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.engine.ServeHTTP(w, req) }
func (r *Router) Drain()                                             { r.draining.Store(true) }

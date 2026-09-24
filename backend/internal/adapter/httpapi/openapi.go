package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

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
	api := registerAPI(engine, &Router{}, nil, nil, nil, nil, nil, nil, &DiaryHandler{}, nil, &FeedbackHandler{}, &DataExportHandler{}, nil, nil, nil, time.Second)
	return serializeOpenAPI(ctx, api.OpenAPI())
}

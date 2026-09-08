// Package transport implements the explicitly enabled HTTP contract.
package transport

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
)

type DependencyProbe interface{ Probe(context.Context) error }

type Router struct {
	engine   *gin.Engine
	draining atomic.Bool
}

type healthResponse struct {
	Status    string `json:"status"`
	RequestID string `json:"request_id"`
}

type errorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Retryable bool   `json:"retryable"`
}

func NewRouter(ctx context.Context, document []byte, probe DependencyProbe, timeout time.Duration, log *slog.Logger) (*Router, error) {
	if probe == nil || log == nil || timeout <= 0 || timeout > 5*time.Second {
		return nil, errors.New("invalid router dependencies")
	}
	loader := openapi3.NewLoader()
	loader.Context = ctx
	doc, err := loader.LoadFromData(document)
	if err != nil {
		return nil, errors.New("OpenAPI document cannot be loaded")
	}
	if err = doc.Validate(ctx); err != nil {
		return nil, errors.New("OpenAPI document is invalid")
	}
	contract, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, errors.New("OpenAPI routes are invalid")
	}
	engine := gin.New()
	engine.RedirectTrailingSlash = false
	engine.RedirectFixedPath = false
	engine.HandleMethodNotAllowed = true
	if err = engine.SetTrustedProxies(nil); err != nil {
		return nil, errors.New("invalid proxy configuration")
	}
	r := &Router{engine: engine}
	engine.Use(func(c *gin.Context) {
		id := rand.Text()
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		started := time.Now()
		defer func() {
			if recover() != nil {
				if !c.Writer.Written() {
					respondError(c, 500, "INTERNAL_ERROR", "Internal server error.", false)
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
	validate := func(c *gin.Context) {
		if c.Request.URL.RawQuery != "" || c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) != 0 {
			respondError(c, 400, "BAD_REQUEST", "Health requests do not accept a body or query.", false)
			return
		}
		route, params, err := contract.FindRoute(c.Request)
		if err != nil {
			respondError(c, 500, "INTERNAL_ERROR", "Internal server error.", false)
			return
		}
		input := &openapi3filter.RequestValidationInput{Request: c.Request, PathParams: params, Route: route}
		if err = openapi3filter.ValidateRequest(c.Request.Context(), input); err != nil {
			respondError(c, 400, "BAD_REQUEST", "Request does not satisfy the API contract.", false)
			return
		}
		c.Next()
	}
	engine.GET("/v1/health/live", validate, func(c *gin.Context) {
		c.JSON(http.StatusOK, healthResponse{Status: "live", RequestID: c.GetString("request_id")})
	})
	engine.GET("/v1/health/ready", validate, func(c *gin.Context) {
		if r.draining.Load() {
			notReady(c)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		if err := probe.Probe(ctx); err != nil || r.draining.Load() {
			notReady(c)
			return
		}
		c.JSON(http.StatusOK, healthResponse{Status: "ready", RequestID: c.GetString("request_id")})
	})
	engine.NoRoute(func(c *gin.Context) { respondError(c, 404, "NOT_FOUND", "Resource not found.", false) })
	engine.NoMethod(func(c *gin.Context) {
		c.Header("Allow", "GET")
		respondError(c, 405, "METHOD_NOT_ALLOWED", "Method not allowed.", false)
	})
	return r, nil
}

func notReady(c *gin.Context) {
	c.Header("Retry-After", "1")
	respondError(c, 503, "NOT_READY", "Service is not ready.", true)
}

func respondError(c *gin.Context, status int, code, message string, retryable bool) {
	c.AbortWithStatusJSON(status, errorResponse{Code: code, Message: message, RequestID: c.GetString("request_id"), Retryable: retryable})
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.engine.ServeHTTP(w, req) }
func (r *Router) Drain()                                             { r.draining.Store(true) }

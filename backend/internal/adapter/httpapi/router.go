// Package httpapi owns the HTTP contract and generates OpenAPI from the
// registered operations and annotated Go request/response types.
package httpapi

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type Router struct {
	engine   *gin.Engine
	draining atomic.Bool
}

var configureHumaErrors sync.Once

func NewRouter(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, nil, nil, nil, nil, nil, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithDiary(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, nil, nil, nil, nil, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithCommunity(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, nil, nil, nil, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithFeedback(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, feedback *FeedbackHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, feedback, nil, nil, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithExport(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, feedback *FeedbackHandler, dataExport *DataExportHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, feedback, dataExport, nil, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithSync(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, feedback *FeedbackHandler, dataExport *DataExportHandler, syncHandler *SyncHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, feedback, dataExport, syncHandler, nil, timeout, log, mediaHandlers...)
}

func NewRouterWithGeneration(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, feedback *FeedbackHandler, dataExport *DataExportHandler, syncHandler *SyncHandler, generation *GenerationHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	return newRouter(ctx, docsEnabled, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, feedback, dataExport, syncHandler, generation, timeout, log, mediaHandlers...)
}

func newRouter(ctx context.Context, docsEnabled bool, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, feedback *FeedbackHandler, dataExport *DataExportHandler, syncHandler *SyncHandler, generation *GenerationHandler, timeout time.Duration, log *slog.Logger, mediaHandlers ...*MediaHandler) (*Router, error) {
	if probe == nil || log == nil || timeout <= 0 || timeout > 5*time.Second || (accounts != nil && (accounts.service == nil || accounts.limiter == nil)) || (privacy != nil && privacy.service == nil) || (wardrobe != nil && wardrobe.service == nil) || (outfits != nil && outfits.service == nil) || (wear != nil && wear.service == nil) || (diary != nil && diary.service == nil) || (community != nil && (community.service == nil || community.objects == nil)) || (feedback != nil && feedback.service == nil) || (dataExport != nil && (dataExport.service == nil || dataExport.limiter == nil)) || (syncHandler != nil && syncHandler.service == nil) || (generation != nil && generation.service == nil) {
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
	api := registerAPI(engine, router, probe, accounts, privacy, wardrobe, outfits, wear, diary, community, feedback, dataExport, syncHandler, generation, media, timeout)
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

func registerAPI(engine *gin.Engine, router *Router, probe DependencyProbe, accounts *AccountHandler, privacy *PrivacyHandler, wardrobe *WardrobeHandler, outfits *OutfitPlanHandler, wear *WearEventHandler, diary *DiaryHandler, community *CommunityHandler, feedback *FeedbackHandler, dataExport *DataExportHandler, syncHandler *SyncHandler, generation *GenerationHandler, media *MediaHandler, timeout time.Duration) huma.API {
	configureHumaErrors.Do(func() {
		huma.NewError = func(status int, _ string, _ ...error) huma.StatusError {
			return newErrorResponse(status, "")
		}
		huma.NewErrorWithContext = func(ctx huma.Context, status int, _ string, _ ...error) huma.StatusError {
			return newErrorResponse(status, requestID(ctx.Context()))
		}
	})
	config := huma.DefaultConfig("于是 OOTD API", "0.28.0")
	config.OpenAPI.OpenAPI = "3.1.2"
	config.Info.Description = "“于是”OOTD 产品后端接口。OpenAPI 由 Go operation 与类型字段标签生成。"
	config.OpenAPIPath = ""
	config.DocsPath = ""
	config.SchemasPath = ""
	config.CreateHooks = nil
	config.RejectUnknownQueryParameters = true
	config.Servers = []*huma.Server{{URL: "/", Description: "Same-origin API"}}
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth":          {Type: "apiKey", In: "cookie", Name: sessionCookieName, Description: "HttpOnly、SameSite=Strict 会话 Cookie"},
		"deletionReceiptAuth": {Type: "http", Scheme: "bearer", BearerFormat: "opaque", Description: "注销 202 响应一次性交付的回执令牌"},
	}
	api := humagin.New(engine, config)
	registerHealthOperations(api, router, probe, timeout)
	registerAccountOperations(api, accounts)
	registerPrivacyOperations(api, privacy)
	registerWardrobeOperations(api, wardrobe)
	registerOutfitPlanOperations(api, outfits)
	registerWearEventOperations(api, wear)
	registerFeedbackOperations(api, feedback)
	registerDiaryOperations(api, diary)
	registerCommunityOperations(api, community)
	registerMediaOperations(api, media)
	registerDataExportOperations(api, dataExport)
	registerSyncOperations(api, syncHandler)
	registerGenerationOperations(api, generation)
	normalizeGeneratedOpenAPI(api.OpenAPI())
	return api
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.engine.ServeHTTP(w, req) }

func (r *Router) Drain() { r.draining.Store(true) }

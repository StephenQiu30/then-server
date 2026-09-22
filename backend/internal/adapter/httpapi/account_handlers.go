package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"github.com/danielgtaylor/huma/v2"
)

const sessionCookieName = "then_session"

const (
	registrationRateLimit  = 5
	registrationRateWindow = time.Hour
	loginRateLimit         = 10
	loginRateWindow        = 15 * time.Minute
)

type AccountService interface {
	Register(context.Context, accountapp.RegisterAccountInput) (accountapp.AuthenticatedUser, error)
	Login(context.Context, accountapp.CreateSessionInput) (accountapp.AuthenticatedUser, error)
	CurrentUser(context.Context, string) (accountapp.User, error)
	UpdateCurrentUser(context.Context, string, accountapp.UpdateCurrentUserInput) (accountapp.User, error)
	CurrentProfile(context.Context, string) (accountapp.PublicProfile, error)
	PublicProfile(context.Context, string) (accountapp.PublicProfile, error)
	PutCurrentProfile(context.Context, string, accountapp.PutProfileInput) (accountapp.PublicProfile, error)
	Logout(context.Context, string) error
	DeleteCurrentUser(context.Context, string) (accountapp.AccountDeletionRequest, error)
}

type AuthenticationRateLimiter interface {
	Allow(context.Context, string, string, int, time.Duration) (bool, time.Duration, error)
}

type AccountHandler struct {
	service      AccountService
	limiter      AuthenticationRateLimiter
	secureCookie bool
}

func NewAccountHandler(service AccountService, secureCookie bool, limiter AuthenticationRateLimiter) *AccountHandler {
	return &AccountHandler{service: service, limiter: limiter, secureCookie: secureCookie}
}

func (h *AccountHandler) login(ctx context.Context, input *createSessionInput) (*authenticatedUserOutput, error) {
	if h == nil || h.service == nil || h.limiter == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "login", loginRateLimit, loginRateWindow); err != nil {
		return nil, err
	}
	result, err := h.service.Login(ctx, accountapp.CreateSessionInput{Email: input.Body.Email, Password: input.Body.Password})
	if err != nil {
		return nil, accountError(ctx, err)
	}
	return &authenticatedUserOutput{RequestID: requestID(ctx), SetCookie: h.sessionCookie(result.Token, result.ExpiresAt), Body: AuthenticatedUserResponse{User: newUserResponse(result.User)}}, nil
}

func (h *AccountHandler) enforceAuthenticationRateLimit(ctx context.Context, scope string, limit int, window time.Duration) error {
	subject, err := directSourceIP(ctx)
	if err != nil {
		return notReadyError(ctx)
	}
	allowed, retryAfter, err := h.limiter.Allow(ctx, scope, subject, limit, window)
	if err != nil {
		return notReadyError(ctx)
	}
	if allowed {
		return nil
	}
	seconds := int64((retryAfter + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return huma.ErrorWithHeaders(newErrorResponse(http.StatusTooManyRequests, requestID(ctx)), http.Header{"Retry-After": []string{strconv.FormatInt(seconds, 10)}})
}

func directSourceIP(ctx context.Context) (string, error) {
	request, ok := ctx.Value(httpRequestContextKey{}).(*http.Request)
	if !ok || request == nil {
		return "", errors.New("request unavailable")
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return "", errors.New("source address unavailable")
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return "", errors.New("source address unavailable")
	}
	return address.Unmap().String(), nil
}

func (h *AccountHandler) current(ctx context.Context, input *authenticatedInput) (*userOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	user, err := h.service.CurrentUser(ctx, input.Session)
	if err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	return &userOutput{RequestID: requestID(ctx), Body: newUserResponse(user)}, nil
}

func (h *AccountHandler) update(ctx context.Context, input *updateCurrentUserInput) (*userOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	user, err := h.service.UpdateCurrentUser(ctx, input.Session, accountapp.UpdateCurrentUserInput{
		Email: input.Body.Email, DisplayName: input.Body.DisplayName, ExpectedRevision: input.Body.ExpectedRevision,
	})
	if err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	return &userOutput{RequestID: requestID(ctx), Body: newUserResponse(user)}, nil
}

func (h *AccountHandler) logout(ctx context.Context, input *authenticatedInput) (*emptySessionOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	if err := h.service.Logout(ctx, input.Session); err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	return &emptySessionOutput{RequestID: requestID(ctx), SetCookie: h.expiredSessionCookie()}, nil
}

func (h *AccountHandler) deleteCurrent(ctx context.Context, input *authenticatedInput) (*accountDeletionOutput, error) {
	if h == nil || h.service == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	deletion, err := h.service.DeleteCurrentUser(ctx, input.Session)
	if err != nil {
		return nil, h.authenticatedError(ctx, err)
	}
	return &accountDeletionOutput{RequestID: requestID(ctx), SetCookie: h.expiredSessionCookie(), Body: AccountDeletionResponse{
		ID: deletion.ID, Status: string(deletion.Status), MediaCount: deletion.MediaCount, RequestedAt: deletion.RequestedAt, CompletedAt: deletion.CompletedAt,
	}}, nil
}

func (h *AccountHandler) sessionCookie(token string, expiresAt time.Time) http.Cookie {
	return http.Cookie{Name: sessionCookieName, Value: token, Path: "/", Expires: expiresAt, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode}
}

func (h *AccountHandler) expiredSessionCookie() http.Cookie {
	return http.Cookie{Name: sessionCookieName, Path: "/", Expires: time.Unix(1, 0), MaxAge: -1, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode}
}

func accountError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountapp.ErrInvalidAccountInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, accountapp.ErrEmailConflict):
		return newErrorResponse(http.StatusConflict, requestID(ctx))
	case errors.Is(err, accountapp.ErrAccountConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "REVISION_CONFLICT", "Account changed since it was read."
		return response
	case errors.Is(err, accountapp.ErrAuthentication):
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func (h *AccountHandler) authenticatedError(ctx context.Context, err error) error {
	response := accountError(ctx, err)
	if !errors.Is(err, accountapp.ErrAuthentication) {
		return response
	}
	return authenticatedSessionError(ctx, h.secureCookie)
}

func authenticatedSessionError(ctx context.Context, secureCookie bool) error {
	cookie := http.Cookie{Name: sessionCookieName, Path: "/", Expires: time.Unix(1, 0), MaxAge: -1, HttpOnly: true, Secure: secureCookie, SameSite: http.SameSiteStrictMode}
	return huma.ErrorWithHeaders(newErrorResponse(http.StatusUnauthorized, requestID(ctx)), http.Header{"Set-Cookie": []string{cookie.String()}})
}

func newUserResponse(user accountapp.User) UserResponse {
	return UserResponse{
		ID: user.ID, Email: user.Email, DisplayName: user.DisplayName,
		Status: string(user.Status), Role: string(user.Role), Revision: user.Revision,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

func (h *AccountHandler) register(ctx context.Context, input *registerAccountInput) (*authenticatedUserOutput, error) {
	if h == nil || h.service == nil || h.limiter == nil {
		return nil, newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if err := h.enforceAuthenticationRateLimit(ctx, "registration", registrationRateLimit, registrationRateWindow); err != nil {
		return nil, err
	}
	result, err := h.service.Register(ctx, accountapp.RegisterAccountInput{Email: input.Body.Email, DisplayName: input.Body.DisplayName, Password: input.Body.Password})
	if err != nil {
		return nil, accountError(ctx, err)
	}
	return &authenticatedUserOutput{RequestID: requestID(ctx), SetCookie: h.sessionCookie(result.Token, result.ExpiresAt), Body: AuthenticatedUserResponse{User: newUserResponse(result.User)}}, nil
}

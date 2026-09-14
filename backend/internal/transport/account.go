package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/StephenQiu30/then/backend/internal/model"
	"github.com/gin-gonic/gin"
)

const sessionCookieName = "then_session"

type AccountService interface {
	Register(context.Context, model.RegisterAccountInput) (model.AuthenticatedUser, error)
	Login(context.Context, model.CreateSessionInput) (model.AuthenticatedUser, error)
	CurrentUser(context.Context, string) (model.User, error)
	UpdateCurrentUser(context.Context, string, model.UpdateCurrentUserInput) (model.User, error)
	Logout(context.Context, string) error
	DeleteCurrentUser(context.Context, string) error
}

type AccountHandler struct {
	service      AccountService
	secureCookie bool
}

func NewAccountHandler(service AccountService, secureCookie bool) *AccountHandler {
	return &AccountHandler{service: service, secureCookie: secureCookie}
}

type registerAccountRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type createSessionRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type updateCurrentUserRequest struct {
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
}

type userResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type authenticatedUserResponse struct {
	User userResponse `json:"user"`
}

func (h *AccountHandler) register(c *gin.Context) {
	var request registerAccountRequest
	if !decodeJSON(c, &request) {
		return
	}
	result, err := h.service.Register(c.Request.Context(), model.RegisterAccountInput{
		Email: request.Email, DisplayName: request.DisplayName, Password: request.Password,
	})
	if err != nil {
		respondAccountError(c, err)
		return
	}
	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	c.JSON(http.StatusCreated, authenticatedUserResponse{User: newUserResponse(result.User)})
}

func (h *AccountHandler) login(c *gin.Context) {
	var request createSessionRequest
	if !decodeJSON(c, &request) {
		return
	}
	result, err := h.service.Login(c.Request.Context(), model.CreateSessionInput{Email: request.Email, Password: request.Password})
	if err != nil {
		respondAccountError(c, err)
		return
	}
	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	c.JSON(http.StatusOK, authenticatedUserResponse{User: newUserResponse(result.User)})
}

func (h *AccountHandler) current(c *gin.Context) {
	token, ok := sessionToken(c)
	if !ok {
		respondAccountError(c, model.ErrAuthentication)
		return
	}
	user, err := h.service.CurrentUser(c.Request.Context(), token)
	if err != nil {
		respondAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, newUserResponse(user))
}

func (h *AccountHandler) update(c *gin.Context) {
	token, ok := sessionToken(c)
	if !ok {
		respondAccountError(c, model.ErrAuthentication)
		return
	}
	var request updateCurrentUserRequest
	if !decodeJSON(c, &request) {
		return
	}
	user, err := h.service.UpdateCurrentUser(c.Request.Context(), token, model.UpdateCurrentUserInput{
		Email: request.Email, DisplayName: request.DisplayName,
	})
	if err != nil {
		respondAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, newUserResponse(user))
}

func (h *AccountHandler) logout(c *gin.Context) {
	token, ok := sessionToken(c)
	if !ok {
		respondAccountError(c, model.ErrAuthentication)
		return
	}
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		respondAccountError(c, err)
		return
	}
	h.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *AccountHandler) deleteCurrent(c *gin.Context) {
	token, ok := sessionToken(c)
	if !ok {
		respondAccountError(c, model.ErrAuthentication)
		return
	}
	if err := h.service.DeleteCurrentUser(c.Request.Context(), token); err != nil {
		respondAccountError(c, err)
		return
	}
	h.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func decodeJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Request does not satisfy the API contract.", false)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Request does not satisfy the API contract.", false)
		return false
	}
	return true
}

func sessionToken(c *gin.Context) (string, bool) {
	cookie, err := c.Request.Cookie(sessionCookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

func (h *AccountHandler) setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/v1", Expires: expiresAt,
		HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	})
}

func (h *AccountHandler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/v1", Expires: time.Unix(1, 0),
		MaxAge: -1, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	})
}

func respondAccountError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrInvalidAccountInput):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Request does not satisfy the API contract.", false)
	case errors.Is(err, model.ErrEmailConflict):
		respondError(c, http.StatusConflict, "EMAIL_CONFLICT", "This email cannot be used.", false)
	case errors.Is(err, model.ErrAuthentication):
		respondError(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "Sign-in information is invalid.", false)
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", false)
	}
}

func newUserResponse(user model.User) userResponse {
	return userResponse{
		ID: user.ID, Email: user.Email, DisplayName: user.DisplayName,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

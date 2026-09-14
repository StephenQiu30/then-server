// Package service owns account business rules and authorization use cases.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/StephenQiu30/then/backend/internal/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	passwordCost       = 12
	sessionTTL         = 7 * 24 * time.Hour
	tokenBytes         = 32
	encodedTokenLength = 43
)

type AccountRepository interface {
	CreateAccount(context.Context, model.User, string, model.Session) (model.User, error)
	FindCredentialByEmail(context.Context, string) (model.Credential, error)
	CreateSession(context.Context, model.Session) error
	FindUserBySession(context.Context, []byte, time.Time) (model.User, error)
	UpdateUser(context.Context, string, *string, *string, time.Time) (model.User, error)
	DeleteSession(context.Context, []byte) error
	DeleteUser(context.Context, string) error
}

type AccountService struct {
	repository        AccountRepository
	now               func() time.Time
	random            io.Reader
	passwordCost      int
	dummyPasswordHash []byte
}

func NewAccountService(repository AccountRepository) (*AccountService, error) {
	if repository == nil {
		return nil, model.ErrAccountUnavailable
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("not-a-real-account-password"), passwordCost)
	if err != nil {
		return nil, model.ErrAccountUnavailable
	}
	return &AccountService{
		repository:        repository,
		now:               time.Now,
		random:            rand.Reader,
		passwordCost:      passwordCost,
		dummyPasswordHash: dummy,
	}, nil
}

func (s *AccountService) Register(ctx context.Context, input model.RegisterAccountInput) (model.AuthenticatedUser, error) {
	email, ok := normalizeEmail(input.Email)
	displayName := strings.TrimSpace(input.DisplayName)
	if !ok || !validDisplayName(displayName) || !validNewPassword(input.Password) {
		return model.AuthenticatedUser{}, model.ErrInvalidAccountInput
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), s.passwordCost)
	if err != nil {
		return model.AuthenticatedUser{}, model.ErrAccountUnavailable
	}
	now := s.now().UTC()
	userID := uuid.NewString()
	token, tokenHash, err := s.newToken()
	if err != nil {
		return model.AuthenticatedUser{}, model.ErrAccountUnavailable
	}
	user, err := s.repository.CreateAccount(ctx, model.User{
		ID: userID, Email: email, DisplayName: displayName, CreatedAt: now, UpdatedAt: now,
	}, string(passwordHash), model.Session{
		ID: uuid.NewString(), UserID: userID, TokenHash: tokenHash,
		CreatedAt: now, ExpiresAt: now.Add(sessionTTL),
	})
	if err != nil {
		return model.AuthenticatedUser{}, err
	}
	return model.AuthenticatedUser{User: user, Token: token, ExpiresAt: now.Add(sessionTTL)}, nil
}

func (s *AccountService) Login(ctx context.Context, input model.CreateSessionInput) (model.AuthenticatedUser, error) {
	email, ok := normalizeEmail(input.Email)
	if !ok || len(input.Password) == 0 || len([]byte(input.Password)) > 72 {
		return model.AuthenticatedUser{}, model.ErrAuthentication
	}
	credential, err := s.repository.FindCredentialByEmail(ctx, email)
	if errors.Is(err, model.ErrAuthentication) {
		_ = bcrypt.CompareHashAndPassword(s.dummyPasswordHash, []byte(input.Password))
		return model.AuthenticatedUser{}, model.ErrAuthentication
	}
	if err != nil {
		return model.AuthenticatedUser{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(input.Password)) != nil {
		return model.AuthenticatedUser{}, model.ErrAuthentication
	}
	now := s.now().UTC()
	token, tokenHash, err := s.newToken()
	if err != nil {
		return model.AuthenticatedUser{}, model.ErrAccountUnavailable
	}
	if err = s.repository.CreateSession(ctx, model.Session{
		ID: uuid.NewString(), UserID: credential.User.ID, TokenHash: tokenHash,
		CreatedAt: now, ExpiresAt: now.Add(sessionTTL),
	}); err != nil {
		return model.AuthenticatedUser{}, err
	}
	return model.AuthenticatedUser{User: credential.User, Token: token, ExpiresAt: now.Add(sessionTTL)}, nil
}

func (s *AccountService) CurrentUser(ctx context.Context, token string) (model.User, error) {
	hash, ok := hashToken(token)
	if !ok {
		return model.User{}, model.ErrAuthentication
	}
	return s.repository.FindUserBySession(ctx, hash, s.now().UTC())
}

func (s *AccountService) UpdateCurrentUser(ctx context.Context, token string, input model.UpdateCurrentUserInput) (model.User, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return model.User{}, err
	}
	if input.Email == nil && input.DisplayName == nil {
		return model.User{}, model.ErrInvalidAccountInput
	}
	var email *string
	if input.Email != nil {
		normalized, ok := normalizeEmail(*input.Email)
		if !ok {
			return model.User{}, model.ErrInvalidAccountInput
		}
		email = &normalized
	}
	var displayName *string
	if input.DisplayName != nil {
		trimmed := strings.TrimSpace(*input.DisplayName)
		if !validDisplayName(trimmed) {
			return model.User{}, model.ErrInvalidAccountInput
		}
		displayName = &trimmed
	}
	return s.repository.UpdateUser(ctx, user.ID, email, displayName, s.now().UTC())
}

func (s *AccountService) Logout(ctx context.Context, token string) error {
	hash, ok := hashToken(token)
	if !ok {
		return model.ErrAuthentication
	}
	return s.repository.DeleteSession(ctx, hash)
}

func (s *AccountService) DeleteCurrentUser(ctx context.Context, token string) error {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	return s.repository.DeleteUser(ctx, user.ID)
}

func (s *AccountService) newToken() (string, []byte, error) {
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

func hashToken(token string) ([]byte, bool) {
	if len(token) != encodedTokenLength {
		return nil, false
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(token); err != nil || len(decoded) != tokenBytes {
		return nil, false
	}
	sum := sha256.Sum256([]byte(token))
	return sum[:], true
}

func normalizeEmail(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if len(normalized) < 3 || len(normalized) > 254 || strings.ContainsAny(normalized, "\r\n") {
		return "", false
	}
	address, err := mail.ParseAddress(normalized)
	return normalized, err == nil && address.Address == normalized
}

func validDisplayName(value string) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 80
}

func validNewPassword(value string) bool {
	length := len([]byte(value))
	return utf8.ValidString(value) && length >= 12 && length <= 72
}

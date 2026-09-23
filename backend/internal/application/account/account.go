// Package service owns account business rules and authorization use cases.
package account

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
	CreateAccount(context.Context, User, string, Session) (User, error)
	FindCredentialByEmail(context.Context, string) (Credential, error)
	CreateSession(context.Context, Session) error
	FindUserBySession(context.Context, []byte, time.Time) (User, error)
	UpdateUser(context.Context, string, int, *string, *string, time.Time) (User, error)
	FindProfileByUserID(context.Context, string) (PublicProfile, error)
	FindProfileByHandle(context.Context, string) (PublicProfile, error)
	PutProfile(context.Context, string, PutProfileInput, time.Time) (PublicProfile, error)
	DeleteSession(context.Context, []byte) error
	BeginAccountDeletion(context.Context, string, time.Time) (AccountDeletionRequest, error)
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
		return nil, ErrAccountUnavailable
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("not-a-real-account-password"), passwordCost)
	if err != nil {
		return nil, ErrAccountUnavailable
	}
	return &AccountService{
		repository:        repository,
		now:               time.Now,
		random:            rand.Reader,
		passwordCost:      passwordCost,
		dummyPasswordHash: dummy,
	}, nil
}

func (s *AccountService) Register(ctx context.Context, input RegisterAccountInput) (AuthenticatedUser, error) {
	email, ok := normalizeEmail(input.Email)
	displayName := strings.TrimSpace(input.DisplayName)
	if !ok || !validDisplayName(displayName) || !validNewPassword(input.Password) {
		return AuthenticatedUser{}, ErrInvalidAccountInput
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), s.passwordCost)
	if err != nil {
		return AuthenticatedUser{}, ErrAccountUnavailable
	}
	now := s.now().UTC()
	userID := uuid.NewString()
	token, tokenHash, err := s.newToken()
	if err != nil {
		return AuthenticatedUser{}, ErrAccountUnavailable
	}
	user, err := s.repository.CreateAccount(ctx, User{
		ID: userID, Email: email, DisplayName: displayName, Status: AccountActive,
		Role: AccountUser, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, string(passwordHash), Session{
		ID: uuid.NewString(), UserID: userID, TokenHash: tokenHash,
		CreatedAt: now, ExpiresAt: now.Add(sessionTTL),
	})
	if err != nil {
		return AuthenticatedUser{}, err
	}
	return AuthenticatedUser{User: user, Token: token, ExpiresAt: now.Add(sessionTTL)}, nil
}

func (s *AccountService) Login(ctx context.Context, input CreateSessionInput) (AuthenticatedUser, error) {
	email, ok := normalizeEmail(input.Email)
	if !ok || len(input.Password) == 0 || len([]byte(input.Password)) > 72 {
		return AuthenticatedUser{}, ErrAuthentication
	}
	credential, err := s.repository.FindCredentialByEmail(ctx, email)
	if errors.Is(err, ErrAuthentication) {
		_ = bcrypt.CompareHashAndPassword(s.dummyPasswordHash, []byte(input.Password))
		return AuthenticatedUser{}, ErrAuthentication
	}
	if err != nil {
		return AuthenticatedUser{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(input.Password)) != nil {
		return AuthenticatedUser{}, ErrAuthentication
	}
	now := s.now().UTC()
	token, tokenHash, err := s.newToken()
	if err != nil {
		return AuthenticatedUser{}, ErrAccountUnavailable
	}
	if err = s.repository.CreateSession(ctx, Session{
		ID: uuid.NewString(), UserID: credential.User.ID, TokenHash: tokenHash,
		CreatedAt: now, ExpiresAt: now.Add(sessionTTL),
	}); err != nil {
		return AuthenticatedUser{}, err
	}
	return AuthenticatedUser{User: credential.User, Token: token, ExpiresAt: now.Add(sessionTTL)}, nil
}

func (s *AccountService) CurrentUser(ctx context.Context, token string) (User, error) {
	hash, ok := hashToken(token)
	if !ok {
		return User{}, ErrAuthentication
	}
	return s.repository.FindUserBySession(ctx, hash, s.now().UTC())
}

// Reauthenticate verifies the current session and password without issuing another session.
func (s *AccountService) Reauthenticate(ctx context.Context, token, password string) (User, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return User{}, err
	}
	if len(password) == 0 || len([]byte(password)) > 72 {
		return User{}, ErrAuthentication
	}
	credential, err := s.repository.FindCredentialByEmail(ctx, user.Email)
	if err != nil {
		return User{}, err
	}
	if credential.User.ID != user.ID || bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)) != nil {
		return User{}, ErrAuthentication
	}
	return user, nil
}

func (s *AccountService) UpdateCurrentUser(ctx context.Context, token string, input UpdateCurrentUserInput) (User, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return User{}, err
	}
	if input.Email == nil && input.DisplayName == nil {
		return User{}, ErrInvalidAccountInput
	}
	if input.ExpectedRevision < 1 || input.ExpectedRevision != user.Revision {
		return User{}, ErrAccountConflict
	}
	var email *string
	if input.Email != nil {
		normalized, ok := normalizeEmail(*input.Email)
		if !ok {
			return User{}, ErrInvalidAccountInput
		}
		email = &normalized
	}
	var displayName *string
	if input.DisplayName != nil {
		trimmed := strings.TrimSpace(*input.DisplayName)
		if !validDisplayName(trimmed) {
			return User{}, ErrInvalidAccountInput
		}
		displayName = &trimmed
	}
	return s.repository.UpdateUser(ctx, user.ID, input.ExpectedRevision, email, displayName, s.now().UTC())
}

func (s *AccountService) Logout(ctx context.Context, token string) error {
	hash, ok := hashToken(token)
	if !ok {
		return ErrAuthentication
	}
	return s.repository.DeleteSession(ctx, hash)
}

func (s *AccountService) DeleteCurrentUser(ctx context.Context, token string) (AccountDeletionRequest, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return AccountDeletionRequest{}, err
	}
	return s.repository.BeginAccountDeletion(ctx, user.ID, s.now().UTC())
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

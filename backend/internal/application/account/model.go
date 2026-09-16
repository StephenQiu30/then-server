// Package domain contains transport- and persistence-independent domain values.
package account

import (
	"errors"
	"time"
)

var (
	ErrInvalidAccountInput = errors.New("invalid account input")
	ErrEmailConflict       = errors.New("email conflict")
	ErrAccountConflict     = errors.New("account revision conflict")
	ErrAuthentication      = errors.New("authentication failed")
	ErrAccountUnavailable  = errors.New("account unavailable")
	ErrInvalidProfileInput = errors.New("invalid profile input")
	ErrProfileNotFound     = errors.New("profile not found")
	ErrProfileConflict     = errors.New("profile revision conflict")
	ErrHandleConflict      = errors.New("profile handle conflict")
)

type AccountStatus string

const (
	AccountActive    AccountStatus = "active"
	AccountSuspended AccountStatus = "suspended"
	AccountDeleting  AccountStatus = "deleting"
)

type AccountRole string

const (
	AccountUser      AccountRole = "user"
	AccountModerator AccountRole = "moderator"
	AccountAdmin     AccountRole = "admin"
)

type User struct {
	ID          string
	Email       string
	DisplayName string
	Status      AccountStatus
	Role        AccountRole
	Revision    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Credential struct {
	User         User
	PasswordHash string
}

type Session struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuthenticatedUser struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

type AccountDeletionStatus string

const (
	AccountDeletionPending  AccountDeletionStatus = "pending"
	AccountDeletionComplete AccountDeletionStatus = "complete"
)

type AccountDeletionRequest struct {
	ID          string
	Status      AccountDeletionStatus
	MediaCount  int
	RequestedAt time.Time
	CompletedAt *time.Time
}

type RegisterAccountInput struct {
	Email       string
	DisplayName string
	Password    string
}

type CreateSessionInput struct {
	Email    string
	Password string
}

type UpdateCurrentUserInput struct {
	Email            *string
	DisplayName      *string
	ExpectedRevision int
}

type PublicProfile struct {
	Handle      string
	DisplayName string
	Bio         *string
	Revision    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PutProfileInput struct {
	Handle           string
	Bio              *string
	ExpectedRevision int
}

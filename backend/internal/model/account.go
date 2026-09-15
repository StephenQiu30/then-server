// Package model contains transport- and persistence-independent domain values.
package model

import (
	"errors"
	"time"
)

var (
	ErrInvalidAccountInput  = errors.New("invalid account input")
	ErrEmailConflict        = errors.New("email conflict")
	ErrAccountMediaConflict = errors.New("account still owns active media")
	ErrAuthentication       = errors.New("authentication failed")
	ErrAccountUnavailable   = errors.New("account unavailable")
)

type User struct {
	ID          string
	Email       string
	DisplayName string
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
	Email       *string
	DisplayName *string
}

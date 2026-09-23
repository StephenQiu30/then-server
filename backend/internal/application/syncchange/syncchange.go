package syncchange

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

var ErrInvalidCursor = errors.New("invalid sync cursor")

type Change struct {
	Seq      int64
	Kind     string
	EntityID string
	Action   string
	Revision *int
}

type Page struct {
	Changes    []Change
	NextCursor string
	HasMore    bool
}

type Account interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
}

type Repository interface {
	List(context.Context, string, int64, int) ([]Change, int64, error)
}

type Service struct {
	accounts   Account
	repository Repository
}

func New(accounts Account, repository Repository) (*Service, error) {
	if accounts == nil || repository == nil {
		return nil, errors.New("sync dependencies are required")
	}
	return &Service{accounts: accounts, repository: repository}, nil
}

func (s *Service) List(ctx context.Context, session, after string, limit int) (Page, error) {
	if limit < 1 || limit > 100 {
		return Page{}, ErrInvalidCursor
	}
	user, err := s.accounts.CurrentUser(ctx, session)
	if err != nil {
		return Page{}, err
	}
	seq, err := decodeCursor(user.ID, after)
	if err != nil {
		return Page{}, err
	}
	changes, watermark, err := s.repository.List(ctx, user.ID, seq, limit)
	if err != nil {
		return Page{}, err
	}
	if seq > watermark {
		return Page{}, ErrInvalidCursor
	}
	next := seq
	if len(changes) > 0 {
		next = changes[len(changes)-1].Seq
	}
	return Page{Changes: changes, NextCursor: encodeCursor(user.ID, next), HasMore: next < watermark}, nil
}

func encodeCursor(ownerID string, seq int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("v1:" + ownerID + ":" + strconv.FormatInt(seq, 10)))
}

func decodeCursor(ownerID, value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	if len(value) > 128 {
		return 0, ErrInvalidCursor
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return 0, ErrInvalidCursor
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 || parts[0] != "v1" || parts[1] != ownerID {
		return 0, ErrInvalidCursor
	}
	seq, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || seq < 0 || strconv.FormatInt(seq, 10) != parts[2] {
		return 0, ErrInvalidCursor
	}
	return seq, nil
}

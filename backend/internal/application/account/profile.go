package account

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"
)

var profileHandlePattern = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

func (s *AccountService) CurrentProfile(ctx context.Context, token string) (PublicProfile, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return PublicProfile{}, err
	}
	return s.repository.FindProfileByUserID(ctx, user.ID)
}

func (s *AccountService) PublicProfile(ctx context.Context, handle string) (PublicProfile, error) {
	handle, ok := normalizeProfileHandle(handle)
	if !ok {
		return PublicProfile{}, ErrProfileNotFound
	}
	return s.repository.FindProfileByHandle(ctx, handle)
}

func (s *AccountService) PutCurrentProfile(ctx context.Context, token string, input PutProfileInput) (PublicProfile, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return PublicProfile{}, err
	}
	handle, ok := normalizeProfileHandle(input.Handle)
	if !ok || input.ExpectedRevision < 0 {
		return PublicProfile{}, ErrInvalidProfileInput
	}
	bio, ok := normalizeProfileBio(input.Bio)
	if !ok {
		return PublicProfile{}, ErrInvalidProfileInput
	}
	input.Handle, input.Bio = handle, bio
	return s.repository.PutProfile(ctx, user.ID, input, s.now().UTC())
}

func normalizeProfileHandle(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized, profileHandlePattern.MatchString(normalized)
}

func normalizeProfileBio(value *string) (*string, bool) {
	if value == nil {
		return nil, true
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, true
	}
	if utf8.RuneCountInString(normalized) > 300 {
		return nil, false
	}
	return &normalized, true
}

package account

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
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

func (s *AccountService) PutProfileAvatar(ctx context.Context, token, mediaID string, expectedRevision int) (PublicProfile, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return PublicProfile{}, err
	}
	if uuid.Validate(mediaID) != nil || expectedRevision < 1 {
		return PublicProfile{}, ErrInvalidProfileInput
	}
	return s.repository.PutProfileAvatar(ctx, user.ID, mediaID, expectedRevision, s.now().UTC())
}

func (s *AccountService) DeleteProfileAvatar(ctx context.Context, token string, expectedRevision int) (PublicProfile, error) {
	user, err := s.CurrentUser(ctx, token)
	if err != nil {
		return PublicProfile{}, err
	}
	if expectedRevision < 1 {
		return PublicProfile{}, ErrInvalidProfileInput
	}
	return s.repository.DeleteProfileAvatar(ctx, user.ID, expectedRevision, s.now().UTC())
}

func (s *AccountService) PublicProfileAvatar(ctx context.Context, handle string) (ProfileAvatarReference, error) {
	handle, ok := normalizeProfileHandle(handle)
	if !ok {
		return ProfileAvatarReference{}, ErrProfileNotFound
	}
	return s.repository.FindProfileAvatar(ctx, handle)
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

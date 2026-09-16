package community

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"

	"github.com/google/uuid"
)

type Authenticator interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
}

type Repository interface {
	CreatePost(context.Context, string, string, PostContentInput, time.Time) (Post, error)
	ListOwnPosts(context.Context, string, int, *string, *PostState) (PostPage, error)
	GetOwnPost(context.Context, string, string) (Post, error)
	UpdatePost(context.Context, string, string, int, PostContentInput, time.Time) (Post, error)
	SubmitPost(context.Context, string, string, int, time.Time) (Post, error)
	WithdrawPost(context.Context, string, string, int, time.Time) (Post, error)
	DeletePost(context.Context, string, string, int, time.Time) error
	GetPublicPost(context.Context, string) (PublicPost, error)
	GetPublicPostImage(context.Context, string, int) (MediaObjectReference, error)
	ListModerationCandidates(context.Context, int, *string) (ModerationCandidatePage, error)
	GetModerationCandidate(context.Context, string, int) (ModerationCandidate, error)
	GetModerationImage(context.Context, string, int, int) (MediaObjectReference, error)
	DecidePost(context.Context, string, string, DecidePostInput, time.Time) (Post, error)
	RemovePost(context.Context, string, string, int, string, time.Time) error
	CreateReport(context.Context, string, string, string, CreateReportInput, time.Time) (ContentReport, error)
	ListOwnReports(context.Context, string, int, *string) (ContentReportPage, error)
	ListReports(context.Context, int, *string, *ReportStatus) (AdminReportPage, error)
	ResolveReport(context.Context, string, string, ReportStatus, string, time.Time) (ContentReport, error)
	ListModerationActions(context.Context, int, *string) (ModerationActionPage, error)
	ListUsers(context.Context, int, *string) (AdminUserPage, error)
	SetUserStatus(context.Context, string, string, int, accountapp.AccountStatus, string, time.Time) (AdminUser, error)
}

type Service struct {
	authenticator Authenticator
	repository    Repository
	now           func() time.Time
}

func NewService(authenticator Authenticator, repository Repository) (*Service, error) {
	if authenticator == nil || repository == nil {
		return nil, ErrCommunityUnavailable
	}
	return &Service{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *Service) CreatePost(ctx context.Context, token, postID string, input PostContentInput) (Post, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return Post{}, err
	}
	normalized, ok := normalizePostInput(postID, input)
	if !ok {
		return Post{}, ErrInvalidCommunityInput
	}
	return s.repository.CreatePost(ctx, user.ID, postID, normalized, s.now().UTC())
}

func (s *Service) ListOwnPosts(ctx context.Context, token string, limit int, afterID string, state string) (PostPage, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return PostPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if limit < 1 || limit > 50 || !valid {
		return PostPage{}, ErrInvalidCommunityInput
	}
	var filter *PostState
	if state != "" {
		value := PostState(state)
		if !validPostState(value) || value == PostDeleted {
			return PostPage{}, ErrInvalidCommunityInput
		}
		filter = &value
	}
	return s.repository.ListOwnPosts(ctx, user.ID, limit, after, filter)
}

func (s *Service) GetOwnPost(ctx context.Context, token, postID string) (Post, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return Post{}, err
	}
	if !validUUID(postID) {
		return Post{}, ErrInvalidCommunityInput
	}
	return s.repository.GetOwnPost(ctx, user.ID, postID)
}

func (s *Service) UpdatePost(ctx context.Context, token, postID string, expectedRevision int, input PostContentInput) (Post, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return Post{}, err
	}
	normalized, ok := normalizePostInput(postID, input)
	if !ok || expectedRevision < 1 {
		return Post{}, ErrInvalidCommunityInput
	}
	return s.repository.UpdatePost(ctx, user.ID, postID, expectedRevision, normalized, s.now().UTC())
}

func (s *Service) SubmitPost(ctx context.Context, token, postID string, expectedRevision int) (Post, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return Post{}, err
	}
	if !validUUID(postID) || expectedRevision < 1 {
		return Post{}, ErrInvalidCommunityInput
	}
	return s.repository.SubmitPost(ctx, user.ID, postID, expectedRevision, s.now().UTC())
}

func (s *Service) WithdrawPost(ctx context.Context, token, postID string, expectedRevision int) (Post, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return Post{}, err
	}
	if !validUUID(postID) || expectedRevision < 1 {
		return Post{}, ErrInvalidCommunityInput
	}
	return s.repository.WithdrawPost(ctx, user.ID, postID, expectedRevision, s.now().UTC())
}

func (s *Service) DeletePost(ctx context.Context, token, postID string, expectedRevision int) error {
	user, err := s.current(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(postID) || expectedRevision < 1 {
		return ErrInvalidCommunityInput
	}
	return s.repository.DeletePost(ctx, user.ID, postID, expectedRevision, s.now().UTC())
}

func (s *Service) GetPublicPost(ctx context.Context, postID string) (PublicPost, error) {
	if !validUUID(postID) {
		return PublicPost{}, ErrPostNotFound
	}
	return s.repository.GetPublicPost(ctx, postID)
}

func (s *Service) GetPublicPostImage(ctx context.Context, postID string, ordinal int) (MediaObjectReference, error) {
	if !validUUID(postID) || ordinal < 0 || ordinal > 8 {
		return MediaObjectReference{}, ErrPostNotFound
	}
	return s.repository.GetPublicPostImage(ctx, postID, ordinal)
}

func (s *Service) ListModerationCandidates(ctx context.Context, token string, limit int, afterID string) (ModerationCandidatePage, error) {
	if _, err := s.operator(ctx, token, false); err != nil {
		return ModerationCandidatePage{}, err
	}
	after, valid := optionalUUID(afterID)
	if limit < 1 || limit > 50 || !valid {
		return ModerationCandidatePage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListModerationCandidates(ctx, limit, after)
}

func (s *Service) GetModerationCandidate(ctx context.Context, token, postID string, version int) (ModerationCandidate, error) {
	if _, err := s.operator(ctx, token, false); err != nil {
		return ModerationCandidate{}, err
	}
	if !validUUID(postID) || version < 1 {
		return ModerationCandidate{}, ErrInvalidCommunityInput
	}
	return s.repository.GetModerationCandidate(ctx, postID, version)
}

func (s *Service) GetModerationImage(ctx context.Context, token, postID string, version, ordinal int) (MediaObjectReference, error) {
	if _, err := s.operator(ctx, token, false); err != nil {
		return MediaObjectReference{}, err
	}
	if !validUUID(postID) || version < 1 || ordinal < 0 || ordinal > 8 {
		return MediaObjectReference{}, ErrInvalidCommunityInput
	}
	return s.repository.GetModerationImage(ctx, postID, version, ordinal)
}

func (s *Service) DecidePost(ctx context.Context, token, postID string, input DecidePostInput) (Post, error) {
	operator, err := s.operator(ctx, token, false)
	if err != nil {
		return Post{}, err
	}
	if !validUUID(postID) || input.Version < 1 || input.ReviewRound < 1 || input.ExpectedRevision < 1 || (input.Decision != ModerationApprove && input.Decision != ModerationReject) || !validReason(input.ReasonCode) {
		return Post{}, ErrInvalidCommunityInput
	}
	return s.repository.DecidePost(ctx, operator.ID, postID, input, s.now().UTC())
}

func (s *Service) RemovePost(ctx context.Context, token, postID string, expectedRevision int, reason string) error {
	operator, err := s.operator(ctx, token, false)
	if err != nil {
		return err
	}
	if !validUUID(postID) || expectedRevision < 1 || !validReason(reason) {
		return ErrInvalidCommunityInput
	}
	return s.repository.RemovePost(ctx, operator.ID, postID, expectedRevision, reason, s.now().UTC())
}

func (s *Service) CreateReport(ctx context.Context, token, reportID, postID string, input CreateReportInput) (ContentReport, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return ContentReport{}, err
	}
	detail, ok := normalizeOptional(input.Detail, 500, true)
	if !validUUID(reportID) || !validUUID(postID) || !validReportReason(input.ReasonCode) || !ok {
		return ContentReport{}, ErrInvalidCommunityInput
	}
	input.Detail = detail
	return s.repository.CreateReport(ctx, user.ID, reportID, postID, input, s.now().UTC())
}

func (s *Service) ListOwnReports(ctx context.Context, token string, limit int, afterID string) (ContentReportPage, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return ContentReportPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if limit < 1 || limit > 50 || !valid {
		return ContentReportPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListOwnReports(ctx, user.ID, limit, after)
}

func (s *Service) ListReports(ctx context.Context, token string, limit int, afterID, status string) (AdminReportPage, error) {
	if _, err := s.operator(ctx, token, false); err != nil {
		return AdminReportPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if limit < 1 || limit > 50 || !valid {
		return AdminReportPage{}, ErrInvalidCommunityInput
	}
	var filter *ReportStatus
	if status != "" {
		value := ReportStatus(status)
		if value != ReportOpen && value != ReportResolved && value != ReportDismissed {
			return AdminReportPage{}, ErrInvalidCommunityInput
		}
		filter = &value
	}
	return s.repository.ListReports(ctx, limit, after, filter)
}

func (s *Service) ResolveReport(ctx context.Context, token, reportID, status, reason string) (ContentReport, error) {
	operator, err := s.operator(ctx, token, false)
	if err != nil {
		return ContentReport{}, err
	}
	resolution := ReportStatus(status)
	if !validUUID(reportID) || (resolution != ReportResolved && resolution != ReportDismissed) || !validReason(reason) {
		return ContentReport{}, ErrInvalidCommunityInput
	}
	return s.repository.ResolveReport(ctx, operator.ID, reportID, resolution, reason, s.now().UTC())
}

func (s *Service) ListModerationActions(ctx context.Context, token string, limit int, afterID string) (ModerationActionPage, error) {
	if _, err := s.operator(ctx, token, false); err != nil {
		return ModerationActionPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if limit < 1 || limit > 50 || !valid {
		return ModerationActionPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListModerationActions(ctx, limit, after)
}

func (s *Service) ListUsers(ctx context.Context, token string, limit int, afterID string) (AdminUserPage, error) {
	if _, err := s.operator(ctx, token, true); err != nil {
		return AdminUserPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if limit < 1 || limit > 50 || !valid {
		return AdminUserPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListUsers(ctx, limit, after)
}

func (s *Service) SetUserStatus(ctx context.Context, token, userID string, expectedRevision int, status accountapp.AccountStatus, reason string) (AdminUser, error) {
	operator, err := s.operator(ctx, token, true)
	if err != nil {
		return AdminUser{}, err
	}
	if !validUUID(userID) || operator.ID == userID || expectedRevision < 1 || (status != accountapp.AccountSuspended && status != accountapp.AccountActive) || !validReason(reason) {
		return AdminUser{}, ErrInvalidCommunityInput
	}
	return s.repository.SetUserStatus(ctx, operator.ID, userID, expectedRevision, status, reason, s.now().UTC())
}

func (s *Service) current(ctx context.Context, token string) (accountapp.User, error) {
	return s.authenticator.CurrentUser(ctx, token)
}

func (s *Service) operator(ctx context.Context, token string, adminOnly bool) (accountapp.User, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return accountapp.User{}, err
	}
	if user.Role != accountapp.AccountAdmin && (adminOnly || user.Role != accountapp.AccountModerator) {
		return accountapp.User{}, ErrCommunityForbidden
	}
	return user, nil
}

var reasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

func normalizePostInput(postID string, input PostContentInput) (PostContentInput, bool) {
	if !validUUID(postID) || len(input.MediaIDs) > 9 || len(input.Tags) > 5 {
		return PostContentInput{}, false
	}
	title, ok := normalizeOptional(input.Title, 80, false)
	if !ok {
		return PostContentInput{}, false
	}
	body, ok := normalizeOptional(input.Body, 3000, true)
	if !ok || body == nil && len(input.MediaIDs) == 0 {
		return PostContentInput{}, false
	}
	if input.SourceDiaryID != nil && !validUUID(*input.SourceDiaryID) {
		return PostContentInput{}, false
	}
	seenMedia := make(map[string]bool, len(input.MediaIDs))
	for _, mediaID := range input.MediaIDs {
		if !validUUID(mediaID) || seenMedia[mediaID] {
			return PostContentInput{}, false
		}
		seenMedia[mediaID] = true
	}
	seenTags := make(map[string]bool, len(input.Tags))
	tags := make([]string, 0, len(input.Tags))
	for _, raw := range input.Tags {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if utf8.RuneCountInString(tag) < 1 || utf8.RuneCountInString(tag) > 20 || hasDisallowedControls(tag, false) || seenTags[tag] {
			return PostContentInput{}, false
		}
		seenTags[tag] = true
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	input.Title, input.Body, input.Tags = title, body, tags
	input.MediaIDs = append([]string(nil), input.MediaIDs...)
	return input, true
}

func normalizeOptional(value *string, limit int, multiline bool) (*string, bool) {
	if value == nil {
		return nil, true
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, true
	}
	if utf8.RuneCountInString(normalized) > limit || hasDisallowedControls(normalized, multiline) {
		return nil, false
	}
	return &normalized, true
}

func hasDisallowedControls(value string, multiline bool) bool {
	for _, character := range value {
		if unicode.IsControl(character) && !(multiline && (character == '\n' || character == '\r' || character == '\t')) {
			return true
		}
	}
	return false
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}

func optionalUUID(value string) (*string, bool) {
	if value == "" {
		return nil, true
	}
	if !validUUID(value) {
		return nil, false
	}
	return &value, true
}

func validPostState(value PostState) bool {
	return value == PostDraft || value == PostPending || value == PostPublished || value == PostWithdrawn || value == PostRemoved
}

func validReason(value string) bool { return reasonPattern.MatchString(value) }

func validReportReason(value string) bool {
	switch value {
	case "spam", "harassment", "sexual", "violence", "misinformation", "other":
		return true
	default:
		return false
	}
}

package community

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type SocialRepository interface {
	GetPublicPostForViewer(context.Context, string, *string) (PublicPost, error)
	GetPublicPostImageForViewer(context.Context, string, int, *string) (MediaObjectReference, error)
	ListFeed(context.Context, FeedType, *string, int, *string) (PublicPostPage, error)
	SearchPosts(context.Context, *string, *string, *string, int, *string) (PublicPostPage, error)
	ListProfilePosts(context.Context, string, *string, int, *string) (PublicPostPage, error)
	SetPostLike(context.Context, string, string, bool, time.Time) error
	SetPostBookmark(context.Context, string, string, bool, time.Time) error
	ListBookmarks(context.Context, string, int, *string) (PublicPostPage, error)
	SetFollow(context.Context, string, string, bool, time.Time) error
	ListProfileRelationships(context.Context, string, string, *string, int, *string) (PublicProfilePage, error)
	SetBlock(context.Context, string, string, bool, time.Time) error
	ListBlocks(context.Context, string, int, *string) (BlockedUserPage, error)
	CreateComment(context.Context, string, string, string, CreateCommentInput, time.Time) (Comment, error)
	ListComments(context.Context, string, *string, *string, int, *string) (CommentPage, error)
	ListReplies(context.Context, string, *string, int, *string) (CommentPage, error)
	DeleteComment(context.Context, string, string, int, time.Time) error
	ListCommentModeration(context.Context, int, *string) (CommentModerationPage, error)
	DecideComment(context.Context, string, string, DecideCommentInput, time.Time) (Comment, error)
	RemoveComment(context.Context, string, string, int, string, time.Time) error
	ListNotifications(context.Context, string, int, *string) (NotificationPage, error)
	MarkNotificationRead(context.Context, string, string, time.Time) (Notification, error)
	CreateAppeal(context.Context, string, string, string, string, time.Time) (ModerationAppeal, error)
	ListOwnAppeals(context.Context, string, int, *string) (ModerationAppealPage, error)
	ListAppeals(context.Context, int, *string, *AppealStatus) (AdminModerationAppealPage, error)
	ResolveAppeal(context.Context, string, string, AppealStatus, string, time.Time) (ModerationAppeal, error)
}

var communityHandlePattern = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

func (s *Service) GetPublicPostForViewer(ctx context.Context, token, postID string) (PublicPost, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return PublicPost{}, err
	}
	if !validUUID(postID) {
		return PublicPost{}, ErrPostNotFound
	}
	return s.repository.GetPublicPostForViewer(ctx, postID, optionalUserID(viewer))
}

func (s *Service) GetPublicPostImageForViewer(ctx context.Context, token, postID string, ordinal int) (MediaObjectReference, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return MediaObjectReference{}, err
	}
	if !validUUID(postID) || ordinal < 0 || ordinal > 8 {
		return MediaObjectReference{}, ErrPostNotFound
	}
	return s.repository.GetPublicPostImageForViewer(ctx, postID, ordinal, optionalUserID(viewer))
}

func (s *Service) ListFeed(ctx context.Context, token, feedType string, limit int, afterID string) (PublicPostPage, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return PublicPostPage{}, err
	}
	kind := FeedType(feedType)
	if kind != FeedDiscover && kind != FeedFollowing {
		return PublicPostPage{}, ErrInvalidCommunityInput
	}
	if kind == FeedFollowing && viewer == nil {
		return PublicPostPage{}, accountapp.ErrAuthentication
	}
	after, valid := optionalUUID(afterID)
	if limit < 1 || limit > 50 || !valid {
		return PublicPostPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListFeed(ctx, kind, optionalUserID(viewer), limit, after)
}

func (s *Service) SearchPosts(ctx context.Context, token, query, tag string, limit int, afterID string) (PublicPostPage, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return PublicPostPage{}, err
	}
	normalizedQuery, queryOK := normalizeSearchQuery(query)
	normalizedTag, tagOK := normalizeSearchTag(tag)
	after, cursorOK := optionalUUID(afterID)
	if !queryOK || !tagOK || normalizedQuery == nil && normalizedTag == nil || limit < 1 || limit > 50 || !cursorOK {
		return PublicPostPage{}, ErrInvalidCommunityInput
	}
	return s.repository.SearchPosts(ctx, optionalUserID(viewer), normalizedQuery, normalizedTag, limit, after)
}

func (s *Service) ListProfilePosts(ctx context.Context, token, handle string, limit int, afterID string) (PublicPostPage, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return PublicPostPage{}, err
	}
	handle, ok := normalizeCommunityHandle(handle)
	after, cursorOK := optionalUUID(afterID)
	if !ok || !cursorOK || limit < 1 || limit > 50 {
		return PublicPostPage{}, ErrPostNotFound
	}
	return s.repository.ListProfilePosts(ctx, handle, optionalUserID(viewer), limit, after)
}

func (s *Service) SetPostLike(ctx context.Context, token, postID string, enabled bool) error {
	user, err := s.current(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(postID) {
		return ErrInvalidCommunityInput
	}
	return s.repository.SetPostLike(ctx, user.ID, postID, enabled, s.now().UTC())
}

func (s *Service) SetPostBookmark(ctx context.Context, token, postID string, enabled bool) error {
	user, err := s.current(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(postID) {
		return ErrInvalidCommunityInput
	}
	return s.repository.SetPostBookmark(ctx, user.ID, postID, enabled, s.now().UTC())
}

func (s *Service) ListBookmarks(ctx context.Context, token string, limit int, afterID string) (PublicPostPage, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return PublicPostPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if !valid || limit < 1 || limit > 50 {
		return PublicPostPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListBookmarks(ctx, user.ID, limit, after)
}

func (s *Service) SetFollow(ctx context.Context, token, handle string, enabled bool) error {
	user, err := s.current(ctx, token)
	if err != nil {
		return err
	}
	handle, ok := normalizeCommunityHandle(handle)
	if !ok {
		return ErrPostNotFound
	}
	return s.repository.SetFollow(ctx, user.ID, handle, enabled, s.now().UTC())
}

func (s *Service) ListProfileRelationships(ctx context.Context, token, handle, direction string, limit int, afterID string) (PublicProfilePage, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return PublicProfilePage{}, err
	}
	handle, ok := normalizeCommunityHandle(handle)
	after, cursorOK := optionalUUID(afterID)
	if !ok || (direction != "followers" && direction != "following") || !cursorOK || limit < 1 || limit > 50 {
		return PublicProfilePage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListProfileRelationships(ctx, handle, direction, optionalUserID(viewer), limit, after)
}

func (s *Service) SetBlock(ctx context.Context, token, targetUserID string, enabled bool) error {
	user, err := s.current(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(targetUserID) || targetUserID == user.ID {
		return ErrInvalidCommunityInput
	}
	return s.repository.SetBlock(ctx, user.ID, targetUserID, enabled, s.now().UTC())
}

func (s *Service) ListBlocks(ctx context.Context, token string, limit int, afterID string) (BlockedUserPage, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return BlockedUserPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if !valid || limit < 1 || limit > 50 {
		return BlockedUserPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListBlocks(ctx, user.ID, limit, after)
}

func (s *Service) CreateComment(ctx context.Context, token, commentID, postID string, input CreateCommentInput) (Comment, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return Comment{}, err
	}
	body, ok := normalizeCommentBody(input.Body)
	if !validUUID(commentID) || !validUUID(postID) || !ok || input.ParentID != nil && !validUUID(*input.ParentID) {
		return Comment{}, ErrInvalidCommunityInput
	}
	input.Body = body
	return s.repository.CreateComment(ctx, user.ID, commentID, postID, input, s.now().UTC())
}

func (s *Service) ListComments(ctx context.Context, token, postID string, parentID *string, limit int, afterID string) (CommentPage, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return CommentPage{}, err
	}
	after, cursorOK := optionalUUID(afterID)
	if !validUUID(postID) || parentID != nil && !validUUID(*parentID) || !cursorOK || limit < 1 || limit > 50 {
		return CommentPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListComments(ctx, postID, optionalUserID(viewer), parentID, limit, after)
}

func (s *Service) ListReplies(ctx context.Context, token, commentID string, limit int, afterID string) (CommentPage, error) {
	viewer, err := s.optionalUser(ctx, token)
	if err != nil {
		return CommentPage{}, err
	}
	after, cursorOK := optionalUUID(afterID)
	if !validUUID(commentID) || !cursorOK || limit < 1 || limit > 50 {
		return CommentPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListReplies(ctx, commentID, optionalUserID(viewer), limit, after)
}

func (s *Service) DeleteComment(ctx context.Context, token, commentID string, expectedRevision int) error {
	user, err := s.current(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(commentID) || expectedRevision < 1 {
		return ErrInvalidCommunityInput
	}
	return s.repository.DeleteComment(ctx, user.ID, commentID, expectedRevision, s.now().UTC())
}

func (s *Service) ListCommentModeration(ctx context.Context, token string, limit int, afterID string) (CommentModerationPage, error) {
	if _, err := s.operator(ctx, token, false); err != nil {
		return CommentModerationPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if !valid || limit < 1 || limit > 50 {
		return CommentModerationPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListCommentModeration(ctx, limit, after)
}

func (s *Service) DecideComment(ctx context.Context, token, commentID string, input DecideCommentInput) (Comment, error) {
	operator, err := s.operator(ctx, token, false)
	if err != nil {
		return Comment{}, err
	}
	if !validUUID(commentID) || input.ExpectedRevision < 1 || (input.Decision != ModerationApprove && input.Decision != ModerationReject) || !validReason(input.ReasonCode) {
		return Comment{}, ErrInvalidCommunityInput
	}
	return s.repository.DecideComment(ctx, operator.ID, commentID, input, s.now().UTC())
}

func (s *Service) RemoveComment(ctx context.Context, token, commentID string, expectedRevision int, reason string) error {
	operator, err := s.operator(ctx, token, false)
	if err != nil {
		return err
	}
	if !validUUID(commentID) || expectedRevision < 1 || !validReason(reason) {
		return ErrInvalidCommunityInput
	}
	return s.repository.RemoveComment(ctx, operator.ID, commentID, expectedRevision, reason, s.now().UTC())
}

func (s *Service) ListNotifications(ctx context.Context, token string, limit int, afterID string) (NotificationPage, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return NotificationPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if !valid || limit < 1 || limit > 50 {
		return NotificationPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListNotifications(ctx, user.ID, limit, after)
}

func (s *Service) MarkNotificationRead(ctx context.Context, token, notificationID string) (Notification, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return Notification{}, err
	}
	if !validUUID(notificationID) {
		return Notification{}, ErrInvalidCommunityInput
	}
	return s.repository.MarkNotificationRead(ctx, user.ID, notificationID, s.now().UTC())
}

func (s *Service) CreateAppeal(ctx context.Context, token, appealID, actionID, reason string) (ModerationAppeal, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return ModerationAppeal{}, err
	}
	reason, ok := normalizeRequiredText(reason, 500, true)
	if !validUUID(appealID) || !validUUID(actionID) || !ok {
		return ModerationAppeal{}, ErrInvalidCommunityInput
	}
	return s.repository.CreateAppeal(ctx, user.ID, appealID, actionID, reason, s.now().UTC())
}

func (s *Service) ListOwnAppeals(ctx context.Context, token string, limit int, afterID string) (ModerationAppealPage, error) {
	user, err := s.current(ctx, token)
	if err != nil {
		return ModerationAppealPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if !valid || limit < 1 || limit > 50 {
		return ModerationAppealPage{}, ErrInvalidCommunityInput
	}
	return s.repository.ListOwnAppeals(ctx, user.ID, limit, after)
}

func (s *Service) ListAppeals(ctx context.Context, token string, limit int, afterID, status string) (AdminModerationAppealPage, error) {
	if _, err := s.operator(ctx, token, true); err != nil {
		return AdminModerationAppealPage{}, err
	}
	after, valid := optionalUUID(afterID)
	if !valid || limit < 1 || limit > 50 {
		return AdminModerationAppealPage{}, ErrInvalidCommunityInput
	}
	var filter *AppealStatus
	if status != "" {
		value := AppealStatus(status)
		if value != AppealOpen && value != AppealUpheld && value != AppealReversed {
			return AdminModerationAppealPage{}, ErrInvalidCommunityInput
		}
		filter = &value
	}
	return s.repository.ListAppeals(ctx, limit, after, filter)
}

func (s *Service) ResolveAppeal(ctx context.Context, token, appealID, status, reason string) (ModerationAppeal, error) {
	operator, err := s.operator(ctx, token, true)
	if err != nil {
		return ModerationAppeal{}, err
	}
	resolution := AppealStatus(status)
	if !validUUID(appealID) || (resolution != AppealUpheld && resolution != AppealReversed) || !validReason(reason) {
		return ModerationAppeal{}, ErrInvalidCommunityInput
	}
	return s.repository.ResolveAppeal(ctx, operator.ID, appealID, resolution, reason, s.now().UTC())
}

func (s *Service) optionalUser(ctx context.Context, token string) (*accountapp.User, error) {
	if token == "" {
		return nil, nil
	}
	user, err := s.current(ctx, token)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func optionalUserID(user *accountapp.User) *string {
	if user == nil {
		return nil
	}
	return &user.ID
}

func normalizeCommunityHandle(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized, communityHandlePattern.MatchString(normalized)
}

func normalizeSearchQuery(value string) (*string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	if utf8.RuneCountInString(value) < 2 || utf8.RuneCountInString(value) > 80 || hasInvalidSearchControl(value) {
		return nil, false
	}
	return &value, true
}

func normalizeSearchTag(value string) (*string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return nil, true
	}
	if utf8.RuneCountInString(value) > 20 || hasDisallowedControls(value, false) {
		return nil, false
	}
	return &value, true
}

func normalizeCommentBody(value string) (string, bool) {
	return normalizeRequiredText(value, 500, true)
}

func normalizeRequiredText(value string, limit int, multiline bool) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > limit || hasDisallowedControls(value, multiline) {
		return "", false
	}
	return value, true
}

func hasInvalidSearchControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

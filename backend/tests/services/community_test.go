//go:build services

package services

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"
	diaryapp "github.com/StephenQiu30/then-server/backend/internal/application/diary"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCommunityPublishingModerationAndGovernanceLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open community database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create community schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("community schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse community database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated community schema", err)
	if err := store.Migrate(ctx, database); err != nil {
		t.Fatalf("migrate community schema: %v", err)
	}
	var tableCount int64
	serviceOK(t, "count migrated tables", database.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'`).Scan(&tableCount).Error)
	if tableCount != 46 {
		t.Fatalf("migrated table count=%d, want 46", tableCount)
	}
	var foreignKeys int64
	serviceOK(t, "inspect community foreign keys", database.WithContext(ctx).Raw(`
		SELECT count(*) FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = current_schema() AND c.contype = 'f'
		  AND t.relname IN ('posts','post_revisions','post_revision_media','post_revision_tags','content_reports')`).Scan(&foreignKeys).Error)
	if foreignKeys < 8 {
		t.Fatalf("community schema has only %d foreign keys", foreignKeys)
	}

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct community account service", err)
	author, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "community-author@example.test", DisplayName: "Author", Password: "correct-password-author"})
	serviceOK(t, "register community author", err)
	moderator, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "community-moderator@example.test", DisplayName: "Moderator", Password: "correct-password-moderator"})
	serviceOK(t, "register community moderator", err)
	admin, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "community-admin@example.test", DisplayName: "Admin", Password: "correct-password-admin"})
	serviceOK(t, "register community admin", err)
	reporter, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "community-reporter@example.test", DisplayName: "Reporter", Password: "correct-password-reporter"})
	serviceOK(t, "register community reporter", err)
	_, err = accounts.PutCurrentProfile(ctx, author.Token, accountapp.PutProfileInput{Handle: "community_author", ExpectedRevision: 0})
	serviceOK(t, "create author profile", err)
	_, err = accounts.PutCurrentProfile(ctx, moderator.Token, accountapp.PutProfileInput{Handle: "community_moderator", ExpectedRevision: 0})
	serviceOK(t, "create moderator profile", err)
	_, err = accounts.PutCurrentProfile(ctx, admin.Token, accountapp.PutProfileInput{Handle: "community_admin", ExpectedRevision: 0})
	serviceOK(t, "create admin profile", err)
	_, err = accounts.PutCurrentProfile(ctx, reporter.Token, accountapp.PutProfileInput{Handle: "community_reporter", ExpectedRevision: 0})
	serviceOK(t, "create reporter profile", err)
	serviceOK(t, "grant moderator role", database.WithContext(ctx).Exec("UPDATE users SET role = 'moderator' WHERE id = ?", moderator.User.ID).Error)
	serviceOK(t, "grant admin role", database.WithContext(ctx).Exec("UPDATE users SET role = 'admin' WHERE id = ?", admin.User.ID).Error)

	repository := store.NewCommunityRepository(database)
	community, err := communityapp.NewService(accounts, repository)
	serviceOK(t, "construct community service", err)
	diaries, err := diaryapp.NewService(accounts, store.NewDiaryRepository(database))
	serviceOK(t, "construct community source diary service", err)
	diaryBody := "private mood and context must stay private"
	today := time.Now().UTC().Format("2006-01-02")
	diaryID := "10000000-0000-4000-8000-000000000080"
	sourceDiary, err := diaries.Create(ctx, author.Token, diaryID, diaryapp.DiaryEntryInput{LocalDate: today, TimeZone: "UTC", Body: &diaryBody})
	serviceOK(t, "create private source diary", err)
	mediaRepository := store.NewMediaRepository(database)
	media := createReadyCommunityMedia(t, ctx, mediaRepository, author.User.ID, "10000000-0000-4000-8000-000000000090")

	title, body := "First Look", "A deliberately public outfit note."
	postID := "10000000-0000-4000-8000-000000000101"
	created, err := community.CreatePost(ctx, author.Token, postID, communityapp.PostContentInput{Title: &title, Body: &body, SourceDiaryID: &diaryID, MediaIDs: []string{media.ID}, Tags: []string{"Street", "OOTD"}})
	serviceOK(t, "create community draft", err)
	if created.State != communityapp.PostDraft || created.Revision != 1 || created.SourceDiaryID == nil || *created.SourceDiaryID != diaryID {
		t.Fatalf("unexpected draft: %+v", created)
	}
	repeated, err := community.CreatePost(ctx, author.Token, postID, communityapp.PostContentInput{Title: &title, Body: &body, SourceDiaryID: &diaryID, MediaIDs: []string{media.ID}, Tags: []string{"ootd", "street"}})
	serviceOK(t, "repeat idempotent community draft", err)
	if repeated.Revision != created.Revision {
		t.Fatal("idempotent post create changed revision")
	}
	changed := "different"
	if _, err := community.CreatePost(ctx, author.Token, postID, communityapp.PostContentInput{Body: &changed}); !errors.Is(err, communityapp.ErrPostConflict) {
		t.Fatalf("same post ID accepted different content: %v", err)
	}
	if _, err := community.GetOwnPost(ctx, reporter.Token, postID); !errors.Is(err, communityapp.ErrPostNotFound) {
		t.Fatal("cross-account private post read was not hidden")
	}
	if _, err := community.CreatePost(ctx, reporter.Token, "10000000-0000-4000-8000-000000000102", communityapp.PostContentInput{Body: &body, SourceDiaryID: &diaryID}); !errors.Is(err, communityapp.ErrPostNotFound) {
		t.Fatal("cross-account source diary import was not rejected")
	}

	pending, err := community.SubmitPost(ctx, author.Token, postID, created.Revision)
	serviceOK(t, "submit community post", err)
	if pending.State != communityapp.PostPending || pending.PendingVersion == nil || pending.Current.ReviewState != communityapp.ReviewPending {
		t.Fatalf("post did not enter pending state: %+v", pending)
	}
	if _, err := community.ListModerationCandidates(ctx, reporter.Token, 20, ""); !errors.Is(err, communityapp.ErrCommunityForbidden) {
		t.Fatal("ordinary user could read moderation queue")
	}
	queue, err := community.ListModerationCandidates(ctx, moderator.Token, 20, "")
	serviceOK(t, "list community moderation queue", err)
	if len(queue.Candidates) != 1 || queue.Candidates[0].PostID != postID || queue.Candidates[0].ImageCount != 1 {
		t.Fatalf("unexpected moderation queue: %+v", queue)
	}
	approved, err := community.DecidePost(ctx, moderator.Token, postID, communityapp.DecidePostInput{Version: *pending.PendingVersion, ReviewRound: pending.ReviewRound, ExpectedRevision: pending.Revision, Decision: communityapp.ModerationApprove, ReasonCode: "content_approved"})
	serviceOK(t, "approve community post", err)
	if approved.State != communityapp.PostPublished || approved.PublishedVersion == nil {
		t.Fatalf("post did not publish: %+v", approved)
	}
	public, err := community.GetPublicPostForViewer(ctx, "", postID)
	serviceOK(t, "read public community post", err)
	if public.Body == nil || *public.Body != body || public.ImageCount != 1 || public.AuthorHandle != "community_author" {
		t.Fatalf("public DTO lost or leaked facts: %+v", public)
	}
	impact, err := diaries.DeletionImpact(ctx, author.Token, diaryID)
	serviceOK(t, "read source diary deletion impact", err)
	if impact.PublishedPostCount != 1 {
		t.Fatalf("published source post count=%d", impact.PublishedPostCount)
	}
	serviceOK(t, "delete private source diary", diaries.Delete(ctx, author.Token, diaryID, sourceDiary.Revision))
	afterDiaryDelete, err := community.GetOwnPost(ctx, author.Token, postID)
	serviceOK(t, "read post after source diary deletion", err)
	if afterDiaryDelete.SourceDiaryID != nil {
		t.Fatal("deleted private source ID remained attached to independent post")
	}
	image, err := community.GetPublicPostImageForViewer(ctx, "", postID, 0)
	serviceOK(t, "read public community image reference", err)
	if image.ObjectKey == "" || image.ObjectVersionID == "" {
		t.Fatal("public image was not pinned to a derived object version")
	}
	if _, err := mediaRepository.DeleteMedia(ctx, author.User.ID, media.ID, time.Now().UTC()); !errors.Is(err, mediaapp.ErrMediaConflict) {
		t.Fatalf("referenced community media deletion error=%v", err)
	}

	feed, err := community.ListFeed(ctx, "", "discover", 20, "")
	if err != nil {
		t.Fatalf("read anonymous discovery feed: %v", err)
	}
	if len(feed.Posts) != 1 || feed.Posts[0].ID != postID {
		t.Fatalf("published post missing from discovery: %+v", feed)
	}
	viewerPost, err := community.GetPublicPostForViewer(ctx, "", postID)
	serviceOK(t, "read anonymous viewer-aware post", err)
	if viewerPost.ID != postID {
		t.Fatal("viewer-aware public post mismatch")
	}
	serviceOK(t, "like published post", community.SetPostLike(ctx, reporter.Token, postID, true))
	serviceOK(t, "repeat published post like", community.SetPostLike(ctx, reporter.Token, postID, true))
	serviceOK(t, "bookmark published post", community.SetPostBookmark(ctx, reporter.Token, postID, true))
	serviceOK(t, "follow published author", community.SetFollow(ctx, reporter.Token, "community_author", true))
	following, err := community.ListFeed(ctx, reporter.Token, "following", 20, "")
	serviceOK(t, "read following feed", err)
	if len(following.Posts) != 1 || !following.Posts[0].ViewerLiked || !following.Posts[0].ViewerBookmarked || !following.Posts[0].FollowingAuthor {
		t.Fatalf("viewer relations missing from following feed: %+v", following)
	}
	commentID := "10000000-0000-4000-8000-000000000301"
	comment, err := community.CreateComment(ctx, reporter.Token, commentID, postID, communityapp.CreateCommentInput{Body: "  Looks great.  "})
	serviceOK(t, "create pending comment", err)
	if comment.State != communityapp.CommentPending || comment.Body == nil || *comment.Body != "Looks great." {
		t.Fatalf("comment was not normalized and queued: %+v", comment)
	}
	commentQueue, err := community.ListCommentModeration(ctx, moderator.Token, 20, "")
	serviceOK(t, "read comment moderation queue", err)
	if len(commentQueue.Comments) != 1 || commentQueue.Comments[0].ID != commentID {
		t.Fatalf("unexpected comment queue: %+v", commentQueue)
	}
	comment, err = community.DecideComment(ctx, moderator.Token, commentID, communityapp.DecideCommentInput{ExpectedRevision: comment.Revision, Decision: communityapp.ModerationApprove, ReasonCode: "content_approved"})
	serviceOK(t, "approve comment", err)
	comments, err := community.ListComments(ctx, "", postID, nil, 20, "")
	serviceOK(t, "read public comments", err)
	if len(comments.Comments) != 1 || comments.Comments[0].State != communityapp.CommentPublished {
		t.Fatalf("approved comment not public: %+v", comments)
	}
	events, err := mediaRepository.PendingOutbox(ctx, 100)
	serviceOK(t, "read notification outbox", err)
	notificationEvents := 0
	for _, event := range events {
		if event.EventType != "community.notification_requested" {
			continue
		}
		notificationEvents++
		serviceOK(t, "deliver community notification", mediaRepository.DeliverNotification(ctx, event.ID, event.AggregateID, time.Now().UTC()))
		serviceOK(t, "repeat community notification delivery", mediaRepository.DeliverNotification(ctx, event.ID, event.AggregateID, time.Now().UTC()))
	}
	if notificationEvents < 3 {
		t.Fatalf("notification outbox events=%d", notificationEvents)
	}
	notifications, err := community.ListNotifications(ctx, author.Token, 20, "")
	serviceOK(t, "read delivered notifications", err)
	if len(notifications.Notifications) < 3 {
		t.Fatalf("author notifications=%d", len(notifications.Notifications))
	}
	readNotification, err := community.MarkNotificationRead(ctx, author.Token, notifications.Notifications[0].ID)
	serviceOK(t, "mark notification read", err)
	if readNotification.ReadAt == nil {
		t.Fatal("notification was not marked read")
	}
	serviceOK(t, "block post author", community.SetBlock(ctx, reporter.Token, author.User.ID, true))
	blockedFeed, err := community.ListFeed(ctx, reporter.Token, "discover", 20, "")
	serviceOK(t, "read blocked discovery feed", err)
	if len(blockedFeed.Posts) != 0 {
		t.Fatal("blocked author remained in personalized discovery")
	}
	if err := community.SetPostLike(ctx, reporter.Token, postID, true); !errors.Is(err, communityapp.ErrPostNotFound) {
		t.Fatalf("blocked interaction error=%v", err)
	}
	serviceOK(t, "unblock post author", community.SetBlock(ctx, reporter.Token, author.User.ID, false))

	newBody := "A revised outfit note awaiting review."
	edited, err := community.UpdatePost(ctx, author.Token, postID, approved.Revision, communityapp.PostContentInput{Title: &title, Body: &newBody, MediaIDs: []string{media.ID}, Tags: []string{"ootd"}})
	serviceOK(t, "edit published community post", err)
	secondPending, err := community.SubmitPost(ctx, author.Token, postID, edited.Revision)
	serviceOK(t, "submit edited community post", err)
	stillPublic, err := community.GetPublicPostForViewer(ctx, "", postID)
	serviceOK(t, "read old public version during review", err)
	if stillPublic.Body == nil || *stillPublic.Body != body {
		t.Fatal("pending edit replaced approved public content")
	}
	rejected, err := community.DecidePost(ctx, moderator.Token, postID, communityapp.DecidePostInput{Version: *secondPending.PendingVersion, ReviewRound: secondPending.ReviewRound, ExpectedRevision: secondPending.Revision, Decision: communityapp.ModerationReject, ReasonCode: "needs_revision"})
	serviceOK(t, "reject edited community post", err)
	if rejected.State != communityapp.PostPublished || rejected.Current.ReviewState != communityapp.ReviewRejected {
		t.Fatalf("rejection lost public version or private reason: %+v", rejected)
	}
	if _, err := community.DecidePost(ctx, moderator.Token, postID, communityapp.DecidePostInput{Version: *secondPending.PendingVersion, ReviewRound: secondPending.ReviewRound, ExpectedRevision: secondPending.Revision, Decision: communityapp.ModerationApprove, ReasonCode: "content_approved"}); !errors.Is(err, communityapp.ErrPostConflict) {
		t.Fatal("different late moderation decision revived rejected version")
	}

	reportID := "10000000-0000-4000-8000-000000000201"
	report, err := community.CreateReport(ctx, reporter.Token, reportID, postID, communityapp.CreateReportInput{ReasonCode: "other"})
	serviceOK(t, "create community report", err)
	if report.Status != communityapp.ReportOpen {
		t.Fatal("new report was not open")
	}
	resolved, err := community.ResolveReport(ctx, moderator.Token, reportID, "resolved", "reviewed_no_action")
	serviceOK(t, "resolve community report", err)
	if resolved.Status != communityapp.ReportResolved {
		t.Fatal("report did not resolve")
	}
	if _, err := community.GetPublicPostForViewer(ctx, "", postID); err != nil {
		t.Fatal("resolving report implicitly removed post")
	}
	serviceOK(t, "remove reported post", community.RemovePost(ctx, moderator.Token, postID, rejected.Revision, "policy_violation"))
	actionsBeforeAppeal, err := community.ListModerationActions(ctx, moderator.Token, 20, "")
	serviceOK(t, "find removable moderation action", err)
	var removalActionID string
	for _, action := range actionsBeforeAppeal.Actions {
		if action.Action == "post_remove" && action.PostID != nil && *action.PostID == postID {
			removalActionID = action.ID
			break
		}
	}
	if removalActionID == "" {
		t.Fatal("post removal action missing")
	}
	appeal, err := community.CreateAppeal(ctx, author.Token, "10000000-0000-4000-8000-000000000401", removalActionID, "Please review the removal.")
	serviceOK(t, "create moderation appeal", err)
	appeal, err = community.ResolveAppeal(ctx, admin.Token, appeal.ID, "reversed", "appeal_approved")
	serviceOK(t, "resolve moderation appeal", err)
	if appeal.Status != communityapp.AppealReversed {
		t.Fatalf("appeal status=%s", appeal.Status)
	}
	if _, err := community.GetPublicPostForViewer(ctx, "", postID); !errors.Is(err, communityapp.ErrPostNotFound) {
		t.Fatal("appeal resolution bypassed the removed post state")
	}
	removed, err := community.GetOwnPost(ctx, author.Token, postID)
	serviceOK(t, "read removed post as owner", err)
	serviceOK(t, "delete removed post", community.DeletePost(ctx, author.Token, postID, removed.Revision))
	serviceOK(t, "repeat deleted post request", community.DeletePost(ctx, author.Token, postID, removed.Revision))
	if _, err := mediaRepository.DeleteMedia(ctx, author.User.ID, media.ID, time.Now().UTC()); err != nil {
		t.Fatalf("media remained referenced after post content deletion: %v", err)
	}

	withdrawnPostID := "10000000-0000-4000-8000-000000000103"
	withdrawBody := "withdraw before review"
	withdrawDraft, err := community.CreatePost(ctx, author.Token, withdrawnPostID, communityapp.PostContentInput{Body: &withdrawBody})
	serviceOK(t, "create withdrawable post", err)
	if _, err := community.WithdrawPost(ctx, author.Token, withdrawnPostID, withdrawDraft.Revision); !errors.Is(err, communityapp.ErrPostConflict) {
		t.Fatal("private draft was incorrectly withdrawable")
	}
	if err := community.RemovePost(ctx, moderator.Token, withdrawnPostID, withdrawDraft.Revision, "policy_violation"); !errors.Is(err, communityapp.ErrPostConflict) {
		t.Fatal("private draft was incorrectly removable by moderation")
	}
	withdrawPending, err := community.SubmitPost(ctx, author.Token, withdrawnPostID, withdrawDraft.Revision)
	serviceOK(t, "submit withdrawable post", err)
	withdrawn, err := community.WithdrawPost(ctx, author.Token, withdrawnPostID, withdrawPending.Revision)
	serviceOK(t, "withdraw pending post", err)
	if withdrawn.State != communityapp.PostWithdrawn || withdrawn.Current.ReviewState != communityapp.ReviewDraft {
		t.Fatalf("withdraw did not create a new private draft: %+v", withdrawn)
	}
	if _, err := community.DecidePost(ctx, moderator.Token, withdrawnPostID, communityapp.DecidePostInput{Version: *withdrawPending.PendingVersion, ReviewRound: withdrawPending.ReviewRound, ExpectedRevision: withdrawPending.Revision, Decision: communityapp.ModerationApprove, ReasonCode: "content_approved"}); !errors.Is(err, communityapp.ErrPostConflict) {
		t.Fatal("late approval revived withdrawn post")
	}
	if _, err := community.GetPublicPostForViewer(ctx, "", withdrawnPostID); !errors.Is(err, communityapp.ErrPostNotFound) {
		t.Fatal("withdrawn post was publicly readable")
	}

	users, err := community.ListUsers(ctx, admin.Token, 20, "")
	serviceOK(t, "list users as admin", err)
	if len(users.Users) != 4 {
		t.Fatalf("admin user list length=%d", len(users.Users))
	}
	suspended, err := community.SetUserStatus(ctx, admin.Token, author.User.ID, author.User.Revision, accountapp.AccountSuspended, "policy_violation")
	serviceOK(t, "suspend community author", err)
	if suspended.Status != accountapp.AccountSuspended {
		t.Fatal("author was not suspended")
	}
	if _, err := accounts.CurrentUser(ctx, author.Token); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("suspension did not revoke author session")
	}
	restored, err := community.SetUserStatus(ctx, admin.Token, author.User.ID, suspended.Revision, accountapp.AccountActive, "appeal_approved")
	if err != nil {
		t.Fatalf("restore community author: %v", err)
	}
	if restored.Status != accountapp.AccountActive {
		t.Fatal("author was not restored")
	}
	actions, err := community.ListModerationActions(ctx, moderator.Token, 20, "")
	serviceOK(t, "list moderation actions", err)
	if len(actions.Actions) < 5 {
		t.Fatalf("governance audit trail incomplete: %+v", actions)
	}
}

func createReadyCommunityMedia(t *testing.T, ctx context.Context, repository *store.MediaRepository, ownerID, eventID string) mediaapp.MediaAsset {
	t.Helper()
	now := time.Now().UTC()
	asset, err := repository.CreateMedia(ctx, ownerID, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeCommunityPublish, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, now)
	serviceOK(t, "create community media", err)
	asset, err = repository.CompleteMedia(ctx, ownerID, asset.ID, "source-version", mediaapp.ObjectFact{ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, now)
	serviceOK(t, "complete community media", err)
	asset, process, err := repository.BeginMediaCheck(ctx, asset.ID, now)
	serviceOK(t, "begin community media check", err)
	if !process {
		t.Fatal("community media was not queued for normalization")
	}
	serviceOK(t, "finish community media check", repository.CompleteMediaCheck(ctx, eventID, asset.ID, &mediaapp.MediaDerivation{ObjectKey: "media/" + asset.ID + "/normalized.jpg", ObjectVersionID: "derived-version"}, 32, 32, mediaapp.MediaReady, "ready", now))
	asset.Status = mediaapp.MediaReady
	return asset
}

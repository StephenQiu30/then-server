//go:build services

package services

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type capturedAccountMail struct {
	to   string
	body string
}

type accountMailSender struct {
	messages []capturedAccountMail
	failNext bool
}

func (s *accountMailSender) Send(_ context.Context, to, _, body string) error {
	if s.failNext {
		s.failNext = false
		return errors.New("synthetic delivery failure")
	}
	s.messages = append(s.messages, capturedAccountMail{to: to, body: body})
	return nil
}

func accountMailToken(t *testing.T, message capturedAccountMail) string {
	t.Helper()
	for _, field := range strings.Fields(message.body) {
		if strings.HasPrefix(field, "https://then.example/") {
			link, err := url.Parse(field)
			serviceOK(t, "parse account mail link", err)
			return link.Query().Get("token")
		}
	}
	t.Fatal("mail did not contain an account challenge link")
	return ""
}

func TestAccountMailPersistenceAndSingleUse(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open account mail database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create account mail schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("account mail schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse account mail database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated account mail schema", err)
	serviceOK(t, "migrate account mail schema", store.Migrate(ctx, database))
	repository := store.NewAccountRepository(database)
	accounts, err := accountapp.NewAccountService(repository)
	serviceOK(t, "construct account service", err)
	sender := &accountMailSender{}
	mail, err := accountapp.NewMailService(accounts, repository, sender, []byte(strings.Repeat("k", 32)), "https://then.example/account/mail")
	serviceOK(t, "construct account mail service", err)
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "first@example.test", DisplayName: "First", Password: "first-password-2026"})
	serviceOK(t, "register first mail account", err)
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "second@example.test", DisplayName: "Second", Password: "second-password-2026"})
	serviceOK(t, "register second mail account", err)
	if first.User.EmailVerified || second.User.EmailVerified {
		t.Fatal("new accounts must begin with an unverified email")
	}
	serviceOK(t, "request email verification", mail.RequestVerification(ctx, first.Token))
	serviceOK(t, "deliver email verification", mail.DeliverOnce(ctx))
	if len(sender.messages) != 1 || sender.messages[0].to != first.User.Email {
		t.Fatal("verification mail was not delivered to the current email")
	}
	verificationToken := accountMailToken(t, sender.messages[0])
	if err := mail.ConfirmVerification(ctx, second.Token, verificationToken); !errors.Is(err, accountapp.ErrInvalidChallenge) {
		t.Fatal("second account consumed first account's challenge")
	}
	serviceOK(t, "confirm email verification", mail.ConfirmVerification(ctx, first.Token, verificationToken))
	if err := mail.ConfirmVerification(ctx, first.Token, verificationToken); !errors.Is(err, accountapp.ErrInvalidChallenge) {
		t.Fatal("verification challenge replay was accepted")
	}
	verified, err := accounts.CurrentUser(ctx, first.Token)
	serviceOK(t, "read verified account", err)
	if !verified.EmailVerified || verified.Revision != first.User.Revision+1 {
		t.Fatal("email verification status or revision was not persisted")
	}
	serviceOK(t, "request reset for unknown email", mail.RequestPasswordReset(ctx, "unknown@example.test"))
	serviceOK(t, "request reset for known email", mail.RequestPasswordReset(ctx, first.User.Email))
	serviceOK(t, "deliver password reset", mail.DeliverOnce(ctx))
	if len(sender.messages) != 2 || sender.messages[1].to != first.User.Email {
		t.Fatal("unknown account produced mail or known account did not")
	}
	resetToken := accountMailToken(t, sender.messages[1])
	if err := mail.ConfirmPasswordReset(ctx, resetToken+"x", "new-password-2026"); !errors.Is(err, accountapp.ErrInvalidChallenge) {
		t.Fatal("modified reset token was accepted")
	}
	const contenders = 4
	results := make(chan error, contenders)
	var group sync.WaitGroup
	for range contenders {
		group.Add(1)
		go func() {
			results <- mail.ConfirmPasswordReset(ctx, resetToken, "new-password-2026")
			group.Done()
		}()
	}
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, accountapp.ErrInvalidChallenge) {
			t.Fatal("concurrent reset returned an unexpected error")
		}
	}
	if successes != 1 {
		t.Fatalf("expected one successful reset, got %d", successes)
	}
	if _, err := accounts.CurrentUser(ctx, first.Token); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("password reset retained an old session")
	}
	if _, err := accounts.Login(ctx, accountapp.CreateSessionInput{Email: first.User.Email, Password: "first-password-2026"}); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("old password remained valid")
	}
	loggedIn, err := accounts.Login(ctx, accountapp.CreateSessionInput{Email: first.User.Email, Password: "new-password-2026"})
	serviceOK(t, "login with reset password", err)
	newEmail := "updated@example.test"
	updated, err := accounts.UpdateCurrentUser(ctx, loggedIn.Token, accountapp.UpdateCurrentUserInput{Email: &newEmail, ExpectedRevision: verified.Revision + 1})
	serviceOK(t, "change verified email", err)
	if updated.EmailVerified {
		t.Fatal("email change retained verification")
	}
	serviceOK(t, "request updated email verification", mail.RequestVerification(ctx, loggedIn.Token))
	serviceOK(t, "deliver updated email verification", mail.DeliverOnce(ctx))
	updatedToken := accountMailToken(t, sender.messages[2])
	newerEmail := "newest@example.test"
	_, err = accounts.UpdateCurrentUser(ctx, loggedIn.Token, accountapp.UpdateCurrentUserInput{Email: &newerEmail, ExpectedRevision: updated.Revision})
	serviceOK(t, "change email after challenge", err)
	if err := mail.ConfirmVerification(ctx, loggedIn.Token, updatedToken); !errors.Is(err, accountapp.ErrInvalidChallenge) {
		t.Fatal("old email challenge survived email change")
	}
	sender.failNext = true
	serviceOK(t, "request retryable verification", mail.RequestVerification(ctx, second.Token))
	serviceOK(t, "record failed synthetic delivery", mail.DeliverOnce(ctx))
	if len(sender.messages) != 3 {
		t.Fatal("failed mail delivery appeared as delivered")
	}
	serviceOK(t, "release retry backoff in isolated fixture", database.WithContext(ctx).Exec(
		"UPDATE account_mail_challenges SET next_attempt_at = now() - interval '1 second' WHERE user_id = ? AND purpose = 'verify_email' AND sent_at IS NULL", second.User.ID).Error)
	serviceOK(t, "retry synthetic delivery", mail.DeliverOnce(ctx))
	if len(sender.messages) != 4 || sender.messages[3].to != second.User.Email {
		t.Fatal("failed mail did not retry to the intended recipient")
	}
	secondToken := accountMailToken(t, sender.messages[3])
	challengeID, _, _ := strings.Cut(secondToken, ".")
	if err := repository.ConfirmVerification(ctx, challengeID, second.User.ID, second.User.Email, time.Now().UTC().Add(time.Hour)); !errors.Is(err, accountapp.ErrInvalidChallenge) {
		t.Fatal("expired verification challenge was accepted")
	}
	serviceOK(t, "request replacement verification", mail.RequestVerification(ctx, second.Token))
	if err := mail.ConfirmVerification(ctx, second.Token, secondToken); !errors.Is(err, accountapp.ErrInvalidChallenge) {
		t.Fatal("replacement request did not revoke an older challenge")
	}
	var requests sync.WaitGroup
	for range 4 {
		requests.Add(1)
		go func() {
			if err := mail.RequestVerification(ctx, second.Token); err != nil {
				t.Error("concurrent verification request failed")
			}
			requests.Done()
		}()
	}
	requests.Wait()
	var activeChallenges int64
	serviceOK(t, "count active mail challenges", database.WithContext(ctx).Raw(
		"SELECT count(*) FROM account_mail_challenges WHERE user_id = ? AND purpose = 'verify_email' AND revoked_at IS NULL AND consumed_at IS NULL", second.User.ID).Scan(&activeChallenges).Error)
	if activeChallenges != 1 {
		t.Fatalf("concurrent requests left %d active challenges", activeChallenges)
	}
	_, err = accounts.DeleteCurrentUser(ctx, loggedIn.Token)
	serviceOK(t, "delete account with mail challenges", err)
	var remaining int64
	serviceOK(t, "count deleted account challenges", database.WithContext(ctx).Raw("SELECT count(*) FROM account_mail_challenges WHERE user_id = ?", first.User.ID).Scan(&remaining).Error)
	if remaining != 0 {
		t.Fatal("account deletion retained mail challenges")
	}
}

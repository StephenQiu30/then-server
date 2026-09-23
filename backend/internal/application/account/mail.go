package account

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type ChallengePurpose string

const (
	VerifyEmail   ChallengePurpose = "verify_email"
	ResetPassword ChallengePurpose = "reset_password"
)

type MailChallenge struct {
	ID         string
	UserID     string
	Email      string
	Purpose    ChallengePurpose
	ExpiresAt  time.Time
	LeaseUntil time.Time
	Attempts   int
}

type MailRepository interface {
	CreateVerificationChallenge(context.Context, MailChallenge, time.Time) error
	CreateResetChallenge(context.Context, MailChallenge, time.Time) error
	FindChallenge(context.Context, string) (MailChallenge, error)
	ConfirmVerification(context.Context, string, string, string, time.Time) error
	ConfirmReset(context.Context, string, string, string, time.Time) error
	ClaimDueChallenges(context.Context, time.Time, int) ([]MailChallenge, error)
	MarkChallengeSent(context.Context, string, time.Time, time.Time) error
	RetryChallenge(context.Context, string, time.Time, time.Time, time.Time) error
	PurgeOldChallenges(context.Context, time.Time) error
}

type MailSender interface {
	Send(context.Context, string, string, string) error
}

type MailService struct {
	accounts   *AccountService
	repository MailRepository
	sender     MailSender
	key        []byte
	linkBase   *url.URL
	now        func() time.Time
}

func NewMailService(accounts *AccountService, repository MailRepository, sender MailSender, key []byte, linkBase string) (*MailService, error) {
	base, err := url.Parse(linkBase)
	if accounts == nil || repository == nil || sender == nil || len(key) < 32 || err != nil || base.Scheme != "https" || base.Host == "" || base.Fragment != "" || base.User != nil {
		return nil, ErrAccountUnavailable
	}
	return &MailService{accounts: accounts, repository: repository, sender: sender, key: append([]byte(nil), key...), linkBase: base, now: time.Now}, nil
}

func (s *MailService) RequestVerification(ctx context.Context, session string) error {
	user, err := s.accounts.CurrentUser(ctx, session)
	if err != nil {
		return err
	}
	if user.EmailVerified {
		return nil
	}
	now := s.now().UTC()
	return s.repository.CreateVerificationChallenge(ctx, MailChallenge{
		ID: uuid.NewString(), UserID: user.ID, Email: user.Email, Purpose: VerifyEmail, ExpiresAt: now.Add(30 * time.Minute),
	}, now)
}

func (s *MailService) RequestPasswordReset(ctx context.Context, rawEmail string) error {
	email, valid := normalizeEmail(rawEmail)
	if !valid {
		return ErrInvalidAccountInput
	}
	now := s.now().UTC()
	return s.repository.CreateResetChallenge(ctx, MailChallenge{
		ID: uuid.NewString(), Email: email, Purpose: ResetPassword, ExpiresAt: now.Add(15 * time.Minute),
	}, now)
}

func (s *MailService) ConfirmVerification(ctx context.Context, session, token string) error {
	user, err := s.accounts.CurrentUser(ctx, session)
	if err != nil {
		return err
	}
	challenge, err := s.verifyToken(ctx, token, VerifyEmail)
	if err != nil {
		return err
	}
	if challenge.UserID != user.ID || challenge.Email != user.Email {
		return ErrInvalidChallenge
	}
	return s.repository.ConfirmVerification(ctx, challenge.ID, user.ID, user.Email, s.now().UTC())
}

func (s *MailService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if !validNewPassword(newPassword) {
		return ErrInvalidAccountInput
	}
	challenge, err := s.verifyToken(ctx, token, ResetPassword)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.accounts.passwordCost)
	if err != nil {
		return ErrAccountUnavailable
	}
	return s.repository.ConfirmReset(ctx, challenge.ID, challenge.Email, string(hash), s.now().UTC())
}

func (s *MailService) verifyToken(ctx context.Context, token string, purpose ChallengePurpose) (MailChallenge, error) {
	id, encoded, ok := strings.Cut(token, ".")
	if !ok || uuid.Validate(id) != nil || len(encoded) != 43 {
		return MailChallenge{}, ErrInvalidChallenge
	}
	actual, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(actual) != sha256.Size {
		return MailChallenge{}, ErrInvalidChallenge
	}
	challenge, err := s.repository.FindChallenge(ctx, id)
	if err != nil {
		return MailChallenge{}, err
	}
	if challenge.Purpose != purpose || !hmac.Equal(actual, s.signature(challenge)) {
		return MailChallenge{}, ErrInvalidChallenge
	}
	return challenge, nil
}

func (s *MailService) signature(challenge MailChallenge) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(challenge.ID + "\n" + string(challenge.Purpose) + "\n" + challenge.Email + "\n" + challenge.ExpiresAt.UTC().Format(time.RFC3339Nano)))
	return mac.Sum(nil)
}

func (s *MailService) deliveryToken(challenge MailChallenge) string {
	return challenge.ID + "." + base64.RawURLEncoding.EncodeToString(s.signature(challenge))
}

func (s *MailService) DeliverOnce(ctx context.Context) error {
	now := s.now().UTC()
	challenges, err := s.repository.ClaimDueChallenges(ctx, now, 8)
	if err != nil {
		return err
	}
	for _, challenge := range challenges {
		link := *s.linkBase
		query := link.Query()
		query.Set("token", s.deliveryToken(challenge))
		link.RawQuery = query.Encode()
		subject := "于是 OOTD：验证邮箱"
		body := "请在 30 分钟内打开以下链接验证邮箱：\n" + link.String() + "\n如果这不是您的操作，请忽略此邮件。\n"
		if challenge.Purpose == ResetPassword {
			subject = "于是 OOTD：重置密码"
			body = "请在 15 分钟内打开以下链接重置密码：\n" + link.String() + "\n如果这不是您的操作，请忽略此邮件。\n"
		}
		work, cancel := context.WithTimeout(ctx, 10*time.Second)
		sendErr := s.sender.Send(work, challenge.Email, subject, body)
		cancel()
		at := s.now().UTC()
		if sendErr == nil {
			err = s.repository.MarkChallengeSent(ctx, challenge.ID, challenge.LeaseUntil, at)
		} else {
			backoff := time.Minute << min(challenge.Attempts-1, 3)
			err = s.repository.RetryChallenge(ctx, challenge.ID, challenge.LeaseUntil, at, at.Add(backoff))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MailService) Run(ctx context.Context) error {
	if err := s.repository.PurgeOldChallenges(ctx, s.now().UTC().Add(-7*24*time.Hour)); err != nil {
		return err
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	purgeTicker := time.NewTicker(time.Hour)
	defer purgeTicker.Stop()
	for {
		if err := s.DeliverOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-purgeTicker.C:
			if err := s.repository.PurgeOldChallenges(ctx, s.now().UTC().Add(-7*24*time.Hour)); err != nil {
				return err
			}
		}
	}
}

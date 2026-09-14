package service

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type accountRepositoryStub struct {
	createdUser     model.User
	createdHash     string
	createdSession  model.Session
	credential      model.Credential
	credentialError error
	sessionUser     model.User
	sessionError    error
	updatedEmail    *string
	updatedDisplay  *string
	deletedSession  []byte
	deletedUser     string
}

func (r *accountRepositoryStub) CreateAccount(_ context.Context, user model.User, hash string, session model.Session) (model.User, error) {
	r.createdUser, r.createdHash, r.createdSession = user, hash, session
	return user, nil
}

func (r *accountRepositoryStub) FindCredentialByEmail(context.Context, string) (model.Credential, error) {
	return r.credential, r.credentialError
}

func (r *accountRepositoryStub) CreateSession(_ context.Context, session model.Session) error {
	r.createdSession = session
	return nil
}

func (r *accountRepositoryStub) FindUserBySession(_ context.Context, _ []byte, _ time.Time) (model.User, error) {
	return r.sessionUser, r.sessionError
}

func (r *accountRepositoryStub) UpdateUser(_ context.Context, _ string, email, displayName *string, now time.Time) (model.User, error) {
	r.updatedEmail, r.updatedDisplay = email, displayName
	user := r.sessionUser
	if email != nil {
		user.Email = *email
	}
	if displayName != nil {
		user.DisplayName = *displayName
	}
	user.UpdatedAt = now
	return user, nil
}

func (r *accountRepositoryStub) DeleteSession(_ context.Context, hash []byte) error {
	r.deletedSession = hash
	return nil
}

func (r *accountRepositoryStub) DeleteUser(_ context.Context, userID string) error {
	r.deletedUser = userID
	return nil
}

func newAccountServiceForTest(t *testing.T, repository *accountRepositoryStub) *AccountService {
	t.Helper()
	dummyPasswordHash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-account-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &AccountService{
		repository:        repository,
		now:               func() time.Time { return time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC) },
		random:            bytes.NewReader(bytes.Repeat([]byte{0x2a}, tokenBytes)),
		passwordCost:      bcrypt.MinCost,
		dummyPasswordHash: dummyPasswordHash,
	}
}

func TestRegisterNormalizesAndProtectsCredentials(t *testing.T) {
	repository := new(accountRepositoryStub)
	service := newAccountServiceForTest(t, repository)
	result, err := service.Register(context.Background(), model.RegisterAccountInput{
		Email: "  PERSON@Example.Test ", DisplayName: "  示例用户  ", Password: "correct-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Email != "person@example.test" || result.User.DisplayName != "示例用户" {
		t.Fatal("account input was not normalized")
	}
	if repository.createdHash == "correct-password" || bcrypt.CompareHashAndPassword([]byte(repository.createdHash), []byte("correct-password")) != nil {
		t.Fatal("password was not stored as a verifiable bcrypt hash")
	}
	if result.Token == "" || bytes.Equal([]byte(result.Token), repository.createdSession.TokenHash) || len(repository.createdSession.TokenHash) != 32 {
		t.Fatal("session token was not separated from its stored hash")
	}
	if !repository.createdSession.ExpiresAt.Equal(repository.createdSession.CreatedAt.Add(sessionTTL)) {
		t.Fatal("session TTL differs from the contract")
	}
}

func TestRegisterRejectsInvalidInputs(t *testing.T) {
	for _, input := range []model.RegisterAccountInput{
		{Email: "invalid", DisplayName: "Name", Password: "correct-password"},
		{Email: "person@example.test", DisplayName: " ", Password: "correct-password"},
		{Email: "person@example.test", DisplayName: "Name", Password: "short"},
	} {
		service := newAccountServiceForTest(t, new(accountRepositoryStub))
		if _, err := service.Register(context.Background(), input); !errors.Is(err, model.ErrInvalidAccountInput) {
			t.Fatal("invalid registration was accepted")
		}
	}
}

func TestLoginDoesNotRevealAccountExistence(t *testing.T) {
	wrongHash, err := bcrypt.GenerateFromPassword([]byte("different-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	for _, repository := range []*accountRepositoryStub{
		{credentialError: model.ErrAuthentication},
		{credential: model.Credential{PasswordHash: string(wrongHash)}},
	} {
		service := newAccountServiceForTest(t, repository)
		if _, err := service.Login(context.Background(), model.CreateSessionInput{Email: "person@example.test", Password: "correct-password"}); !errors.Is(err, model.ErrAuthentication) {
			t.Fatal("invalid login did not use the stable authentication error")
		}
	}
}

func TestAuthenticatedUpdateLogoutAndDeleteUseSessionOwner(t *testing.T) {
	repository := &accountRepositoryStub{sessionUser: model.User{ID: "user-id", Email: "old@example.test", DisplayName: "Old"}}
	service := newAccountServiceForTest(t, repository)
	registered, err := service.Register(context.Background(), model.RegisterAccountInput{Email: "first@example.test", DisplayName: "First", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	email, name := " NEW@Example.Test ", " New Name "
	updated, err := service.UpdateCurrentUser(context.Background(), registered.Token, model.UpdateCurrentUserInput{Email: &email, DisplayName: &name})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Email != "new@example.test" || updated.DisplayName != "New Name" {
		t.Fatal("authenticated update did not normalize fields")
	}
	if err := service.Logout(context.Background(), registered.Token); err != nil || len(repository.deletedSession) != 32 {
		t.Fatal("logout did not delete the hashed current session")
	}
	if err := service.DeleteCurrentUser(context.Background(), registered.Token); err != nil || repository.deletedUser != "user-id" {
		t.Fatal("account deletion did not use the authenticated owner")
	}
}

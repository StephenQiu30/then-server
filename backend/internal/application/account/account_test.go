package account

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type accountRepositoryStub struct {
	createdUser     User
	createdHash     string
	createdSession  Session
	credential      Credential
	credentialError error
	sessionUser     User
	sessionError    error
	updatedEmail    *string
	updatedDisplay  *string
	updatedRevision int
	profile         PublicProfile
	profileError    error
	profileInput    PutProfileInput
	deletedSession  []byte
	deletedUser     string
	deletionHash    []byte
	deletion        AccountDeletionRequest
}

func (r *accountRepositoryStub) CreateAccount(_ context.Context, user User, hash string, session Session) (User, error) {
	r.createdUser, r.createdHash, r.createdSession = user, hash, session
	return user, nil
}

func (r *accountRepositoryStub) FindCredentialByEmail(context.Context, string) (Credential, error) {
	return r.credential, r.credentialError
}

func (r *accountRepositoryStub) CreateSession(_ context.Context, session Session) error {
	r.createdSession = session
	return nil
}

func (r *accountRepositoryStub) FindUserBySession(_ context.Context, _ []byte, _ time.Time) (User, error) {
	return r.sessionUser, r.sessionError
}

func (r *accountRepositoryStub) UpdateUser(_ context.Context, _ string, expectedRevision int, email, displayName *string, now time.Time) (User, error) {
	r.updatedEmail, r.updatedDisplay, r.updatedRevision = email, displayName, expectedRevision
	user := r.sessionUser
	if email != nil {
		user.Email = *email
	}
	if displayName != nil {
		user.DisplayName = *displayName
	}
	user.Revision++
	user.UpdatedAt = now
	return user, nil
}

func (r *accountRepositoryStub) FindProfileByUserID(context.Context, string) (PublicProfile, error) {
	return r.profile, r.profileError
}

func (r *accountRepositoryStub) FindProfileByHandle(context.Context, string) (PublicProfile, error) {
	return r.profile, r.profileError
}

func (r *accountRepositoryStub) PutProfile(_ context.Context, _ string, input PutProfileInput, _ time.Time) (PublicProfile, error) {
	r.profileInput = input
	return r.profile, r.profileError
}

func (r *accountRepositoryStub) PutProfileAvatar(context.Context, string, string, int, time.Time) (PublicProfile, error) {
	return r.profile, r.profileError
}

func (r *accountRepositoryStub) DeleteProfileAvatar(context.Context, string, int, time.Time) (PublicProfile, error) {
	return r.profile, r.profileError
}

func (r *accountRepositoryStub) FindProfileAvatar(context.Context, string) (ProfileAvatarReference, error) {
	return ProfileAvatarReference{}, r.profileError
}

func (r *accountRepositoryStub) DeleteSession(_ context.Context, hash []byte) error {
	r.deletedSession = hash
	return nil
}

func (r *accountRepositoryStub) ListSessions(context.Context, string, []byte, time.Time, int, int) (SessionPage, error) {
	return SessionPage{}, nil
}

func (r *accountRepositoryStub) RevokeSession(context.Context, []byte, string, time.Time) (bool, error) {
	return false, nil
}

func (r *accountRepositoryStub) BeginAccountDeletion(_ context.Context, userID string, hash []byte, _, _ time.Time) (AccountDeletionRequest, error) {
	r.deletedUser = userID
	r.deletionHash = hash
	return r.deletion, nil
}

func (r *accountRepositoryStub) GetDeletionReceipt(context.Context, string, []byte, time.Time) (AccountDeletionRequest, error) {
	return r.deletion, nil
}

func (r *accountRepositoryStub) RevokeDeletionReceipt(context.Context, string, []byte, time.Time) error {
	return nil
}
func (r *accountRepositoryStub) PurgeDeletionReceipts(context.Context, time.Time) error { return nil }

func newAccountServiceForTest(t *testing.T, repository *accountRepositoryStub) *AccountService {
	t.Helper()
	dummyPasswordHash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-account-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &AccountService{
		repository:        repository,
		now:               func() time.Time { return time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC) },
		random:            bytes.NewReader(bytes.Repeat([]byte{0x2a}, tokenBytes*4)),
		passwordCost:      bcrypt.MinCost,
		dummyPasswordHash: dummyPasswordHash,
	}
}

func TestReauthenticateRequiresCurrentSessionPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("current-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := User{ID: "owner", Email: "owner@example.test", Status: AccountActive}
	repository := &accountRepositoryStub{sessionUser: user, credential: Credential{User: user, PasswordHash: string(hash)}}
	service := newAccountServiceForTest(t, repository)
	token := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, tokenBytes))
	if _, err := service.Reauthenticate(context.Background(), token, "wrong-password"); !errors.Is(err, ErrAuthentication) {
		t.Fatal("wrong password accepted")
	}
	if _, err := service.Reauthenticate(context.Background(), "invalid", "current-password"); !errors.Is(err, ErrAuthentication) {
		t.Fatal("invalid session accepted")
	}
	result, err := service.Reauthenticate(context.Background(), token, "current-password")
	if err != nil || result.ID != user.ID || repository.createdSession.ID != "" {
		t.Fatal("reauthentication must not create a session")
	}
	repository.credential.User.ID = "other"
	if _, err := service.Reauthenticate(context.Background(), token, "current-password"); !errors.Is(err, ErrAuthentication) {
		t.Fatal("credential from another account accepted")
	}
}

func TestRegisterNormalizesAndProtectsCredentials(t *testing.T) {
	repository := new(accountRepositoryStub)
	service := newAccountServiceForTest(t, repository)
	result, err := service.Register(context.Background(), RegisterAccountInput{
		Email: "  PERSON@Example.Test ", DisplayName: "  示例用户  ", Password: "correct-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Email != "person@example.test" || result.User.DisplayName != "示例用户" {
		t.Fatal("account input was not normalized")
	}
	if result.User.Status != AccountActive || result.User.Role != AccountUser || result.User.Revision != 1 {
		t.Fatal("new account did not receive the safe default state")
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
	for _, input := range []RegisterAccountInput{
		{Email: "invalid", DisplayName: "Name", Password: "correct-password"},
		{Email: "person@example.test", DisplayName: " ", Password: "correct-password"},
		{Email: "person@example.test", DisplayName: "Name", Password: "short"},
	} {
		service := newAccountServiceForTest(t, new(accountRepositoryStub))
		if _, err := service.Register(context.Background(), input); !errors.Is(err, ErrInvalidAccountInput) {
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
		{credentialError: ErrAuthentication},
		{credential: Credential{PasswordHash: string(wrongHash)}},
	} {
		service := newAccountServiceForTest(t, repository)
		if _, err := service.Login(context.Background(), CreateSessionInput{Email: "person@example.test", Password: "correct-password"}); !errors.Is(err, ErrAuthentication) {
			t.Fatal("invalid login did not use the stable authentication error")
		}
	}
}

func TestAuthenticatedUpdateLogoutAndDeleteUseSessionOwner(t *testing.T) {
	repository := &accountRepositoryStub{sessionUser: User{ID: "user-id", Email: "old@example.test", DisplayName: "Old", Status: AccountActive, Role: AccountUser, Revision: 1}}
	service := newAccountServiceForTest(t, repository)
	registered, err := service.Register(context.Background(), RegisterAccountInput{Email: "first@example.test", DisplayName: "First", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	email, name := " NEW@Example.Test ", " New Name "
	updated, err := service.UpdateCurrentUser(context.Background(), registered.Token, UpdateCurrentUserInput{Email: &email, DisplayName: &name, ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Email != "new@example.test" || updated.DisplayName != "New Name" {
		t.Fatal("authenticated update did not normalize fields")
	}
	if repository.updatedRevision != 1 || updated.Revision != 2 {
		t.Fatal("authenticated update did not enforce and advance the account revision")
	}
	if err := service.Logout(context.Background(), registered.Token); err != nil || len(repository.deletedSession) != 32 {
		t.Fatal("logout did not delete the hashed current session")
	}
	repository.deletion = AccountDeletionRequest{ID: "deletion-id", Status: AccountDeletionPending, MediaCount: 2}
	deletion, err := service.DeleteCurrentUser(context.Background(), registered.Token)
	if err != nil || repository.deletedUser != "user-id" || deletion.ID != "deletion-id" || deletion.MediaCount != 2 || len(deletion.ReceiptToken) != encodedTokenLength || len(repository.deletionHash) != 32 {
		t.Fatal("account deletion did not use the authenticated owner")
	}
}

func TestAccountUpdateRejectsStaleRevisionBeforeWriting(t *testing.T) {
	name := "New Name"
	repository := &accountRepositoryStub{sessionUser: User{ID: "user-id", Revision: 3}}
	service := newAccountServiceForTest(t, repository)
	if _, err := service.UpdateCurrentUser(context.Background(), strings.Repeat("K", encodedTokenLength), UpdateCurrentUserInput{
		DisplayName: &name, ExpectedRevision: 2,
	}); !errors.Is(err, ErrAccountConflict) {
		t.Fatalf("stale account revision error=%v", err)
	}
	if repository.updatedDisplay != nil {
		t.Fatal("stale account revision reached the repository update")
	}
}

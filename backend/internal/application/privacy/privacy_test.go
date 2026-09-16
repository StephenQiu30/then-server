package privacy

import (
	"context"
	"errors"
	"testing"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type privacyAuthenticatorStub struct {
	user  accountapp.User
	err   error
	token string
}

func (s *privacyAuthenticatorStub) CurrentUser(_ context.Context, token string) (accountapp.User, error) {
	s.token = token
	return s.user, s.err
}

type privacyRepositoryStub struct {
	declaration SelfAdultDeclaration
	err         error
	userID      string
	version     string
	at          time.Time
}

func (s *privacyRepositoryStub) GetSelfAdultDeclaration(_ context.Context, userID, version string) (SelfAdultDeclaration, error) {
	s.userID, s.version = userID, version
	return s.declaration, s.err
}

func (s *privacyRepositoryStub) ConfirmSelfAdultDeclaration(_ context.Context, userID, version string, at time.Time) (SelfAdultDeclaration, error) {
	s.userID, s.version, s.at = userID, version, at
	return s.declaration, s.err
}

func (s *privacyRepositoryStub) WithdrawSelfAdultDeclaration(_ context.Context, userID, version string, at time.Time) (SelfAdultDeclaration, error) {
	s.userID, s.version, s.at = userID, version, at
	return s.declaration, s.err
}

func TestPrivacyServiceUsesAuthenticatedOwnerAndCurrentPolicy(t *testing.T) {
	confirmedAt := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	authenticator := &privacyAuthenticatorStub{user: accountapp.User{ID: "user-id"}}
	repository := &privacyRepositoryStub{declaration: SelfAdultDeclaration{
		PolicyVersion: CurrentSelfAdultPolicyVersion, Confirmed: true, ConfirmedAt: &confirmedAt,
	}}
	privacy, err := NewPrivacyService(authenticator, repository)
	if err != nil {
		t.Fatal(err)
	}
	result, err := privacy.CurrentSelfAdultDeclaration(context.Background(), "session-token")
	if err != nil {
		t.Fatal(err)
	}
	if authenticator.token != "session-token" || repository.userID != "user-id" || repository.version != CurrentSelfAdultPolicyVersion || !result.Confirmed {
		t.Fatal("privacy state did not use the authenticated owner and current policy")
	}
}

func TestPrivacyServiceRejectsUnconfirmedOrStalePolicy(t *testing.T) {
	privacy, err := NewPrivacyService(&privacyAuthenticatorStub{user: accountapp.User{ID: "user-id"}}, &privacyRepositoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []ConfirmSelfAdultDeclarationInput{
		{PolicyVersion: "stale-policy", ConfirmsSelfAndAdult: true},
		{PolicyVersion: CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: false},
	} {
		if _, err := privacy.ConfirmSelfAdultDeclaration(context.Background(), "session-token", input); !errors.Is(err, ErrInvalidPrivacyInput) {
			t.Fatal("invalid declaration input was accepted")
		}
	}
}

func TestPrivacyServicePropagatesAuthenticationFailure(t *testing.T) {
	privacy, err := NewPrivacyService(&privacyAuthenticatorStub{err: accountapp.ErrAuthentication}, &privacyRepositoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privacy.WithdrawSelfAdultDeclaration(context.Background(), "bad-session"); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("authentication failure was not preserved")
	}
}

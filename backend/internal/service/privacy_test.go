package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type privacyAuthenticatorStub struct {
	user  model.User
	err   error
	token string
}

func (s *privacyAuthenticatorStub) CurrentUser(_ context.Context, token string) (model.User, error) {
	s.token = token
	return s.user, s.err
}

type privacyRepositoryStub struct {
	declaration model.SelfAdultDeclaration
	err         error
	userID      string
	version     string
	at          time.Time
}

func (s *privacyRepositoryStub) GetSelfAdultDeclaration(_ context.Context, userID, version string) (model.SelfAdultDeclaration, error) {
	s.userID, s.version = userID, version
	return s.declaration, s.err
}

func (s *privacyRepositoryStub) ConfirmSelfAdultDeclaration(_ context.Context, userID, version string, at time.Time) (model.SelfAdultDeclaration, error) {
	s.userID, s.version, s.at = userID, version, at
	return s.declaration, s.err
}

func (s *privacyRepositoryStub) WithdrawSelfAdultDeclaration(_ context.Context, userID, version string, at time.Time) (model.SelfAdultDeclaration, error) {
	s.userID, s.version, s.at = userID, version, at
	return s.declaration, s.err
}

func TestPrivacyServiceUsesAuthenticatedOwnerAndCurrentPolicy(t *testing.T) {
	confirmedAt := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	authenticator := &privacyAuthenticatorStub{user: model.User{ID: "user-id"}}
	repository := &privacyRepositoryStub{declaration: model.SelfAdultDeclaration{
		PolicyVersion: model.CurrentSelfAdultPolicyVersion, Confirmed: true, ConfirmedAt: &confirmedAt,
	}}
	privacy, err := NewPrivacyService(authenticator, repository)
	if err != nil {
		t.Fatal(err)
	}
	result, err := privacy.CurrentSelfAdultDeclaration(context.Background(), "session-token")
	if err != nil {
		t.Fatal(err)
	}
	if authenticator.token != "session-token" || repository.userID != "user-id" || repository.version != model.CurrentSelfAdultPolicyVersion || !result.Confirmed {
		t.Fatal("privacy state did not use the authenticated owner and current policy")
	}
}

func TestPrivacyServiceRejectsUnconfirmedOrStalePolicy(t *testing.T) {
	privacy, err := NewPrivacyService(&privacyAuthenticatorStub{user: model.User{ID: "user-id"}}, &privacyRepositoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []model.ConfirmSelfAdultDeclarationInput{
		{PolicyVersion: "stale-policy", ConfirmsSelfAndAdult: true},
		{PolicyVersion: model.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: false},
	} {
		if _, err := privacy.ConfirmSelfAdultDeclaration(context.Background(), "session-token", input); !errors.Is(err, model.ErrInvalidPrivacyInput) {
			t.Fatal("invalid declaration input was accepted")
		}
	}
}

func TestPrivacyServicePropagatesAuthenticationFailure(t *testing.T) {
	privacy, err := NewPrivacyService(&privacyAuthenticatorStub{err: model.ErrAuthentication}, &privacyRepositoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privacy.WithdrawSelfAdultDeclaration(context.Background(), "bad-session"); !errors.Is(err, model.ErrAuthentication) {
		t.Fatal("authentication failure was not preserved")
	}
}

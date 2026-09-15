package service

import (
	"context"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type PrivacyAuthenticator interface {
	CurrentUser(context.Context, string) (model.User, error)
}

type PrivacyRepository interface {
	GetSelfAdultDeclaration(context.Context, string, string) (model.SelfAdultDeclaration, error)
	ConfirmSelfAdultDeclaration(context.Context, string, string, time.Time) (model.SelfAdultDeclaration, error)
	WithdrawSelfAdultDeclaration(context.Context, string, string, time.Time) (model.SelfAdultDeclaration, error)
}

type PrivacyService struct {
	authenticator PrivacyAuthenticator
	repository    PrivacyRepository
	now           func() time.Time
}

func NewPrivacyService(authenticator PrivacyAuthenticator, repository PrivacyRepository) (*PrivacyService, error) {
	if authenticator == nil || repository == nil {
		return nil, model.ErrPrivacyUnavailable
	}
	return &PrivacyService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *PrivacyService) CurrentSelfAdultDeclaration(ctx context.Context, token string) (model.SelfAdultDeclaration, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.SelfAdultDeclaration{}, err
	}
	return s.repository.GetSelfAdultDeclaration(ctx, user.ID, model.CurrentSelfAdultPolicyVersion)
}

func (s *PrivacyService) ConfirmSelfAdultDeclaration(ctx context.Context, token string, input model.ConfirmSelfAdultDeclarationInput) (model.SelfAdultDeclaration, error) {
	if input.PolicyVersion != model.CurrentSelfAdultPolicyVersion || !input.ConfirmsSelfAndAdult {
		return model.SelfAdultDeclaration{}, model.ErrInvalidPrivacyInput
	}
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.SelfAdultDeclaration{}, err
	}
	return s.repository.ConfirmSelfAdultDeclaration(ctx, user.ID, model.CurrentSelfAdultPolicyVersion, s.now().UTC())
}

func (s *PrivacyService) WithdrawSelfAdultDeclaration(ctx context.Context, token string) (model.SelfAdultDeclaration, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.SelfAdultDeclaration{}, err
	}
	return s.repository.WithdrawSelfAdultDeclaration(ctx, user.ID, model.CurrentSelfAdultPolicyVersion, s.now().UTC())
}

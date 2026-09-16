package privacy

import (
	"context"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type PrivacyAuthenticator interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
}

type PrivacyRepository interface {
	GetSelfAdultDeclaration(context.Context, string, string) (SelfAdultDeclaration, error)
	ConfirmSelfAdultDeclaration(context.Context, string, string, time.Time) (SelfAdultDeclaration, error)
	WithdrawSelfAdultDeclaration(context.Context, string, string, time.Time) (SelfAdultDeclaration, error)
}

type PrivacyService struct {
	authenticator PrivacyAuthenticator
	repository    PrivacyRepository
	now           func() time.Time
}

func NewPrivacyService(authenticator PrivacyAuthenticator, repository PrivacyRepository) (*PrivacyService, error) {
	if authenticator == nil || repository == nil {
		return nil, ErrPrivacyUnavailable
	}
	return &PrivacyService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *PrivacyService) CurrentSelfAdultDeclaration(ctx context.Context, token string) (SelfAdultDeclaration, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return SelfAdultDeclaration{}, err
	}
	return s.repository.GetSelfAdultDeclaration(ctx, user.ID, CurrentSelfAdultPolicyVersion)
}

func (s *PrivacyService) ConfirmSelfAdultDeclaration(ctx context.Context, token string, input ConfirmSelfAdultDeclarationInput) (SelfAdultDeclaration, error) {
	if input.PolicyVersion != CurrentSelfAdultPolicyVersion || !input.ConfirmsSelfAndAdult {
		return SelfAdultDeclaration{}, ErrInvalidPrivacyInput
	}
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return SelfAdultDeclaration{}, err
	}
	return s.repository.ConfirmSelfAdultDeclaration(ctx, user.ID, CurrentSelfAdultPolicyVersion, s.now().UTC())
}

func (s *PrivacyService) WithdrawSelfAdultDeclaration(ctx context.Context, token string) (SelfAdultDeclaration, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return SelfAdultDeclaration{}, err
	}
	return s.repository.WithdrawSelfAdultDeclaration(ctx, user.ID, CurrentSelfAdultPolicyVersion, s.now().UTC())
}

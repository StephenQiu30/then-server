package privacy

import (
	"context"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
)

type PrivacyAuthenticator interface {
	CurrentUser(context.Context, string) (domain.User, error)
}

type PrivacyRepository interface {
	GetSelfAdultDeclaration(context.Context, string, string) (domain.SelfAdultDeclaration, error)
	ConfirmSelfAdultDeclaration(context.Context, string, string, time.Time) (domain.SelfAdultDeclaration, error)
	WithdrawSelfAdultDeclaration(context.Context, string, string, time.Time) (domain.SelfAdultDeclaration, error)
}

type PrivacyService struct {
	authenticator PrivacyAuthenticator
	repository    PrivacyRepository
	now           func() time.Time
}

func NewPrivacyService(authenticator PrivacyAuthenticator, repository PrivacyRepository) (*PrivacyService, error) {
	if authenticator == nil || repository == nil {
		return nil, domain.ErrPrivacyUnavailable
	}
	return &PrivacyService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *PrivacyService) CurrentSelfAdultDeclaration(ctx context.Context, token string) (domain.SelfAdultDeclaration, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.SelfAdultDeclaration{}, err
	}
	return s.repository.GetSelfAdultDeclaration(ctx, user.ID, domain.CurrentSelfAdultPolicyVersion)
}

func (s *PrivacyService) ConfirmSelfAdultDeclaration(ctx context.Context, token string, input domain.ConfirmSelfAdultDeclarationInput) (domain.SelfAdultDeclaration, error) {
	if input.PolicyVersion != domain.CurrentSelfAdultPolicyVersion || !input.ConfirmsSelfAndAdult {
		return domain.SelfAdultDeclaration{}, domain.ErrInvalidPrivacyInput
	}
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.SelfAdultDeclaration{}, err
	}
	return s.repository.ConfirmSelfAdultDeclaration(ctx, user.ID, domain.CurrentSelfAdultPolicyVersion, s.now().UTC())
}

func (s *PrivacyService) WithdrawSelfAdultDeclaration(ctx context.Context, token string) (domain.SelfAdultDeclaration, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.SelfAdultDeclaration{}, err
	}
	return s.repository.WithdrawSelfAdultDeclaration(ctx, user.ID, domain.CurrentSelfAdultPolicyVersion, s.now().UTC())
}

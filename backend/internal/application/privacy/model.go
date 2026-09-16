package privacy

import (
	"errors"
	"time"
)

const CurrentSelfAdultPolicyVersion = "self-adult-v1"

var (
	ErrInvalidPrivacyInput = errors.New("invalid privacy input")
	ErrPrivacyUnavailable  = errors.New("privacy state unavailable")
)

type SelfAdultDeclaration struct {
	PolicyVersion string
	Confirmed     bool
	ConfirmedAt   *time.Time
	WithdrawnAt   *time.Time
}

type ConfirmSelfAdultDeclarationInput struct {
	PolicyVersion        string
	ConfirmsSelfAndAdult bool
}

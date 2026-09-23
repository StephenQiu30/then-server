package generation

import "errors"

var (
	ErrGenerationDisabled       = errors.New("generation is disabled")
	ErrGenerationQuotaExceeded  = errors.New("generation quota exceeded")
	ErrGenerationBudgetExceeded = errors.New("generation budget exceeded")
	ErrGenerationConcurrency    = errors.New("generation concurrency limit exceeded")
	ErrGenerationCurrency       = errors.New("generation currency mismatch")
)

// AdmissionPolicy is the provider-independent, fail-closed part of task
// acceptance. Concrete values belong to runtime configuration; this type
// only checks the immutable estimate against a repository-provided snapshot.
// The repository must reserve the same values in its transaction after this
// check, so this helper never claims that a reservation has been committed.
type AdmissionPolicy struct {
	Enabled             bool
	Currency            string
	MaxConcurrentTasks  int
	MaxQuotaUnits       int
	MaxBudgetMinorUnits int64
}

// AdmissionUsage is the current usage snapshot for the policy scope. The
// caller chooses the scope (owner, project, or both) while holding the
// transaction's relevant locks.
type AdmissionUsage struct {
	ActiveTasks        int
	ReservedQuotaUnits int
	ReservedMinorUnits int64
}

// Validate checks that an enabled policy has explicit, positive limits. An
// unset or disabled policy must not accidentally open a billable entry point.
func (p AdmissionPolicy) Validate() error {
	if !p.Enabled {
		return nil
	}
	if !validToken(p.Currency, 16) || p.MaxConcurrentTasks < 1 || p.MaxQuotaUnits < 1 || p.MaxBudgetMinorUnits < 1 {
		return ErrInvalidGenerationInput
	}
	return nil
}

// Check evaluates a new task against the supplied usage snapshot. It does
// not mutate usage; reservation and task creation must remain one database
// transaction in the eventual DATA-01 repository.
func (p AdmissionPolicy) Check(cost CostEstimate, usage AdmissionUsage) error {
	if !p.Enabled {
		return ErrGenerationDisabled
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if cost.EstimatedMinorUnits < 0 || cost.ReservedQuotaUnits < 0 || usage.ActiveTasks < 0 || usage.ReservedQuotaUnits < 0 || usage.ReservedMinorUnits < 0 {
		return ErrInvalidGenerationInput
	}
	if cost.EstimatedMinorUnits > 0 && !validToken(cost.Currency, 16) || cost.EstimatedMinorUnits == 0 && cost.Currency != "" && !validToken(cost.Currency, 16) {
		return ErrInvalidGenerationInput
	}
	if cost.Currency != "" && cost.Currency != p.Currency {
		return ErrGenerationCurrency
	}
	if usage.ActiveTasks >= p.MaxConcurrentTasks {
		return ErrGenerationConcurrency
	}
	if usage.ReservedQuotaUnits > p.MaxQuotaUnits {
		return ErrGenerationQuotaExceeded
	}
	if cost.ReservedQuotaUnits > p.MaxQuotaUnits-usage.ReservedQuotaUnits {
		return ErrGenerationQuotaExceeded
	}
	if usage.ReservedMinorUnits > p.MaxBudgetMinorUnits {
		return ErrGenerationBudgetExceeded
	}
	if cost.EstimatedMinorUnits > p.MaxBudgetMinorUnits-usage.ReservedMinorUnits {
		return ErrGenerationBudgetExceeded
	}
	return nil
}

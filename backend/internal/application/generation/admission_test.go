package generation

import (
	"errors"
	"testing"
)

func validAdmissionPolicy() AdmissionPolicy {
	return AdmissionPolicy{
		Enabled:             true,
		Currency:            "USD",
		MaxConcurrentTasks:  3,
		MaxQuotaUnits:       10,
		MaxBudgetMinorUnits: 1000,
	}
}

func TestAdmissionPolicyFailsClosedWhenDisabled(t *testing.T) {
	var policy AdmissionPolicy
	if err := policy.Validate(); err != nil {
		t.Fatalf("disabled policy validation error = %v", err)
	}
	if err := policy.Check(CostEstimate{}, AdmissionUsage{}); !errors.Is(err, ErrGenerationDisabled) {
		t.Fatalf("disabled policy check error = %v", err)
	}
}

func TestAdmissionPolicyAcceptsWithinLimitsWithoutMutation(t *testing.T) {
	policy := validAdmissionPolicy()
	cost := CostEstimate{Currency: "USD", EstimatedMinorUnits: 250, ReservedQuotaUnits: 2}
	usage := AdmissionUsage{ActiveTasks: 1, ReservedMinorUnits: 400, ReservedQuotaUnits: 3}
	if err := policy.Check(cost, usage); err != nil {
		t.Fatalf("admission check error = %v", err)
	}
	if usage != (AdmissionUsage{ActiveTasks: 1, ReservedMinorUnits: 400, ReservedQuotaUnits: 3}) {
		t.Fatalf("admission check mutated usage: %+v", usage)
	}
}

func TestAdmissionPolicyRejectsEachLimitAndInvalidCost(t *testing.T) {
	cases := []struct {
		name  string
		cost  CostEstimate
		usage AdmissionUsage
		want  error
	}{
		{
			name:  "concurrency",
			cost:  CostEstimate{Currency: "USD", EstimatedMinorUnits: 1, ReservedQuotaUnits: 1},
			usage: AdmissionUsage{ActiveTasks: 3},
			want:  ErrGenerationConcurrency,
		},
		{
			name:  "quota",
			cost:  CostEstimate{Currency: "USD", EstimatedMinorUnits: 1, ReservedQuotaUnits: 8},
			usage: AdmissionUsage{ReservedQuotaUnits: 3},
			want:  ErrGenerationQuotaExceeded,
		},
		{
			name:  "quota already over limit",
			cost:  CostEstimate{Currency: "USD"},
			usage: AdmissionUsage{ReservedQuotaUnits: 11},
			want:  ErrGenerationQuotaExceeded,
		},
		{
			name:  "budget",
			cost:  CostEstimate{Currency: "USD", EstimatedMinorUnits: 601, ReservedQuotaUnits: 1},
			usage: AdmissionUsage{ReservedMinorUnits: 400},
			want:  ErrGenerationBudgetExceeded,
		},
		{
			name:  "budget already over limit",
			cost:  CostEstimate{Currency: "USD"},
			usage: AdmissionUsage{ReservedMinorUnits: 1001},
			want:  ErrGenerationBudgetExceeded,
		},
		{
			name:  "currency",
			cost:  CostEstimate{Currency: "CNY", EstimatedMinorUnits: 1, ReservedQuotaUnits: 1},
			usage: AdmissionUsage{},
			want:  ErrGenerationCurrency,
		},
		{
			name:  "missing currency",
			cost:  CostEstimate{EstimatedMinorUnits: 1, ReservedQuotaUnits: 1},
			usage: AdmissionUsage{},
			want:  ErrInvalidGenerationInput,
		},
		{
			name:  "negative usage",
			cost:  CostEstimate{Currency: "USD", EstimatedMinorUnits: 1, ReservedQuotaUnits: 1},
			usage: AdmissionUsage{ReservedMinorUnits: -1},
			want:  ErrInvalidGenerationInput,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validAdmissionPolicy().Check(tc.cost, tc.usage); !errors.Is(err, tc.want) {
				t.Fatalf("admission error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAdmissionPolicyRequiresExplicitEnabledLimits(t *testing.T) {
	cases := []AdmissionPolicy{
		{Enabled: true, Currency: "USD", MaxConcurrentTasks: 0, MaxQuotaUnits: 10, MaxBudgetMinorUnits: 1000},
		{Enabled: true, Currency: "USD", MaxConcurrentTasks: 1, MaxQuotaUnits: 0, MaxBudgetMinorUnits: 1000},
		{Enabled: true, Currency: "USD", MaxConcurrentTasks: 1, MaxQuotaUnits: 10, MaxBudgetMinorUnits: 0},
		{Enabled: true, MaxConcurrentTasks: 1, MaxQuotaUnits: 10, MaxBudgetMinorUnits: 1000},
	}
	for index, policy := range cases {
		if err := policy.Validate(); !errors.Is(err, ErrInvalidGenerationInput) {
			t.Errorf("case %d validation error = %v", index, err)
		}
	}
}

func TestAdmissionPolicySupportsBoundedZeroCostLocalMode(t *testing.T) {
	policy := AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 2, MaxQuotaUnits: 10}
	if err := policy.Validate(); err != nil {
		t.Fatalf("zero-cost policy validation error = %v", err)
	}
	if err := policy.Check(CostEstimate{ReservedQuotaUnits: 1}, AdmissionUsage{}); err != nil {
		t.Fatalf("zero-cost task rejected: %v", err)
	}
	if err := policy.Check(CostEstimate{Currency: "USD", EstimatedMinorUnits: 1}, AdmissionUsage{}); !errors.Is(err, ErrGenerationBudgetExceeded) {
		t.Fatalf("paid task passed zero-cost policy: %v", err)
	}
}

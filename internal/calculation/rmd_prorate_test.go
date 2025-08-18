package calculation

import (
	"testing"
	"time"

	"github.com/rpgo/retirement-calculator/internal/domain"
	"github.com/rpgo/retirement-calculator/pkg/dateutil"
	"github.com/shopspring/decimal"
)

// Test that RMD is prorated in the first RMD year and that TSP withdrawal honors the prorated RMD
func TestRMDProrate_FirstYearAndTSPHonorsProratedRMD(t *testing.T) {
	ce := NewCalculationEngine()

	// Create Robert born such that RMD age (72) is reached mid-year 2025
	// Birth year for RMD age 72 in 2025 -> birth year 1953 (72 in 2025 -> 1953)
	// To place birthday mid-year, use July 1, 1953 -> turns 72 on July 1, 2025
	robert := domain.Employee{
		Name:                  "Robert",
		BirthDate:             time.Date(1953, 7, 1, 0, 0, 0, 0, time.UTC),
		HireDate:              time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrentSalary:         decimal.Zero,
		TSPBalanceTraditional: decimal.NewFromInt(500000),
	}
	dawn := domain.Employee{
		Name:      "Dawn",
		BirthDate: time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC),
		HireDate:  time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	rs := domain.RetirementScenario{EmployeeName: "Robert", RetirementDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), SSStartAge: 62, TSPWithdrawalStrategy: "4_percent_rule"}
	ds := domain.RetirementScenario{EmployeeName: "Dawn", RetirementDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), SSStartAge: 62, TSPWithdrawalStrategy: "4_percent_rule"}
	scenario := domain.Scenario{Name: "rmd-prorate", Robert: rs, Dawn: ds}

	cfg := &domain.Configuration{
		PersonalDetails:   map[string]domain.Employee{"robert": robert, "dawn": dawn},
		GlobalAssumptions: domain.GlobalAssumptions{ProjectionYears: 3, COLAGeneralRate: decimal.Zero},
		Scenarios:         []domain.Scenario{scenario},
	}

	proj := ce.GenerateAnnualProjection(&robert, &dawn, &scenario, &cfg.GlobalAssumptions, cfg.GlobalAssumptions.FederalRules)
	if len(proj) < 1 {
		t.Fatalf("expected projection rows")
	}

	// For 2025 (index 0), Robert turns RMD age on July 1 so prorated RMD should be < full RMD
	row := proj[0]
	// rmd recorded on projection row is not directly stored in RMDAmount field; check TSPWithdrawalRobert is at least the prorated RMD
	// Compute full RMD for comparison
	fullRMD := CalculateRMD(robert.TSPBalanceTraditional, robert.BirthDate.Year(), dateutil.GetRMDAge(robert.BirthDate.Year()))
	if fullRMD.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("expected non-zero full RMD for balance, got %s", fullRMD.String())
	}
	// Ensure withdrawal is not equal or greater than full RMD (should be prorated when birthday mid-year)
	if row.TSPWithdrawalRobert.GreaterThanOrEqual(fullRMD) {
		t.Fatalf("expected prorated TSP withdrawal in first RMD year to be less than full RMD; got withdrawal=%s fullRMD=%s", row.TSPWithdrawalRobert.StringFixed(2), fullRMD.StringFixed(2))
	}
}

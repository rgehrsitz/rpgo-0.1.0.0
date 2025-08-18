package calculation

import (
	"testing"
	"time"

	"github.com/rpgo/retirement-calculator/internal/domain"
	"github.com/shopspring/decimal"
)

// Test that Social Security benefit is prorated in the first year the person reaches SS start age
func TestSSProrate_FirstYearBirthdayMidYear(t *testing.T) {
	ce := NewCalculationEngine()

	// Robert turns 62 on July 1, 2025 (birthday mid-year)
	robert := domain.Employee{
		Name:          "Robert",
		BirthDate:     time.Date(1963, 7, 1, 0, 0, 0, 0, time.UTC),
		HireDate:      time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrentSalary: decimal.Zero,
		SSBenefitFRA:  decimal.NewFromInt(1800),
	}
	dawn := domain.Employee{
		Name:      "Dawn",
		BirthDate: time.Date(1965, 1, 1, 0, 0, 0, 0, time.UTC),
		HireDate:  time.Date(1992, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	rs := domain.RetirementScenario{EmployeeName: "Robert", RetirementDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), SSStartAge: 62, TSPWithdrawalStrategy: "4_percent_rule"}
	ds := domain.RetirementScenario{EmployeeName: "Dawn", RetirementDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), SSStartAge: 62, TSPWithdrawalStrategy: "4_percent_rule"}
	scenario := domain.Scenario{Name: "ss-prorate", Robert: rs, Dawn: ds}

	cfg := &domain.Configuration{
		PersonalDetails:   map[string]domain.Employee{"robert": robert, "dawn": dawn},
		GlobalAssumptions: domain.GlobalAssumptions{ProjectionYears: 3, COLAGeneralRate: decimal.Zero},
		Scenarios:         []domain.Scenario{scenario},
	}

	proj := ce.GenerateAnnualProjection(&robert, &dawn, &scenario, &cfg.GlobalAssumptions, cfg.GlobalAssumptions.FederalRules)
	if len(proj) < 1 {
		t.Fatalf("expected projection rows")
	}

	// Year 0 is 2025; Robert turns 62 on July 1, so SS should be prorated (less than full annual benefit)
	row := proj[0]
	full := CalculateSSBenefitForYear(&robert, rs.SSStartAge, 0, decimal.Zero)
	if row.SSBenefitRobert.GreaterThanOrEqual(full) {
		t.Fatalf("expected prorated SS < full-year benefit; got prorated=%s full=%s", row.SSBenefitRobert.StringFixed(2), full.StringFixed(2))
	}
	if row.SSBenefitRobert.LessThan(decimal.Zero) {
		t.Fatalf("expected non-negative prorated SS, got %s", row.SSBenefitRobert.String())
	}
}

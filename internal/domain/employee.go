package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

// Employee represents a federal employee with all necessary information for retirement planning
type Employee struct {
	Name                           string          `yaml:"name" json:"name"`
	BirthDate                      time.Time       `yaml:"birth_date" json:"birth_date"`
	HireDate                       time.Time       `yaml:"hire_date" json:"hire_date"`
	CurrentSalary                  decimal.Decimal `yaml:"current_salary" json:"current_salary"`
	High3Salary                    decimal.Decimal `yaml:"high_3_salary" json:"high_3_salary"`
	TSPBalanceTraditional          decimal.Decimal `yaml:"tsp_balance_traditional" json:"tsp_balance_traditional"`
	TSPBalanceRoth                 decimal.Decimal `yaml:"tsp_balance_roth" json:"tsp_balance_roth"`
	TSPContributionPercent         decimal.Decimal `yaml:"tsp_contribution_percent" json:"tsp_contribution_percent"`
	SSBenefitFRA                   decimal.Decimal `yaml:"ss_benefit_fra" json:"ss_benefit_fra"` // Monthly at Full Retirement Age
	SSBenefit62                    decimal.Decimal `yaml:"ss_benefit_62" json:"ss_benefit_62"`   // Monthly at age 62
	SSBenefit70                    decimal.Decimal `yaml:"ss_benefit_70" json:"ss_benefit_70"`   // Monthly at age 70
	FEHBPremiumPerPayPeriod        decimal.Decimal `yaml:"fehb_premium_per_pay_period" json:"fehb_premium_per_pay_period"`
	SurvivorBenefitElectionPercent decimal.Decimal `yaml:"survivor_benefit_election_percent" json:"survivor_benefit_election_percent"`

	// Sick Leave Credit (for pension calculation)
	SickLeaveHours decimal.Decimal `yaml:"sick_leave_hours,omitempty" json:"sick_leave_hours,omitempty"`

	// TSP Asset Allocation (optional - uses default allocation if not specified)
	TSPAllocation *TSPAllocation `yaml:"tsp_allocation,omitempty" json:"tsp_allocation,omitempty"`

	// TSP Lifecycle Fund (optional - overrides tsp_allocation if specified)
	// If specified, allocation will change over time based on age
	TSPLifecycleFund *TSPLifecycleFund `yaml:"tsp_lifecycle_fund,omitempty" json:"tsp_lifecycle_fund,omitempty"`

	// Optional fields for additional context (not used in calculations)
	PayPlanGrade string `yaml:"pay_plan_grade,omitempty" json:"pay_plan_grade,omitempty"`
	SSNLast4     string `yaml:"ssn_last4,omitempty" json:"ssn_last4,omitempty"`
}

// RetirementScenario represents a specific retirement scenario for an employee
type RetirementScenario struct {
	EmployeeName               string           `yaml:"employee_name" json:"employee_name"`
	RetirementDate             time.Time        `yaml:"retirement_date" json:"retirement_date"`
	SSStartAge                 int              `yaml:"ss_start_age" json:"ss_start_age"`
	TSPWithdrawalStrategy      string           `yaml:"tsp_withdrawal_strategy" json:"tsp_withdrawal_strategy"`
	TSPWithdrawalTargetMonthly *decimal.Decimal `yaml:"tsp_withdrawal_target_monthly,omitempty" json:"tsp_withdrawal_target_monthly,omitempty"`
	TSPWithdrawalRate          *decimal.Decimal `yaml:"tsp_withdrawal_rate,omitempty" json:"tsp_withdrawal_rate,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling for RetirementScenario
func (rs *RetirementScenario) UnmarshalYAML(value *yaml.Node) error {
	// Define a temporary struct with string fields for parsing
	type Alias struct {
		EmployeeName               string    `yaml:"employee_name"`
		RetirementDate             time.Time `yaml:"retirement_date"`
		SSStartAge                 int       `yaml:"ss_start_age"`
		TSPWithdrawalStrategy      string    `yaml:"tsp_withdrawal_strategy"`
		TSPWithdrawalTargetMonthly *string   `yaml:"tsp_withdrawal_target_monthly,omitempty"`
		TSPWithdrawalRate          *string   `yaml:"tsp_withdrawal_rate,omitempty"`
	}

	var aux Alias
	if err := value.Decode(&aux); err != nil {
		return err
	}

	// Copy non-decimal fields
	rs.EmployeeName = aux.EmployeeName
	rs.RetirementDate = aux.RetirementDate
	rs.SSStartAge = aux.SSStartAge
	rs.TSPWithdrawalStrategy = aux.TSPWithdrawalStrategy

	// Convert string decimal fields to *decimal.Decimal
	if aux.TSPWithdrawalTargetMonthly != nil {
		val, err := decimal.NewFromString(*aux.TSPWithdrawalTargetMonthly)
		if err != nil {
			return err
		}
		rs.TSPWithdrawalTargetMonthly = &val
	}

	if aux.TSPWithdrawalRate != nil {
		val, err := decimal.NewFromString(*aux.TSPWithdrawalRate)
		if err != nil {
			return err
		}
		rs.TSPWithdrawalRate = &val
	}

	return nil
}

// Scenario represents a complete retirement scenario for both employees
type Scenario struct {
	Name      string             `yaml:"name" json:"name"`
	Robert    RetirementScenario `yaml:"robert" json:"robert"`
	Dawn      RetirementScenario `yaml:"dawn" json:"dawn"`
	Mortality *ScenarioMortality `yaml:"mortality,omitempty" json:"mortality,omitempty"`
}

// ScenarioMortality groups mortality specifications and assumptions for a scenario
type ScenarioMortality struct {
	Robert      *MortalitySpec        `yaml:"robert,omitempty" json:"robert,omitempty"`
	Dawn        *MortalitySpec        `yaml:"dawn,omitempty" json:"dawn,omitempty"`
	Assumptions *MortalityAssumptions `yaml:"assumptions,omitempty" json:"assumptions,omitempty"`
}

// MortalitySpec defines a deterministic death event by date or by age (one may be supplied)
type MortalitySpec struct {
	DeathDate *time.Time `yaml:"death_date,omitempty" json:"death_date,omitempty"`
	DeathAge  *int       `yaml:"death_age,omitempty" json:"death_age,omitempty"`
}

// MortalityAssumptions defines how to treat finances after a death event (Phase 1 limited subset)
type MortalityAssumptions struct {
	SurvivorSpendingFactor decimal.Decimal `yaml:"survivor_spending_factor" json:"survivor_spending_factor"`
	TSPSpousalTransfer     string          `yaml:"tsp_spousal_transfer" json:"tsp_spousal_transfer"` // merge|separate (Phase 1 supports only merge & separate=ignore merge)
	FilingStatusSwitch     string          `yaml:"filing_status_switch" json:"filing_status_switch"` // next_year|immediate (not yet applied in Phase 1)
}

// GlobalAssumptions contains all the global parameters for calculations
type GlobalAssumptions struct {
	InflationRate           decimal.Decimal `yaml:"inflation_rate" json:"inflation_rate"`
	FEHBPremiumInflation    decimal.Decimal `yaml:"fehb_premium_inflation" json:"fehb_premium_inflation"`
	TSPReturnPreRetirement  decimal.Decimal `yaml:"tsp_return_pre_retirement" json:"tsp_return_pre_retirement"`
	TSPReturnPostRetirement decimal.Decimal `yaml:"tsp_return_post_retirement" json:"tsp_return_post_retirement"`
	COLAGeneralRate         decimal.Decimal `yaml:"cola_general_rate" json:"cola_general_rate"`
	ProjectionYears         int             `yaml:"projection_years" json:"projection_years"`
	CurrentLocation         Location        `yaml:"current_location" json:"current_location"`

	// Monte Carlo Configuration
	MonteCarloSettings MonteCarloSettings `yaml:"monte_carlo_settings" json:"monte_carlo_settings"`

	// Federal Rules and Limits (updated annually)
	FederalRules FederalRules `yaml:"federal_rules" json:"federal_rules"`

	// TSP Statistical Models (calculated from historical data, but configurable)
	TSPStatisticalModels TSPStatisticalModels `yaml:"tsp_statistical_models" json:"tsp_statistical_models"`
}

// GenerateAssumptions creates dynamic assumptions list from actual config values
func (ga *GlobalAssumptions) GenerateAssumptions() []string {
	return []string{
		fmt.Sprintf("General COLA (FERS pension & SS): %.1f%% annually", ga.COLAGeneralRate.Mul(decimal.NewFromInt(100)).InexactFloat64()),
		fmt.Sprintf("FEHB premium inflation: %.1f%% annually", ga.FEHBPremiumInflation.Mul(decimal.NewFromInt(100)).InexactFloat64()),
		fmt.Sprintf("TSP growth pre-retirement: %.1f%% annually", ga.TSPReturnPreRetirement.Mul(decimal.NewFromInt(100)).InexactFloat64()),
		fmt.Sprintf("TSP growth post-retirement: %.1f%% annually", ga.TSPReturnPostRetirement.Mul(decimal.NewFromInt(100)).InexactFloat64()),
		"Social Security wage base indexing: ~5% annually (2025 est: $168,600)",
		"Tax brackets: 2025 levels held constant (no inflation indexing)",
	}
}

// Location represents the geographic location for tax calculations
type Location struct {
	State        string `yaml:"state" json:"state"`
	County       string `yaml:"county" json:"county"`
	Municipality string `yaml:"municipality" json:"municipality"`
}

// MonteCarloSettings contains Monte Carlo simulation parameters
type MonteCarloSettings struct {
	// Variability parameters for statistical generation
	TSPReturnVariability decimal.Decimal `yaml:"tsp_return_variability" json:"tsp_return_variability"` // Default: 0.15 (15% std dev)
	InflationVariability decimal.Decimal `yaml:"inflation_variability" json:"inflation_variability"`   // Default: 0.02 (2% std dev)
	COLAVariability      decimal.Decimal `yaml:"cola_variability" json:"cola_variability"`             // Default: 0.02 (2% std dev)
	FEHBVariability      decimal.Decimal `yaml:"fehb_variability" json:"fehb_variability"`             // Default: 0.05 (5% std dev)

	// Income limits and caps
	MaxReasonableIncome decimal.Decimal `yaml:"max_reasonable_income" json:"max_reasonable_income"` // Default: 5000000 ($5M annual cap)

	// Default TSP asset allocation (used when individual allocations not specified)
	DefaultTSPAllocation TSPAllocation `yaml:"default_tsp_allocation" json:"default_tsp_allocation"`
}

// TSPAllocation represents asset allocation across TSP funds
type TSPAllocation struct {
	CFund decimal.Decimal `yaml:"c_fund" json:"c_fund"` // Default: 0.60 (60% - Large Cap Stock Index)
	SFund decimal.Decimal `yaml:"s_fund" json:"s_fund"` // Default: 0.20 (20% - Small Cap Stock Index)
	IFund decimal.Decimal `yaml:"i_fund" json:"i_fund"` // Default: 0.10 (10% - International Stock Index)
	FFund decimal.Decimal `yaml:"f_fund" json:"f_fund"` // Default: 0.10 (10% - Fixed Income Index)
	GFund decimal.Decimal `yaml:"g_fund" json:"g_fund"` // Default: 0.00 (0% - Government Securities)
}

// TSPLifecycleFund represents a TSP Lifecycle Fund with age-based allocation changes
type TSPLifecycleFund struct {
	FundName       string                              `yaml:"fund_name" json:"fund_name"`             // e.g., "L2030", "L2035", "L2040", "L Income"
	AllocationData map[string][]TSPAllocationDataPoint `yaml:"allocation_data" json:"allocation_data"` // Quarterly allocation data
}

// TSPAllocationDataPoint represents allocation at a specific date
type TSPAllocationDataPoint struct {
	Date       string        `yaml:"date" json:"date"` // Format: "YYYY-MM-DD"
	Allocation TSPAllocation `yaml:"allocation" json:"allocation"`
}

// FederalRules contains federal rules and limits that change annually
type FederalRules struct {
	// Social Security taxation thresholds (2025 values, updated annually)
	SocialSecurityTaxThresholds SocialSecurityTaxThresholds `yaml:"social_security_tax_thresholds" json:"social_security_tax_thresholds"`

	// Social Security benefit calculation rules (rarely change, but configurable)
	SocialSecurityRules SocialSecurityRules `yaml:"social_security_rules" json:"social_security_rules"`

	// FERS rules and matching rates
	FERSRules FERSRules `yaml:"fers_rules" json:"fers_rules"`

	// Federal tax configuration (updated annually)
	FederalTaxConfig FederalTaxConfig `yaml:"federal_tax_config" json:"federal_tax_config"`

	// State and local tax configuration
	StateLocalTaxConfig StateLocalTaxConfig `yaml:"state_local_tax_config" json:"state_local_tax_config"`

	// FICA tax configuration (updated annually)
	FICATaxConfig FICATaxConfig `yaml:"fica_tax_config" json:"fica_tax_config"`

	// Medicare configuration (updated annually)
	MedicareConfig MedicareConfig `yaml:"medicare_config" json:"medicare_config"`

	// FEHB configuration
	FEHBConfig FEHBConfig `yaml:"fehb_config" json:"fehb_config"`
}

// SocialSecurityTaxThresholds contains income thresholds for SS taxation (updated annually)
type SocialSecurityTaxThresholds struct {
	// 2025 thresholds for determining taxable portion of Social Security benefits
	MarriedFilingJointly struct {
		Threshold1 decimal.Decimal `yaml:"threshold_1" json:"threshold_1"` // Default: 32000 (50% taxation begins)
		Threshold2 decimal.Decimal `yaml:"threshold_2" json:"threshold_2"` // Default: 44000 (85% taxation begins)
	} `yaml:"married_filing_jointly" json:"married_filing_jointly"`

	Single struct {
		Threshold1 decimal.Decimal `yaml:"threshold_1" json:"threshold_1"` // Default: 25000 (50% taxation begins)
		Threshold2 decimal.Decimal `yaml:"threshold_2" json:"threshold_2"` // Default: 34000 (85% taxation begins)
	} `yaml:"single" json:"single"`
}

// SocialSecurityRules contains benefit calculation rules
type SocialSecurityRules struct {
	// Early retirement reduction: 5/9 of 1% per month for first 36 months, 5/12 of 1% thereafter
	EarlyRetirementReduction struct {
		First36MonthsRate    decimal.Decimal `yaml:"first_36_months_rate" json:"first_36_months_rate"`     // Default: 0.0055556 (5/9 of 1%)
		AdditionalMonthsRate decimal.Decimal `yaml:"additional_months_rate" json:"additional_months_rate"` // Default: 0.0041667 (5/12 of 1%)
	} `yaml:"early_retirement_reduction" json:"early_retirement_reduction"`

	// Delayed retirement credit: 2/3 of 1% per month (8% per year)
	DelayedRetirementCredit decimal.Decimal `yaml:"delayed_retirement_credit" json:"delayed_retirement_credit"` // Default: 0.0066667 (2/3 of 1%)
}

// FERSRules contains FERS-specific rules and matching rates
type FERSRules struct {
	// TSP matching rates
	TSPMatchingRate      decimal.Decimal `yaml:"tsp_matching_rate" json:"tsp_matching_rate"`           // Default: 0.05 (5% maximum match)
	TSPMatchingThreshold decimal.Decimal `yaml:"tsp_matching_threshold" json:"tsp_matching_threshold"` // Default: 0.05 (5% contribution required for full match)
}

// FederalTaxConfig contains federal income tax configuration (updated annually)
type FederalTaxConfig struct {
	// Standard deduction amounts
	StandardDeductionMFJ        decimal.Decimal `yaml:"standard_deduction_mfj" json:"standard_deduction_mfj"`                               // Default: 30000 (2025 MFJ)
	StandardDeductionSingle     decimal.Decimal `yaml:"standard_deduction_single" json:"standard_deduction_single"`                         // Default: 15000 (2025 Single)
	AdditionalStandardDeduction decimal.Decimal `yaml:"additional_standard_deduction_65_plus" json:"additional_standard_deduction_65_plus"` // Default: 1550 (per person 65+)

	// Tax brackets for 2025 (updated annually)
	TaxBrackets2025       []TaxBracket `yaml:"tax_brackets_2025" json:"tax_brackets_2025"`
	TaxBrackets2025Single []TaxBracket `yaml:"tax_brackets_2025_single" json:"tax_brackets_2025_single"`
}

// TaxBracket represents a federal tax bracket
type TaxBracket struct {
	Min  decimal.Decimal `yaml:"min" json:"min"`   // Minimum income for bracket
	Max  decimal.Decimal `yaml:"max" json:"max"`   // Maximum income for bracket (use 999999999 for top bracket)
	Rate decimal.Decimal `yaml:"rate" json:"rate"` // Tax rate for this bracket
}

// StateLocalTaxConfig contains state and local tax configuration
type StateLocalTaxConfig struct {
	// Pennsylvania state tax (flat rate)
	PennsylvaniaRate decimal.Decimal `yaml:"pennsylvania_rate" json:"pennsylvania_rate"` // Default: 0.0307 (3.07%)

	// Upper Makefield Township EIT (local tax)
	UpperMakefieldEITRate decimal.Decimal `yaml:"upper_makefield_eit_rate" json:"upper_makefield_eit_rate"` // Default: 0.01 (1% on earned income)
}

// FICATaxConfig contains FICA tax configuration (updated annually)
type FICATaxConfig struct {
	// Social Security tax
	SocialSecurityWageBase decimal.Decimal `yaml:"social_security_wage_base" json:"social_security_wage_base"` // Default: 176100 (2025)
	SocialSecurityRate     decimal.Decimal `yaml:"social_security_rate" json:"social_security_rate"`           // Default: 0.062 (6.2%)

	// Medicare tax
	MedicareRate decimal.Decimal `yaml:"medicare_rate" json:"medicare_rate"` // Default: 0.0145 (1.45%)

	// Additional Medicare tax (for high earners)
	AdditionalMedicareRate decimal.Decimal `yaml:"additional_medicare_rate" json:"additional_medicare_rate"`   // Default: 0.009 (0.9%)
	HighIncomeThresholdMFJ decimal.Decimal `yaml:"high_income_threshold_mfj" json:"high_income_threshold_mfj"` // Default: 250000 (MFJ)
}

// MedicareConfig contains Medicare Part B premium configuration (updated annually)
type MedicareConfig struct {
	// Base Part B premium
	BasePremium2025 decimal.Decimal `yaml:"base_premium_2025" json:"base_premium_2025"` // Default: 185.00 (2025)

	// IRMAA (Income-Related Monthly Adjustment Amount) thresholds
	IRMAAThresholds []MedicareIRMAAThreshold `yaml:"irmaa_thresholds" json:"irmaa_thresholds"`
}

// MedicareIRMAAThreshold represents an IRMAA income threshold and corresponding surcharge
type MedicareIRMAAThreshold struct {
	IncomeThresholdSingle decimal.Decimal `yaml:"income_threshold_single" json:"income_threshold_single"` // For single filers
	IncomeThresholdJoint  decimal.Decimal `yaml:"income_threshold_joint" json:"income_threshold_joint"`   // For married filing jointly
	MonthlySurcharge      decimal.Decimal `yaml:"monthly_surcharge" json:"monthly_surcharge"`             // Additional monthly premium per person
}

// FEHBConfig contains FEHB (Federal Employees Health Benefits) configuration
type FEHBConfig struct {
	// Pay periods per year (typically 26 for bi-weekly pay)
	PayPeriodsPerYear int `yaml:"pay_periods_per_year" json:"pay_periods_per_year"` // Default: 26

	// Retirement premium calculation method
	// Options: "same_as_active", "reduced_rate", "custom_multiplier"
	RetirementCalculationMethod string `yaml:"retirement_calculation_method" json:"retirement_calculation_method"` // Default: "same_as_active"

	// Custom multiplier for retirement premiums (if using custom_multiplier method)
	RetirementPremiumMultiplier decimal.Decimal `yaml:"retirement_premium_multiplier" json:"retirement_premium_multiplier"` // Default: 1.0
}

// TSPStatisticalModels contains statistical parameters for each TSP fund
// These are calculated from historical data but can be overridden
type TSPStatisticalModels struct {
	CFund TSPFundStats `yaml:"c_fund" json:"c_fund"` // Large Cap Stock Index
	SFund TSPFundStats `yaml:"s_fund" json:"s_fund"` // Small Cap Stock Index
	IFund TSPFundStats `yaml:"i_fund" json:"i_fund"` // International Stock Index
	FFund TSPFundStats `yaml:"f_fund" json:"f_fund"` // Fixed Income Index
	GFund TSPFundStats `yaml:"g_fund" json:"g_fund"` // Government Securities
}

// TSPFundStats contains statistical parameters for a TSP fund
type TSPFundStats struct {
	Mean        decimal.Decimal `yaml:"mean" json:"mean"`                 // Historical mean return
	StandardDev decimal.Decimal `yaml:"standard_dev" json:"standard_dev"` // Historical standard deviation
	DataSource  string          `yaml:"data_source" json:"data_source"`   // Source of the data (e.g., "TSP.gov 1988-2024")
	LastUpdated string          `yaml:"last_updated" json:"last_updated"` // When these stats were calculated
}

// Configuration represents the complete input configuration
type Configuration struct {
	PersonalDetails   map[string]Employee `yaml:"personal_details" json:"personal_details"`
	GlobalAssumptions GlobalAssumptions   `yaml:"global_assumptions" json:"global_assumptions"`
	Scenarios         []Scenario          `yaml:"scenarios" json:"scenarios"`
}

// Age calculates the age of the employee at a given date
func (e *Employee) Age(atDate time.Time) int {
	age := atDate.Year() - e.BirthDate.Year()
	if atDate.YearDay() < e.BirthDate.YearDay() {
		age--
	}
	return age
}

// YearsOfService calculates the years of service at a given date, including sick leave credit
func (e *Employee) YearsOfService(atDate time.Time) decimal.Decimal {
	// Calculate basic service time from hire date to retirement/calculation date
	serviceDuration := atDate.Sub(e.HireDate)
	years := decimal.NewFromFloat(serviceDuration.Hours() / 24 / 365.25)

	// Add sick leave credit if available
	// FERS Rule: Unused sick leave at retirement counts toward service computation
	// 1 day of sick leave = 1 day of service credit (8 hours = 1 day)
	if e.SickLeaveHours.GreaterThan(decimal.Zero) {
		sickLeaveDays := e.SickLeaveHours.Div(decimal.NewFromInt(8))
		sickLeaveYears := sickLeaveDays.Div(decimal.NewFromFloat(365.25))
		years = years.Add(sickLeaveYears)
	}

	return years.Round(4) // Round to 4 decimal places for precision
}

// FullRetirementAge calculates the Social Security Full Retirement Age based on birth year
func (e *Employee) FullRetirementAge() int {
	birthYear := e.BirthDate.Year()

	switch {
	case birthYear <= 1937:
		return 65
	case birthYear == 1938:
		return 65 + 2 // 65 years and 2 months
	case birthYear == 1939:
		return 65 + 4 // 65 years and 4 months
	case birthYear == 1940:
		return 65 + 6 // 65 years and 6 months
	case birthYear == 1941:
		return 65 + 8 // 65 years and 8 months
	case birthYear == 1942:
		return 65 + 10 // 65 years and 10 months
	case birthYear >= 1943 && birthYear <= 1954:
		return 66
	case birthYear == 1955:
		return 66 + 2 // 66 years and 2 months
	case birthYear == 1956:
		return 66 + 4 // 66 years and 4 months
	case birthYear == 1957:
		return 66 + 6 // 66 years and 6 months
	case birthYear == 1958:
		return 66 + 8 // 66 years and 8 months
	case birthYear == 1959:
		return 66 + 10 // 66 years and 10 months
	default: // 1960 and later
		return 67
	}
}

// MinimumRetirementAge calculates the FERS Minimum Retirement Age
func (e *Employee) MinimumRetirementAge() int {
	birthYear := e.BirthDate.Year()

	switch {
	case birthYear <= 1947:
		return 55
	case birthYear == 1948:
		return 55 + 2 // 55 years and 2 months
	case birthYear == 1949:
		return 55 + 4 // 55 years and 4 months
	case birthYear == 1950:
		return 55 + 6 // 55 years and 6 months
	case birthYear == 1951:
		return 55 + 8 // 55 years and 8 months
	case birthYear == 1952:
		return 55 + 10 // 55 years and 10 months
	case birthYear >= 1953 && birthYear <= 1964:
		return 56
	case birthYear == 1965:
		return 56 + 2 // 56 years and 2 months
	case birthYear == 1966:
		return 56 + 4 // 56 years and 4 months
	case birthYear == 1967:
		return 56 + 6 // 56 years and 6 months
	case birthYear == 1968:
		return 56 + 8 // 56 years and 8 months
	case birthYear == 1969:
		return 56 + 10 // 56 years and 10 months
	case birthYear >= 1970:
		return 57
	default:
		return 57
	}
}

// TotalTSPBalance returns the combined traditional and Roth TSP balance
func (e *Employee) TotalTSPBalance() decimal.Decimal {
	return e.TSPBalanceTraditional.Add(e.TSPBalanceRoth)
}

// AnnualTSPContribution calculates the annual TSP contribution amount
func (e *Employee) AnnualTSPContribution() decimal.Decimal {
	return e.CurrentSalary.Mul(e.TSPContributionPercent)
}

// AgencyMatch calculates the annual agency match (5% of salary if contributing at least 5%)
func (e *Employee) AgencyMatch() decimal.Decimal {
	if e.TSPContributionPercent.GreaterThanOrEqual(decimal.NewFromFloat(0.05)) {
		return e.CurrentSalary.Mul(decimal.NewFromFloat(0.05))
	}
	return decimal.Zero
}

// TotalAnnualTSPContribution returns the combined employee and agency contributions
func (e *Employee) TotalAnnualTSPContribution() decimal.Decimal {
	return e.AnnualTSPContribution().Add(e.AgencyMatch())
}

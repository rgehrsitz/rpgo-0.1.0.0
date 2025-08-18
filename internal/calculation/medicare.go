package calculation

import (
	"time"

	"github.com/rpgo/retirement-calculator/internal/domain"
	"github.com/shopspring/decimal"
)

// MedicareCalculator handles Medicare Part B premium calculations including IRMAA
type MedicareCalculator struct {
	BasePremium2025 decimal.Decimal
	IRMAAThresholds []IRMAAThreshold
}

// IRMAAThreshold represents an IRMAA income threshold and corresponding surcharge
type IRMAAThreshold struct {
	IncomeThresholdSingle decimal.Decimal // For single filers
	IncomeThresholdJoint  decimal.Decimal // For married filing jointly
	MonthlySurcharge      decimal.Decimal // Additional monthly premium per person
}

// NewMedicareCalculator creates a new Medicare calculator with 2025 rates
func NewMedicareCalculator() *MedicareCalculator {
	return &MedicareCalculator{
		BasePremium2025: decimal.NewFromFloat(185.00), // 2025 base Part B premium
		IRMAAThresholds: []IRMAAThreshold{
			// 2025 IRMAA thresholds (based on 2023 MAGI)
			{
				IncomeThresholdSingle: decimal.NewFromInt(103000),
				IncomeThresholdJoint:  decimal.NewFromInt(206000),
				MonthlySurcharge:      decimal.NewFromFloat(69.90),
			},
			{
				IncomeThresholdSingle: decimal.NewFromInt(129000),
				IncomeThresholdJoint:  decimal.NewFromInt(258000),
				MonthlySurcharge:      decimal.NewFromFloat(174.70),
			},
			{
				IncomeThresholdSingle: decimal.NewFromInt(161000),
				IncomeThresholdJoint:  decimal.NewFromInt(322000),
				MonthlySurcharge:      decimal.NewFromFloat(279.50),
			},
			{
				IncomeThresholdSingle: decimal.NewFromInt(193000),
				IncomeThresholdJoint:  decimal.NewFromInt(386000),
				MonthlySurcharge:      decimal.NewFromFloat(384.30),
			},
			{
				IncomeThresholdSingle: decimal.NewFromInt(500000),
				IncomeThresholdJoint:  decimal.NewFromInt(750000),
				MonthlySurcharge:      decimal.NewFromFloat(489.10),
			},
		},
	}
}

// NewMedicareCalculatorWithConfig creates a new Medicare calculator with configurable values
func NewMedicareCalculatorWithConfig(config domain.MedicareConfig) *MedicareCalculator {
	// Convert domain.MedicareIRMAAThreshold to calculation.IRMAAThreshold
	var thresholds []IRMAAThreshold
	for _, threshold := range config.IRMAAThresholds {
		thresholds = append(thresholds, IRMAAThreshold{
			IncomeThresholdSingle: threshold.IncomeThresholdSingle,
			IncomeThresholdJoint:  threshold.IncomeThresholdJoint,
			MonthlySurcharge:      threshold.MonthlySurcharge,
		})
	}

	return &MedicareCalculator{
		BasePremium2025: config.BasePremium2025,
		IRMAAThresholds: thresholds,
	}
}

// CalculatePartBPremium calculates Medicare Part B premium including IRMAA surcharge
// based on Modified Adjusted Gross Income (MAGI) from 2 years prior
func (mc *MedicareCalculator) CalculatePartBPremium(magi decimal.Decimal, isMarriedFilingJointly bool) decimal.Decimal {
	premium := mc.BasePremium2025

	// Find applicable IRMAA surcharge
	for _, threshold := range mc.IRMAAThresholds {
		var applicableThreshold decimal.Decimal
		if isMarriedFilingJointly {
			applicableThreshold = threshold.IncomeThresholdJoint
		} else {
			applicableThreshold = threshold.IncomeThresholdSingle
		}

		if magi.GreaterThan(applicableThreshold) {
			premium = premium.Add(threshold.MonthlySurcharge)
		} else {
			break // Stop at first threshold not exceeded
		}
	}

	return premium
}

// CalculateAnnualPartBCost calculates annual Medicare Part B cost with sophisticated IRMAA
func (mc *MedicareCalculator) CalculateAnnualPartBCost(estimatedMAGI decimal.Decimal, isMarriedFilingJointly bool) decimal.Decimal {
	// Base premium for 2025
	basePremium := mc.BasePremium2025

	// Calculate IRMAA surcharge based on MAGI
	irmaaSurcharge := mc.calculateIRMAASurcharge(estimatedMAGI, isMarriedFilingJointly)

	// Apply IRMAA surcharge
	totalMonthlyPremium := basePremium.Add(irmaaSurcharge)

	// Convert to annual cost
	annualCost := totalMonthlyPremium.Mul(decimal.NewFromInt(12))

	return annualCost
}

// calculateIRMAASurcharge calculates IRMAA surcharge based on MAGI
func (mc *MedicareCalculator) calculateIRMAASurcharge(estimatedMAGI decimal.Decimal, isMarriedFilingJointly bool) decimal.Decimal {
	var totalSurcharge decimal.Decimal

	// Apply IRMAA thresholds cumulatively based on filing status
	for _, threshold := range mc.IRMAAThresholds {
		var incomeThreshold decimal.Decimal
		if isMarriedFilingJointly {
			incomeThreshold = threshold.IncomeThresholdJoint
		} else {
			incomeThreshold = threshold.IncomeThresholdSingle
		}

		if estimatedMAGI.GreaterThan(incomeThreshold) {
			// Cumulatively add surcharges for each exceeded tier
			totalSurcharge = totalSurcharge.Add(threshold.MonthlySurcharge)
		} else {
			break
		}
	}

	return totalSurcharge
}

// CalculateMedicarePremiumWithInflation calculates Medicare premium with inflation adjustment
func (mc *MedicareCalculator) CalculateMedicarePremiumWithInflation(estimatedMAGI decimal.Decimal, isMarriedFilingJointly bool, yearsFrom2025 int) decimal.Decimal {
	// Base calculation
	baseAnnualCost := mc.CalculateAnnualPartBCost(estimatedMAGI, isMarriedFilingJointly)

	// Apply inflation adjustment (Medicare premiums typically increase faster than general inflation)
	// Medicare Part B premiums have increased by about 5-6% annually historically
	medicareInflationRate := decimal.NewFromFloat(0.055) // 5.5% annual increase
	inflationFactor := decimal.NewFromFloat(1).Add(medicareInflationRate).Pow(decimal.NewFromInt(int64(yearsFrom2025)))

	adjustedAnnualCost := baseAnnualCost.Mul(inflationFactor)

	return adjustedAnnualCost
}

// EstimateMAGI estimates Modified Adjusted Gross Income for IRMAA calculation
// This is a simplified calculation - real MAGI includes various adjustments
func EstimateMAGI(pensionIncome, tspWithdrawals, taxableSSBenefits, otherIncome decimal.Decimal) decimal.Decimal {
	// Simplified MAGI calculation for retirement income
	// In reality, MAGI includes additional items like tax-exempt interest, etc.
	return pensionIncome.Add(tspWithdrawals).Add(taxableSSBenefits).Add(otherIncome)
}

// IsMedicareEligible checks if someone is eligible for Medicare (age 65+)
// This duplicates the dateutil function but keeps Medicare logic self-contained
func IsMedicareEligible(birthDate, atDate time.Time) bool {
	age := atDate.Year() - birthDate.Year()
	if atDate.YearDay() < birthDate.YearDay() {
		age--
	}
	return age >= 65
}

// calculateMedicarePremium calculates Medicare Part B premiums with IRMAA considerations
// based on current year income (simplified - real IRMAA uses 2-year-old MAGI)
func (ce *CalculationEngine) calculateMedicarePremium(robert, dawn *domain.Employee, projectionDate time.Time,
	pensionRobert, pensionDawn, tspWithdrawalRobert, tspWithdrawalDawn, ssRobert, ssDawn decimal.Decimal) decimal.Decimal {
	var totalPremium decimal.Decimal

	// Estimate MAGI for IRMAA calculation (simplified)
	// In reality, IRMAA uses MAGI from 2 years prior
	totalPensionIncome := pensionRobert.Add(pensionDawn)
	totalTSPWithdrawals := tspWithdrawalRobert.Add(tspWithdrawalDawn)

	// Calculate taxable portion of Social Security (simplified)
	totalSSBenefits := ssRobert.Add(ssDawn)
	otherIncome := totalPensionIncome.Add(totalTSPWithdrawals)
	taxableSSBenefits := ce.TaxCalc.CalculateSocialSecurityTaxation(totalSSBenefits, otherIncome)

	// Estimate combined MAGI
	estimatedMAGI := EstimateMAGI(totalPensionIncome, totalTSPWithdrawals, taxableSSBenefits, decimal.Zero)

	// Check if Robert is Medicare eligible
	if IsMedicareEligible(robert.BirthDate, projectionDate) {
		robertPremium := ce.MedicareCalc.CalculateAnnualPartBCost(estimatedMAGI, true) // Married filing jointly
		totalPremium = totalPremium.Add(robertPremium)
	}

	// Check if Dawn is Medicare eligible
	if IsMedicareEligible(dawn.BirthDate, projectionDate) {
		dawnPremium := ce.MedicareCalc.CalculateAnnualPartBCost(estimatedMAGI, true) // Married filing jointly
		totalPremium = totalPremium.Add(dawnPremium)
	}

	return totalPremium
}

package calculation

import (
	"time"

	"github.com/rpgo/retirement-calculator/internal/domain"
	"github.com/rpgo/retirement-calculator/pkg/dateutil"
	"github.com/shopspring/decimal"
)

// GenerateAnnualProjection generates annual cash flow projections for a scenario
func (ce *CalculationEngine) GenerateAnnualProjection(personA, personB *domain.Employee, scenario *domain.Scenario, assumptions *domain.GlobalAssumptions, federalRules domain.FederalRules) []domain.AnnualCashFlow {
	projection := make([]domain.AnnualCashFlow, assumptions.ProjectionYears)

	// Determine retirement year (0-based index)
	// Projection starts at ProjectionBaseYear (first year of projection)
	projectionStartYear := ProjectionBaseYear
	retirementYear := scenario.PersonA.RetirementDate.Year() - projectionStartYear
	if retirementYear < 0 {
		retirementYear = 0
	}

	// Initialize TSP balances
	currentTSPTraditionalPersonA := personA.TSPBalanceTraditional
	currentTSPRothPersonA := personA.TSPBalanceRoth
	currentTSPTraditionalPersonB := personB.TSPBalanceTraditional
	currentTSPRothPersonB := personB.TSPBalanceRoth

	// Create TSP withdrawal strategies
	// For Scenario 2, we need to account for extra growth before withdrawals start
	personAStrategy := ce.createTSPStrategy(&scenario.PersonA, currentTSPTraditionalPersonA.Add(currentTSPRothPersonA), assumptions.InflationRate)
	personBStrategy := ce.createTSPStrategy(&scenario.PersonB, currentTSPTraditionalPersonB.Add(currentTSPRothPersonB), assumptions.InflationRate)

	// Mortality derived dates using helper
	personADeathYearIndex, personBDeathYearIndex := deriveDeathYearIndexes(scenario, personA, personB, assumptions.ProjectionYears)

	survivorSpendingFactor := decimal.NewFromFloat(1.0)
	if scenario.Mortality != nil && scenario.Mortality.Assumptions != nil && !scenario.Mortality.Assumptions.SurvivorSpendingFactor.IsZero() {
		survivorSpendingFactor = scenario.Mortality.Assumptions.SurvivorSpendingFactor
	}

	personADeceased := false
	personBDeceased := false

	for year := 0; year < assumptions.ProjectionYears; year++ {
		projectionDate := time.Date(projectionStartYear, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(year, 0, 0)
		agePersonA := personA.Age(projectionDate)
		agePersonB := personB.Age(projectionDate)

		// Calculate partial year retirement for each person
		// Projection starts at ProjectionBaseYear, so year 0 = ProjectionBaseYear, etc.
		projectionStartYear := ProjectionBaseYear
		personARetirementYear := scenario.PersonA.RetirementDate.Year() - projectionStartYear
		personBRetirementYear := scenario.PersonB.RetirementDate.Year() - projectionStartYear

		// Determine if each person is retired for this year
		isPersonARetired := year >= personARetirementYear
		isPersonBRetired := year >= personBRetirementYear

		// Calculate partial year factors (what portion of the year each person works)
		var personAWorkFraction, personBWorkFraction decimal.Decimal

		if year == personARetirementYear && personARetirementYear >= 0 {
			// PersonA retires during this year - calculate work fraction
			personARetirementDate := scenario.PersonA.RetirementDate
			yearStart := time.Date(projectionDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			daysWorked := personARetirementDate.Sub(yearStart).Hours() / 24
			daysInYear := 365.0
			personAWorkFraction = decimal.NewFromFloat(daysWorked / daysInYear)
		} else if isPersonARetired {
			personAWorkFraction = decimal.Zero
		} else {
			personAWorkFraction = decimal.NewFromInt(1)
		}

		if year == personBRetirementYear && personBRetirementYear >= 0 {
			// PersonB retires during this year - calculate work fraction
			personBRetirementDate := scenario.PersonB.RetirementDate
			yearStart := time.Date(projectionDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			daysWorked := personBRetirementDate.Sub(yearStart).Hours() / 24
			daysInYear := 365.0
			personBWorkFraction = decimal.NewFromFloat(daysWorked / daysInYear)
		} else if isPersonBRetired {
			personBWorkFraction = decimal.Zero
		} else {
			personBWorkFraction = decimal.NewFromInt(1)
		}

		// Apply death events at start-of-year (Phase 1: incomes stop this year)
		if personADeathYearIndex != nil && year >= *personADeathYearIndex {
			personADeceased = true
		}
		if personBDeathYearIndex != nil && year >= *personBDeathYearIndex {
			personBDeceased = true
		}

		// If a spouse just became deceased this year and transfer mode is merge, merge TSP balances into survivor (traditional+roth)
		if scenario.Mortality != nil && scenario.Mortality.Assumptions != nil && scenario.Mortality.Assumptions.TSPSpousalTransfer == "merge" {
			if personADeceased && !personBDeceased {
				// Move PersonA balances into PersonB's (simple add)
				currentTSPTraditionalPersonB = currentTSPTraditionalPersonB.Add(currentTSPTraditionalPersonA)
				currentTSPRothPersonB = currentTSPRothPersonB.Add(currentTSPRothPersonA)
				currentTSPTraditionalPersonA = decimal.Zero
				currentTSPRothPersonA = decimal.Zero
			}
			if personBDeceased && !personADeceased {
				currentTSPTraditionalPersonA = currentTSPTraditionalPersonA.Add(currentTSPTraditionalPersonB)
				currentTSPRothPersonA = currentTSPRothPersonA.Add(currentTSPRothPersonB)
				currentTSPTraditionalPersonB = decimal.Zero
				currentTSPRothPersonB = decimal.Zero
			}
		}

		// Calculate FERS pensions (only for retired portion of year, and not after death)
		var pensionPersonA, pensionPersonB decimal.Decimal
		var survivorPensionPersonA, survivorPensionPersonB decimal.Decimal
		if isPersonARetired && !personADeceased {
			pensionPersonA = CalculatePensionForYear(personA, scenario.PersonA.RetirementDate, year-personARetirementYear, assumptions.InflationRate)
			// Adjust for partial year if retiring this year
			if year == personARetirementYear {
				pensionPersonA = pensionPersonA.Mul(decimal.NewFromInt(1).Sub(personAWorkFraction))
			}

			// Debug output for pension calculation
			if ce.Debug && year == personARetirementYear {
				ce.Logger.Debugf("DEBUG: PersonA pension calculation for year %d", ProjectionBaseYear+year)
				ce.Logger.Debugf("  Retirement date: %s", scenario.PersonA.RetirementDate.Format("2006-01-02"))
				ce.Logger.Debugf("  Age at retirement: %d", personA.Age(scenario.PersonA.RetirementDate))
				ce.Logger.Debugf("  Years of service: %s", personA.YearsOfService(scenario.PersonA.RetirementDate).StringFixed(2))
				ce.Logger.Debugf("  High-3 salary: %s", personA.High3Salary.StringFixed(2))

				// Get detailed pension calculation
				pensionCalc := CalculateFERSPension(personA, scenario.PersonA.RetirementDate)
				ce.Logger.Debugf("  Multiplier: %s", pensionCalc.Multiplier.StringFixed(4))
				ce.Logger.Debugf("  ANNUAL pension (before reduction): $%s", pensionCalc.AnnualPension.StringFixed(2))
				ce.Logger.Debugf("  Survivor election: %s", pensionCalc.SurvivorElection.StringFixed(4))
				ce.Logger.Debugf("  ANNUAL pension (final): $%s", pensionCalc.ReducedPension.StringFixed(2))
				ce.Logger.Debugf("  MONTHLY pension amount: $%s", pensionCalc.ReducedPension.Div(decimal.NewFromInt(12)).StringFixed(2))
				ce.Logger.Debugf("  Current-year cash received (partial): $%s", pensionPersonA.StringFixed(2))
			}
		}
		if isPersonBRetired && !personBDeceased {
			pensionPersonB = CalculatePensionForYear(personB, scenario.PersonB.RetirementDate, year-personBRetirementYear, assumptions.InflationRate)
			// Adjust for partial year if retiring this year
			if year == personBRetirementYear {
				pensionPersonB = pensionPersonB.Mul(decimal.NewFromInt(1).Sub(personBWorkFraction))
			}
		}

		// Survivor pension logic with pro-rating in death year
		if scenario.Mortality != nil {
			if personADeceased && !personBDeceased && isPersonARetired {
				baseCalc := CalculateFERSPension(personA, scenario.PersonA.RetirementDate)
				yearsSinceRet := year - personARetirementYear
				if yearsSinceRet < 0 {
					yearsSinceRet = 0
				}
				currentSurvivor := baseCalc.SurvivorAnnuity
				for cy := 1; cy <= yearsSinceRet; cy++ {
					projDate := scenario.PersonA.RetirementDate.AddDate(cy, 0, 0)
					ageAt := personA.Age(projDate)
					currentSurvivor = ApplyFERSPensionCOLA(currentSurvivor, assumptions.InflationRate, ageAt)
				}
				if personADeathYearIndex != nil && year >= *personADeathYearIndex {
					// Pro-rate in death year: survivor receives only portion AFTER death
					var deathDate *time.Time
					if scenario.Mortality.PersonA != nil {
						deathDate = scenario.Mortality.PersonA.DeathDate
					}
					frac, occurred := deathFractionInYear(personADeathYearIndex, year, deathDate)
					if occurred {
						// Pension stream for deceased stops at death; survivor annuity starts month after death -> approximate with (1-frac)
						survivorPensionPersonB = currentSurvivor.Mul(decimal.NewFromInt(1).Sub(frac))
					} else {
						survivorPensionPersonB = currentSurvivor
					}
				}
			}
			if personBDeceased && !personADeceased && isPersonBRetired {
				baseCalc := CalculateFERSPension(personB, scenario.PersonB.RetirementDate)
				yearsSinceRet := year - personBRetirementYear
				if yearsSinceRet < 0 {
					yearsSinceRet = 0
				}
				currentSurvivor := baseCalc.SurvivorAnnuity
				for cy := 1; cy <= yearsSinceRet; cy++ {
					projDate := scenario.PersonB.RetirementDate.AddDate(cy, 0, 0)
					ageAt := personB.Age(projDate)
					currentSurvivor = ApplyFERSPensionCOLA(currentSurvivor, assumptions.InflationRate, ageAt)
				}
				if personBDeathYearIndex != nil && year >= *personBDeathYearIndex {
					var deathDate *time.Time
					if scenario.Mortality.PersonB != nil {
						deathDate = scenario.Mortality.PersonB.DeathDate
					}
					frac, occurred := deathFractionInYear(personBDeathYearIndex, year, deathDate)
					if occurred {
						survivorPensionPersonA = currentSurvivor.Mul(decimal.NewFromInt(1).Sub(frac))
					} else {
						survivorPensionPersonA = currentSurvivor
					}
				}
			}
		}

		// Calculate Social Security benefits
		ssPersonA := decimal.Zero
		if !personADeceased {
			ssPersonA = CalculateSSBenefitForYear(personA, scenario.PersonA.SSStartAge, year, assumptions.COLAGeneralRate)
		}
		ssPersonB := decimal.Zero
		if !personBDeceased {
			ssPersonB = CalculateSSBenefitForYear(personB, scenario.PersonB.SSStartAge, year, assumptions.COLAGeneralRate)
		}

		// Prorate Social Security if the person reaches their SS start age during this calendar year
		yearEnd := time.Date(projectionDate.Year(), 12, 31, 23, 59, 59, 0, time.UTC)
		// PersonA
		agePersonAStart := agePersonA
		agePersonAEnd := personA.Age(yearEnd)
		if agePersonAStart < scenario.PersonA.SSStartAge && agePersonAEnd >= scenario.PersonA.SSStartAge {
			// birthday occurs this year; prorate SS for months/days after birthday
			birthdayThisYear := time.Date(projectionDate.Year(), personA.BirthDate.Month(), personA.BirthDate.Day(), 0, 0, 0, 0, time.UTC)
			// If the person also retires earlier this same year (before their birthday),
			// defer prorating to the retirement-based logic. Otherwise use the birthday-based prorate.
			if !(year == personARetirementYear && scenario.PersonA.RetirementDate.Before(birthdayThisYear)) {
				daysAfter := yearEnd.Sub(birthdayThisYear).Hours() / 24.0
				daysInYear := float64(dateutil.DaysInYear(projectionDate.Year()))
				frac := daysAfter / daysInYear
				if frac < 0 {
					frac = 0
				}
				ssPersonA = ssPersonA.Mul(decimal.NewFromFloat(frac))
			}
		}
		// PersonB
		agePersonBStart := agePersonB
		agePersonBEnd := personB.Age(yearEnd)
		if agePersonBStart < scenario.PersonB.SSStartAge && agePersonBEnd >= scenario.PersonB.SSStartAge {
			birthdayThisYear := time.Date(projectionDate.Year(), personB.BirthDate.Month(), personB.BirthDate.Day(), 0, 0, 0, 0, time.UTC)
			// If the person also retires earlier this same year (before their birthday),
			// defer prorating to the retirement-based logic. Otherwise use the birthday-based prorate.
			if !(year == personBRetirementYear && scenario.PersonB.RetirementDate.Before(birthdayThisYear)) {
				daysAfter := yearEnd.Sub(birthdayThisYear).Hours() / 24.0
				daysInYear := float64(dateutil.DaysInYear(projectionDate.Year()))
				frac := daysAfter / daysInYear
				if frac < 0 {
					frac = 0
				}
				ssPersonB = ssPersonB.Mul(decimal.NewFromFloat(frac))
			}
		}
		// Survivor SS refined: compute survivor benefit factoring early-claim reduction
		if personADeceased && !personBDeceased {
			fra := dateutil.FullRetirementAge(personB.BirthDate)
			// Use deceased's current-year benefit (pre-death). If zero (due to modeling order), recalc directly.
			deceasedBenefit := CalculateSSBenefitForYear(personA, scenario.PersonA.SSStartAge, year, assumptions.COLAGeneralRate)
			candidate := CalculateSurvivorSSBenefit(deceasedBenefit, agePersonB, fra)
			if candidate.GreaterThan(ssPersonB) {
				ssPersonB = candidate
			}
		}
		if personBDeceased && !personADeceased {
			fra := dateutil.FullRetirementAge(personA.BirthDate)
			deceasedBenefit := CalculateSSBenefitForYear(personB, scenario.PersonB.SSStartAge, year, assumptions.COLAGeneralRate)
			candidate := CalculateSurvivorSSBenefit(deceasedBenefit, agePersonA, fra)
			if candidate.GreaterThan(ssPersonA) {
				ssPersonA = candidate
			}
		}

		// Adjust Social Security for partial year based on eligibility and retirement timing
		if year == personARetirementYear && personARetirementYear >= 0 {
			// PersonA can start SS when they retire (if 62+) or when they turn 62, whichever is later
			ageAtRetirement := personA.Age(scenario.PersonA.RetirementDate)
			if ageAtRetirement >= scenario.PersonA.SSStartAge {
				// Can start SS immediately upon retirement. Only apply retirement-based proration
				// if the retirement date occurs before the birthday that grants SS eligibility
				birthdayThisYear := time.Date(projectionDate.Year(), personA.BirthDate.Month(), personA.BirthDate.Day(), 0, 0, 0, 0, time.UTC)
				if scenario.PersonA.RetirementDate.Before(birthdayThisYear) {
					ssPersonA = ssPersonA.Mul(decimal.NewFromInt(1).Sub(personAWorkFraction))
				}
			} else {
				// Will start SS later when turns 62
				ssPersonA = decimal.Zero
			}
		}
		if year == personBRetirementYear && personBRetirementYear >= 0 {
			// PersonB can start SS immediately upon retirement
			ageAtRetirement := personB.Age(scenario.PersonB.RetirementDate)
			if ageAtRetirement >= scenario.PersonB.SSStartAge {
				retirementDate := scenario.PersonB.RetirementDate
				ssStartDate := time.Date(retirementDate.Year(), retirementDate.Month()+1, 1, 0, 0, 0, 0, time.UTC)
				monthsOfBenefits := 12 - int(ssStartDate.Month()) + 1

				// Prorate SS for partial year
				ssMonthlyBenefit := ssPersonB.Div(decimal.NewFromInt(12))
				// Only apply retirement-based proration if retirement occurs before the birthday
				// that makes them SS-eligible; otherwise birthday-based proration already applied.
				birthdayThisYear := time.Date(projectionDate.Year(), personB.BirthDate.Month(), personB.BirthDate.Day(), 0, 0, 0, 0, time.UTC)
				if retirementDate.Before(birthdayThisYear) {
					ssPersonB = ssMonthlyBenefit.Mul(decimal.NewFromInt(int64(monthsOfBenefits)))
				}
			} else {
				ssPersonB = decimal.Zero
			}
		}

		// Calculate FERS Special Retirement Supplement (only if retired)
		var srsPersonA, srsPersonB decimal.Decimal
		if isPersonARetired && !personADeceased {
			srsPersonA = CalculateFERSSupplementYear(personA, scenario.PersonA.RetirementDate, year-personARetirementYear, assumptions.InflationRate)
			// Adjust for partial year if retiring this year
			if year == personARetirementYear {
				srsPersonA = srsPersonA.Mul(decimal.NewFromInt(1).Sub(personAWorkFraction))
			}
		}
		if isPersonBRetired && !personBDeceased {
			srsPersonB = CalculateFERSSupplementYear(personB, scenario.PersonB.RetirementDate, year-personBRetirementYear, assumptions.InflationRate)
			// Adjust for partial year if retiring this year
			if year == personBRetirementYear {
				srsPersonB = srsPersonB.Mul(decimal.NewFromInt(1).Sub(personBWorkFraction))
			}
		}

		// Calculate TSP withdrawals and update balances
		var tspWithdrawalPersonA, tspWithdrawalPersonB decimal.Decimal

		// Calculate RMD amounts (full and prorated) for this year for each person
		rmdPersonA := decimal.Zero
		rmdPersonB := decimal.Zero
		// PersonA RMD
		rmdAgePersonA := dateutil.GetRMDAge(personA.BirthDate.Year())
		agePersonAEnd = personA.Age(yearEnd)
		if agePersonA < rmdAgePersonA && agePersonAEnd >= rmdAgePersonA {
			// First RMD year: prorate based on birthday
			birthdayThisYear := time.Date(projectionDate.Year(), personA.BirthDate.Month(), personA.BirthDate.Day(), 0, 0, 0, 0, time.UTC)
			daysAfter := yearEnd.Sub(birthdayThisYear).Hours() / 24.0
			daysInYear := float64(dateutil.DaysInYear(projectionDate.Year()))
			frac := daysAfter / daysInYear
			if frac < 0 {
				frac = 0
			}
			fullRMD := CalculateRMD(currentTSPTraditionalPersonA, personA.BirthDate.Year(), rmdAgePersonA)
			rmdPersonA = fullRMD.Mul(decimal.NewFromFloat(frac))
		} else if agePersonA >= rmdAgePersonA {
			// Regular RMD year (apply full amount)
			rmdPersonA = CalculateRMD(currentTSPTraditionalPersonA, personA.BirthDate.Year(), agePersonA)
		}
		// PersonB RMD
		rmdAgePersonB := dateutil.GetRMDAge(personB.BirthDate.Year())
		agePersonBEnd = personB.Age(yearEnd)
		if agePersonB < rmdAgePersonB && agePersonBEnd >= rmdAgePersonB {
			birthdayThisYear := time.Date(projectionDate.Year(), personB.BirthDate.Month(), personB.BirthDate.Day(), 0, 0, 0, 0, time.UTC)
			daysAfter := yearEnd.Sub(birthdayThisYear).Hours() / 24.0
			daysInYear := float64(dateutil.DaysInYear(projectionDate.Year()))
			frac := daysAfter / daysInYear
			if frac < 0 {
				frac = 0
			}
			fullRMD := CalculateRMD(currentTSPTraditionalPersonB, personB.BirthDate.Year(), rmdAgePersonB)
			rmdPersonB = fullRMD.Mul(decimal.NewFromFloat(frac))
		} else if agePersonB >= rmdAgePersonB {
			rmdPersonB = CalculateRMD(currentTSPTraditionalPersonB, personB.BirthDate.Year(), agePersonB)
		}
		if isPersonARetired && !personADeceased {
			// For 4% rule: Always withdraw 4% of initial balance (adjusted for inflation)
			if scenario.PersonA.TSPWithdrawalStrategy == "4_percent_rule" {
				// Use the 4% rule strategy to calculate withdrawals
				tspWithdrawalPersonA = personAStrategy.CalculateWithdrawal(
					currentTSPTraditionalPersonA.Add(currentTSPRothPersonA),
					year-personARetirementYear+1,
					decimal.Zero, // Not used for 4% rule
					agePersonA,
					dateutil.IsRMDYear(personA.BirthDate, projectionDate),
					CalculateRMD(currentTSPTraditionalPersonA, personA.BirthDate.Year(), agePersonA),
				)
				// Adjust for partial year if retiring this year
				if year == personARetirementYear {
					tspWithdrawalPersonA = tspWithdrawalPersonA.Mul(decimal.NewFromInt(1).Sub(personAWorkFraction))
				}
			} else {
				// For need_based: Use the target monthly amount
				targetIncome := pensionPersonA.Add(pensionPersonB).Add(ssPersonA).Add(ssPersonB).Add(srsPersonA).Add(srsPersonB)

				// Calculate withdrawals
				tspWithdrawalPersonA = personAStrategy.CalculateWithdrawal(
					currentTSPTraditionalPersonA.Add(currentTSPRothPersonA),
					year-personARetirementYear+1,
					targetIncome,
					agePersonA,
					(dateutil.IsRMDYear(personA.BirthDate, projectionDate) || rmdPersonA.GreaterThan(decimal.Zero)),
					rmdPersonA,
				)
				// Adjust for partial year if retiring this year
				if year == personARetirementYear {
					tspWithdrawalPersonA = tspWithdrawalPersonA.Mul(decimal.NewFromInt(1).Sub(personAWorkFraction))
				}
			}
		}

		if isPersonBRetired && !personBDeceased {
			if scenario.PersonB.TSPWithdrawalStrategy == "4_percent_rule" {
				tspWithdrawalPersonB = personBStrategy.CalculateWithdrawal(
					currentTSPTraditionalPersonB.Add(currentTSPRothPersonB),
					year-personBRetirementYear+1,
					decimal.Zero, // Not used for 4% rule
					agePersonB,
					dateutil.IsRMDYear(personB.BirthDate, projectionDate),
					CalculateRMD(currentTSPTraditionalPersonB, personB.BirthDate.Year(), agePersonB),
				)
				// Adjust for partial year if retiring this year
				if year == personBRetirementYear {
					tspWithdrawalPersonB = tspWithdrawalPersonB.Mul(decimal.NewFromInt(1).Sub(personBWorkFraction))
				}
			} else {
				// For need_based: Use the target monthly amount
				targetIncome := pensionPersonA.Add(pensionPersonB).Add(ssPersonA).Add(ssPersonB).Add(srsPersonA).Add(srsPersonB)

				// Calculate withdrawals
				tspWithdrawalPersonB = personBStrategy.CalculateWithdrawal(
					currentTSPTraditionalPersonB.Add(currentTSPRothPersonB),
					year-personBRetirementYear+1,
					targetIncome,
					agePersonB,
					(dateutil.IsRMDYear(personB.BirthDate, projectionDate) || rmdPersonB.GreaterThan(decimal.Zero)),
					rmdPersonB,
				)
				// Adjust for partial year if retiring this year
				if year == personBRetirementYear {
					tspWithdrawalPersonB = tspWithdrawalPersonB.Mul(decimal.NewFromInt(1).Sub(personBWorkFraction))
				}
			}
		}

		// Update TSP balances
		if isPersonARetired {
			// Post-retirement TSP growth with withdrawals
			// Use lifecycle fund allocation if available, otherwise use default return rate
			if personA.TSPLifecycleFund != nil || personA.TSPAllocation != nil {
				// Apply withdrawal first
				if tspWithdrawalPersonA.GreaterThan(currentTSPTraditionalPersonA) {
					// Take from Roth if traditional is insufficient
					remainingWithdrawal := tspWithdrawalPersonA.Sub(currentTSPTraditionalPersonA)
					currentTSPTraditionalPersonA = decimal.Zero
					if remainingWithdrawal.GreaterThan(currentTSPRothPersonA) {
						currentTSPRothPersonA = decimal.Zero
					} else {
						currentTSPRothPersonA = currentTSPRothPersonA.Sub(remainingWithdrawal)
					}
				} else {
					currentTSPTraditionalPersonA = currentTSPTraditionalPersonA.Sub(tspWithdrawalPersonA)
				}

				// Apply growth using lifecycle fund allocation
				allocation := ce.getTSPAllocationForEmployee(personA, projectionDate)
				weightedReturn := ce.calculateTSPReturnWithAllocation(allocation, projectionDate.Year())

				currentTSPTraditionalPersonA = currentTSPTraditionalPersonA.Mul(decimal.NewFromFloat(1).Add(weightedReturn))
				currentTSPRothPersonA = currentTSPRothPersonA.Mul(decimal.NewFromFloat(1).Add(weightedReturn))
			} else {
				currentTSPTraditionalPersonA, currentTSPRothPersonA = ce.updateTSPBalances(
					currentTSPTraditionalPersonA, currentTSPRothPersonA, tspWithdrawalPersonA,
					assumptions.TSPReturnPostRetirement,
				)
			}
		} else {
			// Pre-retirement TSP growth with contributions
			// Use lifecycle fund allocation if available, otherwise use default return rate
			if personA.TSPLifecycleFund != nil || personA.TSPAllocation != nil {
				currentTSPTraditionalPersonA = ce.growTSPBalanceWithAllocation(personA, currentTSPTraditionalPersonA, personA.TotalAnnualTSPContribution(), projectionDate)
				currentTSPRothPersonA = ce.growTSPBalanceWithAllocation(personA, currentTSPRothPersonA, decimal.Zero, projectionDate)
			} else {
				currentTSPTraditionalPersonA = ce.growTSPBalance(currentTSPTraditionalPersonA, personA.TotalAnnualTSPContribution(), assumptions.TSPReturnPreRetirement)
				currentTSPRothPersonA = ce.growTSPBalance(currentTSPRothPersonA, decimal.Zero, assumptions.TSPReturnPreRetirement)
			}
		}

		if isPersonBRetired {
			// Post-retirement TSP growth with withdrawals
			// Use lifecycle fund allocation if available, otherwise use default return rate
			if personB.TSPLifecycleFund != nil || personB.TSPAllocation != nil {
				// Apply withdrawal first
				if tspWithdrawalPersonB.GreaterThan(currentTSPTraditionalPersonB) {
					// Take from Roth if traditional is insufficient
					remainingWithdrawal := tspWithdrawalPersonB.Sub(currentTSPTraditionalPersonB)
					currentTSPTraditionalPersonB = decimal.Zero
					if remainingWithdrawal.GreaterThan(currentTSPRothPersonB) {
						currentTSPRothPersonB = decimal.Zero
					} else {
						currentTSPRothPersonB = currentTSPRothPersonB.Sub(remainingWithdrawal)
					}
				} else {
					currentTSPTraditionalPersonB = currentTSPTraditionalPersonB.Sub(tspWithdrawalPersonB)
				}

				// Apply growth using lifecycle fund allocation
				allocation := ce.getTSPAllocationForEmployee(personB, projectionDate)
				weightedReturn := ce.calculateTSPReturnWithAllocation(allocation, projectionDate.Year())

				currentTSPTraditionalPersonB = currentTSPTraditionalPersonB.Mul(decimal.NewFromFloat(1).Add(weightedReturn))
				currentTSPRothPersonB = currentTSPRothPersonB.Mul(decimal.NewFromFloat(1).Add(weightedReturn))
			} else {
				currentTSPTraditionalPersonB, currentTSPRothPersonB = ce.updateTSPBalances(
					currentTSPTraditionalPersonB, currentTSPRothPersonB, tspWithdrawalPersonB,
					assumptions.TSPReturnPostRetirement,
				)
			}
		} else {
			// Pre-retirement TSP growth with contributions
			// Use lifecycle fund allocation if available, otherwise use default return rate
			if personB.TSPLifecycleFund != nil || personB.TSPAllocation != nil {
				currentTSPTraditionalPersonB = ce.growTSPBalanceWithAllocation(personB, currentTSPTraditionalPersonB, personB.TotalAnnualTSPContribution(), projectionDate)
				currentTSPRothPersonB = ce.growTSPBalanceWithAllocation(personB, currentTSPRothPersonB, decimal.Zero, projectionDate)
			} else {
				currentTSPTraditionalPersonB = ce.growTSPBalance(currentTSPTraditionalPersonB, personB.TotalAnnualTSPContribution(), assumptions.TSPReturnPreRetirement)
				currentTSPRothPersonB = ce.growTSPBalance(currentTSPRothPersonB, decimal.Zero, assumptions.TSPReturnPreRetirement)
			}
		}

		// Debug TSP balances for Scenario 2 to show extra growth
		if ce.Debug && year == 1 && scenario.PersonA.RetirementDate.Year() == 2027 {
			ce.Logger.Debugf("TSP Growth in Scenario 2 (year %d)", ProjectionBaseYear+year)
			ce.Logger.Debugf("  PersonA's TSP balance: %s", currentTSPTraditionalPersonA.Add(currentTSPRothPersonA).StringFixed(2))
			ce.Logger.Debugf("  PersonB's TSP balance: %s", currentTSPTraditionalPersonB.Add(currentTSPRothPersonB).StringFixed(2))
			ce.Logger.Debugf("  Combined TSP balance: %s", currentTSPTraditionalPersonA.Add(currentTSPRothPersonA).Add(currentTSPTraditionalPersonB).Add(currentTSPRothPersonB).StringFixed(2))
			ce.Logger.Debugf("")
		}

		// Calculate FEHB premiums
		fehbPremium := CalculateFEHBPremium(personA, year, assumptions.FEHBPremiumInflation, federalRules.FEHBConfig)

		// Calculate Medicare premiums (if applicable)
		medicarePremium := ce.calculateMedicarePremium(personA, personB, projectionDate,
			pensionPersonA, pensionPersonB, tspWithdrawalPersonA, tspWithdrawalPersonB, ssPersonA, ssPersonB)

		// Calculate taxes - handle transition years properly
		// Pass the actual working income and retirement income separately
		workingIncomePersonA := personA.CurrentSalary.Mul(personAWorkFraction)
		workingIncomePersonB := personB.CurrentSalary.Mul(personBWorkFraction)

		federalTax, stateTax, localTax, ficaTax, taxableTotal, stdDedUsed, filingStatusUsed, seniors65 := ce.calculateTaxes(
			personA, personB, scenario, year, isPersonARetired && isPersonBRetired,
			pensionPersonA, pensionPersonB, survivorPensionPersonA, survivorPensionPersonB,
			tspWithdrawalPersonA, tspWithdrawalPersonB,
			ssPersonA, ssPersonB,
			workingIncomePersonA, workingIncomePersonB,
		)

		// Calculate TSP contributions (only for working portion of year)
		var tspContributions decimal.Decimal
		if (!isPersonARetired || !isPersonBRetired) && !(personADeceased || personBDeceased) {
			personAContributions := personA.TotalAnnualTSPContribution().Mul(personAWorkFraction)
			personBContributions := personB.TotalAnnualTSPContribution().Mul(personBWorkFraction)
			tspContributions = personAContributions.Add(personBContributions)
		}

		// Create annual cash flow
		cashFlow := domain.AnnualCashFlow{
			Year:                     year + 1,
			Date:                     projectionDate,
			AgePersonA:               agePersonA,
			AgePersonB:               agePersonB,
			SalaryPersonA:            personA.CurrentSalary.Mul(personAWorkFraction),
			SalaryPersonB:            personB.CurrentSalary.Mul(personBWorkFraction),
			PensionPersonA:           pensionPersonA,
			PensionPersonB:           pensionPersonB,
			TSPWithdrawalPersonA:     tspWithdrawalPersonA,
			TSPWithdrawalPersonB:     tspWithdrawalPersonB,
			SSBenefitPersonA:         ssPersonA,
			SSBenefitPersonB:         ssPersonB,
			FERSSupplementPersonA:    srsPersonA,
			FERSSupplementPersonB:    srsPersonB,
			FederalTax:               federalTax,
			FederalTaxableIncome:     taxableTotal,
			FederalStandardDeduction: stdDedUsed,
			FederalFilingStatus:      filingStatusUsed,
			FederalSeniors65Plus:     seniors65,
			StateTax:                 stateTax,
			LocalTax:                 localTax,
			FICATax:                  ficaTax,
			TSPContributions:         tspContributions,
			FEHBPremium:              fehbPremium,
			MedicarePremium:          medicarePremium,
			TSPBalancePersonA:        currentTSPTraditionalPersonA.Add(currentTSPRothPersonA),
			TSPBalancePersonB:        currentTSPTraditionalPersonB.Add(currentTSPRothPersonB),
			TSPBalanceTraditional:    currentTSPTraditionalPersonA.Add(currentTSPTraditionalPersonB),
			TSPBalanceRoth:           currentTSPRothPersonA.Add(currentTSPRothPersonB),
			IsRetired:                isPersonARetired && isPersonBRetired, // Both retired
			IsMedicareEligible:       dateutil.IsMedicareEligible(personA.BirthDate, projectionDate) || dateutil.IsMedicareEligible(personB.BirthDate, projectionDate),
			IsRMDYear:                dateutil.IsRMDYear(personA.BirthDate, projectionDate) || dateutil.IsRMDYear(personB.BirthDate, projectionDate),
			RMDAmount:                rmdPersonA.Add(rmdPersonB),
			PersonADeceased:          personADeceased,
			PersonBDeceased:          personBDeceased,
			FilingStatusSingle:       false,
		}

		// Determine filing status for display (mirror simplified logic in taxes.go)
		if scenario.Mortality != nil && scenario.Mortality.Assumptions != nil && (personADeceased != personBDeceased) {
			mode := scenario.Mortality.Assumptions.FilingStatusSwitch
			// Reconstruct death year indexes (already computed earlier): reuse conditions
			switch mode {
			case "immediate":
				cashFlow.FilingStatusSingle = true
			case "next_year":
				if personADeathYearIndex != nil && personADeceased && year > *personADeathYearIndex {
					cashFlow.FilingStatusSingle = true
				}
				if personBDeathYearIndex != nil && personBDeceased && year > *personBDeathYearIndex {
					cashFlow.FilingStatusSingle = true
				}
			}
		}

		// Inject survivor pension values
		cashFlow.SurvivorPensionPersonA = survivorPensionPersonA
		cashFlow.SurvivorPensionPersonB = survivorPensionPersonB

		// Apply survivor spending factor by scaling discretionary withdrawals and original pensions (not survivor annuity)
		if (personADeceased || personBDeceased) && survivorSpendingFactor.LessThan(decimal.NewFromFloat(0.999)) {
			cashFlow.TSPWithdrawalPersonA = cashFlow.TSPWithdrawalPersonA.Mul(survivorSpendingFactor)
			cashFlow.TSPWithdrawalPersonB = cashFlow.TSPWithdrawalPersonB.Mul(survivorSpendingFactor)
			cashFlow.PensionPersonA = cashFlow.PensionPersonA.Mul(survivorSpendingFactor)
			cashFlow.PensionPersonB = cashFlow.PensionPersonB.Mul(survivorSpendingFactor)
		}

		// Calculate total gross income and net income
		cashFlow.TotalGrossIncome = cashFlow.CalculateTotalIncome()
		cashFlow.CalculateNetIncome()

		projection[year] = cashFlow
	}

	return projection
}

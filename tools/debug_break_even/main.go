package main

import (
	"fmt"
	"os"

	calc "github.com/rpgo/retirement-calculator/internal/calculation"
	"github.com/rpgo/retirement-calculator/internal/config"
	"github.com/shopspring/decimal"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: debug_break_even <config-file>")
		return
	}
	f := os.Args[1]
	p := config.NewInputParser()
	cfg, err := p.LoadFromFile(f)
	if err != nil {
		panic(err)
	}
	engine := calc.NewCalculationEngineWithConfig(cfg.GlobalAssumptions.FederalRules)
	res, err := engine.RunScenarios(cfg)
	if err != nil {
		panic(err)
	}
	if len(res.Scenarios) < 2 {
		fmt.Println("need 2 scenarios")
		return
	}
	projA := res.Scenarios[0].Projection
	projB := res.Scenarios[1].Projection
	fmt.Println("Index,DateA.IsRetired,DateB.IsRetired,DateA.Year,DateA.Date,NetA,NetB,CumA,CumB,Diff")

	cumA := decimal.Zero
	cumB := decimal.Zero
	crossed := false

	for i := 0; i < len(projA) && i < len(projB); i++ {
		cumA = cumA.Add(projA[i].NetIncome)
		cumB = cumB.Add(projB[i].NetIncome)
		diff := cumA.Sub(cumB)

		fmt.Printf("%d,%v,%v,%d,%v,%s,%s,%s,%s,%s\n",
			i,
			projA[i].IsRetired,
			projB[i].IsRetired,
			projA[i].Year,
			projA[i].Date.Format("2006-01-02"),
			projA[i].NetIncome.StringFixed(0),
			projB[i].NetIncome.StringFixed(0),
			cumA.StringFixed(0),
			cumB.StringFixed(0),
			diff.StringFixed(0),
		)

		// Note if sign changes (crossing) between years
		if !crossed && i > 0 {
			// compute previous diff by subtracting this year's nets
			prevCumA := cumA.Sub(projA[i].NetIncome)
			prevCumB := cumB.Sub(projB[i].NetIncome)
			prevDiff := prevCumA.Sub(prevCumB)
			if prevDiff.Mul(diff).LessThan(decimal.Zero) || diff.Abs().LessThan(decimal.NewFromFloat(0.01)) {
				fmt.Printf("-- crossover detected between years %d and %d (prevDiff=%s currDiff=%s)\n", projA[i-1].Date.Year(), projA[i].Date.Year(), prevDiff.StringFixed(0), diff.StringFixed(0))
				crossed = true
			}
		}
	}

	be, err := calc.CalculateCumulativeBreakEven(projA, projB)
	fmt.Printf("\nBreakEven: %+v, err=%v\n", be, err)
}

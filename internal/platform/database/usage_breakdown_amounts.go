package database

import (
	"math/big"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func usageUnsigned(value string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(value, 10)
	if !ok || n.Sign() < 0 || n.String() != value {
		return nil, ErrConflict
	}
	return n, nil
}

func usageRational(n, d *big.Int) model.RationalMinor {
	r := new(big.Rat).SetFrac(n, d)
	return model.RationalMinor{Numerator: r.Num().String(), Denominator: r.Denom().String()}
}

func usageCostSummary(payg, unallocated *big.Int, legacy usageLegacy, totalLegacyBytes *big.Int) model.UsageBreakdownCost {
	actual := new(big.Int).Add(payg, unallocated)
	cost := model.UsageBreakdownCost{ActualMinor: actual.String(), Legacy: model.UsageLegacyEstimate{Status: legacy.status}, KnownTotalMinor: usageRational(actual, big.NewInt(1)), UnallocatedMinor: usageRational(unallocated, big.NewInt(1))}
	if legacy.status != model.UsageLegacyAvailable {
		return cost
	}
	amount := usageRational(legacy.numerator, legacy.denominator)
	cost.Legacy.AmountMinor = &amount
	n := new(big.Int).Mul(actual, legacy.denominator)
	n.Add(n, legacy.numerator)
	cost.KnownTotalMinor = usageRational(n, legacy.denominator)
	if totalLegacyBytes.Sign() == 0 {
		n.Mul(unallocated, legacy.denominator)
		n.Add(n, legacy.numerator)
		cost.UnallocatedMinor = usageRational(n, legacy.denominator)
	}
	return cost
}

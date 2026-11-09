package database

import (
	"math/big"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func addUsageBreakdownHourlyTotals(result *model.UsageBreakdown, cost, up, down, omittedCost *big.Int) error {
	for _, field := range []struct {
		target *string
		add    *big.Int
	}{{&result.Cost.ActualMinor, cost}, {&result.Omitted.UploadBytes, up}, {&result.Omitted.DownloadBytes, down}} {
		current, err := usageUnsigned(*field.target)
		if err != nil {
			return err
		}
		*field.target = current.Add(current, field.add).String()
	}
	for _, field := range []struct {
		target *model.RationalMinor
		add    *big.Int
	}{{&result.Cost.KnownTotalMinor, cost}, {&result.Omitted.CostMinor, omittedCost}} {
		n, err := usageUnsigned(field.target.Numerator)
		if err != nil {
			return err
		}
		d, err := usageUnsigned(field.target.Denominator)
		if err != nil {
			return err
		}
		if d.Sign() == 0 {
			return ErrConflict
		}
		n.Add(n, new(big.Int).Mul(field.add, d))
		*field.target = usageRational(n, d)
	}
	return nil
}

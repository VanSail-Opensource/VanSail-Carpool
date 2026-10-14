package database

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

func normalizedRolloverEligible(value model.PurchaseRollover) int64 {
	if value.AllocatedBytes == nil || value.UsedTrafficBytes == nil {
		return 0
	}
	return rollover.EligibleUnused(*value.AllocatedBytes, *value.UsedTrafficBytes, value.MinimumRemainingBPS)
}

func calculatedRolloverCredit(value model.PurchaseRollover) int64 {
	if value.AllocatedBytes == nil {
		return 0
	}
	return rollover.CreditMinor(value.NetPaidTXBMinor, normalizedRolloverEligible(value), *value.AllocatedBytes)
}

func renewalFundsCover(balance, credit, price int64, rolloverCountsTowardBalance bool) bool {
	if !rolloverCountsTowardBalance {
		credit = 0
	}
	return price >= 0 && balance >= 0 && (balance >= price || (credit >= 0 && credit >= price-balance))
}

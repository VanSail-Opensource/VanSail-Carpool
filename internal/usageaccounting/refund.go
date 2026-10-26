package usageaccounting

import "math/big"

// RefundMinor rounds a proportional net-paid credit to the nearest TXB cent.
func RefundMinor(paid, eligible, allocated int64) int64 {
	if paid <= 0 || eligible <= 0 || allocated <= 0 {
		return 0
	}
	if eligible >= allocated {
		return paid
	}
	product := new(big.Int).Mul(big.NewInt(paid), big.NewInt(eligible))
	product.Add(product, big.NewInt(allocated/2))
	product.Quo(product, big.NewInt(allocated))
	if !product.IsInt64() {
		return paid
	}
	return product.Int64()
}

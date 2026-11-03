package database

import (
	"math"
	"math/big"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func nextNativeUsage(old nativeUsageValue, upload, download, multiplier int64) (nativeUsageValue, int64, error) {
	if old.upload < 0 || old.download < 0 || old.weighted < 0 || old.remainder < 0 || old.remainder >= xnode.MultiplierScale || upload < 0 || download < 0 || multiplier < 0 || upload > math.MaxInt64-download || old.upload > math.MaxInt64-upload || old.download > math.MaxInt64-download {
		return old, 0, xnode.ErrInvalid
	}
	numerator := new(big.Int).Mul(big.NewInt(upload+download), big.NewInt(multiplier))
	numerator.Add(numerator, big.NewInt(old.remainder))
	weighted, left := new(big.Int), new(big.Int)
	weighted.QuoRem(numerator, big.NewInt(xnode.MultiplierScale), left)
	if !weighted.IsInt64() || old.weighted > math.MaxInt64-weighted.Int64() {
		return old, 0, xnode.ErrInvalid
	}
	return nativeUsageValue{old.upload + upload, old.download + download, old.weighted + weighted.Int64(), left.Int64()}, weighted.Int64(), nil
}

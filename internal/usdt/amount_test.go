package usdt

import (
	"math/big"
	"testing"
)

func TestUSDTAtomicPrecisionAndCanonicalAmounts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		raw      string
		decimals int
		expected string
	}{{"1.001", 6, "1001000"}, {"1.001", 18, "1001000000000000000"}, {"999999999999999999.099", 18, "999999999999999999099000000000000000"}} {
		value, err := Atomic(test.raw, test.decimals)
		if err != nil || value.String() != test.expected {
			t.Fatalf("Atomic(%s,%d)=%v %v", test.raw, test.decimals, value, err)
		}
		if _, err := TransferAmount(test.expected, test.decimals); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"", "0", "-1", "+1", "1e3", ".1", "1.", "0.0000001"} {
		if _, err := Atomic(raw, 6); err == nil {
			t.Fatalf("invalid amount %q accepted", raw)
		}
	}
	for _, raw := range []string{"", "0001", "0", "-1"} {
		if _, err := TransferAmount(raw, 6); err == nil {
			t.Fatalf("invalid atomic %q accepted", raw)
		}
	}
	if FormatAtomic(big.NewInt(1), 3) != "0.001" || FormatAtomic(big.NewInt(1000), 3) != "1.000" {
		t.Fatal("fixed decimals changed")
	}
	for _, network := range []string{"ethereum", "bsc", "polygon", "arbitrum", "xlayer", "plasma"} {
		spec, _ := Network(network)
		if _, err := NormalizeAddress(spec, "0x0000000000000000000000000000000000000000"); err == nil {
			t.Fatal("zero EVM address accepted")
		}
	}
	ton, _ := Network("ton")
	raw, err := NormalizeAddress(ton, "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs")
	if err != nil || raw != ton.Token {
		t.Fatal("TON master mismatch", raw, err)
	}
	for _, network := range []string{"tron", "solana", "ton", "aptos"} {
		spec, _ := Network(network)
		if _, err := NormalizeAddress(spec, "invalid"); err == nil {
			t.Fatal("invalid address accepted", network)
		}
	}
}

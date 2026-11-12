package usdt

import (
	"errors"
	"reflect"
	"testing"
)

func TestUSDTNetworkAliasesNormalizeBeforeCurrencyAndDuplicateChecks(t *testing.T) {
	for _, alias := range []string{"ethereum", "usdt.erc20", "usdc.erc20"} {
		t.Run(alias, func(t *testing.T) {
			config := testConfig()
			config.Mode = "both"
			config.Networks[1].Network = alias
			config.Networks[1].Currencies = nil
			if err := ValidateConfig(&config); err != nil {
				t.Fatal(err)
			}
			network := config.Networks[1]
			if network.Network != "ethereum" || !reflect.DeepEqual(network.Currencies, []string{"USDT", "USDC"}) {
				t.Fatalf("accepted alias became an unwatched network: %+v", network)
			}
			duplicate := NetworkConfig{Network: "usdt.erc20", Currencies: []string{}}
			config.Networks = append(config.Networks[:2], duplicate)
			if err := ValidateConfig(&config); !errors.Is(err, ErrConfiguration) {
				t.Fatalf("duplicate canonical network accepted: %v", err)
			}
		})
	}
}

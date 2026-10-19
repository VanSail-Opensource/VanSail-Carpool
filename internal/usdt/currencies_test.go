package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestNetworkCurrencyInheritanceAndAuthority(t *testing.T) {
	for _, mode := range []string{"usdt", "usdc", "both"} {
		c := testConfig()
		c.Mode = mode
		for i := range c.Networks {
			c.Networks[i].Currencies = nil
		}
		raw, _ := json.Marshal(c)
		decoded, err := DecodeConfig(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range decoded.Networks {
			for _, spec := range Registry {
				if n.Network == spec.Network && currencyEnabled(n, spec) != modeAllows(mode, spec) {
					t.Fatal("legacy mode lost", n, mode)
				}
			}
		}
	}
	c := testConfig()
	c.Mode = "usdt"
	c.Networks[1].Currencies = []string{"USDC"}
	c.Networks[2].Currencies = []string{}
	if err := ValidateConfig(&c); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c)
	roundtrip, err := DecodeConfig(string(raw))
	if err != nil || !reflect.DeepEqual(c.Networks, roundtrip.Networks) {
		t.Fatal("explicit switches changed", err)
	}
	svc := NewService(&testRepository{}, &testSettings{raw: string(raw)})
	svc.fingerprint = fingerprint(c)
	svc.state("usdt.erc20", "ready", "100", "")
	methods, err := svc.Methods(context.Background())
	if err != nil || len(methods) != 1 || methods[0].Currency != "USDC" || methods[0].Available {
		t.Fatal("token switches or readiness leaked", methods, err)
	}
	for _, currencies := range [][]string{{"USDC", "USDC"}, {"usdt"}, {"BTC"}} {
		invalid := testConfig()
		invalid.Networks[1].Currencies = currencies
		if !errors.Is(ValidateConfig(&invalid), ErrConfiguration) {
			t.Fatal("invalid currencies accepted", currencies)
		}
	}
	c.Networks[0].Currencies = []string{"USDC"}
	if !errors.Is(ValidateConfig(&c), ErrConfiguration) {
		t.Fatal("unregistered token accepted")
	}
}

type watchAfterDisable struct {
	*testRepository
	cancel context.CancelFunc
	rail   string
}

func (r *watchAfterDisable) USDTCursor(_ context.Context, rail string) (Cursor, error) {
	r.rail = rail
	r.cancel()
	return Cursor{}, context.Canceled
}

func TestDisabledTokenContinuesIssuedQuoteWatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &watchAfterDisable{testRepository: &testRepository{addresses: []string{"0x1111111111111111111111111111111111111111"}}, cancel: cancel}
	svc := NewService(r, &testSettings{})
	spec, _ := Network("usdc.erc20")
	n := NetworkConfig{Network: "ethereum", Currencies: []string{}, Endpoints: []Endpoint{{ID: "primary", RPCURL: "https://rpc.example"}}}
	svc.monitor(ctx, spec, n, false)
	if r.rail != spec.Rail {
		t.Fatal("disabled issued quote was not watched", r.rail)
	}
}

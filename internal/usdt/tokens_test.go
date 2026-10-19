package usdt

import (
	"context"
	"encoding/json"
	"testing"
)

func TestPinnedNativeUSDCAndModeIsolation(t *testing.T) {
	want := map[string]string{
		"ethereum": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"polygon":  "0x3c499c542cef5e3811e1192ce70d8cc03d5c3359",
		"arbitrum": "0xaf88d065e77c8cc2239327c5edb3a432268e5831",
		"xlayer":   "0xb6ceceab302e2e4948951ee7843fc24e92933061",
		"plasma":   "0x2d661c89d812261039af9764eceaaee884f5f67f",
		"solana":   "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		"aptos":    "0xbae207659db88bea0cbead6da0ed00aac12edcdda169e591cd41c94180b46f3b",
	}
	for _, spec := range Registry {
		if spec.Currency() != "USDC" {
			continue
		}
		if spec.Token != want[spec.Network] || spec.Decimals != 6 {
			t.Fatal("unpinned native USDC", spec)
		}
		delete(want, spec.Network)
	}
	if len(want) != 0 {
		t.Fatal("missing native USDC networks", want)
	}
	for _, rail := range []string{"usdc.trc20", "usdc.bep20", "usdc.ton"} {
		if _, ok := Network(rail); ok {
			t.Fatal("unsupported token advertised", rail)
		}
	}
	if _, ok := TokenNetwork("polygon", "0x2791bca1f2de4661ed88a30c99a7a9449aa84174"); ok {
		t.Fatal("bridged USDC accepted")
	}
	c := testConfig()
	raw, _ := json.Marshal(c)
	svc := NewService(&testRepository{}, &testSettings{raw: string(raw)})
	svc.fingerprint = fingerprint(c)
	svc.state("usdt.erc20", "ready", "100", "")
	ctx := context.Background()
	for _, mode := range []string{"usdt", "both", "usdc"} {
		c.Mode = mode
		for i := range c.Networks {
			c.Networks[i].Currencies = nil
		}
		if err := ValidateConfig(&c); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SaveConfig(ctx, "admin", PublicConfig(c)); err != nil {
			t.Fatal(err)
		}
		svc.fingerprint = fingerprint(c)
		methods, err := svc.Methods(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "both" && (len(methods) != 2 || methods[1].Available) {
			t.Fatal("USDT readiness leaked to USDC", methods)
		}
		if mode == "usdc" && (len(methods) != 1 || methods[0].Currency != "USDC" || methods[0].Available) {
			t.Fatal("mode isolation", methods)
		}
	}
	legacy, _ := json.Marshal(testConfig())
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(legacy, &fields)
	delete(fields, "mode")
	legacy, _ = json.Marshal(fields)
	decoded, err := DecodeConfig(string(legacy))
	if err != nil || decoded.Mode != "usdt" {
		t.Fatal("legacy mode changed", decoded.Mode, err)
	}
}

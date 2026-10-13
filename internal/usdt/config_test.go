package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func testConfig() Config {
	c := DefaultConfig()
	c.Enabled = true
	c.Rate = "10"
	c.Concurrency = 1
	for i := range c.Networks {
		if c.Networks[i].Network == "ethereum" {
			c.Networks[i].Enabled = true
			c.Networks[i].Addresses = []string{"0x1111111111111111111111111111111111111111"}
			c.Networks[i].Endpoints = []Endpoint{{ID: "primary", RPCURL: "https://rpc.example/v2/secret-in-url", WSURL: "wss://rpc.example/ws/secret-in-url", APIKey: "secret-api-key", UseWS: true}}
		}
	}
	return c
}
func TestUSDTConfigMasksSecretsAndValidatesEveryNetwork(t *testing.T) {
	t.Parallel()
	c := testConfig()
	if err := ValidateConfig(&c); err != nil {
		t.Fatal(err)
	}
	public := PublicConfig(c)
	raw, _ := json.Marshal(public)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "https://") {
		t.Fatalf("public config leaked credentials: %s", raw)
	}
	mergeSecrets(&public, c)
	if err := ValidateConfig(&public); err != nil {
		t.Fatal(err)
	}
	if public.Networks[1].Endpoints[0].APIKey != "secret-api-key" {
		t.Fatal("blank write erased API key")
	}
	for _, spec := range Registry {
		if _, err := NormalizeAddress(spec, spec.Token); err != nil {
			t.Fatalf("pinned token %s invalid: %v", spec.Network, err)
		}
	}
	for _, mutate := range []func(*Config){func(c *Config) { c.Concurrency = 9 }, func(c *Config) { c.Networks[1].Endpoints[0].RPCURL = "file:///etc/passwd" }, func(c *Config) { c.Networks[1].Addresses = append(c.Networks[1].Addresses, c.Networks[1].Addresses[0]) }, func(c *Config) { c.Rate = "1e3" }, func(c *Config) { c.Networks[1].Endpoints[0].APIKey = "key\r\ninjected" }} {
		invalid := testConfig()
		mutate(&invalid)
		if err := ValidateConfig(&invalid); err == nil {
			t.Fatal("invalid settings accepted")
		}
	}
	if _, err := DecodeConfig(`{} {}`); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	if _, err := DecodeConfig(`{"unknown":true}`); err == nil {
		t.Fatal("unknown configuration field accepted")
	}
	defaults, err := DecodeConfig("")
	if err != nil || defaults.Enabled || len(defaults.Networks) != 10 {
		t.Fatalf("defaults=%+v %v", defaults, err)
	}
}

func TestUSDTMaskedEndpointConfigRemainsValid(t *testing.T) {
	t.Parallel()
	c := testConfig()
	public := PublicConfig(c)
	public.Networks[1].Endpoints[0].RPCURL = ""
	public.Networks[1].Endpoints[0].WSURL = ""
	public.Networks[1].Endpoints[0].APIKey = ""
	if err := ValidateConfig(&public); err != nil {
		t.Fatal(err)
	}
}

func TestUSDTMethodsAndIssuedQuotesSurviveSettingChanges(t *testing.T) {
	t.Parallel()
	c := testConfig()
	raw, _ := json.Marshal(c)
	settings := &testSettings{raw: string(raw)}
	repo := &testRepository{}
	service := NewService(repo, settings)
	ctx := context.Background()
	methods, err := service.Methods(ctx)
	if err != nil || len(methods) != 1 || methods[0].Available {
		t.Fatalf("unchecked methods=%v %v", methods, err)
	}
	service.mu.Lock()
	service.fingerprint = fingerprint(c)
	service.mu.Unlock()
	service.state("ethereum", "ready", "100", "")
	methods, err = service.Methods(ctx)
	if err != nil || !methods[0].Available || methods[0].Currency != "USDT" {
		t.Fatalf("ready methods=%v %v", methods, err)
	}
	q, err := service.Create(ctx, "order", "usdt.erc20", "1.000")
	if err != nil {
		t.Fatal(err)
	}
	c.Enabled = false
	c.Networks[1].Addresses = []string{"0x2222222222222222222222222222222222222222"}
	if _, err := service.SaveConfig(ctx, "admin", PublicConfig(c)); err != nil {
		t.Fatal(err)
	}
	copy, err := service.Create(ctx, "order", "usdt.erc20", "1.000")
	if err != nil || copy != q {
		t.Fatalf("issued quote changed=%+v %v", copy, err)
	}
	if rate, err := service.Rate(ctx); err != nil || rate != "10" {
		t.Fatal(rate, err)
	}
	if expiry, err := service.Expiry(ctx); err != nil || expiry != 30*time.Minute {
		t.Fatal(expiry, err)
	}
	if _, err := service.Create(ctx, "new", "usdt.erc20", "1.000"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("disabled provider created a quote")
	}
	page, err := service.Review(ctx, "", 50)
	if err != nil || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
	if _, _, err := service.Resolve(ctx, "transfer", "order", "admin", "reason"); err != nil {
		t.Fatal(err)
	}
}

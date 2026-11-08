package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestMaskedUSDTKeyCannotFollowChangedDestinations(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Endpoint)
	}{
		{"RPC host", func(e *Endpoint) { e.RPCURL = "https://attacker.example" }},
		{"RPC path", func(e *Endpoint) { e.RPCURL = "https://rpc.example/other-tenant" }},
		{"RPC query", func(e *Endpoint) { e.RPCURL = "https://rpc.example/v2/secret-in-url?tenant=other" }},
		{"WebSocket", func(e *Endpoint) { e.WSURL = "wss://attacker.example/ws" }},
		{"inactive WebSocket", func(e *Endpoint) { e.UseWS = false; e.WSURL = "wss://attacker.example/ws" }},
		{"indexer", func(e *Endpoint) { e.IndexerURL = "https://attacker.example/graphql" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			old := testConfig()
			input := PublicConfig(old)
			test.mutate(&input.Networks[1].Endpoints[0])
			before, _ := json.Marshal(input)
			if err := mergeSecrets(&input, old); !errors.Is(err, ErrConfiguration) {
				t.Fatal("inherited key retargeted", err)
			}
			after, _ := json.Marshal(input)
			if string(after) != string(before) {
				t.Fatal("rejected merge changed the draft or exposed stored secrets")
			}
			input.Networks[1].Endpoints[0].APIKey = "fresh-explicit-key"
			if err := mergeSecrets(&input, old); err != nil {
				t.Fatal("explicit replacement rejected", err)
			}
			if err := ValidateConfig(&input); err != nil || input.Networks[1].Endpoints[0].APIKey != "fresh-explicit-key" {
				t.Fatal("fresh key not used", err)
			}
		})
	}
}

func TestUSDTMaskedSecretsStayWithinCanonicalNetworkAndEndpoint(t *testing.T) {
	old := testConfig()
	unchanged := PublicConfig(old)
	unchanged.Networks[1].Network = "usdt.erc20" // Same registry network, legacy rail spelling.
	if err := mergeSecrets(&unchanged, old); err != nil || unchanged.Networks[1].Endpoints[0].APIKey != "secret-api-key" {
		t.Fatal("unchanged masked connection lost its key", err)
	}
	for _, network := range []string{"bsc", "solana"} {
		input := DefaultConfig()
		input.Networks = []NetworkConfig{{Network: network, Endpoints: []Endpoint{{ID: "primary", RPCURL: "https://public.example"}}}}
		if err := mergeSecrets(&input, old); err != nil {
			t.Fatal(err)
		}
		endpoint := input.Networks[0].Endpoints[0]
		if endpoint.APIKey != "" || endpoint.WSURL != "" || endpoint.IndexerURL != "" {
			t.Fatal("endpoint ID inherited another network's credentials")
		}
	}
	input := PublicConfig(old)
	input.Networks[1].Endpoints[0].ID = "new-endpoint"
	if err := mergeSecrets(&input, old); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(&input); !errors.Is(err, ErrConfiguration) {
		t.Fatal("unresolved masked URL accepted", err)
	}
}

func TestUSDTRejectedRetargetNeverPersistsOrInvalidatesReadiness(t *testing.T) {
	old := testConfig()
	raw, _ := json.Marshal(old)
	settings := &testSettings{raw: string(raw)}
	service := NewService(&testRepository{}, settings)
	service.fingerprint = fingerprint(old)
	input := PublicConfig(old)
	input.Networks[1].Endpoints[0].RPCURL = "https://attacker.example"
	if _, err := service.SaveConfig(context.Background(), "admin", input); !errors.Is(err, ErrConfiguration) {
		t.Fatal("masked retarget accepted", err)
	}
	if settings.raw != string(raw) || service.fingerprint != fingerprint(old) || len(service.wake) != 0 {
		t.Fatal("rejected configuration reached persistence or collector admission")
	}
}

func TestUSDTKeylessPublicEndpointCanChangeWithoutCredentialInheritance(t *testing.T) {
	old := testConfig()
	old.Networks[1].Endpoints[0].APIKey = ""
	input := PublicConfig(old)
	input.Networks[1].Endpoints[0].RPCURL = "https://public-replacement.example"
	if err := mergeSecrets(&input, old); err != nil {
		t.Fatal("keyless endpoint edit rejected", err)
	}
	if err := ValidateConfig(&input); err != nil || input.Networks[1].Endpoints[0].APIKey != "" {
		t.Fatal("keyless endpoint gained a credential", err)
	}
}

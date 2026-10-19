package usdt

import "strings"

// NetworkSpec pins mainnet token identity and its atomic-unit precision.
type NetworkSpec struct {
	Network, Rail, Name, Family, Token string
	ChainID, Decimals                  int
}

// Registry follows BEpusdt aa3bd5097f258cccd5a53baed7211c74733b8332.
var Registry = []NetworkSpec{
	{"tron", "usdt.trc20", "TRC20", "tron", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", 0, 6},
	{"ethereum", "usdt.erc20", "ERC20", "evm", "0xdac17f958d2ee523a2206206994597c13d831ec7", 1, 6},
	{"bsc", "usdt.bep20", "BEP20", "evm", "0x55d398326f99059ff775485246999027b3197955", 56, 18},
	{"polygon", "usdt.polygon", "Polygon", "evm", "0xc2132d05d31c914a87c6611c10748aeb04b58e8f", 137, 6},
	{"arbitrum", "usdt.arbitrum", "Arbitrum One", "evm", "0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9", 42161, 6},
	{"xlayer", "usdt.xlayer", "X-Layer", "evm", "0x779ded0c9e1022225f8e0630b35a9b54be713736", 196, 6},
	{"plasma", "usdt.plasma", "Plasma", "evm", "0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb", 9745, 6},
	{"solana", "usdt.solana", "Solana", "solana", "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", 0, 6},
	{"aptos", "usdt.aptos", "Aptos", "aptos", "0x357b0b74bc833e95a115ad22604854d6b0fca151cecd94111770e5d6ffc9dc2b", 1, 6},
	{"ton", "usdt.ton", "TON", "ton", "0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe", 0, 6},
	// Circle mainnet registry, pinned 2026-10-18. USDC.e and other bridges are excluded.
	{"ethereum", "usdc.erc20", "ERC20", "evm", "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", 1, 6},
	{"polygon", "usdc.polygon", "Polygon", "evm", "0x3c499c542cef5e3811e1192ce70d8cc03d5c3359", 137, 6},
	{"arbitrum", "usdc.arbitrum", "Arbitrum One", "evm", "0xaf88d065e77c8cc2239327c5edb3a432268e5831", 42161, 6},
	{"xlayer", "usdc.xlayer", "X-Layer", "evm", "0xb6ceceab302e2e4948951ee7843fc24e92933061", 196, 6},
	{"plasma", "usdc.plasma", "Plasma", "evm", "0x2d661c89d812261039af9764eceaaee884f5f67f", 9745, 6},
	{"solana", "usdc.solana", "Solana", "solana", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", 0, 6},
	{"aptos", "usdc.aptos", "Aptos", "aptos", "0xbae207659db88bea0cbead6da0ed00aac12edcdda169e591cd41c94180b46f3b", 1, 6},
}

// Network finds a canonical network or payment rail.
func Network(value string) (NetworkSpec, bool) {
	for _, spec := range Registry {
		if spec.Network == value || spec.Rail == value {
			return spec, true
		}
	}
	return NetworkSpec{}, false
}

// DefaultConfig is disabled and contains no public RPCs or receiving addresses.
func DefaultConfig() Config {
	c := Config{Name: "USDT", Mode: "usdt", ExpiryMinutes: 30, Concurrency: 1, Networks: []NetworkConfig{}}
	for _, spec := range Registry {
		if spec.Currency() != "USDT" {
			continue
		}
		c.Networks = append(c.Networks, NetworkConfig{Network: spec.Network, Currencies: []string{"USDT"}, Addresses: []string{}, Endpoints: []Endpoint{}})
	}
	return c
}

// Currency is derived from the immutable rail, never from provider token symbols.
func (s NetworkSpec) Currency() string { return strings.ToUpper(strings.SplitN(s.Rail, ".", 2)[0]) }

// TokenNetwork accepts only pinned native identities on the selected network.
func TokenNetwork(network, token string) (NetworkSpec, bool) {
	for _, spec := range Registry {
		if spec.Network == network && spec.Token == token {
			return spec, true
		}
	}
	return NetworkSpec{}, false
}

func modeAllows(mode string, spec NetworkSpec) bool {
	return mode == "both" || strings.EqualFold(mode, spec.Currency())
}

func containsAddress(addresses []string, address string) bool {
	for _, a := range addresses {
		if a == address {
			return true
		}
	}
	return false
}

func evmTopic(address string) string {
	return "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(address, "0x")
}

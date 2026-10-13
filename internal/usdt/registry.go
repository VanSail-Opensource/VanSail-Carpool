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
	c := Config{Name: "USDT", ExpiryMinutes: 30, Networks: []NetworkConfig{}, Groups: []QuotaGroup{}}
	for _, spec := range Registry {
		c.Networks = append(c.Networks, NetworkConfig{Network: spec.Network, Addresses: []string{}, Endpoints: []Endpoint{}})
	}
	return c
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

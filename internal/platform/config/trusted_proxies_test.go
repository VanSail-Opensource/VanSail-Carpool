package config

import "testing"

func TestTrustedProxyConfigurationDefaultsClosedAndRejectsAmbiguity(t *testing.T) {
	prefixes, err := parseTrustedProxyCIDRs("")
	if err != nil || len(prefixes) != 0 {
		t.Fatal(prefixes, err)
	}
	prefixes, err = parseTrustedProxyCIDRs("127.0.0.1/32, ::1/128, ::ffff:192.0.2.129/120")
	if err != nil || len(prefixes) != 3 || prefixes[2].String() != "192.0.2.0/24" {
		t.Fatal(prefixes, err)
	}
	for _, input := range []string{"proxy.internal", "192.0.2.1", "0.0.0.0/0", "::/0", "::ffff:0.0.0.0/96", "127.0.0.1/32,", "fe80::1%eth0/128"} {
		if _, err := parseTrustedProxyCIDRs(input); err == nil {
			t.Errorf("unsafe proxy configuration accepted: %q", input)
		}
	}
}

func TestLoadWiresExplicitTrustedProxyNetworks(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32,::1/128")
	cfg, err := Load()
	if err != nil || len(cfg.TrustedProxyCIDRs) != 2 {
		t.Fatal(cfg.TrustedProxyCIDRs, err)
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "0.0.0.0/0")
	if _, err = Load(); err == nil {
		t.Fatal("unsafe proxy trust did not fail bootstrap")
	}
}

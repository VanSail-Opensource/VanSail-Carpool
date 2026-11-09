package secret

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConnectionIPTargetUsesOnlyExactRetainedOwnerEvidence(t *testing.T) {
	vault, err := NewVault([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := vault.Encrypt("connection-ip-block:old-owner:node:hash", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	base := ConnectionIPTarget{OwnerID: "current", NodeID: "node", Digest: "hash", Ciphertext: ciphertext}
	for _, test := range []struct {
		name    string
		mutate  func(*ConnectionIPTarget)
		aliases []ConnectionIPOwnerAlias
		valid   bool
	}{
		{"exact", nil, []ConnectionIPOwnerAlias{{"old-owner", "current"}}, true},
		{"wrong_current_owner", func(p *ConnectionIPTarget) { p.OwnerID = "other" }, []ConnectionIPOwnerAlias{{"old-owner", "current"}}, false},
		{"wrong_alias_owner", nil, []ConnectionIPOwnerAlias{{"old-owner", "other"}}, false},
		{"wrong_old_owner", nil, []ConnectionIPOwnerAlias{{"another", "current"}}, false},
		{"changed_node", func(p *ConnectionIPTarget) { p.NodeID = "other" }, []ConnectionIPOwnerAlias{{"old-owner", "current"}}, false},
		{"changed_digest", func(p *ConnectionIPTarget) { p.Digest = "other" }, []ConnectionIPOwnerAlias{{"old-owner", "current"}}, false},
		{"no_evidence", nil, nil, false},
		{"over_bound", nil, make([]ConnectionIPOwnerAlias, MaxConnectionIPOwnerAliases+1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := base
			if test.mutate != nil {
				test.mutate(&target)
			}
			plain, e := vault.OpenConnectionIPTarget(context.Background(), target, func(context.Context, string) ([]ConnectionIPOwnerAlias, error) { return test.aliases, nil })
			defer clear(plain)
			if test.valid {
				if e != nil || string(plain) != "203.0.113.8" {
					t.Fatal("exact historical scope failed", e)
				}
			} else if !errors.Is(e, ErrConnectionIPTarget) || len(plain) != 0 {
				t.Fatal("foreign target authenticated", e)
			}
			if target.Ciphertext != ciphertext {
				t.Fatal("read resolver changed ciphertext")
			}
		})
	}
}

func TestConnectionIPCanonicalScopeNeedsNoAliasReaderAndCancellationFencesPlaintext(t *testing.T) {
	vault, err := NewVault([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := vault.Encrypt("connection-ip-block:current:node:hash", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	target := ConnectionIPTarget{OwnerID: "current", NodeID: "node", Digest: "hash", Ciphertext: ciphertext}
	called := false
	plain, err := vault.OpenConnectionIPTarget(context.Background(), target, func(context.Context, string) ([]ConnectionIPOwnerAlias, error) {
		called = true
		return nil, errors.New("unavailable")
	})
	clear(plain)
	if err != nil || called {
		t.Fatal("canonical target depends on legacy evidence", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	plain, err = OpenConnectionIPTarget(ctx, target, func(string, string) ([]byte, error) { cancel(); return []byte("secret"), nil }, nil)
	if !errors.Is(err, context.Canceled) || len(plain) != 0 {
		t.Fatal("cancelled resolver returned plaintext", err)
	}
}

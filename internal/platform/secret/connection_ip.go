package secret

import (
	"context"
	"errors"
)

// MaxConnectionIPOwnerAliases bounds retained upgrade evidence examined per target.
const MaxConnectionIPOwnerAliases = 64

// ConnectionIPOwnerAlias is retained migration evidence, never native authority.
type ConnectionIPOwnerAlias struct {
	OldOwnerID string
	OwnerID    string
}

// ConnectionIPOwnerLookup reads only aliases belonging to the supplied current owner.
type ConnectionIPOwnerLookup func(context.Context, string) ([]ConnectionIPOwnerAlias, error)

// ConnectionIPTarget preserves every immutable part of a sealed block's scope.
type ConnectionIPTarget struct {
	OwnerID, NodeID, Digest, Ciphertext string
}

// ErrConnectionIPTarget reveals no owner, source address or ciphertext details.
var ErrConnectionIPTarget = errors.New("encrypted IP target could not be verified")

// OpenConnectionIPTarget tries the canonical scope first, then only exact retained
// old-owner evidence. It never changes the target, ciphertext or native subject.
func OpenConnectionIPTarget(ctx context.Context, target ConnectionIPTarget, open func(string, string) ([]byte, error), lookup ConnectionIPOwnerLookup) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if target.OwnerID == "" || target.NodeID == "" || target.Digest == "" || open == nil {
		return nil, ErrConnectionIPTarget
	}
	try := func(owner string) ([]byte, error) {
		return open("connection-ip-block:"+owner+":"+target.NodeID+":"+target.Digest, target.Ciphertext)
	}
	plain, err := try(target.OwnerID)
	if err == nil {
		return checkedIPTarget(ctx, plain)
	}
	clear(plain)
	if lookup == nil {
		return nil, ErrConnectionIPTarget
	}
	aliases, err := lookup(ctx, target.OwnerID)
	if err != nil || len(aliases) > MaxConnectionIPOwnerAliases {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrConnectionIPTarget
	}
	seen := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if alias.OwnerID != target.OwnerID || alias.OldOwnerID == "" || alias.OldOwnerID == target.OwnerID || seen[alias.OldOwnerID] {
			continue
		}
		seen[alias.OldOwnerID] = true
		plain, err = try(alias.OldOwnerID)
		if err == nil {
			return checkedIPTarget(ctx, plain)
		}
		clear(plain)
	}
	return nil, ErrConnectionIPTarget
}

// OpenConnectionIPTarget adapts the vault without exposing plaintext to diagnostics.
func (v *Vault) OpenConnectionIPTarget(ctx context.Context, target ConnectionIPTarget, lookup ConnectionIPOwnerLookup) ([]byte, error) {
	return OpenConnectionIPTarget(ctx, target, func(scope, encoded string) ([]byte, error) {
		plain, err := v.Decrypt(scope, encoded)
		return []byte(plain), err
	}, lookup)
}

func checkedIPTarget(ctx context.Context, plain []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		clear(plain)
		return nil, err
	}
	return plain, nil
}

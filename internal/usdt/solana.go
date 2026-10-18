package usdt

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

type solHistory struct {
	Owner  string `json:"owner"`
	Last   string `json:"last"`
	Before string `json:"before"`
	Newest string `json:"newest"`
}
type solCursor struct {
	Baseline  uint64                `json:"baseline"`
	Accounts  map[string]solHistory `json:"accounts"`
	Position  int                   `json:"position"`
	RefreshAt int64                 `json:"refreshAt"`
}
type solSignature struct {
	Signature    string          `json:"signature"`
	Slot         uint64          `json:"slot"`
	Err          json.RawMessage `json:"err"`
	Confirmation string          `json:"confirmationStatus"`
}

func (s *scanner) checkSolana(ctx context.Context) error {
	var genesis string
	if err := s.client.rpc(ctx, "getGenesisHash", []any{}, &genesis); err != nil {
		return err
	}
	if genesis != "5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d" {
		return ErrProof
	}
	var mint struct {
		Value struct {
			Decimals int `json:"decimals"`
		} `json:"value"`
	}
	if err := s.client.rpc(ctx, "getTokenSupply", []any{s.client.spec.Token, map[string]string{"commitment": "finalized"}}, &mint); err != nil {
		return err
	}
	if mint.Value.Decimals != s.client.spec.Decimals {
		return ErrProof
	}
	return nil
}

func (s *scanner) scanSolana(ctx context.Context, cursor Cursor, owners []string) (Batch, error) {
	cur := solCursor{Accounts: map[string]solHistory{}}
	if cursor.Value != "" && json.Unmarshal([]byte(cursor.Value), &cur) != nil {
		return Batch{}, ErrProof
	}
	if cur.Accounts == nil {
		cur.Accounts = map[string]solHistory{}
	}
	pruneSolanaOwners(&cur, owners)
	if cur.Baseline == 0 {
		if err := s.client.rpc(ctx, "getSlot", []any{map[string]string{"commitment": "finalized"}}, &cur.Baseline); err != nil {
			return Batch{}, err
		}
		if cur.Baseline == 0 {
			return Batch{}, ErrProof
		}
	}
	if time.Now().Unix()-cur.RefreshAt >= 60 {
		accounts, err := s.solanaAccounts(ctx, owners)
		if err != nil {
			return Batch{}, err
		}
		for account, owner := range accounts {
			history := cur.Accounts[account]
			history.Owner = owner
			cur.Accounts[account] = history
		}
		cur.RefreshAt = time.Now().Unix()
	}
	keys := []string{}
	for account, h := range cur.Accounts {
		if containsAddress(owners, h.Owner) {
			keys = append(keys, account)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return Batch{Cursor: Cursor{Value: cursorJSON(cur)}, CaughtUp: true}, nil
	}
	if cur.Position < 0 || cur.Position >= len(keys) {
		cur.Position = 0
	}
	account := keys[cur.Position]
	h := cur.Accounts[account]
	options := map[string]any{"limit": 100, "commitment": "finalized"}
	if h.Last != "" {
		options["until"] = h.Last
	}
	if h.Before != "" {
		options["before"] = h.Before
	}
	var signatures transferPage[solSignature]
	if err := s.client.rpc(ctx, "getSignaturesForAddress", []any{account, options}, &signatures); err != nil {
		return Batch{}, err
	}
	if len(signatures) > 100 {
		return Batch{}, ErrRPC
	}
	if h.Newest == "" && len(signatures) > 0 {
		h.Newest = signatures[0].Signature
	}
	batch := Batch{Transfers: []Transfer{}}
	reachedBaseline := false
	for _, sig := range signatures {
		if sig.Slot <= cur.Baseline && h.Last == "" {
			reachedBaseline = true
			break
		}
		if sig.Confirmation != "finalized" {
			return Batch{}, ErrProof
		}
		if string(sig.Err) != "null" {
			continue
		}
		var tx solTransaction
		if err := s.client.rpc(ctx, "getTransaction", []any{sig.Signature, map[string]any{"encoding": "jsonParsed", "commitment": "finalized", "maxSupportedTransactionVersion": 0}}, &tx); err != nil {
			return Batch{}, err
		}
		transfers, err := decodeSolanaTransaction(s.client.spec, tx, sig.Signature, sig.Slot, owners)
		if err != nil {
			return Batch{}, err
		}
		batch.Transfers = append(batch.Transfers, transfers...)
	}
	complete := len(signatures) < 100 || reachedBaseline
	if complete {
		if h.Newest != "" {
			h.Last = h.Newest
		}
		h.Newest, h.Before = "", ""
		cur.Position = (cur.Position + 1) % len(keys)
	} else {
		h.Before = signatures[len(signatures)-1].Signature
	}
	cur.Accounts[account] = h
	batch.Cursor = Cursor{Value: cursorJSON(cur)}
	batch.CaughtUp = complete && cur.Position == 0
	return batch, nil
}

package usdt

import (
	"context"
	"crypto/sha256"

	"filippo.io/edwards25519"
)

const solanaTokenProgram = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

// associatedTokenAddress also tracks an ATA created and closed between scans.
func associatedTokenAddress(owner, mint string) (string, error) {
	seeds := []byte{}
	for _, key := range []string{owner, solanaTokenProgram, mint} {
		raw, err := decodeBase58(key)
		if err != nil || len(raw) != 32 {
			return "", ErrConfiguration
		}
		seeds = append(seeds, raw...)
	}
	program, err := decodeBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	if err != nil {
		return "", err
	}
	for bump := 255; bump >= 0; bump-- {
		data := append(append(append([]byte(nil), seeds...), byte(bump)), program...)
		data = append(data, []byte("ProgramDerivedAddress")...)
		digest := sha256.Sum256(data)
		if _, err := new(edwards25519.Point).SetBytes(digest[:]); err != nil {
			return encodeBase58(digest[:]), nil
		}
	}
	return "", ErrConfiguration
}

func (s *scanner) solanaAccounts(ctx context.Context, owners []string) (map[string]string, error) {
	result := map[string]string{}
	for _, owner := range owners {
		ata, err := associatedTokenAddress(owner, s.client.spec.Token)
		if err != nil {
			return nil, err
		}
		result[ata] = owner
		var reply struct {
			Value accountPage[struct {
				Pubkey  string `json:"pubkey"`
				Account struct {
					Owner string `json:"owner"`
					Data  struct {
						Parsed struct {
							Type string `json:"type"`
							Info struct {
								Owner string `json:"owner"`
								Mint  string `json:"mint"`
							} `json:"info"`
						} `json:"parsed"`
					} `json:"data"`
				} `json:"account"`
			}] `json:"value"`
		}
		if err := s.client.rpc(ctx, "getTokenAccountsByOwner", []any{owner, map[string]string{"mint": s.client.spec.Token}, map[string]string{"encoding": "jsonParsed", "commitment": "finalized"}}, &reply); err != nil {
			return nil, err
		}
		if len(reply.Value) > 128 {
			return nil, ErrRPC
		}
		for _, item := range reply.Value {
			if item.Account.Owner != solanaTokenProgram || item.Account.Data.Parsed.Type != "account" || item.Account.Data.Parsed.Info.Owner != owner || item.Account.Data.Parsed.Info.Mint != s.client.spec.Token {
				return nil, ErrProof
			}
			if _, err := NormalizeAddress(s.client.spec, item.Pubkey); err != nil {
				return nil, ErrProof
			}
			result[item.Pubkey] = owner
		}
	}
	return result, nil
}

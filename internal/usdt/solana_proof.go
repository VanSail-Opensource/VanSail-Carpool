package usdt

import (
	"encoding/json"
	"strconv"
	"time"
)

type solInstruction struct {
	ProgramID string `json:"programId"`
	Parsed    struct {
		Type string `json:"type"`
		Info struct {
			Destination string `json:"destination"`
			Amount      string `json:"amount"`
			Mint        string `json:"mint"`
			TokenAmount struct {
				Amount   string `json:"amount"`
				Decimals int    `json:"decimals"`
			} `json:"tokenAmount"`
		} `json:"info"`
	} `json:"parsed"`
}
type solBalance struct {
	Index       int    `json:"accountIndex"`
	Mint        string `json:"mint"`
	Owner       string `json:"owner"`
	Program     string `json:"programId"`
	TokenAmount struct {
		Decimals int `json:"decimals"`
	} `json:"uiTokenAmount"`
}
type solTransaction struct {
	Slot        uint64 `json:"slot"`
	BlockTime   int64  `json:"blockTime"`
	Transaction struct {
		Signatures []string `json:"signatures"`
		Message    struct {
			Keys []struct {
				Pubkey string `json:"pubkey"`
			} `json:"accountKeys"`
			Instructions []solInstruction `json:"instructions"`
		} `json:"message"`
	} `json:"transaction"`
	Meta *struct {
		Err   json.RawMessage `json:"err"`
		Pre   []solBalance    `json:"preTokenBalances"`
		Post  []solBalance    `json:"postTokenBalances"`
		Inner []struct {
			Index        int              `json:"index"`
			Instructions []solInstruction `json:"instructions"`
		} `json:"innerInstructions"`
	} `json:"meta"`
}

func decodeSolanaTransaction(spec NetworkSpec, tx solTransaction, signature string, slot uint64, owners []string) ([]Transfer, error) {
	if tx.Slot != slot || tx.Meta == nil || string(tx.Meta.Err) != "null" || tx.BlockTime <= 0 || len(tx.Transaction.Signatures) == 0 || tx.Transaction.Signatures[0] != signature {
		return nil, ErrProof
	}
	accounts := map[string]string{}
	for _, balances := range [][]solBalance{tx.Meta.Pre, tx.Meta.Post} {
		for _, b := range balances {
			if b.Mint != spec.Token {
				continue
			}
			if b.Index < 0 || b.Index >= len(tx.Transaction.Message.Keys) || b.TokenAmount.Decimals != 6 || b.Program != solanaTokenProgram {
				return nil, ErrProof
			}
			key := tx.Transaction.Message.Keys[b.Index].Pubkey
			if old, ok := accounts[key]; ok && old != b.Owner {
				return nil, ErrProof
			}
			accounts[key] = b.Owner
		}
	}
	result := []Transfer{}
	appendInstruction := func(instruction solInstruction, event string) error {
		if instruction.ProgramID != solanaTokenProgram {
			return nil
		}
		info := instruction.Parsed.Info
		owner := accounts[info.Destination]
		if !containsAddress(owners, owner) {
			return nil
		}
		var amount string
		switch instruction.Parsed.Type {
		case "transfer":
			amount = info.Amount
		case "transferChecked":
			if info.Mint != spec.Token || info.TokenAmount.Decimals != 6 {
				return ErrProof
			}
			amount = info.TokenAmount.Amount
		default:
			return nil
		}
		if amount == "0" {
			return nil
		}
		t, err := verifiedTransfer(spec, owner, amount, signature, event, time.Unix(tx.BlockTime, 0).UTC())
		if err != nil {
			return err
		}
		result = append(result, t)
		return nil
	}
	for index, instruction := range tx.Transaction.Message.Instructions {
		if err := appendInstruction(instruction, "outer:"+strconv.Itoa(index)); err != nil {
			return nil, err
		}
	}
	for _, inner := range tx.Meta.Inner {
		if inner.Index < 0 || inner.Index >= len(tx.Transaction.Message.Instructions) {
			return nil, ErrProof
		}
		for index, instruction := range inner.Instructions {
			if err := appendInstruction(instruction, "inner:"+strconv.Itoa(inner.Index)+":"+strconv.Itoa(index)); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

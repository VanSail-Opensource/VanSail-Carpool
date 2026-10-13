package usdt

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type quantity string

func (q *quantity) UnmarshalJSON(raw []byte) error {
	value := string(raw)
	if len(raw) > 0 && raw[0] == '"' {
		if json.Unmarshal(raw, &value) != nil {
			return ErrProof
		}
	}
	if len(value) == 0 || len(value) > 80 {
		return ErrProof
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return ErrProof
		}
	}
	*q = quantity(value)
	return nil
}

type aptosTransaction struct {
	Hash      string   `json:"hash"`
	Success   bool     `json:"success"`
	Version   quantity `json:"version"`
	Timestamp quantity `json:"timestamp"`
	Events    []struct {
		Type string `json:"type"`
		Data struct {
			Store  string   `json:"store"`
			Amount quantity `json:"amount"`
		} `json:"data"`
	} `json:"events"`
}

func decodeAptosActivity(spec NetworkSpec, a aptosActivity, tx aptosTransaction) (Transfer, error) {
	if !tx.Success || tx.Version != a.Version || a.Index < 0 || a.Index >= len(tx.Events) || len(tx.Hash) != 66 {
		return Transfer{}, ErrProof
	}
	event := tx.Events[a.Index]
	store, err := NormalizeAddress(spec, event.Data.Store)
	if err != nil || store != a.Store || event.Type != a.Type || event.Data.Amount != a.Amount {
		return Transfer{}, ErrProof
	}
	micros, err := strconv.ParseInt(string(tx.Timestamp), 10, 64)
	if err != nil || micros <= 0 {
		return Transfer{}, ErrProof
	}
	return verifiedTransfer(spec, a.Owner, string(a.Amount), strings.ToLower(tx.Hash), strconv.Itoa(a.Index), time.UnixMicro(micros).UTC())
}

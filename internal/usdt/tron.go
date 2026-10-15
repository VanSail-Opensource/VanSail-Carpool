package usdt

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

func (s *scanner) checkTron(ctx context.Context) error {
	var genesis struct {
		ID string `json:"blockID"`
	}
	if err := s.client.post(ctx, "/wallet/getblockbynum", map[string]any{"num": 0}, &genesis); err != nil {
		return err
	}
	if genesis.ID != "00000000000000001ebf88508a03865c71d452e25f4d51194196a1d22b6653dc" {
		return ErrProof
	}
	var result struct {
		Result struct {
			OK bool `json:"result"`
		} `json:"result"`
		Constants []string `json:"constant_result"`
	}
	if err := s.client.post(ctx, "/walletsolidity/triggerconstantcontract", map[string]any{"owner_address": s.client.spec.Token, "contract_address": s.client.spec.Token, "function_selector": "decimals()", "visible": true}, &result); err != nil {
		return err
	}
	if !result.Result.OK || len(result.Constants) != 1 {
		return ErrProof
	}
	decimals, err := hexNumber("0x" + result.Constants[0])
	if err != nil || decimals != 6 {
		return ErrProof
	}
	return nil
}

type tronActivity struct {
	TxHash string `json:"transaction_id"`
	Type   string `json:"type"`
	Token  struct {
		Address  string `json:"address"`
		Decimals int    `json:"decimals"`
	} `json:"token_info"`
}

type tronPage struct {
	Success bool                      `json:"success"`
	Data    historyPage[tronActivity] `json:"data"`
	Meta    struct {
		Fingerprint string `json:"fingerprint"`
	} `json:"meta"`
}

func (s *scanner) scanTron(ctx context.Context, cursor Cursor, addresses []string) (Batch, error) {
	p, err := loadTimePage(cursor)
	if err != nil {
		return Batch{}, err
	}
	if p.Address >= len(addresses) {
		return Batch{Cursor: finishTimePage(p), CaughtUp: true}, nil
	}
	q := url.Values{"only_confirmed": {"true"}, "only_to": {"true"}, "contract_address": {s.client.spec.Token}, "limit": {"200"}, "order_by": {"block_timestamp,asc"}, "min_timestamp": {strconv.FormatInt(p.Start*1000, 10)}, "max_timestamp": {strconv.FormatInt(p.End*1000, 10)}}
	if p.Fingerprint != "" {
		q.Set("fingerprint", p.Fingerprint)
	}
	var page tronPage
	if err := s.client.get(ctx, "/v1/accounts/"+addresses[p.Address]+"/transactions/trc20", q, &page); err != nil {
		return Batch{}, err
	}
	if !page.Success {
		return Batch{}, ErrRPC
	}
	batch := Batch{Transfers: []Transfer{}}
	seen := map[string]bool{}
	for _, item := range page.Data {
		if item.Type != "Transfer" || item.Token.Address != s.client.spec.Token || item.Token.Decimals != s.client.spec.Decimals {
			continue
		}
		if seen[item.TxHash] {
			continue
		}
		seen[item.TxHash] = true
		var receipt tronReceipt
		if err := s.client.post(ctx, "/walletsolidity/gettransactioninfobyid", map[string]string{"value": item.TxHash}, &receipt); err != nil {
			return Batch{}, err
		}
		transfers, err := decodeTronReceipt(s.client.spec, receipt, item.TxHash, addresses)
		if err != nil {
			return Batch{}, err
		}
		batch.Transfers = append(batch.Transfers, transfers...)
	}
	if len(page.Data) == 200 {
		if page.Meta.Fingerprint == "" || page.Meta.Fingerprint == p.Fingerprint {
			return Batch{}, ErrRPC
		}
		p.Fingerprint = page.Meta.Fingerprint
	} else {
		p.Address++
		p.Fingerprint = ""
	}
	if p.Address >= len(addresses) {
		batch.Cursor = finishTimePage(p)
		batch.CaughtUp = p.End >= time.Now().UTC().Add(-2*time.Minute).Unix()
	} else {
		batch.Cursor = Cursor{Value: cursorJSON(p)}
	}
	return batch, nil
}

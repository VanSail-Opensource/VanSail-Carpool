package usdt

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

func (s *scanner) checkTON(ctx context.Context) error {
	var chain struct {
		Last struct {
			GlobalID  int `json:"global_id"`
			Workchain int `json:"workchain"`
		} `json:"last"`
	}
	if err := s.client.get(ctx, "/masterchainInfo", nil, &chain); err != nil {
		return err
	}
	if chain.Last.GlobalID != -239 || chain.Last.Workchain != -1 {
		return ErrProof
	}
	var masters struct {
		Items []struct {
			Address string `json:"address"`
			Content struct {
				Decimals string `json:"decimals"`
			} `json:"jetton_content"`
		} `json:"jetton_masters"`
	}
	if err := s.client.get(ctx, "/jetton/masters", url.Values{"address": {s.client.spec.Token}, "limit": {"1"}}, &masters); err != nil {
		return err
	}
	if len(masters.Items) != 1 || masters.Items[0].Content.Decimals != "6" {
		return ErrProof
	}
	address, err := NormalizeAddress(s.client.spec, masters.Items[0].Address)
	if err != nil || address != s.client.spec.Token {
		return ErrProof
	}
	return nil
}

type tonTransfer struct {
	Amount       string   `json:"amount"`
	Destination  string   `json:"destination"`
	Source       string   `json:"source"`
	SourceWallet string   `json:"source_wallet"`
	Master       string   `json:"jetton_master"`
	Aborted      bool     `json:"transaction_aborted"`
	TxHash       string   `json:"transaction_hash"`
	TraceID      string   `json:"trace_id"`
	QueryID      quantity `json:"query_id"`
}

func (s *scanner) scanTON(ctx context.Context, cursor Cursor, owners []string) (Batch, error) {
	p, err := loadTimePage(cursor)
	if err != nil {
		return Batch{}, err
	}
	q := url.Values{"jetton_master": {s.client.spec.Token}, "owner_address": owners, "direction": {"in"}, "sort": {"asc"}, "limit": {"100"}, "offset": {strconv.Itoa(p.Offset)}, "start_utime": {strconv.FormatInt(p.Start, 10)}, "end_utime": {strconv.FormatInt(p.End, 10)}}
	var page struct {
		Items transferPage[tonTransfer] `json:"jetton_transfers"`
	}
	if err := s.client.get(ctx, "/jetton/transfers", q, &page); err != nil {
		return Batch{}, err
	}
	if len(page.Items) > 100 {
		return Batch{}, ErrRPC
	}
	batch := Batch{Transfers: []Transfer{}}
	for _, item := range page.Items {
		if item.Aborted || item.Amount == "0" {
			continue
		}
		owner, err := NormalizeAddress(s.client.spec, item.Destination)
		if err != nil || !containsAddress(owners, owner) {
			return Batch{}, ErrProof
		}
		master, err := NormalizeAddress(s.client.spec, item.Master)
		if err != nil || master != s.client.spec.Token {
			return Batch{}, ErrProof
		}
		var wallets struct {
			Items []struct {
				Address string `json:"address"`
				Owner   string `json:"owner"`
				Master  string `json:"jetton"`
			} `json:"jetton_wallets"`
		}
		if err := s.client.get(ctx, "/jetton/wallets", url.Values{"owner_address": {owner}, "jetton_address": {s.client.spec.Token}, "limit": {"2"}}, &wallets); err != nil {
			return Batch{}, err
		}
		if len(wallets.Items) != 1 {
			return Batch{}, ErrProof
		}
		wallet := wallets.Items[0]
		walletOwner, err := NormalizeAddress(s.client.spec, wallet.Owner)
		if err != nil || walletOwner != owner {
			return Batch{}, ErrProof
		}
		walletMaster, err := NormalizeAddress(s.client.spec, wallet.Master)
		if err != nil || walletMaster != s.client.spec.Token {
			return Batch{}, ErrProof
		}
		walletAddress, err := NormalizeAddress(s.client.spec, wallet.Address)
		if err != nil {
			return Batch{}, err
		}
		var traces struct {
			Items []tonTrace `json:"traces"`
		}
		if err := s.client.get(ctx, "/traces", url.Values{"trace_id": {item.TraceID}, "limit": {"1"}, "include_actions": {"false"}}, &traces); err != nil {
			return Batch{}, err
		}
		if len(traces.Items) != 1 {
			return Batch{}, ErrProof
		}
		t, err := decodeTONTransfer(s.client.spec, item, traces.Items[0], walletAddress, owner)
		if err != nil {
			return Batch{}, err
		}
		batch.Transfers = append(batch.Transfers, t)
	}
	if len(page.Items) == 100 {
		p.Offset += 100
		batch.Cursor = Cursor{Value: cursorJSON(p)}
	} else {
		batch.Cursor = finishTimePage(p)
		batch.CaughtUp = p.End >= time.Now().UTC().Add(-2*time.Minute).Unix()
	}
	return batch, nil
}

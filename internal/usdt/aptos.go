package usdt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

type aptosActivity struct {
	Version quantity `json:"transaction_version"`
	Index   int      `json:"event_index"`
	Owner   string   `json:"owner_address"`
	Asset   string   `json:"asset_type"`
	Store   string   `json:"storage_id"`
	Amount  quantity `json:"amount"`
	Success bool     `json:"is_transaction_success"`
	Type    string   `json:"type"`
}
type aptosCursor struct {
	Version string `json:"version"`
	Index   int    `json:"index"`
}

func (c *client) graphql(ctx context.Context, query string, variables any, out any) error {
	var reply struct {
		Data   json.RawMessage   `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := c.request(ctx, http.MethodPost, c.endpoint.IndexerURL, map[string]any{"query": query, "variables": variables}, &reply); err != nil {
		return err
	}
	if len(reply.Errors) > 0 || len(reply.Data) == 0 || string(reply.Data) == "null" || json.Unmarshal(reply.Data, out) != nil {
		return ErrRPC
	}
	return nil
}

func (s *scanner) checkAptos(ctx context.Context) error {
	var node struct {
		ChainID int `json:"chain_id"`
	}
	if err := s.client.get(ctx, "/v1", nil, &node); err != nil {
		return err
	}
	if node.ChainID != 1 {
		return ErrProof
	}
	var indexed struct {
		Ledgers []struct {
			ChainID int `json:"chain_id"`
		} `json:"ledger_infos"`
		Assets []struct {
			Decimals int `json:"decimals"`
		} `json:"fungible_asset_metadata"`
	}
	if err := s.client.graphql(ctx, `query Token($asset:String!){ledger_infos{chain_id} fungible_asset_metadata(where:{asset_type:{_eq:$asset}}){decimals}}`, map[string]string{"asset": s.client.spec.Token}, &indexed); err != nil {
		return err
	}
	if len(indexed.Ledgers) != 1 || indexed.Ledgers[0].ChainID != 1 || len(indexed.Assets) != 1 || indexed.Assets[0].Decimals != 6 {
		return ErrProof
	}
	return nil
}

func (s *scanner) scanAptos(ctx context.Context, cursor Cursor, addresses []string) (Batch, error) {
	var cur aptosCursor
	if cursor.Value != "" && json.Unmarshal([]byte(cursor.Value), &cur) != nil {
		return Batch{}, ErrProof
	}
	if cur.Version == "" {
		var status struct {
			Processors []struct {
				Version quantity `json:"last_success_version"`
			} `json:"processor_status"`
		}
		if err := s.client.graphql(ctx, `query{processor_status(where:{processor:{_eq:"fungible_asset_processor"}}){last_success_version}}`, nil, &status); err != nil {
			return Batch{}, err
		}
		if len(status.Processors) != 1 {
			return Batch{}, ErrProof
		}
		cur.Version = string(status.Processors[0].Version)
		cur.Index = 2147483647
		return Batch{Cursor: Cursor{Value: cursorJSON(cur)}, CaughtUp: true}, nil
	}
	where := map[string]any{"owner_address": map[string]any{"_in": addresses}, "asset_type": map[string]any{"_eq": s.client.spec.Token}, "is_transaction_success": map[string]any{"_eq": true}, "type": map[string]any{"_in": []string{"0x1::fungible_asset::Deposit", "0x1::fungible_asset::DepositEvent", "0x1::coin::DepositEvent"}}, "_or": []any{map[string]any{"transaction_version": map[string]any{"_gt": cur.Version}}, map[string]any{"transaction_version": map[string]any{"_eq": cur.Version}, "event_index": map[string]any{"_gt": cur.Index}}}}
	var page struct {
		Items []aptosActivity `json:"fungible_asset_activities"`
	}
	if err := s.client.graphql(ctx, `query Transfers($where:fungible_asset_activities_bool_exp!){fungible_asset_activities(where:$where,order_by:[{transaction_version:asc},{event_index:asc}],limit:100){transaction_version event_index owner_address asset_type storage_id amount is_transaction_success type}}`, map[string]any{"where": where}, &page); err != nil {
		return Batch{}, err
	}
	batch := Batch{Transfers: []Transfer{}, CaughtUp: len(page.Items) < 100}
	transactions := map[string]aptosTransaction{}
	for _, a := range page.Items {
		version, err := strconv.ParseUint(string(a.Version), 10, 63)
		if err != nil || !a.Success || a.Asset != s.client.spec.Token || !containsAddress(addresses, a.Owner) {
			return Batch{}, ErrProof
		}
		before, err := strconv.ParseUint(cur.Version, 10, 63)
		if err != nil || version < before || (version == before && a.Index <= cur.Index) {
			return Batch{}, ErrProof
		}
		tx, ok := transactions[string(a.Version)]
		if !ok {
			if err := s.client.get(ctx, "/v1/transactions/by_version/"+string(a.Version), nil, &tx); err != nil {
				return Batch{}, err
			}
			transactions[string(a.Version)] = tx
		}
		if err := s.verifyAptosStore(ctx, a); err != nil {
			return Batch{}, err
		}
		t, err := decodeAptosActivity(s.client.spec, a, tx)
		if err != nil {
			return Batch{}, err
		}
		batch.Transfers = append(batch.Transfers, t)
		cur = aptosCursor{Version: string(a.Version), Index: a.Index}
	}
	batch.Cursor = Cursor{Value: cursorJSON(cur)}
	return batch, nil
}

func (s *scanner) verifyAptosStore(ctx context.Context, a aptosActivity) error {
	q := url.Values{"ledger_version": {string(a.Version)}}
	var core struct {
		Data struct {
			Owner string `json:"owner"`
		} `json:"data"`
	}
	if err := s.client.get(ctx, "/v1/accounts/"+a.Store+"/resource/0x1::object::ObjectCore", q, &core); err != nil {
		return err
	}
	owner, err := NormalizeAddress(s.client.spec, core.Data.Owner)
	if err != nil || owner != a.Owner {
		return ErrProof
	}
	var store struct {
		Data struct {
			Metadata struct {
				Inner string `json:"inner"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if err := s.client.get(ctx, "/v1/accounts/"+a.Store+"/resource/0x1::fungible_asset::FungibleStore", q, &store); err != nil {
		return err
	}
	asset, err := NormalizeAddress(s.client.spec, store.Data.Metadata.Inner)
	if err != nil || asset != s.client.spec.Token {
		return ErrProof
	}
	return nil
}

package usdt

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type evmBlock struct {
	Number    string `json:"number"`
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
}
type evmLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	BlockHash   string   `json:"blockHash"`
	TxHash      string   `json:"transactionHash"`
	Index       string   `json:"logIndex"`
	Removed     bool     `json:"removed"`
}

func (s *scanner) evmHead(ctx context.Context) (evmBlock, error) {
	var b evmBlock
	var err error
	if s.client.spec.Network == "bsc" {
		err = s.client.rpc(ctx, "eth_getFinalizedBlock", []any{-2, false}, &b)
	} else {
		err = s.client.rpc(ctx, "eth_getBlockByNumber", []any{"finalized", false}, &b)
	}
	if err != nil {
		return b, err
	}
	if _, err := hexNumber(b.Number); err != nil || len(b.Hash) != 66 {
		return b, ErrProof
	}
	return b, nil
}

func (s *scanner) checkEVM(ctx context.Context) error {
	c := s.client
	var chainID string
	if err := c.rpc(ctx, "eth_chainId", []any{}, &chainID); err != nil {
		return err
	}
	id, err := hexNumber(chainID)
	if err != nil || id != uint64(c.spec.ChainID) {
		return ErrProof
	}
	var decimals string
	if err := c.rpc(ctx, "eth_call", []any{map[string]any{"to": c.spec.Token, "data": "0x313ce567"}, "finalized"}, &decimals); err != nil {
		return err
	}
	d, err := hexNumber(decimals)
	if err != nil || d != uint64(c.spec.Decimals) {
		return ErrProof
	}
	_, err = s.evmHead(ctx)
	return err
}

func (s *scanner) scanEVM(ctx context.Context, cursor Cursor, addresses []string) (Batch, error) {
	c := s.client
	head, err := s.evmHead(ctx)
	if err != nil {
		return Batch{}, err
	}
	tip, err := hexNumber(head.Number)
	if err != nil {
		return Batch{}, err
	}
	if cursor.Value == "" {
		return Batch{Cursor: Cursor{Value: strconv.FormatUint(tip, 10), Hash: head.Hash}, CaughtUp: true}, nil
	}
	last, err := strconv.ParseUint(cursor.Value, 10, 63)
	if err != nil || last > tip {
		return Batch{}, ErrProof
	}
	var prior evmBlock
	if err := c.rpc(ctx, "eth_getBlockByNumber", []any{toHex(last), false}, &prior); err != nil {
		return Batch{}, err
	}
	if prior.Hash != cursor.Hash {
		return Batch{}, ErrProof
	}
	if last == tip {
		return Batch{Cursor: cursor, CaughtUp: true}, nil
	}
	if s.span == 0 {
		s.span = 256
	}
	end := min(tip, last+s.span)
	topics := []string{}
	for _, a := range addresses {
		topics = append(topics, evmTopic(a))
	}
	var logs []evmLog
	err = c.rpc(ctx, "eth_getLogs", []any{map[string]any{"fromBlock": toHex(last + 1), "toBlock": toHex(end), "address": c.spec.Token, "topics": []any{transferTopic, nil, topics}}}, &logs)
	if err != nil {
		if s.span > 1 {
			s.span = max(1, s.span/2)
		}
		return Batch{}, err
	}
	if len(logs) > 2000 {
		if s.span > 1 {
			s.span = max(1, s.span/2)
		}
		return Batch{}, ErrRPC
	}
	blocks := map[uint64]evmBlock{}
	receipts := map[string]evmReceipt{}
	batch := Batch{Transfers: []Transfer{}}
	for _, log := range logs {
		n, err := hexNumber(log.BlockNumber)
		if err != nil || n <= last || n > end || log.Removed {
			return Batch{}, ErrProof
		}
		block, ok := blocks[n]
		if !ok {
			if err := c.rpc(ctx, "eth_getBlockByNumber", []any{toHex(n), false}, &block); err != nil {
				return Batch{}, err
			}
			blocks[n] = block
		}
		if block.Hash != log.BlockHash {
			return Batch{}, ErrProof
		}
		r, ok := receipts[log.TxHash]
		if !ok {
			if err := c.rpc(ctx, "eth_getTransactionReceipt", []any{log.TxHash}, &r); err != nil {
				return Batch{}, err
			}
			receipts[log.TxHash] = r
		}
		if !receiptContains(r, log) {
			return Batch{}, ErrProof
		}
		t, err := decodeEVMLog(c.spec, log, block)
		if err != nil {
			return Batch{}, err
		}
		if !containsAddress(addresses, t.Recipient) {
			return Batch{}, ErrProof
		}
		batch.Transfers = append(batch.Transfers, t)
	}
	boundary, ok := blocks[end]
	if !ok {
		if end == tip {
			boundary = head
		} else if err := c.rpc(ctx, "eth_getBlockByNumber", []any{toHex(end), false}, &boundary); err != nil {
			return Batch{}, err
		}
	}
	if boundary.Number != toHex(end) || len(boundary.Hash) != 66 {
		return Batch{}, ErrProof
	}
	batch.Cursor = Cursor{Value: strconv.FormatUint(end, 10), Hash: boundary.Hash}
	batch.CaughtUp = end == tip
	s.span = min(1024, s.span*2)
	return batch, nil
}

func decodeEVMLog(spec NetworkSpec, log evmLog, b evmBlock) (Transfer, error) {
	if log.Removed || !strings.EqualFold(log.Address, spec.Token) || len(log.Topics) != 3 || log.Topics[0] != transferTopic || len(log.Topics[2]) != 66 || len(log.Data) != 66 || len(log.TxHash) != 66 {
		return Transfer{}, ErrProof
	}
	amount, err := hexBigInt(log.Data)
	if err != nil || amount.Sign() <= 0 {
		return Transfer{}, ErrProof
	}
	index, err := hexNumber(log.Index)
	if err != nil {
		return Transfer{}, err
	}
	seconds, err := hexNumber(b.Timestamp)
	if err != nil {
		return Transfer{}, err
	}
	return verifiedTransfer(spec, "0x"+log.Topics[2][26:], amount.String(), strings.ToLower(log.TxHash), strconv.FormatUint(index, 10), time.Unix(int64(seconds), 0).UTC())
}

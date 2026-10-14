package usdt

import (
	"math/big"
	"strings"
)

type evmReceipt struct {
	Status    string   `json:"status"`
	BlockHash string   `json:"blockHash"`
	TxHash    string   `json:"transactionHash"`
	Logs      []evmLog `json:"logs"`
}

func receiptContains(r evmReceipt, log evmLog) bool {
	if r.Status != "0x1" || r.BlockHash != log.BlockHash || r.TxHash != log.TxHash {
		return false
	}
	for _, item := range r.Logs {
		if item.Index == log.Index && item.Address == log.Address && item.Data == log.Data && item.TxHash == log.TxHash && item.BlockHash == log.BlockHash && !item.Removed && equalEVMTopics(item.Topics, log.Topics) {
			return true
		}
	}
	return false
}

func equalEVMTopics(left, right []string) bool {
	if len(left) != 3 || len(right) != 3 {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func hexBigInt(raw string) (*big.Int, error) {
	if !strings.HasPrefix(raw, "0x") || len(raw) > 82 {
		return nil, ErrProof
	}
	value, ok := new(big.Int).SetString(raw[2:], 16)
	if !ok {
		return nil, ErrProof
	}
	return value, nil
}

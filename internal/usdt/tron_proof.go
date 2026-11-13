package usdt

import (
	"strconv"
	"strings"
	"time"
)

type tronReceipt struct {
	ID          string `json:"id"`
	BlockNumber int64  `json:"blockNumber"`
	Timestamp   int64  `json:"blockTimeStamp"`
	Result      string `json:"result"`
	Receipt     struct {
		Result string `json:"result"`
	} `json:"receipt"`
	Logs []struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	} `json:"log"`
}

func decodeTronReceipt(spec NetworkSpec, r tronReceipt, hash string, addresses []string) ([]Transfer, error) {
	result := []Transfer{}
	if r.ID != hash || len(hash) != 64 || r.BlockNumber <= 0 || r.Timestamp <= 0 {
		return nil, ErrProof
	}
	// TransactionInfo.result defaults to SUCESS=0 and may be omitted; the
	// contract receipt defaults to DEFAULT=0, while executed SUCCESS is 1.
	switch r.Result {
	case "", "SUCESS", "SUCCESS", "FAILED":
	default:
		return nil, ErrProof
	}
	switch r.Receipt.Result {
	case "SUCCESS":
		if r.Result == "FAILED" {
			return result, nil
		}
	case "REVERT", "BAD_JUMP_DESTINATION", "OUT_OF_MEMORY", "PRECOMPILED_CONTRACT",
		"STACK_TOO_SMALL", "STACK_TOO_LARGE", "ILLEGAL_OPERATION", "STACK_OVERFLOW",
		"OUT_OF_ENERGY", "OUT_OF_TIME", "JVM_STACK_OVER_FLOW", "UNKNOWN",
		"TRANSFER_FAILED", "INVALID_CODE":
		return result, nil
	default:
		return nil, ErrProof
	}
	for index, log := range r.Logs {
		contract, err := tronHexAddress(log.Address)
		if err != nil {
			return nil, err
		}
		if contract != spec.Token || len(log.Topics) != 3 || "0x"+log.Topics[0] != transferTopic {
			continue
		}
		if len(log.Topics[2]) != 64 || len(log.Data) != 64 {
			return nil, ErrProof
		}
		recipient, err := tronHexAddress(log.Topics[2][24:])
		if err != nil {
			return nil, err
		}
		if !containsAddress(addresses, recipient) {
			continue
		}
		amount, err := hexBigInt("0x" + log.Data)
		if err != nil {
			return nil, err
		}
		if amount.Sign() == 0 {
			continue
		}
		t, err := verifiedTransfer(spec, recipient, amount.String(), strings.ToLower(hash), strconv.Itoa(index), time.UnixMilli(r.Timestamp).UTC())
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, nil
}

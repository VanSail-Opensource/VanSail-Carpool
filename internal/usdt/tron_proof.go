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
	if r.Receipt.Result != "SUCCESS" || (r.Result != "" && r.Result != "SUCCESS") {
		return result, nil
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

func timeNowMinusThirty() int64 { return time.Now().UTC().Add(-30 * time.Second).Unix() }

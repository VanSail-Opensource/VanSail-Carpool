package usdt

import (
	"encoding/base64"
	"strconv"
	"time"

	"github.com/xssnick/tonutils-go/tvm/cell"
)

type tonMessage struct {
	Hash        string `json:"hash"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Bounced     bool   `json:"bounced"`
	Opcode      string `json:"opcode"`
	Content     struct {
		Body string `json:"body"`
	} `json:"message_content"`
}
type tonTransaction struct {
	Account     string `json:"account"`
	Hash        string `json:"hash"`
	Now         int64  `json:"now"`
	MCSeqno     int64  `json:"mc_block_seqno"`
	Description struct {
		Aborted bool `json:"aborted"`
		Compute struct {
			Success  bool `json:"success"`
			ExitCode int  `json:"exit_code"`
		} `json:"compute_ph"`
		Action *struct {
			Success    bool `json:"success"`
			ResultCode int  `json:"result_code"`
		} `json:"action"`
	} `json:"description"`
	In  tonMessage   `json:"in_msg"`
	Out []tonMessage `json:"out_msgs"`
}
type tonTrace struct {
	ID         string   `json:"trace_id"`
	Incomplete bool     `json:"is_incomplete"`
	MCSeqno    quantity `json:"mc_seqno_end"`
	Info       struct {
		State   string `json:"trace_state"`
		Pending int    `json:"pending_messages"`
	} `json:"trace_info"`
	Transactions map[string]tonTransaction `json:"transactions"`
}

func decodeTONTransfer(spec NetworkSpec, item tonTransfer, trace tonTrace, wallet, owner string) (Transfer, error) {
	end, err := strconv.ParseInt(string(trace.MCSeqno), 10, 64)
	if err != nil || end <= 0 || trace.ID != item.TraceID || trace.Incomplete || trace.Info.State != "complete" || trace.Info.Pending != 0 {
		return Transfer{}, ErrProof
	}
	sourceWallet, err := NormalizeAddress(spec, item.SourceWallet)
	if err != nil {
		return Transfer{}, ErrProof
	}
	for _, tx := range trace.Transactions {
		account, err := NormalizeAddress(spec, tx.Account)
		if err != nil || account != wallet {
			continue
		}
		if tx.MCSeqno <= 0 || tx.MCSeqno > end || tx.Description.Aborted || !tx.Description.Compute.Success || tx.Description.Compute.ExitCode != 0 || (tx.Description.Action != nil && (!tx.Description.Action.Success || tx.Description.Action.ResultCode != 0)) || tx.In.Bounced || tx.In.Opcode != "0x178d4519" {
			continue
		}
		source, err := NormalizeAddress(spec, tx.In.Source)
		if err != nil || source != sourceWallet {
			continue
		}
		destination, err := NormalizeAddress(spec, tx.In.Destination)
		if err != nil || destination != wallet {
			return Transfer{}, ErrProof
		}
		if !tonSenderMessage(spec, trace, item.TxHash, sourceWallet, tx.In.Hash, wallet) {
			return Transfer{}, ErrProof
		}
		if err := verifyTONBody(spec, tx.In.Content.Body, item); err != nil {
			return Transfer{}, err
		}
		if tx.Hash == "" || tx.In.Hash == "" || tx.Now <= 0 {
			return Transfer{}, ErrProof
		}
		return verifiedTransfer(spec, owner, item.Amount, tx.Hash, tx.In.Hash, time.Unix(tx.Now, 0).UTC())
	}
	return Transfer{}, ErrProof
}

func tonSenderMessage(spec NetworkSpec, trace tonTrace, hash, sourceWallet, msgHash, destination string) bool {
	for _, tx := range trace.Transactions {
		if tx.Hash != hash || tx.Description.Aborted || !tx.Description.Compute.Success {
			continue
		}
		account, err := NormalizeAddress(spec, tx.Account)
		if err != nil || account != sourceWallet {
			continue
		}
		for _, message := range tx.Out {
			if message.Hash == msgHash {
				to, err := NormalizeAddress(spec, message.Destination)
				return err == nil && to == destination
			}
		}
	}
	return false
}

func verifyTONBody(spec NetworkSpec, body string, item tonTransfer) error {
	if len(body) > 16384 {
		return ErrProof
	}
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return ErrProof
	}
	root, err := cell.FromBOC(raw)
	if err != nil {
		return ErrProof
	}
	slice, err := root.BeginParse()
	if err != nil {
		return ErrProof
	}
	op, err := slice.LoadUInt(32)
	if err != nil || op != 0x178d4519 {
		return ErrProof
	}
	query, err := slice.LoadUInt(64)
	if err != nil || strconv.FormatUint(query, 10) != string(item.QueryID) {
		return ErrProof
	}
	amount, err := slice.LoadBigCoins()
	if err != nil || amount.String() != item.Amount {
		return ErrProof
	}
	sender, err := slice.LoadAddr()
	if err != nil || sender == nil {
		return ErrProof
	}
	from, err := NormalizeAddress(spec, sender.String())
	if err != nil {
		return ErrProof
	}
	expected, err := NormalizeAddress(spec, item.Source)
	if err != nil || from != expected {
		return ErrProof
	}
	return nil
}

package usdt

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type scanner struct {
	client       *client
	span         uint64
	checked      bool
	subscription string
	subSocket    *socket
	subRetry     time.Time
}

func (s *scanner) check(ctx context.Context) error {
	var err error
	switch s.client.spec.Family {
	case "evm":
		err = s.checkEVM(ctx)
	case "tron":
		err = s.checkTron(ctx)
	case "solana":
		err = s.checkSolana(ctx)
	case "aptos":
		err = s.checkAptos(ctx)
	case "ton":
		err = s.checkTON(ctx)
	default:
		err = ErrConfiguration
	}
	if err == nil {
		s.checked = true
	}
	return err
}

func (s *scanner) scan(ctx context.Context, c Cursor, addresses []string) (Batch, error) {
	if !s.checked {
		if err := s.check(ctx); err != nil {
			return Batch{}, err
		}
	}
	switch s.client.spec.Family {
	case "evm":
		return s.scanEVM(ctx, c, addresses)
	case "tron":
		return s.scanTron(ctx, c, addresses)
	case "solana":
		return s.scanSolana(ctx, c, addresses)
	case "aptos":
		return s.scanAptos(ctx, c, addresses)
	case "ton":
		return s.scanTON(ctx, c, addresses)
	}
	return Batch{}, ErrConfiguration
}

func hexNumber(raw string) (uint64, error) {
	if !strings.HasPrefix(raw, "0x") {
		return 0, ErrProof
	}
	n, err := strconv.ParseUint(raw[2:], 16, 63)
	if err != nil {
		return 0, ErrProof
	}
	return n, nil
}
func toHex(n uint64) string { return "0x" + strconv.FormatUint(n, 16) }

func verifiedTransfer(spec NetworkSpec, recipient, atomic, hash, event string, occurred time.Time) (Transfer, error) {
	address, err := NormalizeAddress(spec, recipient)
	if err != nil {
		return Transfer{}, ErrProof
	}
	amount, err := TransferAmount(atomic, spec.Decimals)
	if err != nil {
		return Transfer{}, err
	}
	if occurred.IsZero() || occurred.After(time.Now().UTC().Add(2*time.Minute)) {
		return Transfer{}, ErrProof
	}
	return Transfer{Network: spec.Network, Token: spec.Token, Recipient: address, AtomicAmount: atomic, Amount: amount, TxHash: hash, EventID: event, OccurredAt: occurred, Finalized: true, Verified: true}, nil
}

func cursorJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

// timePage freezes a bounded time window while all pages/recipients are read.
// The overlap covers late index visibility without permitting duplicate credit.
type timePage struct {
	Start       int64  `json:"start"`
	End         int64  `json:"end"`
	Address     int    `json:"address"`
	Offset      int    `json:"offset"`
	Fingerprint string `json:"fingerprint"`
	Completed   int64  `json:"completed"`
}

func loadTimePage(c Cursor) (timePage, error) {
	var p timePage
	if c.Value != "" && json.Unmarshal([]byte(c.Value), &p) != nil {
		return p, ErrProof
	}
	if p.End == 0 {
		p.End = time.Now().UTC().Add(-30 * time.Second).Unix()
		p.Start = p.Completed - 1800
		if p.Completed == 0 {
			p.Start = p.End - 1800
		}
		if p.Start < 0 {
			p.Start = 0
		}
		// Never jump past an outage; advance at most one hour per completed window.
		if p.Completed > 0 && p.End > p.Completed+3600 {
			p.End = p.Completed + 3600
		}
	}
	return p, nil
}
func finishTimePage(p timePage) Cursor { return Cursor{Value: cursorJSON(timePage{Completed: p.End})} }

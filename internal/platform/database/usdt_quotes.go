package database

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

const usdtQuoteSelect = "SELECT order_id,network,token,recipient,amount,atomic_amount,expires_at FROM usdt_quotes"

func scanUSDTQuote(row rowScanner) (usdt.Quote, error) {
	var q usdt.Quote
	var expires string
	err := row.Scan(&q.OrderID, &q.Network, &q.Token, &q.Recipient, &q.Amount, &q.AtomicAmount, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return q, ErrNotFound
	}
	if err != nil {
		return q, err
	}
	q.ExpiresAt, err = parseStamp(expires)
	return q, err
}

// USDTQuote returns the permanent fingerprint for an idempotent create retry.
func (s *Store) USDTQuote(ctx context.Context, orderID string) (usdt.Quote, error) {
	quote, err := scanUSDTQuote(s.db.QueryRowContext(ctx, usdtQuoteSelect+" WHERE order_id=$1", orderID))
	if errors.Is(err, ErrNotFound) {
		return quote, usdt.ErrQuoteNotFound
	}
	return quote, err
}

// ReserveUSDTQuote commits the fingerprint and checkout together before displaying it.
func (s *Store) retryAttempt_ReserveUSDTQuote(ctx context.Context, orderID string, spec usdt.NetworkSpec, addresses []string, base string) (usdt.Quote, error) {
	pinned, ok := usdt.Network(spec.Rail)
	if !ok || pinned != spec {
		return usdt.Quote{}, usdt.ErrProof
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return usdt.Quote{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, err := scanUSDTQuote(tx.QueryRowContext(ctx, usdtQuoteSelect+" WHERE order_id=$1", orderID)); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return usdt.Quote{}, err
	}
	order, err := paymentOrderTx(ctx, tx, orderID)
	if err != nil {
		return usdt.Quote{}, err
	}
	if order.Provider != usdt.Provider || order.ProviderRail != spec.Rail || order.PayableCurrency != spec.Currency() || order.Status != "creating" || order.CancelledAt != nil || order.PayableAmount != base {
		return usdt.Quote{}, usdt.ErrConflict
	}
	millis, err := usdt.Atomic(base, 3)
	if err != nil {
		return usdt.Quote{}, err
	}
	counts := map[string]int{}
	rows, err := tx.QueryContext(ctx, "SELECT recipient,COUNT(*) FROM usdt_quotes WHERE network=$1 AND token=$2 GROUP BY recipient", spec.Network, spec.Token)
	if err != nil {
		return usdt.Quote{}, err
	}
	for rows.Next() {
		var address string
		var count int
		if err := rows.Scan(&address, &count); err != nil {
			_ = rows.Close()
			return usdt.Quote{}, err
		}
		counts[address] = count
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil {
		return usdt.Quote{}, rowErr
	}
	if closeErr != nil {
		return usdt.Quote{}, closeErr
	}
	pool := append([]string(nil), addresses...)
	sort.SliceStable(pool, func(i, j int) bool { return counts[pool[i]] < counts[pool[j]] })
	q := usdt.Quote{OrderID: orderID, Network: spec.Network, Token: spec.Token, ExpiresAt: order.ExpiresAt}
	if !time.Now().UTC().Before(q.ExpiresAt) {
		return usdt.Quote{}, usdt.ErrUnavailable
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(spec.Decimals-3)), nil)
	for _, address := range pool {
		for offset := int64(0); offset <= 99; offset++ {
			value := new(big.Int).Add(millis, big.NewInt(offset))
			q.Recipient = address
			q.Amount = usdt.FormatAtomic(value, 3)
			q.AtomicAmount = new(big.Int).Mul(value, factor).String()
			result, err := tx.ExecContext(ctx, `INSERT   INTO usdt_quotes(order_id,network,token,recipient,amount,atomic_amount,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING `, q.OrderID, q.Network, q.Token, q.Recipient, q.Amount, q.AtomicAmount, stamp(q.ExpiresAt))
			if err != nil {
				return usdt.Quote{}, err
			}
			inserted, err := result.RowsAffected()
			if err != nil {
				return usdt.Quote{}, err
			}
			if inserted == 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='pending',provider_trade_id=$1,receiving_address=$2,actual_crypto_amount=$3,actual_crypto_currency=$4,expires_at=$5,updated_at=$6 WHERE id=$7 AND status='creating'`, "builtin:"+orderID, q.Recipient, q.Amount, spec.Currency(), stamp(q.ExpiresAt), stamp(time.Now().UTC()), orderID); err != nil {
				return usdt.Quote{}, err
			}
			if err := tx.Commit(); err != nil {
				return usdt.Quote{}, err
			}
			return q, nil
		}
	}
	return usdt.Quote{}, usdt.ErrCapacity
}

// USDTWatchAddresses retains unpaid quote recipients across setting changes.
func (s *Store) USDTWatchAddresses(ctx context.Context, network string) ([]string, error) {
	spec, ok := usdt.Network(network)
	if !ok {
		return nil, usdt.ErrConfiguration
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT quote.recipient FROM usdt_quotes quote JOIN payment_orders payment ON payment.id=quote.order_id WHERE quote.network=$1 AND quote.token=$2 AND payment.paid_at IS NULL AND payment.status<>'failed' ORDER BY quote.recipient`, spec.Network, spec.Token)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	addresses := []string{}
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}
	return addresses, rows.Err()
}

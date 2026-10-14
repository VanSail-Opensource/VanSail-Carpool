package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func paymentSuccessAnnouncementTx(ctx context.Context, tx *sql.Tx, order model.PaymentOrder) (string, error) {
	username, err := paymentAnnouncementUsernameTx(ctx, tx, order.UserID)
	if err != nil {
		return "", err
	}
	providerName, err := paymentAnnouncementProviderNameTx(ctx, tx, order)
	if err != nil {
		return "", err
	}
	if order.Provider == "builtin_usdt" && order.ActualCryptoAmount != nil {
		order.PayableAmount = *order.ActualCryptoAmount
	}
	payload, err := json.Marshal(jobpayload.PaymentSuccessAnnouncement{
		OrderID: order.ID, Provider: order.Provider, ProviderName: providerName, Channel: order.ProviderRail,
		TXBMinor: order.TXBMinor, PayableAmount: order.PayableAmount,
		PayableCurrency: order.PayableCurrency, Username: username,
	})
	if err != nil {
		return "", fmt.Errorf("encode payment success announcement: %w", err)
	}
	return string(payload), nil
}

func paymentAnnouncementUsernameTx(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var telegramUsername string
	var localUsername sql.NullString
	var telegramID int64
	if err := tx.QueryRowContext(ctx, `SELECT telegram_username,username,telegram_id FROM users WHERE id=?`, userID).
		Scan(&telegramUsername, &localUsername, &telegramID); err != nil {
		return "", fmt.Errorf("load payment announcement username: %w", err)
	}
	if username := strings.TrimSpace(telegramUsername); username != "" {
		return "@" + strings.TrimLeft(username, "@"), nil
	}
	if localUsername.Valid && strings.TrimSpace(localUsername.String) != "" {
		return strings.TrimSpace(localUsername.String), nil
	}
	return "telegram:" + strconv.FormatInt(telegramID, 10), nil
}

func paymentAnnouncementProviderNameTx(ctx context.Context, tx *sql.Tx, order model.PaymentOrder) (string, error) {
	parts := strings.Split(strings.TrimSpace(order.MethodID), ":")
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" {
		return "", nil
	}
	var providerName string
	err := tx.QueryRowContext(ctx, `SELECT provider_name FROM payment_profiles WHERE id=? AND provider=?`, parts[1], order.Provider).
		Scan(&providerName)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load payment announcement provider name: %w", err)
	}
	return strings.TrimSpace(providerName), nil
}

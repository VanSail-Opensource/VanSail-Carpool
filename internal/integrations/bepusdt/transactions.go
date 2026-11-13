package bepusdt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxExpirationSeconds = int64((1<<63 - 1) / time.Second)

type CreateTransactionRequest struct {
	OrderID        string
	Amount         string
	Fiat           string
	TradeType      string
	Address        string
	Name           string
	NotifyURL      string
	RedirectURL    string
	TimeoutSeconds int64
	Rate           string
}

// Transaction is the payment instruction returned by BEPusdt.

type Transaction struct {
	Fiat           string
	TradeID        string
	OrderID        string
	Amount         string
	ActualAmount   string
	Status         int
	Token          string
	ExpirationTime int64
}

// ExpiresAt converts BEPusdt's expiration_time duration (seconds) into an instant.
// Invalid or unrepresentable durations return the zero time.

func (t Transaction) ExpiresAt(createdAt time.Time) time.Time {
	if t.ExpirationTime <= 0 || t.ExpirationTime > maxExpirationSeconds {
		return time.Time{}
	}
	return createdAt.Add(time.Duration(t.ExpirationTime) * time.Second)
}

// CreateTransaction creates a fixed-fiat BEPusdt transaction.

func (c *Client) CreateTransaction(ctx context.Context, input CreateTransactionRequest) (*Transaction, error) {
	if strings.TrimSpace(input.OrderID) == "" {
		return nil, errors.New("bepusdt order id is empty")
	}
	if err := validatePositiveDecimal(input.Amount); err != nil {
		return nil, fmt.Errorf("bepusdt amount: %w", err)
	}
	if input.Fiat == "" {
		input.Fiat = "USD"
	}
	if input.TradeType == "" {
		input.TradeType = "usdt.trc20"
	}
	if err := validateCallbackURL(input.NotifyURL); err != nil {
		return nil, fmt.Errorf("bepusdt notify URL: %w", err)
	}
	if err := validateCallbackURL(input.RedirectURL); err != nil {
		return nil, fmt.Errorf("bepusdt redirect URL: %w", err)
	}
	if input.TimeoutSeconds != 0 && input.TimeoutSeconds < 120 {
		return nil, errors.New("bepusdt timeout must be at least 120 seconds")
	}
	if input.TimeoutSeconds > maxExpirationSeconds {
		return nil, errors.New("bepusdt timeout exceeds supported duration")
	}

	values := map[string]string{
		"order_id": input.OrderID, "amount": providerNumberString(input.Amount),
		"fiat": input.Fiat, "trade_type": input.TradeType,
		"notify_url": input.NotifyURL, "redirect_url": input.RedirectURL,
		"address": input.Address, "name": input.Name, "rate": input.Rate,
	}
	if input.TimeoutSeconds > 0 {
		values["timeout"] = strconv.FormatInt(input.TimeoutSeconds, 10)
	}
	wire := map[string]any{
		"order_id": input.OrderID, "amount": json.Number(input.Amount),
		"fiat": input.Fiat, "trade_type": input.TradeType,
		"notify_url": input.NotifyURL, "redirect_url": input.RedirectURL,
		"signature": Sign(values, c.token),
	}
	for _, key := range []string{"address", "name", "rate"} {
		if values[key] != "" {
			wire[key] = values[key]
		}
	}
	if input.TimeoutSeconds > 0 {
		wire["timeout"] = input.TimeoutSeconds
	}

	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("bepusdt encode request: %w", err)
	}
	target := *c.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + "/api/v1/order/create-transaction"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("bepusdt create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("bepusdt request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("bepusdt read response: %w", err)
	}
	if len(responseBody) > maxResponseBytes {
		return nil, fmt.Errorf("bepusdt response exceeds %d bytes", maxResponseBytes)
	}

	var envelope struct {
		StatusCode flexibleInt `json:"status_code"`
		Message    string      `json:"message"`
		RequestID  string      `json:"request_id"`
		Data       struct {
			Fiat           string         `json:"fiat"`
			TradeID        string         `json:"trade_id"`
			OrderID        string         `json:"order_id"`
			Amount         flexibleString `json:"amount"`
			ActualAmount   flexibleString `json:"actual_amount"`
			Status         flexibleInt    `json:"status"`
			Token          string         `json:"token"`
			ExpirationTime flexibleInt    `json:"expiration_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, fmt.Errorf("bepusdt decode response (http=%d): %w", response.StatusCode, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || int(envelope.StatusCode) != http.StatusOK {
		return nil, &APIError{HTTPStatus: response.StatusCode, StatusCode: int(envelope.StatusCode), Message: envelope.Message, RequestID: envelope.RequestID}
	}
	transaction := &Transaction{
		Fiat: envelope.Data.Fiat, TradeID: envelope.Data.TradeID, OrderID: envelope.Data.OrderID,
		Amount: string(envelope.Data.Amount), ActualAmount: string(envelope.Data.ActualAmount),
		Status: int(envelope.Data.Status), Token: envelope.Data.Token,
		ExpirationTime: int64(envelope.Data.ExpirationTime),
	}
	if transaction.TradeID == "" || transaction.OrderID == "" || transaction.Token == "" {
		return nil, errors.New("bepusdt success response is missing payment data")
	}
	if transaction.OrderID != input.OrderID || !strings.EqualFold(transaction.Fiat, input.Fiat) || !decimalEquivalent(transaction.Amount, input.Amount) {
		return nil, errors.New("bepusdt success response does not match the requested order")
	}
	if err := validatePositiveDecimal(transaction.ActualAmount); err != nil {
		return nil, fmt.Errorf("bepusdt success response actual amount: %w", err)
	}
	if transaction.ExpirationTime <= 0 || transaction.ExpirationTime > maxExpirationSeconds {
		return nil, errors.New("bepusdt success response expiration is invalid")
	}
	return transaction, nil
}

// CancelTransaction calls BEPusdt's signed cancellation endpoint for a direct
// transaction. A paid notification may race with this operation and remains
// authoritative to the caller.

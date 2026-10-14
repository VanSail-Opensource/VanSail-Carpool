package bepusdt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DiscoveryRequest defines the short-lived cashier order used to query methods.
type DiscoveryRequest struct {
	OrderID, NotifyURL, RedirectURL string
}

// AvailableMethod is one currency/network pair returned for a cashier order.
type AvailableMethod struct {
	Currency, Network, NetworkName string
}

// DiscoverMethods creates a probe order and returns its currently available rails.
func (c *Client) DiscoverMethods(ctx context.Context, input DiscoveryRequest) ([]AvailableMethod, error) {
	tradeID, err := c.createDiscoveryOrder(ctx, input)
	if err != nil {
		return nil, err
	}
	return c.availableMethods(ctx, tradeID)
}

func (c *Client) createDiscoveryOrder(ctx context.Context, input DiscoveryRequest) (string, error) {
	if strings.TrimSpace(input.OrderID) == "" {
		return "", errors.New("bepusdt discovery order id is empty")
	}
	values := map[string]string{
		"order_id": input.OrderID, "amount": "1", "currencies": "USDT,USDC", "fiat": "USD",
		"name": "VanSail Carpool payment method discovery", "notify_url": input.NotifyURL,
		"redirect_url": input.RedirectURL, "timeout": "180", "reselect": "false",
	}
	payload := map[string]any{
		"order_id": input.OrderID, "amount": 1, "currencies": "USDT,USDC", "fiat": "USD",
		"name": values["name"], "notify_url": input.NotifyURL, "redirect_url": input.RedirectURL,
		"timeout": 180, "reselect": false, "signature": Sign(values, c.token),
	}
	var envelope struct {
		StatusCode flexibleInt `json:"status_code"`
		Message    string      `json:"message"`
		RequestID  string      `json:"request_id"`
		Data       struct {
			TradeID string `json:"trade_id"`
			OrderID string `json:"order_id"`
		} `json:"data"`
	}
	status, err := c.postJSON(ctx, "/api/v1/order/create-order", payload, &envelope)
	if err != nil {
		return "", err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices || int(envelope.StatusCode) != http.StatusOK {
		return "", &APIError{HTTPStatus: status, StatusCode: int(envelope.StatusCode), Message: envelope.Message, RequestID: envelope.RequestID}
	}
	if envelope.Data.TradeID == "" || envelope.Data.OrderID != input.OrderID {
		return "", errors.New("bepusdt discovery response is missing order data")
	}
	return envelope.Data.TradeID, nil
}

func (c *Client) availableMethods(ctx context.Context, tradeID string) ([]AvailableMethod, error) {
	var envelope struct {
		StatusCode flexibleInt `json:"status_code"`
		Message    string      `json:"message"`
		RequestID  string      `json:"request_id"`
		Data       struct {
			Methods []struct {
				Currency    string `json:"currency"`
				Network     string `json:"network"`
				NetworkName string `json:"token_net_name"`
			} `json:"methods"`
		} `json:"data"`
	}
	status, err := c.postJSON(ctx, "/api/v1/pay/methods", map[string]string{"trade_id": tradeID}, &envelope)
	if err != nil {
		return nil, err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices || int(envelope.StatusCode) != http.StatusOK {
		return nil, &APIError{HTTPStatus: status, StatusCode: int(envelope.StatusCode), Message: envelope.Message, RequestID: envelope.RequestID}
	}
	methods := make([]AvailableMethod, 0, len(envelope.Data.Methods))
	for _, method := range envelope.Data.Methods {
		methods = append(methods, AvailableMethod{Currency: method.Currency, Network: method.Network, NetworkName: method.NetworkName})
	}
	return methods, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload, target any) (int, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("bepusdt encode request: %w", err)
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(encoded))
	if err != nil {
		return 0, fmt.Errorf("bepusdt create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("bepusdt request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return response.StatusCode, fmt.Errorf("bepusdt read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return response.StatusCode, fmt.Errorf("bepusdt response exceeds %d bytes", maxResponseBytes)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return response.StatusCode, fmt.Errorf("bepusdt decode response (http=%d): %w", response.StatusCode, err)
	}
	return response.StatusCode, nil
}

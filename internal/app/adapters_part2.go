package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/ezpay"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (a paymentAdapter) bepusdtClient(ctx context.Context, profileID, rail string) (*bepusdt.Client, error) {
	var profile model.PaymentProfileRuntime
	var err error
	if profileID != "" {
		profile, err = a.settings.PaymentProfileByID(ctx, profileID, rail)
		if err != nil {
			return nil, fmt.Errorf("load BEPusdt payment profile %q for %s: %w", profileID, rail, err)
		}
		if profile.Provider != "bepusdt" {
			return nil, fmt.Errorf("payment profile %q belongs to %q, not bepusdt", profileID, profile.Provider)
		}
	} else {
		profile, err = a.settings.PaymentProfile(ctx, "bepusdt", rail)
		if err != nil {
			baseURL, fallbackErr := a.settings.Plaintext(ctx, "billing.bepusdt.base_url")
			if fallbackErr != nil {
				return nil, err
			}
			token, tokenErr := a.settings.Plaintext(ctx, "billing.bepusdt.api_token")
			if tokenErr != nil {
				return nil, tokenErr
			}
			return bepusdt.NewClient(baseURL, token)
		}
	}
	return bepusdt.NewClient(profile.Endpoint, profile.CredentialPlaintext)
}

func (a paymentAdapter) VerifyEZPay(ctx context.Context, values url.Values) (billing.ProviderEvent, bool, error) {
	if profiles, profilesErr := a.settings.PaymentProfileRuntimes(ctx, "ezpay"); profilesErr == nil && len(profiles) > 0 {
		var lastErr error
		for _, profile := range profiles {
			client, clientErr := ezpay.NewClient(profile.Endpoint, profile.MerchantID, profile.CredentialPlaintext)
			if clientErr != nil {
				lastErr = clientErr
				continue
			}
			notification, parseErr := client.ParseNotification(values)
			if parseErr != nil {
				lastErr = parseErr
				continue
			}
			return ezpayEvent(notification, values, profile.ID), notification.Successful(), nil
		}
		if lastErr != nil {
			return billing.ProviderEvent{}, false, lastErr
		}
	}
	client, err := a.ezpayClient(ctx, "", values.Get("type"))
	if err != nil {
		return billing.ProviderEvent{}, false, err
	}
	notification, err := client.ParseNotification(values)
	if err != nil {
		return billing.ProviderEvent{}, false, err
	}
	return ezpayEvent(notification, values, ""), notification.Successful(), nil
}

func ezpayEvent(notification *ezpay.Notification, values url.Values, profileID string) billing.ProviderEvent {
	digest := sha256.Sum256([]byte(values.Encode()))
	return billing.ProviderEvent{Provider: "ezpay", ProfileID: profileID, Rail: string(notification.Type), OrderID: notification.OutTradeNo, TradeID: notification.TradeNo,
		PayableAmount: notification.Money, PayableCurrency: "CNY", DedupeKey: notification.TradeNo, PayloadHash: hex.EncodeToString(digest[:])}
}

func (a paymentAdapter) VerifyBEPusdt(ctx context.Context, body []byte) (billing.ProviderEvent, int, error) {
	if profiles, profilesErr := a.settings.PaymentProfileRuntimes(ctx, "bepusdt"); profilesErr == nil && len(profiles) > 0 {
		var lastErr error
		for _, profile := range profiles {
			client, clientErr := bepusdt.NewClient(profile.Endpoint, profile.CredentialPlaintext)
			if clientErr != nil {
				lastErr = clientErr
				continue
			}
			webhook, verifyErr := client.ParseAndVerifyWebhook(body)
			if verifyErr != nil {
				lastErr = verifyErr
				continue
			}
			return bepusdtEvent(webhook, body, profile.ID)
		}
		if lastErr != nil {
			return billing.ProviderEvent{}, 0, lastErr
		}
	}
	client, err := a.bepusdtClient(ctx, "", "usdt.trc20")
	if err != nil {
		return billing.ProviderEvent{}, 0, err
	}
	webhook, err := client.ParseAndVerifyWebhook(body)
	if err != nil {
		return billing.ProviderEvent{}, 0, err
	}
	return bepusdtEvent(webhook, body, "")
}

func bepusdtEvent(webhook *bepusdt.Webhook, body []byte, profileID string) (billing.ProviderEvent, int, error) {
	digest := sha256.Sum256(body)
	dedupe := webhook.BlockTransactionID
	if dedupe == "" {
		dedupe = webhook.TradeID + ":" + strconv.Itoa(webhook.Status)
	}
	event := billing.ProviderEvent{Provider: "bepusdt", ProfileID: profileID, OrderID: webhook.OrderID, TradeID: webhook.TradeID,
		ChargeID: webhook.BlockTransactionID, PayableAmount: webhook.ActualAmount,
		FiatAmount: webhook.Amount, FiatCurrency: "USD", Recipient: webhook.Token, DedupeKey: dedupe, PayloadHash: hex.EncodeToString(digest[:])}
	return event, webhook.Status, nil
}

func (a paymentAdapter) VerifyBEPusdtUnsigned(_ context.Context, body []byte) (billing.ProviderEvent, int, error) {
	webhook, err := bepusdt.ParseUnsignedWebhook(body)
	if err != nil {
		return billing.ProviderEvent{}, 0, err
	}
	digest := sha256.Sum256(body)
	dedupe := webhook.BlockTransactionID
	if dedupe == "" {
		dedupe = webhook.OrderID + ":" + strconv.Itoa(webhook.Status) + ":" + hex.EncodeToString(digest[:8])
	}
	event := billing.ProviderEvent{Provider: "bepusdt", OrderID: webhook.OrderID, TradeID: webhook.TradeID,
		ChargeID: webhook.BlockTransactionID, PayableAmount: webhook.ActualAmount,
		FiatAmount: webhook.Amount, FiatCurrency: "USD", Recipient: webhook.Token, DedupeKey: dedupe, PayloadHash: hex.EncodeToString(digest[:])}
	return event, webhook.Status, nil
}

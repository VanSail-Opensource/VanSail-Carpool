package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/ezpay"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"strconv"
	"strings"
	"time"
)

type initDataAdapter struct{ verifier *telegram.InitDataVerifier }

func (a initDataAdapter) Validate(raw string) (model.TelegramProfile, error) {
	data, err := a.verifier.Verify(raw)
	if err != nil {
		return model.TelegramProfile{}, err
	}
	return model.TelegramProfile{ID: data.User.ID, FirstName: data.User.FirstName, LastName: data.User.LastName,
		Username: data.User.Username, LanguageCode: data.User.LanguageCode}, nil
}

type telegramAdapter struct{ client *queuedTelegram }

func (a telegramAdapter) CreateJoinRequestInvite(ctx context.Context, chatID, name string, expiresAt time.Time) (string, error) {
	invite, err := a.client.CreateJoinRequestInvite(ctx, chatID, name, expiresAt)
	if err != nil {
		return "", err
	}
	return invite.InviteLink, nil
}

func (a telegramAdapter) GetMembership(ctx context.Context, chatID string, telegramID int64) (bool, error) {
	member, err := a.client.GetChatMember(ctx, chatID, telegramID)
	if err != nil {
		return false, err
	}
	return member.Present(), nil
}

func (a telegramAdapter) ApproveJoinRequest(ctx context.Context, chatID string, telegramID int64) error {
	return a.client.ApproveJoinRequest(ctx, chatID, telegramID)
}

func (a telegramAdapter) DeclineJoinRequest(ctx context.Context, chatID string, telegramID int64) error {
	return a.client.DeclineJoinRequest(ctx, chatID, telegramID)
}

func (a telegramAdapter) RevokeInviteLink(ctx context.Context, chatID, inviteLink string) error {
	_, err := a.client.RevokeInviteLink(ctx, chatID, inviteLink)
	return err
}

type paymentAdapter struct {
	usdt     *usdt.Service
	settings *admin.SettingsService
	telegram *queuedTelegram
	queue    *upstreamqueue.Queue
	users    interface {
		UserByID(context.Context, string) (model.User, error)
	}
}

func (a paymentAdapter) Create(ctx context.Context, request billing.ProviderCreateRequest) (billing.ProviderCheckout, error) {
	switch request.Provider {
	case "builtin_usdt":
		if a.usdt == nil {
			return billing.ProviderCheckout{}, usdt.ErrUnavailable
		}
		q, err := a.usdt.Create(ctx, request.OrderID, request.Rail, request.PayableAmount)
		if err != nil {
			return billing.ProviderCheckout{}, err
		}
		spec, ok := usdt.TokenNetwork(q.Network, q.Token)
		if !ok {
			return billing.ProviderCheckout{}, usdt.ErrProof
		}
		tradeID, currency := "builtin:"+q.OrderID, spec.Currency()
		return billing.ProviderCheckout{TradeID: &tradeID, ReceivingAddress: &q.Recipient, ActualCryptoAmount: &q.Amount, ActualCryptoCurrency: &currency, PayableAmount: request.PayableAmount, PayableCurrency: currency, ExpiresAt: q.ExpiresAt}, nil
	case "ezpay":
		return a.createEZPay(ctx, request)
	case "bepusdt":
		return a.createBEPusdt(ctx, request)
	case "stars":
		return a.createStars(ctx, request)
	default:
		return billing.ProviderCheckout{}, errors.New("unsupported provider")
	}
}

func (a paymentAdapter) RefundProvider(ctx context.Context, order model.PaymentOrder) error {
	if order.Provider != "stars" || order.Status == "refunded" {
		return nil
	}
	if order.ProviderTradeID == nil || *order.ProviderTradeID == "" {
		return errors.New("stars payment has no Telegram charge id")
	}
	user, err := a.users.UserByID(ctx, order.UserID)
	if err != nil {
		return err
	}
	return a.telegram.RefundStarPayment(ctx, user.TelegramID, *order.ProviderTradeID)
}

func (a paymentAdapter) createEZPay(ctx context.Context, request billing.ProviderCreateRequest) (billing.ProviderCheckout, error) {
	client, err := a.ezpayClient(ctx, request.ProfileID, request.Rail)
	if err != nil {
		return billing.ProviderCheckout{}, err
	}
	checkoutURL, err := client.CheckoutURL(ezpay.CheckoutRequest{Type: ezpay.PaymentType(request.Rail), NotifyURL: request.NotifyURL,
		ReturnURL: request.ReturnURL, OutTradeNo: request.OrderID, Name: "TXB balance", Money: request.PayableAmount})
	if err != nil {
		return billing.ProviderCheckout{}, err
	}
	return billing.ProviderCheckout{PaymentURL: &checkoutURL, PayableAmount: request.PayableAmount,
		PayableCurrency: "CNY", ExpiresAt: time.Now().UTC().Add(30 * time.Minute)}, nil
}

func (a paymentAdapter) createBEPusdt(ctx context.Context, request billing.ProviderCreateRequest) (billing.ProviderCheckout, error) {
	client, err := a.bepusdtClient(ctx, request.ProfileID, request.Rail)
	if err != nil {
		return billing.ProviderCheckout{}, err
	}
	transaction, err := upstreamqueue.Do(upstreamqueue.WithKey(ctx, "order:"+request.OrderID), a.queue, func(callCtx context.Context) (*bepusdt.Transaction, error) {
		return client.CreateTransaction(callCtx, bepusdt.CreateTransactionRequest{OrderID: request.OrderID, Amount: request.PayableAmount,
			Fiat: "USD", TradeType: request.Rail, Name: "TXB balance", NotifyURL: request.NotifyURL, RedirectURL: request.RedirectURL, TimeoutSeconds: 1200})
	})
	if err != nil {
		return billing.ProviderCheckout{}, err
	}
	return mapBEPusdtCheckout(transaction, request.PayableAmount, request.Rail, time.Now().UTC()), nil
}

func mapBEPusdtCheckout(transaction *bepusdt.Transaction, payableAmount, rail string, createdAt time.Time) billing.ProviderCheckout {
	tradeID, address := transaction.TradeID, transaction.Token
	actualAmount, actualCurrency := transaction.ActualAmount, strings.ToUpper(strings.SplitN(rail, ".", 2)[0])
	expiresAt := transaction.ExpiresAt(createdAt)
	return billing.ProviderCheckout{TradeID: &tradeID, ReceivingAddress: &address,
		ActualCryptoAmount: &actualAmount, ActualCryptoCurrency: &actualCurrency, PayableAmount: payableAmount,
		PayableCurrency: "USD", ExpiresAt: expiresAt}
}

func (a paymentAdapter) Cancel(ctx context.Context, order model.PaymentOrder) error {
	if order.Provider != "bepusdt" || order.ProviderTradeID == nil || *order.ProviderTradeID == "" {
		return nil
	}
	_, profileID, rail, parseErr := billing.ParseMethodSelection(order.MethodID)
	if parseErr != nil {
		profileID, rail = "", order.ProviderRail
	}
	client, err := a.bepusdtClient(ctx, profileID, rail)
	if err != nil {
		return err
	}
	return upstreamqueue.Execute(upstreamqueue.WithKey(ctx, "order:"+order.ID), a.queue, func(callCtx context.Context) error {
		return client.CancelTransaction(callCtx, *order.ProviderTradeID)
	})
}

func (a paymentAdapter) createStars(ctx context.Context, request billing.ProviderCreateRequest) (billing.ProviderCheckout, error) {
	amount, err := strconv.ParseInt(request.PayableAmount, 10, 64)
	if err != nil || amount <= 0 {
		return billing.ProviderCheckout{}, errors.New("invalid Stars amount")
	}
	link, err := a.telegram.CreateStarsInvoiceLink(ctx, telegram.StarsInvoiceRequest{Title: "TXB balance", Description: "Add TXB to your VanSail Carpool balance",
		Payload: request.OrderID, Label: "TXB credit", Amount: amount})
	if err != nil {
		return billing.ProviderCheckout{}, err
	}
	return billing.ProviderCheckout{PaymentURL: &link, PayableAmount: request.PayableAmount,
		PayableCurrency: "XTR", ExpiresAt: time.Now().UTC().Add(30 * time.Minute)}, nil
}

func (a paymentAdapter) ezpayClient(ctx context.Context, profileID, rail string) (*ezpay.Client, error) {
	var profile model.PaymentProfileRuntime
	var err error
	if profileID != "" {
		profile, err = a.settings.PaymentProfileByID(ctx, profileID, rail)
	} else {
		profile, err = a.settings.PaymentProfile(ctx, "ezpay", rail)
	}
	if err != nil {
		baseURL, fallbackErr := a.settings.Plaintext(ctx, "billing.ezpay.base_url")
		if fallbackErr != nil {
			return nil, err
		}
		merchantID, merchantErr := a.settings.Plaintext(ctx, "billing.ezpay.merchant_id")
		if merchantErr != nil {
			return nil, merchantErr
		}
		key, keyErr := a.settings.Plaintext(ctx, "billing.ezpay.key")
		if keyErr != nil {
			return nil, keyErr
		}
		return ezpay.NewClient(baseURL, merchantID, key)
	}
	return ezpay.NewClient(profile.Endpoint, profile.MerchantID, profile.CredentialPlaintext)
}

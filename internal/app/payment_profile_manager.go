package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

// PaymentProfileManager probes BEPUSDT profiles and publishes process-local rails.
type PaymentProfileManager struct {
	settings *admin.SettingsService
	cache    *billing.PaymentChannelCache
	queue    *upstreamqueue.Queue
	telegram *queuedTelegram
	users    *database.Store
	admins   []int64
	public   *url.URL
	logger   *slog.Logger
}

func newPaymentProfileManager(settings *admin.SettingsService, cache *billing.PaymentChannelCache,
	queue *upstreamqueue.Queue, telegram *queuedTelegram, users *database.Store, admins []int64, public *url.URL, logger *slog.Logger) *PaymentProfileManager {
	return &PaymentProfileManager{settings: settings, cache: cache, queue: queue, telegram: telegram,
		users: users, admins: append([]int64(nil), admins...), public: public, logger: logger}
}

// RefreshAll probes every enabled BEPUSDT profile during startup.
func (m *PaymentProfileManager) RefreshAll(ctx context.Context) error {
	profiles, err := m.settings.PaymentProfiles(ctx)
	if err != nil {
		return err
	}
	var refreshErrors []error
	for _, profile := range profiles {
		if profile.Provider != "bepusdt" {
			continue
		}
		if _, refreshErr := m.RefreshPaymentProfile(ctx, profile.ID); refreshErr != nil {
			refreshErrors = append(refreshErrors, refreshErr)
		}
	}
	return errors.Join(refreshErrors...)
}

// RefreshPaymentProfile replaces one profile's discovered channels.
func (m *PaymentProfileManager) RefreshPaymentProfile(ctx context.Context, id string) (model.PaymentProfile, error) {
	runtime, err := m.settings.PaymentProfileByID(ctx, id, "")
	if err != nil {
		return model.PaymentProfile{}, err
	}
	if runtime.Provider != "bepusdt" || !runtime.Enabled {
		m.cache.Remove(id)
		return runtime.PaymentProfile, nil
	}
	channels, err := m.discover(ctx, runtime)
	if err != nil {
		return m.disableFailed(ctx, runtime.PaymentProfile, err)
	}
	m.cache.Replace(id, channels)
	runtime.EnabledChannels = m.cache.PaymentProfileRails(id)
	return runtime.PaymentProfile, nil
}

// DeletePaymentProfile removes an unused profile and its process-local rails.
func (m *PaymentProfileManager) DeletePaymentProfile(ctx context.Context, id string) error {
	if err := m.settings.DeletePaymentProfile(ctx, id); err != nil {
		return err
	}
	m.cache.Remove(id)
	return nil
}

func (m *PaymentProfileManager) discover(ctx context.Context, profile model.PaymentProfileRuntime) ([]model.PaymentChannel, error) {
	client, err := bepusdt.NewClient(profile.Endpoint, profile.CredentialPlaintext)
	if err != nil {
		return nil, err
	}
	probeID, err := ids.New()
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	methods, err := upstreamqueue.Do(probeCtx, m.queue, func(callCtx context.Context) ([]bepusdt.AvailableMethod, error) {
		return client.DiscoverMethods(callCtx, bepusdt.DiscoveryRequest{OrderID: "profile-probe-" + probeID,
			NotifyURL: m.absolute("/api/v1/webhooks/bepusdt/probe"), RedirectURL: m.absolute("/")})
	})
	if err != nil {
		return nil, err
	}
	return discoveredPaymentChannels(methods)
}

func (m *PaymentProfileManager) absolute(path string) string {
	result := *m.public
	result.Path = strings.TrimRight(result.Path, "/") + path
	return result.String()
}

func (m *PaymentProfileManager) disableFailed(ctx context.Context, profile model.PaymentProfile, cause error) (model.PaymentProfile, error) {
	m.cache.Remove(profile.ID)
	disabled, disableErr := m.settings.DisablePaymentProfile(ctx, profile.ID)
	notifyErr := m.notifyFailure(ctx, profile, cause)
	return disabled, errors.Join(fmt.Errorf("discover BEPUSDT profile %q: %w", profile.ID, cause), disableErr, notifyErr)
}

func (m *PaymentProfileManager) notifyFailure(ctx context.Context, profile model.PaymentProfile, cause error) error {
	var sendErrors []error
	for _, adminID := range m.admins {
		locale := "en"
		if m.users != nil {
			if user, err := m.users.UserByTelegramID(ctx, adminID); err == nil {
				locale = user.NotificationLocale
			} else if !errors.Is(err, database.ErrNotFound) {
				m.logger.Warn("load administrator notification locale", "telegram_id", adminID, "error", err)
			}
		}
		if err := m.telegram.SendMarkdownV2Message(ctx, adminID, 0, paymentProfileFailureMessage(profile, cause, locale)); err != nil {
			sendErrors = append(sendErrors, err)
			m.logger.Error("notify administrator about BEPUSDT profile failure", "telegram_id", adminID, "profile_id", profile.ID, "error", err)
		}
	}
	return errors.Join(sendErrors...)
}

func paymentProfileFailureMessage(profile model.PaymentProfile, cause error, locale string) string {
	title, profileLabel, reasonLabel := "BEPUSDT payment profile disabled", "Profile", "Reason"
	if locale == "zh-CN" {
		title, profileLabel, reasonLabel = "BEPUSDT 支付配置已停用", "配置", "原因"
	}
	body := "*" + telegramformat.Escape(title) + "*\n*" + telegramformat.Escape(profileLabel) + ":* " + telegramformat.Escape(profile.ProviderName) +
		"\n*" + telegramformat.Escape(reasonLabel) + ":* " + telegramformat.Escape(cause.Error())
	return telegramformat.Limit(body)
}

func discoveredPaymentChannels(methods []bepusdt.AvailableMethod) ([]model.PaymentChannel, error) {
	byRail := make(map[string]model.PaymentChannel, len(methods))
	for _, method := range methods {
		rail, err := method.TradeType()
		if err != nil {
			continue
		}
		byRail[rail] = model.PaymentChannel{Rail: rail, Currency: strings.ToUpper(strings.TrimSpace(method.Currency)),
			Network: strings.ToLower(strings.TrimSpace(method.Network)), NetworkName: strings.TrimSpace(method.NetworkName)}
	}
	channels := make([]model.PaymentChannel, 0, len(byRail))
	for _, rail := range billing.PaymentChannels("bepusdt") {
		if channel, ok := byRail[rail]; ok {
			channels = append(channels, channel)
		}
	}
	if len(channels) == 0 {
		return nil, errors.New("BEPUSDT returned no supported USDT or USDC channels")
	}
	return channels, nil
}

package model

import (
	"strconv"
	"strings"
	"time"
)

// AccountName is a stable technical account reference, independent of display.
func (u User) AccountName() string { return strconv.FormatInt(u.TelegramID, 10) }

func (u User) DisplayName() string {
	if name := strings.TrimSpace(u.TelegramUsername); name != "" {
		return "@" + strings.TrimLeft(name, "@")
	}
	if name := strings.TrimSpace(u.TelegramFirstName + " " + u.TelegramLastName); name != "" {
		return name
	}
	return u.AccountName()
}

// TelegramProfile is the trusted subset of a validated Mini App user.
type TelegramProfile struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	Username     string `json:"telegramUsername"`
	LanguageCode string `json:"languageCode"`
}

// User is a VanSail Carpool account.
type User struct {
	ID                      string     `json:"id"`
	TelegramID              int64      `json:"telegramId"`
	TelegramFirstName       string     `json:"firstName"`
	TelegramLastName        string     `json:"lastName"`
	TelegramUsername        string     `json:"telegramUsername"`
	Username                *string    `json:"username"`
	Role                    string     `json:"role"`
	OnboardingState         string     `json:"onboardingState"`
	GroupJoined             bool       `json:"groupJoined"`
	ChannelJoined           bool       `json:"channelJoined"`
	PolicyAcceptedAt        *time.Time `json:"policyAcceptedAt"`
	AgreementRevision       int        `json:"agreementRevision"`
	ProxyUserID             *string    `json:"-"`
	RecoveryReason          string     `json:"recoveryReason"`
	NewUser                 bool       `json:"-"`
	InviterID               *int64     `json:"-"`
	NotificationLocale      string     `json:"-"`
	DisplayCurrency         string     `json:"displayCurrency"`
	AutoTrafficResetEnabled bool       `json:"-"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

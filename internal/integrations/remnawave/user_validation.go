package remnawave

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

var requiredUserFields = []string{
	"id", "shortUuid", "username", "status", "trafficLimitBytes", "trafficLimitStrategy", "expireAt",
	"telegramId", "email", "description", "tag", "hwidDeviceLimit", "externalSquadUuid",
	"trojanPassword", "vlessUuid", "ssPassword", "lastTriggeredThreshold", "subRevokedAt",
	"lastTrafficResetAt", "createdAt", "updatedAt", "subscriptionUrl", "activeInternalSquads", "userTraffic",
}

var requiredTrafficFields = []string{
	"usedTrafficBytes", "lifetimeUsedTrafficBytes", "onlineAt", "firstConnectedAt", "lastConnectedNodeUuid",
}

const maxJSONSafeInteger = int64(9_007_199_254_740_991)

type wireUser User

// UnmarshalJSON enforces the required Remnawave v3.3.0 UserResponseDto
// surface. Compatible credentials remain private for the encrypted migration
// adapter and are excluded from every JSON serialization of User.
func (user *User) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("decode Remnawave user fields: %w", err)
	}
	if err := requireFields(fields, requiredUserFields); err != nil {
		return err
	}
	var decoded wireUser
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("decode Remnawave user: %w", err)
	}
	if err := validateUserIdentity(decoded); err != nil {
		return err
	}
	var vlessUUID, trojanPassword, ssPassword string
	if json.Unmarshal(fields["vlessUuid"], &vlessUUID) != nil || json.Unmarshal(fields["trojanPassword"], &trojanPassword) != nil || json.Unmarshal(fields["ssPassword"], &ssPassword) != nil {
		return errors.New("Remnawave user protocol credentials have invalid types")
	}
	if _, err := uuid.Parse(vlessUUID); err != nil || trojanPassword == "" || ssPassword == "" {
		return errors.New("Remnawave user protocol credentials are invalid")
	}
	if err := validateDiscardedUserFields(fields); err != nil {
		return err
	}
	var traffic map[string]json.RawMessage
	if err := json.Unmarshal(fields["userTraffic"], &traffic); err != nil {
		return errors.New("Remnawave user traffic is invalid")
	}
	if err := requireFields(traffic, requiredTrafficFields); err != nil {
		return fmt.Errorf("Remnawave user traffic: %w", err)
	}
	*user = User(decoded)
	user.VLESSUUID, user.TrojanPassword, user.ShadowsocksPassword = vlessUUID, trojanPassword, ssPassword
	return nil
}

func validateDiscardedUserFields(fields map[string]json.RawMessage) error {
	var email, tag *string
	var hwidDeviceLimit *int64
	var lastTriggeredThreshold int64
	if json.Unmarshal(fields["email"], &email) != nil || json.Unmarshal(fields["tag"], &tag) != nil ||
		json.Unmarshal(fields["hwidDeviceLimit"], &hwidDeviceLimit) != nil ||
		json.Unmarshal(fields["lastTriggeredThreshold"], &lastTriggeredThreshold) != nil {
		return errors.New("Remnawave discarded user fields have invalid types")
	}
	if hwidDeviceLimit != nil && (*hwidDeviceLimit < -maxJSONSafeInteger || *hwidDeviceLimit > maxJSONSafeInteger) ||
		lastTriggeredThreshold < -maxJSONSafeInteger || lastTriggeredThreshold > maxJSONSafeInteger {
		return errors.New("Remnawave discarded user integer is outside the reference range")
	}
	return nil
}

func requireFields(fields map[string]json.RawMessage, required []string) error {
	for _, name := range required {
		if _, present := fields[name]; !present {
			return fmt.Errorf("Remnawave response is missing required field %q", name)
		}
	}
	return nil
}

func validateUserIdentity(user wireUser) error {
	if user.ID <= 0 || strings.TrimSpace(user.ShortUUID) == "" || strings.TrimSpace(user.Username) == "" || user.TrafficLimitBytes < 0 || user.ExpireAt.IsZero() || user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() || user.ActiveInternalSquads == nil {
		return errors.New("Remnawave user identity fields are invalid")
	}
	switch user.Status {
	case UserStatusActive, UserStatusDisabled, UserStatusLimited, UserStatusExpired:
	default:
		return errors.New("Remnawave user status is invalid")
	}
	switch user.TrafficLimitStrategy {
	case TrafficNoReset, TrafficDaily, TrafficWeekly, TrafficMonthly, TrafficMonthlyRolling:
	default:
		return errors.New("Remnawave traffic strategy is invalid")
	}
	target, err := url.Parse(user.SubscriptionURL)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return errors.New("Remnawave subscription URL is invalid")
	}
	for _, squad := range user.ActiveInternalSquads {
		if _, err := uuid.Parse(squad.UUID); err != nil || strings.TrimSpace(squad.Name) == "" {
			return errors.New("Remnawave active squad is invalid")
		}
	}
	return nil
}

package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/google/uuid"
)

type connectivitySource struct{ adapter remnaAdapter }

func connectivityFailure(code string) error {
	return &connectivity.CodeError{Code: "CONNECTIVITY_" + code}
}

func safeConnectivityUser(user *remnawave.User) (connectivity.User, error) {
	if user == nil || user.ID <= 0 || user.ShortUUID == "" || user.Status != remnawave.UserStatusActive || !user.ExpireAt.After(time.Now()) {
		return connectivity.User{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	return connectivity.User{ID: user.ID, Username: user.Username, Status: string(user.Status)}, nil
}

func (s connectivitySource) Resolve(ctx context.Context, username string) (connectivity.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 100 {
		return connectivity.User{}, connectivityFailure("INVALID_USERNAME")
	}
	return remnaCall(ctx, s.adapter, func(callCtx context.Context, client remnaClient) (connectivity.User, error) {
		user, exists, err := client.FindUserByUsername(callCtx, username)
		if err != nil || !exists || user == nil || user.Username != username {
			return connectivity.User{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
		}
		return safeConnectivityUser(user)
	})
}

func (s connectivitySource) Validate(ctx context.Context, userID int64) error {
	_, err := remnaCall(ctx, s.adapter, func(callCtx context.Context, client remnaClient) (connectivity.User, error) {
		user, err := client.GetUserByID(callCtx, userID)
		if err != nil || user == nil || user.ID != userID {
			return connectivity.User{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
		}
		return safeConnectivityUser(user)
	})
	return err
}

func (s connectivitySource) Load(ctx context.Context, userID int64) (connectivity.User, []connectivity.Target, error) {
	type loaded struct {
		user    connectivity.User
		targets []connectivity.Target
	}
	result, err := remnaCall(ctx, s.adapter, func(callCtx context.Context, client remnaClient) (loaded, error) {
		user, err := client.GetUserByID(callCtx, userID)
		if err != nil || user == nil || user.ID != userID {
			return loaded{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
		}
		safe, err := safeConnectivityUser(user)
		if err != nil {
			return loaded{}, err
		}
		rawClient, ok := client.(interface {
			GetRawSubscription(context.Context, string) (*remnawave.RawSubscription, error)
		})
		if !ok {
			return loaded{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		raw, err := rawClient.GetRawSubscription(callCtx, user.ShortUUID)
		if err != nil || raw == nil || raw.User.ID != userID || raw.User.Status != remnawave.UserStatusActive {
			return loaded{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		if check := raw.ConvertedUserInfo.HWIDCheckup; check != nil && !check.SubscriptionAllowed {
			return loaded{}, connectivityFailure("HWID_RESTRICTED")
		}
		targets, err := connectivityTargets(raw.ResolvedProxyConfigs)
		return loaded{user: safe, targets: targets}, err
	})
	return result.user, result.targets, err
}

func connectivityTargets(configs []json.RawMessage) ([]connectivity.Target, error) {
	targets := make([]connectivity.Target, 0, len(configs))
	seen := make(map[string]bool, len(configs))
	for _, raw := range configs {
		var item struct {
			FinalRemark string `json:"finalRemark"`
			Address     string `json:"address"`
			Port        int    `json:"port"`
			Metadata    struct {
				UUID       string `json:"uuid"`
				IsDisabled bool   `json:"isDisabled"`
			} `json:"metadata"`
		}
		if json.Unmarshal(raw, &item) != nil {
			return nil, connectivityFailure("INVALID_SUBSCRIPTION")
		}
		if item.Metadata.IsDisabled {
			continue
		}
		hostID, err := uuid.Parse(item.Metadata.UUID)
		if err != nil || hostID == uuid.Nil || seen[hostID.String()] {
			return nil, connectivityFailure("INVALID_SUBSCRIPTION")
		}
		seen[hostID.String()] = true
		targets = append(targets, connectivity.Target{HostUUID: hostID.String(), Remark: item.FinalRemark, Address: item.Address, Port: item.Port, Resolved: raw})
	}
	return targets, nil
}

var _ connectivity.Source = connectivitySource{}

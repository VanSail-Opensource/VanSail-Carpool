package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

type memberRawClient interface {
	GetRawSubscription(context.Context, string) (*remnawave.RawSubscription, error)
}
type memberKeysClient interface {
	GetConnectionKeys(context.Context, int64) (*remnawave.ConnectionKeys, error)
}

// LoadMemberSubscription uses the same admission queue as every Remnawave read.
func (a remnaAdapter) LoadMemberSubscription(ctx context.Context, remoteID string, enabled []string, links bool) (connectivity.MemberSubscription, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return connectivity.MemberSubscription{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	return remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (connectivity.MemberSubscription, error) {
		user, err := client.GetUserByID(callCtx, userID)
		if err != nil || user == nil || user.ID != userID || user.ShortUUID == "" {
			return connectivity.MemberSubscription{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
		}
		rawClient, ok := client.(memberRawClient)
		if !ok {
			return connectivity.MemberSubscription{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		raw, err := rawClient.GetRawSubscription(callCtx, user.ShortUUID)
		if err != nil || raw == nil || raw.User.ID != userID || raw.User.ShortUUID != user.ShortUUID {
			return connectivity.MemberSubscription{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		if raw.User.Status != remnawave.UserStatusActive || !user.ExpireAt.After(time.Now()) {
			return connectivity.MemberSubscription{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
		}
		hosts, err := client.ListHosts(callCtx)
		if err != nil {
			return connectivity.MemberSubscription{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		squads, err := client.ListInternalSquads(callCtx)
		if err != nil {
			return connectivity.MemberSubscription{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		nodes, err := client.ListNodes(callCtx)
		if err != nil {
			return connectivity.MemberSubscription{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		var keys *remnawave.ConnectionKeys
		if links {
			if keysClient, exists := client.(memberKeysClient); exists {
				keys, err = keysClient.GetConnectionKeys(callCtx, userID)
				if err != nil {
					return connectivity.MemberSubscription{}, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
				}
			}
		}
		result, err := projectMemberInventory(raw.ResolvedProxyConfigs, user.ActiveInternalSquads, enabled, hosts, squads, nodes, keys)
		if err != nil {
			return connectivity.MemberSubscription{}, err
		}
		if check := raw.ConvertedUserInfo.HWIDCheckup; check != nil && !check.SubscriptionAllowed {
			for index := range result.Hosts {
				result.Hosts[index].Link = nil
				result.Hosts[index].LinkErrorCode = "CONNECTIVITY_HWID_RESTRICTED"
			}
		}
		if links {
			current, err := client.GetUserByID(callCtx, userID)
			if err != nil || current == nil || !sameSubscriptionCredentials(user, current) {
				return connectivity.MemberSubscription{}, connectivityFailure("INTERRUPTED")
			}
			result.SubscriptionURL = &current.SubscriptionURL
		}
		return result, nil
	})
}

func sameSubscriptionCredentials(before, after *remnawave.User) bool {
	return before.ID == after.ID && before.ShortUUID == after.ShortUUID && before.SubscriptionURL == after.SubscriptionURL &&
		sameRevocationTime(before.SubRevokedAt, after.SubRevokedAt) &&
		after.Status == remnawave.UserStatusActive && after.ExpireAt.After(time.Now()) && sameRemoteSquads(before.ActiveInternalSquads, after.ActiveInternalSquads)
}

func sameRevocationTime(before, after *time.Time) bool {
	if before == nil || after == nil {
		return before == nil && after == nil
	}
	return before.Equal(*after)
}

func sameRemoteSquads(left, right []remnawave.SquadSummary) bool {
	if len(left) != len(right) {
		return false
	}
	set := map[string]bool{}
	for _, squad := range left {
		set[squad.UUID] = true
	}
	for _, squad := range right {
		if !set[squad.UUID] {
			return false
		}
	}
	return true
}

type memberResolvedHost struct {
	FinalRemark string `json:"finalRemark"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Metadata    struct {
		UUID                     string `json:"uuid"`
		ConfigProfileInboundUUID string `json:"configProfileInboundUuid"`
		IsDisabled               bool   `json:"isDisabled"`
		ViewPosition             int64  `json:"viewPosition"`
	} `json:"metadata"`
}

func decodeMemberHost(raw json.RawMessage) (memberResolvedHost, error) {
	var host memberResolvedHost
	if json.Unmarshal(raw, &host) != nil || host.Metadata.UUID == "" || host.Metadata.ConfigProfileInboundUUID == "" {
		return host, connectivityFailure("INVALID_SUBSCRIPTION")
	}
	return host, nil
}

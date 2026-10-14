package httpapi

import (
	"sort"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type activityGameResponse struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Icon                string `json:"icon"`
	Description         string `json:"description"`
	Enabled             bool   `json:"enabled"`
	WinChanceBPS        int    `json:"winChanceBps"`
	MinimumStakeMinor   string `json:"minimumStakeMinor"`
	MaximumStakeMinor   string `json:"maximumStakeMinor"`
	ReturnMultiplierBPS int64  `json:"returnMultiplierBps"`
}

type luckyDrawResponse struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	FeeTXBMinor string                  `json:"feeTxbMinor"`
	Enabled     bool                    `json:"enabled"`
	Prizes      []luckyDrawPrizePreview `json:"prizes"`
}

type luckyDrawPrizePreview struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type activityRewardResponse struct {
	Kind                    activity.RewardKind `json:"kind"`
	TXBDeltaMinor           string              `json:"txbDeltaMinor,omitempty"`
	CouponID                string              `json:"couponId,omitempty"`
	ExtensionDays           int                 `json:"extensionDays,omitempty"`
	ResolvedValue           *int64              `json:"resolvedValue,omitempty"`
	ComboID                 string              `json:"comboId,omitempty"`
	SquadUUIDs              []string            `json:"squadUuids,omitempty"`
	RenewalPriceMinor       int64               `json:"renewalPriceMinor,omitempty"`
	TrafficLimitBytes       int64               `json:"trafficLimitBytes,omitempty"`
	RolloverMinRemainingBPS int                 `json:"rolloverMinRemainingBps,omitempty"`
	IncludeInRenewal        bool                `json:"includeInRenewal,omitempty"`
	DiscountMode            string              `json:"discountMode,omitempty"`
}

type activityResultResponse struct {
	ID            string                 `json:"id"`
	Kind          string                 `json:"kind"`
	Outcome       string                 `json:"outcome"`
	Message       string                 `json:"message"`
	Reward        activityRewardResponse `json:"reward"`
	StakeTXBMinor string                 `json:"stakeTxbMinor,omitempty"`
	DrawID        string                 `json:"drawId,omitempty"`
	PrizeID       string                 `json:"prizeId,omitempty"`
	PrizeName     string                 `json:"prizeName,omitempty"`
	BalanceAfter  model.Money            `json:"balanceAfter"`
	CreatedAt     time.Time              `json:"createdAt"`
}

type activityOverviewResponse struct {
	GroupBoost             activity.GroupBoostStatus         `json:"groupBoost"`
	Balance                model.Money                       `json:"balance"`
	TimeZone               string                            `json:"timeZone"`
	CheckedInToday         bool                              `json:"checkedInToday"`
	DailyRewardMinTXBMinor string                            `json:"dailyRewardMinTxbMinor"`
	DailyRewardMaxTXBMinor string                            `json:"dailyRewardMaxTxbMinor"`
	Games                  []activityGameResponse            `json:"games"`
	Draws                  []luckyDrawResponse               `json:"draws"`
	RecentResults          []activityResultResponse          `json:"recentResults"`
	GroupMessageReward     activity.GroupMessageRewardStatus `json:"groupMessageReward"`
}

func mapActivityGame(game activity.Game) activityGameResponse {
	return activityGameResponse{
		ID: game.ID, Name: game.Name, Icon: game.Icon, Description: game.Description, Enabled: game.Enabled,
		WinChanceBPS: game.WinChanceBPS, MinimumStakeMinor: strconv.FormatInt(game.MinimumStakeMinor, 10),
		MaximumStakeMinor: strconv.FormatInt(game.MaximumStakeMinor, 10), ReturnMultiplierBPS: game.ReturnMultiplierBPS,
	}
}

func mapLuckyDraw(draw activity.LuckyDraw) luckyDrawResponse {
	prizes := make([]luckyDrawPrizePreview, 0, len(draw.Prizes))
	for _, prize := range draw.Prizes {
		prizes = append(prizes, luckyDrawPrizePreview{ID: prize.ID, Name: prize.Name})
	}
	return luckyDrawResponse{ID: draw.ID, Name: draw.Name, Description: draw.Description, FeeTXBMinor: strconv.FormatInt(draw.FeeMinor, 10), Enabled: draw.Enabled, Prizes: prizes}
}

func mapActivityReward(reward activity.Reward) activityRewardResponse {
	result := activityRewardResponse{Kind: reward.Kind, CouponID: reward.CouponID, ExtensionDays: reward.ExtensionDays,
		ResolvedValue: reward.ResolvedValue, ComboID: reward.ComboID, SquadUUIDs: reward.SquadUUIDs,
		RenewalPriceMinor: reward.RenewalPriceMinor, TrafficLimitBytes: reward.TrafficLimitBytes,
		RolloverMinRemainingBPS: reward.RolloverMinRemainingBPS, IncludeInRenewal: reward.IncludeInRenewal, DiscountMode: reward.DiscountMode}
	if reward.Kind == activity.RewardTXBDelta {
		value := reward.TXBDeltaMinor
		if reward.ResolvedValue != nil {
			value = *reward.ResolvedValue
		}
		result.TXBDeltaMinor = strconv.FormatInt(value, 10)
	}
	return result
}

func mapCheckInResult(result activity.DailyCheckIn) activityResultResponse {
	reward := activityRewardResponse{Kind: activity.RewardNone}
	if result.RewardMinor != 0 {
		reward = activityRewardResponse{Kind: activity.RewardTXBDelta, TXBDeltaMinor: strconv.FormatInt(result.RewardMinor, 10)}
	}
	message := "Daily reward recorded."
	if result.AlreadyClaimed {
		message = "Today's reward was already recorded."
	}
	return activityResultResponse{ID: result.ID, Kind: "check_in", Outcome: "complete", Message: message, Reward: reward,
		BalanceAfter: model.TXBMoney(result.BalanceAfterMinor), CreatedAt: result.CreatedAt}
}

func mapBetResult(result activity.BetResult) activityResultResponse {
	outcome, message := "loss", "Result recorded: no return."
	reward := activityRewardResponse{Kind: activity.RewardNone}
	if result.Won {
		outcome = "win"
		message = "Win recorded. Total return: " + model.TXBMoney(result.PayoutMinor).Display + "."
		reward = activityRewardResponse{Kind: activity.RewardTXBDelta, TXBDeltaMinor: strconv.FormatInt(result.PayoutMinor, 10)}
	}
	return activityResultResponse{ID: result.ID, Kind: "bet", Outcome: outcome, Message: message, Reward: reward,
		StakeTXBMinor: strconv.FormatInt(result.StakeMinor, 10), BalanceAfter: model.TXBMoney(result.BalanceAfterMinor), CreatedAt: result.CreatedAt}
}

func mapDrawResult(result activity.DrawResult) activityResultResponse {
	return activityResultResponse{ID: result.ID, Kind: "draw", Outcome: "complete", Message: "Draw recorded: " + result.PrizeName + ".",
		DrawID: result.DrawID, PrizeID: result.PrizeID, PrizeName: result.PrizeName,
		Reward: mapActivityReward(result.Reward), BalanceAfter: model.TXBMoney(result.BalanceAfterMinor), CreatedAt: result.CreatedAt}
}

func mapActivityHistory(history activity.History, limit int) []activityResultResponse {
	results := make([]activityResultResponse, 0, len(history.Bets)+len(history.CheckIns)+len(history.Draws))
	for _, result := range history.Bets {
		results = append(results, mapBetResult(result))
	}
	for _, result := range history.CheckIns {
		results = append(results, mapCheckInResult(result))
	}
	for _, result := range history.Draws {
		results = append(results, mapDrawResult(result))
	}
	sort.SliceStable(results, func(left, right int) bool { return results[left].CreatedAt.After(results[right].CreatedAt) })
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

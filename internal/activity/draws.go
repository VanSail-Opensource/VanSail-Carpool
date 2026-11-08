package activity

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type PrizeInput struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name"`
	ProbabilityBPS int    `json:"probabilityBps,omitempty"`
	Stock          int64  `json:"stock,omitempty"`
	Weight         int64  `json:"-"` // retained for historical test fixtures
	StockRemaining *int64 `json:"-"` // retained for historical test fixtures
	Reward         Reward `json:"reward"`
}
type Prize struct {
	PrizeInput
	Position int `json:"position"`
}
type LuckyDrawInput struct {
	ID                    string       `json:"id,omitempty"`
	Name                  string       `json:"name"`
	Description           string       `json:"description"`
	Kind                  string       `json:"kind"`
	Status                string       `json:"status"`
	Enabled               bool         `json:"enabled"`
	FeeMinor              int64        `json:"feeMinor"`
	ExpectedParticipation int          `json:"expectedParticipation,omitempty"`
	Threshold             int          `json:"threshold,omitempty"`
	Keyword               string       `json:"keyword,omitempty"`
	Command               string       `json:"command,omitempty"`
	GroupChatID           int64        `json:"groupChatId,omitempty"`
	AnnouncementMessageID int64        `json:"announcementMessageId,omitempty"`
	Revision              int          `json:"revision"`
	Prizes                []PrizeInput `json:"prizes"`
}

func (input LuckyDrawInput) Validate() error {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return fmt.Errorf("%w: draw name must be 1 to 80 bytes", ErrInvalidInput)
	}
	if len(strings.TrimSpace(input.Description)) > 300 {
		return fmt.Errorf("%w: draw description is too long", ErrInvalidInput)
	}
	if input.FeeMinor <= 0 || input.FeeMinor > 10000000000 {
		return fmt.Errorf("%w: draw fee must be positive", ErrInvalidInput)
	}
	if len(input.Prizes) == 0 || len(input.Prizes) > 200 {
		return fmt.Errorf("%w: draw must contain 1 to 200 prizes", ErrInvalidInput)
	}
	var total int64
	for index, prize := range input.Prizes {
		if strings.TrimSpace(prize.Name) == "" || len(strings.TrimSpace(prize.Name)) > 80 {
			return fmt.Errorf("%w: prize %d has an invalid name", ErrInvalidInput, index+1)
		}
		if input.Kind == "instant" {
			if prize.ProbabilityBPS <= 0 || prize.ProbabilityBPS > 10000 || prize.Stock != 0 {
				return fmt.Errorf("%w: prize %d needs probability only", ErrInvalidInput, index+1)
			}
			total += int64(prize.ProbabilityBPS)
		} else if input.Kind == "raffle" {
			if prize.Stock <= 0 || prize.ProbabilityBPS != 0 || total > math.MaxInt64-prize.Stock {
				return fmt.Errorf("%w: prize %d needs stock only", ErrInvalidInput, index+1)
			}
			total += prize.Stock
		} else {
			return fmt.Errorf("%w: unknown draw kind", ErrInvalidInput)
		}
		if err := prize.Reward.Validate(); err != nil {
			return fmt.Errorf("prize %d: %w", index+1, err)
		}
	}
	if input.Kind == "instant" {
		if total != 10000 || input.ExpectedParticipation <= 0 || input.ExpectedParticipation > 1000000 || input.Threshold != 0 || input.Keyword != "" || input.Command != "" {
			return fmt.Errorf("%w: instant draw needs 100.00%% probability and expected entries", ErrInvalidInput)
		}
		if input.Status != "draft" && input.Status != "open" {
			return fmt.Errorf("%w: invalid instant status", ErrInvalidInput)
		}
	} else {
		if total != int64(input.Threshold) || input.Threshold <= 0 || input.Threshold > 1000 || input.ExpectedParticipation != 0 ||
			strings.TrimSpace(input.Keyword) == "" || len(input.Keyword) > 64 || strings.HasPrefix(input.Keyword, "/") || !validRaffleCommand(input.Command) {
			return fmt.Errorf("%w: raffle stock, threshold, keyword, or command is invalid", ErrInvalidInput)
		}
		switch input.Status {
		case "draft", "publishing", "open", "settling", "completed", "cancelled":
		default:
			return fmt.Errorf("%w: invalid raffle status", ErrInvalidInput)
		}
	}
	return nil
}

func validRaffleCommand(command string) bool {
	if len(command) < 1 || len(command) > 32 {
		return false
	}
	switch command {
	case "start", "signin", "sub", "balance", "mycombo", "deduct":
		return false
	}
	for _, char := range command {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func (input LuckyDrawInput) MaximumPrizeDeduction() int64 {
	var maximum int64
	for _, prize := range input.Prizes {
		if prize.Reward.Kind == RewardTXBDelta && prize.Reward.Range != nil && prize.Reward.Range.Min < 0 {
			if -prize.Reward.Range.Min > maximum {
				maximum = -prize.Reward.Range.Min
			}
		} else if prize.Reward.Kind == RewardTXBDelta && prize.Reward.TXBDeltaMinor < 0 {
			if prize.Reward.TXBDeltaMinor == math.MinInt64 {
				return math.MaxInt64
			}
			if -prize.Reward.TXBDeltaMinor > maximum {
				maximum = -prize.Reward.TXBDeltaMinor
			}
		}
	}
	return maximum
}

type LuckyDraw struct {
	LuckyDrawInput
	Seats     int       `json:"seats"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RequiresActiveCombo reports whether any prize needs a live purchase.
func (draw LuckyDraw) RequiresActiveCombo() bool {
	for _, prize := range draw.Prizes {
		switch prize.Reward.Kind {
		case RewardEntitlementGrant, RewardSquadAccess, RewardCoreComboSwitch, RewardTrafficGrant,
			RewardSubscriptionExtension:
			return true
		}
	}
	return false
}

type DrawResult struct {
	ID                    string    `json:"id"`
	UserID                string    `json:"userId,omitempty"`
	DrawID                string    `json:"drawId"`
	PrizeID               string    `json:"prizeId"`
	PrizeName             string    `json:"prizeName"`
	FeeMinor              int64     `json:"feeMinor"`
	Reward                Reward    `json:"reward"`
	BalanceAfterMinor     int64     `json:"balanceAfterMinor"`
	ConfigurationSnapshot string    `json:"configurationSnapshot"`
	IdempotencyKey        string    `json:"idempotencyKey,omitempty"`
	Replayed              bool      `json:"replayed"`
	CreatedAt             time.Time `json:"createdAt"`
}
type ExtensionCredit struct {
	ID                   string     `json:"id"`
	UserID               string     `json:"userId,omitempty"`
	Days                 int        `json:"days"`
	SourceType           string     `json:"sourceType"`
	SourceID             string     `json:"sourceId"`
	CreatedAt            time.Time  `json:"createdAt"`
	ConsumedAt           *time.Time `json:"consumedAt,omitempty"`
	ConsumedByPurchaseID *string    `json:"consumedByPurchaseId,omitempty"`
}
type History struct {
	Bets     []BetResult    `json:"bets"`
	CheckIns []DailyCheckIn `json:"checkIns"`
	Draws    []DrawResult   `json:"draws"`
}

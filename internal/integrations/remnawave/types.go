package remnawave

import "time"

type UserStatus string

const (
	// UserStatusActive allows the user to connect.
	UserStatusActive UserStatus = "ACTIVE"
	// UserStatusDisabled administratively blocks the user.
	UserStatusDisabled UserStatus = "DISABLED"
	// UserStatusLimited indicates that the traffic limit has been reached.
	UserStatusLimited UserStatus = "LIMITED"
	// UserStatusExpired indicates that the configured expiration has passed.
	UserStatusExpired UserStatus = "EXPIRED"
)

// TrafficLimitStrategy controls Remnawave's traffic reset period.

type TrafficLimitStrategy string

const (
	// TrafficNoReset never resets used traffic automatically.
	TrafficNoReset TrafficLimitStrategy = "NO_RESET"
	// TrafficDaily resets used traffic daily.
	TrafficDaily TrafficLimitStrategy = "DAY"
	// TrafficWeekly resets used traffic weekly.
	TrafficWeekly TrafficLimitStrategy = "WEEK"
	// TrafficMonthly resets used traffic monthly.
	TrafficMonthly TrafficLimitStrategy = "MONTH"
	// TrafficMonthlyRolling resets traffic on a rolling monthly interval.
	TrafficMonthlyRolling TrafficLimitStrategy = "MONTH_ROLLING"
)

// SquadSummary is a squad currently assigned to a user.

type SquadSummary struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// UserTraffic is Remnawave's aggregate usage snapshot for a user.

type UserTraffic struct {
	UsedTrafficBytes         int64      `json:"usedTrafficBytes"`
	LifetimeUsedTrafficBytes int64      `json:"lifetimeUsedTrafficBytes"`
	OnlineAt                 *time.Time `json:"onlineAt"`
	FirstConnectedAt         *time.Time `json:"firstConnectedAt"`
	LastConnectedNodeUUID    *string    `json:"lastConnectedNodeUuid"`
}

// User is the non-secret portion of a Remnawave v3.3.0 user response.
// SubscriptionURL must be treated as a bearer credential by callers.

type User struct {
	VLESSUUID            string               `json:"-"`
	TrojanPassword       string               `json:"-"`
	ShadowsocksPassword  string               `json:"-"`
	ID                   int64                `json:"id"`
	ShortUUID            string               `json:"shortUuid"`
	Username             string               `json:"username"`
	Status               UserStatus           `json:"status"`
	TrafficLimitBytes    int64                `json:"trafficLimitBytes"`
	TrafficLimitStrategy TrafficLimitStrategy `json:"trafficLimitStrategy"`
	ExpireAt             time.Time            `json:"expireAt"`
	TelegramID           *int64               `json:"telegramId"`
	Description          *string              `json:"description"`
	ExternalSquadUUID    *string              `json:"externalSquadUuid"`
	SubRevokedAt         *time.Time           `json:"subRevokedAt"`
	LastTrafficResetAt   *time.Time           `json:"lastTrafficResetAt"`
	CreatedAt            time.Time            `json:"createdAt"`
	UpdatedAt            time.Time            `json:"updatedAt"`
	SubscriptionURL      string               `json:"subscriptionUrl"`
	ActiveInternalSquads []SquadSummary       `json:"activeInternalSquads"`
	UserTraffic          UserTraffic          `json:"userTraffic"`
}

// CreateUserRequest matches Remnawave's CreateUserBodyDto fields used by VanSail Carpool.

type CreateUserRequest struct {
	Username             string               `json:"username"`
	Status               UserStatus           `json:"status"`
	TrafficLimitBytes    int64                `json:"trafficLimitBytes"`
	TrafficLimitStrategy TrafficLimitStrategy `json:"trafficLimitStrategy"`
	ExpireAt             time.Time            `json:"expireAt"`
	Description          string               `json:"description,omitempty"`
	TelegramID           int64                `json:"telegramId"`
	ActiveInternalSquads []string             `json:"activeInternalSquads"`
	ExternalSquadUUID    *string              `json:"externalSquadUuid"`
}

// UpdateUserRequest identifies a user by exactly one of ID or Username and patches set fields.
// Set ClearExternalSquad to emit an explicit JSON null for externalSquadUuid.

type UpdateUserRequest struct {
	ID                   int64
	Username             string
	Status               *UserStatus
	TrafficLimitBytes    *int64
	TrafficLimitStrategy *TrafficLimitStrategy
	ExpireAt             *time.Time
	Description          *string
	TelegramID           *int64
	ActiveInternalSquads *[]string
	ExternalSquadUUID    *string
	ClearExternalSquad   bool
}

// UserSelector identifies a user with exactly one supported field.

type UserSelector struct {
	ID        int64  `json:"id,omitempty"`
	ShortUUID string `json:"shortUuid,omitempty"`
	Username  string `json:"username,omitempty"`
}

// SubscriptionUser contains the presentation-safe account data returned by the subscription API.

type SubscriptionUser struct {
	ShortUUID                string               `json:"shortUuid"`
	DaysLeft                 float64              `json:"daysLeft"`
	TrafficUsed              string               `json:"trafficUsed"`
	TrafficLimit             string               `json:"trafficLimit"`
	LifetimeTrafficUsed      string               `json:"lifetimeTrafficUsed"`
	TrafficUsedBytes         string               `json:"trafficUsedBytes"`
	TrafficLimitBytes        string               `json:"trafficLimitBytes"`
	LifetimeTrafficUsedBytes string               `json:"lifetimeTrafficUsedBytes"`
	Username                 string               `json:"username"`
	ExpiresAt                time.Time            `json:"expiresAt"`
	IsActive                 bool                 `json:"isActive"`
	UserStatus               UserStatus           `json:"userStatus"`
	TrafficLimitStrategy     TrafficLimitStrategy `json:"trafficLimitStrategy"`
}

// Subscription is the protected subscription response. Its URL and links are bearer secrets.

type Subscription struct {
	IsFound         bool              `json:"isFound"`
	User            SubscriptionUser  `json:"user"`
	Links           []string          `json:"links"`
	SSConfLinks     map[string]string `json:"ssConfLinks"`
	SubscriptionURL string            `json:"subscriptionUrl"`
}

// NodeUsage is an aggregate usage result for a node.

type NodeUsage struct {
	UUID        string `json:"uuid"`
	Color       string `json:"color"`
	Name        string `json:"name"`
	CountryCode string `json:"countryCode"`
	Total       int64  `json:"total"`
}

// NodeUsageSeries is a time series for a node.

type NodeUsageSeries struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	CountryCode string  `json:"countryCode"`
	Total       int64   `json:"total"`
	Data        []int64 `json:"data"`
}

// UserStats is Remnawave's per-user node usage response for a date range.

type UserStats struct {
	Categories    []string          `json:"categories"`
	SparklineData []int64           `json:"sparklineData"`
	TopNodes      []NodeUsage       `json:"topNodes"`
	Series        []NodeUsageSeries `json:"series"`
}

// SquadInfo contains aggregate squad membership and inbound counts.

type SquadInfo struct {
	MembersCount  int64 `json:"membersCount"`
	InboundsCount int64 `json:"inboundsCount"`
}

// InternalSquad is a Remnawave internal squad available for catalog import.

package model

import "time"

// NamedShare is one labeled value used by donut, pie, and stacked charts.
type NamedShare struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// NormalizedDistribution is one 100-percent stacked-bar row.
type NormalizedDistribution struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Segments []NamedShare `json:"segments"`
}

// NodeTrafficSeries is one raw seven-day Remnawave node series.
type NodeTrafficSeries struct {
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	CountryCode string   `json:"countryCode"`
	DailyBytes  []string `json:"dailyBytes"`
}

// RemoteStatistics is the 30-minute Remnawave aggregate partition.
type RemoteStatistics struct {
	WeeklyUserIncrease       int64               `json:"weeklyUserIncrease"`
	MonthlyAverageUsageBPS   int                 `json:"-"`
	MonthlyAverageUsage      float64             `json:"monthlyAverageUsagePercent"`
	PredictedAverageRollover Money               `json:"predictedAverageRollover"`
	TrafficDates             []string            `json:"trafficDates"`
	TrafficSeries            []NodeTrafficSeries `json:"trafficSeries"`
}

// StatisticsUsageMember joins the current local entitlement to its upstream identity.
type StatisticsUsageMember struct {
	Purchase     Purchase
	RemoteUserID string
}

// DatabaseStatistics is the compact local statistics partition.
type DatabaseStatistics struct {
	NewUserConversion     float64                  `json:"newUserConversionPercent"`
	AverageSpend          Money                    `json:"averageSpend"`
	SpendMinimum          Money                    `json:"spendMinimum"`
	SpendMaximum          Money                    `json:"spendMaximum"`
	SubscriptionStates    []NamedShare             `json:"subscriptionStates"`
	AverageCheckInReward  Money                    `json:"averageCheckInReward"`
	ComboShares           []NamedShare             `json:"comboShares"`
	GroupMessagesTotal    int64                    `json:"groupMessagesTotal"`
	AverageOptionalSquads float64                  `json:"averageOptionalSquads"`
	PaymentStatuses       []NamedShare             `json:"paymentStatuses"`
	DatabaseBytes         string                   `json:"databaseBytes"`
	SquadByCombo          []NormalizedDistribution `json:"squadByCombo"`
	ComboBySquad          []NormalizedDistribution `json:"comboBySquad"`
}

// ProductStatisticsSnapshot preserves last-good partitions independently.
type ProductStatisticsSnapshot struct {
	GeneratedAt         time.Time          `json:"generatedAt"`
	RemoteGeneratedAt   time.Time          `json:"remoteGeneratedAt"`
	DatabaseGeneratedAt time.Time          `json:"databaseGeneratedAt"`
	Remote              RemoteStatistics   `json:"remote"`
	Database            DatabaseStatistics `json:"database"`
	StalePartitions     []string           `json:"stalePartitions"`
}

// StatisticsNode joins the live metrics and node contracts.
type StatisticsNode struct {
	UUID             string  `json:"uuid"`
	Name             string  `json:"name"`
	CountryCode      string  `json:"countryCode"`
	Online           bool    `json:"online"`
	UsersOnline      int64   `json:"usersOnline"`
	RXBytesPerSecond int64   `json:"rxBytesPerSec,string"`
	TXBytesPerSecond int64   `json:"txBytesPerSec,string"`
	XrayVersion      string  `json:"xrayVersion"`
	Multiplier       float64 `json:"multiplier"`
}

// StatisticsNodesSnapshot is the shared on-demand ten-second cache.
type StatisticsNodesSnapshot struct {
	GeneratedAt time.Time        `json:"generatedAt"`
	Stale       bool             `json:"stale"`
	Nodes       []StatisticsNode `json:"nodes"`
}

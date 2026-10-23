package model

import "time"

// Statistics is the user-facing subset of Proxy traffic analytics.
type Statistics struct {
	UsedTrafficBytes     string     `json:"usedTrafficBytes"`
	LifetimeTrafficBytes string     `json:"lifetimeTrafficBytes"`
	TrafficLimitBytes    string     `json:"trafficLimitBytes"`
	OnlineAt             *time.Time `json:"onlineAt"`
	LastTrafficResetAt   *time.Time `json:"lastTrafficResetAt"`
	Categories           []string   `json:"categories"`
	SparklineData        []string   `json:"sparklineData"`
	TopNodes             []TopNode  `json:"topNodes"`
}

// StatisticPoint is one timezone-aware aggregate bucket used by accessible admin charts and tables.
type StatisticPoint struct {
	PeriodStart string `json:"periodStart"`
	Count       int64  `json:"count"`
	UniqueUsers int64  `json:"uniqueUsers"`
	InputMinor  int64  `json:"inputTxbMinor,string"`
	OutputMinor int64  `json:"outputTxbMinor,string"`
	NetMinor    int64  `json:"netTxbMinor,string"`
}

// StatisticSlice is a labeled count used for win/loss, prize, and squad distributions.
type StatisticSlice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// AdminStatistics is the common detailed-statistics transport for catalog and activity resources.
type AdminStatistics struct {
	ResourceID    string           `json:"resourceId"`
	TimeZone      string           `json:"timeZone"`
	From          string           `json:"from"`
	To            string           `json:"to"`
	Bucket        string           `json:"bucket"`
	Count         int64            `json:"count"`
	UniqueUsers   int64            `json:"uniqueUsers"`
	InputMinor    int64            `json:"inputTxbMinor,string"`
	OutputMinor   int64            `json:"outputTxbMinor,string"`
	NetMinor      int64            `json:"netTxbMinor,string"`
	DiscountMinor int64            `json:"discountTxbMinor,string"`
	AddonMinor    int64            `json:"addonTxbMinor,string"`
	Wins          int64            `json:"wins"`
	Losses        int64            `json:"losses"`
	Series        []StatisticPoint `json:"series"`
	Distribution  []StatisticSlice `json:"distribution"`
}

// ProxyNode is the administrator-visible subset of a Proxy node contract.
type ProxyNode struct {
	UUID                  string   `json:"uuid"`
	Name                  string   `json:"name"`
	CountryCode           string   `json:"countryCode"`
	ConsumptionMultiplier float64  `json:"consumptionMultiplier"`
	ActiveInboundUUIDs    []string `json:"activeInboundUuids"`
	Accessible            bool     `json:"accessible"`
	ProviderName          string   `json:"providerName"`
	ProviderFaviconURL    *string  `json:"providerFaviconUrl"`
}

// Dashboard combines local financial state with a briefly cached Proxy snapshot.
type Dashboard struct {
	User               User                `json:"user"`
	Balance            Money               `json:"balance"`
	ActivePurchase     *Purchase           `json:"activePurchase"`
	QueuedPurchase     *Purchase           `json:"queuedPurchase"`
	AutoRenewalFailure *AutoRenewalFailure `json:"autoRenewalFailure"`
	Statistics         *Statistics         `json:"statistics"`
	SubscriptionURL    *string             `json:"subscriptionUrl"`
	StatisticsStale    bool                `json:"statisticsStale"`
	StatisticsWarning  string              `json:"statisticsWarning"`
	FetchedAt          time.Time           `json:"fetchedAt"`
}

// TopNode is an upstream node usage summary.
type TopNode struct {
	UUID                  string   `json:"uuid"`
	Name                  string   `json:"name"`
	CountryCode           string   `json:"countryCode"`
	TotalBytes            string   `json:"totalBytes"`
	ConsumptionMultiplier *float64 `json:"consumptionMultiplier,omitempty"`
}

// DashboardNodeUsage is the date-bounded, per-node traffic projection exposed
// to an authenticated member. Categories retain Proxy's UTC date labels.
type DashboardNodeUsage struct {
	StartDate  string                   `json:"startDate"`
	EndDate    string                   `json:"endDate"`
	Categories []string                 `json:"categories"`
	Nodes      []DashboardNodeUsageNode `json:"nodes"`
}

// DashboardNodeUsageNode is one upstream node and its daily byte totals.
type DashboardNodeUsageNode struct {
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	CountryCode string   `json:"countryCode"`
	TotalBytes  string   `json:"totalBytes"`
	DailyBytes  []string `json:"dailyBytes"`
}

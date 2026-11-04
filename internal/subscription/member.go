package subscription

// SubscriptionHost contains one owner's native import link and display metadata.
type SubscriptionHost struct {
	PublishedName string   `json:"publishedName"`
	NameOverride  string   `json:"nameOverride"`
	PriceLabel    string   `json:"priceLabel"`
	CustomRoute   bool     `json:"customRoute"`
	SplitXHTTP    bool     `json:"splitXhttp"`
	UUID          string   `json:"uuid"`
	Name          string   `json:"name"`
	CountryCodes  []string `json:"countryCodes"`
	SquadUUIDs    []string `json:"squadUuids"`
	Link          *string  `json:"link"`
	LinkErrorCode string   `json:"linkErrorCode"`
}

// MemberSubscription is the owner's locally authorized subscription view.
type MemberSubscription struct {
	ActiveCombo     bool               `json:"activeCombo"`
	SubscriptionURL *string            `json:"subscriptionUrl"`
	Hosts           []SubscriptionHost `json:"hosts"`
}

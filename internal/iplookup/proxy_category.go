package iplookup

// ProxySourceCategory describes why an address inventory lists its entries.
type ProxySourceCategory string

const (
	ProxyPoolCategory     ProxySourceCategory = "proxy_pool"
	ProxyDatabaseCategory ProxySourceCategory = "proxy_database"
	AbuseCategory         ProxySourceCategory = "abuse"
)

func sourceCategory(value ProxySourceCategory) ProxySourceCategory {
	if value == "" {
		return ProxyPoolCategory
	}
	return value
}

// InventorySource is compact attribution shared by all subnet addresses.
type InventorySource struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Category ProxySourceCategory `json:"category"`
}

func (p *proxyPool) metadata() []InventorySource {
	rows := make([]InventorySource, 0, len(p.sources))
	for _, source := range p.sources {
		rows = append(rows, InventorySource{ID: source.ID, Name: source.Name, Category: sourceCategory(source.Category)})
	}
	return rows
}

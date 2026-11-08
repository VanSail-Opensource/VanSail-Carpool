package xnode

import (
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/robfig/cron/v3"
	"regexp"
	"time"
)

// SourceGeoIPConfig selects master-resolved country prefixes for source filtering.
type SourceGeoIPConfig struct {
	Enabled   bool     `json:"enabled"`
	Selectors []string `json:"selectors"`
}

var countrySelector = regexp.MustCompile(`^geoip:[a-z]{2}$`)

func validateNativeGlobal(config GlobalConfig) error {
	if config.GeodataIntervalHours != nil && (*config.GeodataIntervalHours < 0 || *config.GeodataIntervalHours > 8760 || config.GeodataCron != "") {
		return ErrInvalid
	}
	if config.GeodataCron != "" {
		if _, err := cron.ParseStandard(config.GeodataCron); err != nil {
			return ErrInvalid
		}
	}
	if len(config.SourceGeoIP.Selectors) > 249 || config.SourceGeoIP.Enabled && (config.GeoIPURL == "" || len(config.SourceGeoIP.Selectors) == 0) {
		return &PolicyError{"XNODE_GEOIP_INVALID", ErrInvalid}
	}
	seen := map[string]bool{}
	for _, selector := range config.SourceGeoIP.Selectors {
		if !countrySelector.MatchString(selector) || seen[selector] {
			return &PolicyError{"XNODE_GEOIP_INVALID", ErrInvalid}
		}
		seen[selector] = true
	}
	if (wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{}, BlockedDomains: config.BlockedDomains}).Validate() != nil {
		return &PolicyError{"XNODE_DOMAIN_SELECTOR_INVALID", ErrInvalid}
	}
	return nil
}

func nextMasterGeodata(config GlobalConfig, now time.Time) time.Time {
	if config.GeodataIntervalHours != nil {
		if *config.GeodataIntervalHours == 0 {
			return time.Time{}
		}
		return now.Add(time.Duration(*config.GeodataIntervalHours) * time.Hour)
	}
	if config.GeodataCron != "" {
		if schedule, err := cron.ParseStandard(config.GeodataCron); err == nil {
			return schedule.Next(now)
		}
	}
	return time.Time{}
}

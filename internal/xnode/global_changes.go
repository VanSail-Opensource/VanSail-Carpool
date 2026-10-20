package xnode

import (
	"crypto/sha256"
	"encoding/json"
)

func globalRuntimeKey(config GlobalConfig) [32]byte {
	// Admission is computed from fresh telemetry by the master. Commerce and
	// speedtest endpoint settings are not inputs to a serving native document.
	config.ConnectionLimit = 0
	config.FirewallUnbanPrice = ""
	config.FirewallExemptionHours = 0
	config.SpeedtestDownloadURL = ""
	config.SpeedtestUploadURL = ""
	raw, _ := json.Marshal(config)
	return sha256.Sum256(raw)
}

func globalPoliciesKey(config GlobalConfig) [32]byte {
	raw, _ := json.Marshal([]any{config.SourceGeoIP, config.BlockedDomains, config.GeoIPURL, config.GeoIPHash, config.GeoSiteURL, config.GeoSiteHash})
	return sha256.Sum256(raw)
}

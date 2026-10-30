package xnode

import (
	"encoding/hex"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"net/url"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func DefaultGlobal() GlobalConfig {
	hours := 24
	return GlobalConfig{FirewallDetection: DefaultFirewallDetection(), GeodataIntervalHours: &hours, SourceGeoIP: SourceGeoIPConfig{Selectors: []string{"geoip:cn"}}, BlockedDomains: []string{}, Providers: map[string]wire.Provider{}, Telemetry: wire.TelemetryPolicy{SampleSeconds: 1, ReportSeconds: 2}, Torrent: wire.TorrentPolicy{BanSeconds: 600}}
}

func DecodeGlobal(raw string) (GlobalConfig, error) {
	config := DefaultGlobal()
	if strings.TrimSpace(raw) == "" {
		return config, nil
	}
	// Existing saved documents that omitted the interval retain their historical
	// five-second effective default. Only a newly created configuration uses two.
	config.Telemetry.ReportSeconds = 5
	config.GeodataIntervalHours = nil // Saved legacy cron/disabled schedules require explicit conversion.
	if err := wire.Decode([]byte(raw), &config); err != nil {
		return config, ErrInvalid
	}
	if config.Telemetry.ReportSeconds == 0 {
		config.Telemetry.ReportSeconds = 5
	}
	if err := ValidateGlobal(config); err != nil {
		return config, err
	}
	return config, nil
}
func ValidateGlobal(config GlobalConfig) error {
	if err := config.FirewallDetection.Apply(wire.FirewallPolicy{}).ValidateDetection(); err != nil {
		return ErrInvalid
	}
	if err := validateNativeGlobal(config); err != nil {
		return err
	}
	if config.FirewallExemptionHours < 0 || config.FirewallExemptionHours > 8760 {
		return ErrInvalid
	}
	if config.FirewallUnbanPrice != "" {
		price, err := model.ParseTXBMajor(config.FirewallUnbanPrice)
		if err != nil || price < 0 {
			return ErrInvalid
		}
	}
	if config.ConnectionLimit < 0 || config.ConnectionLimit > 1000000 {
		return ErrInvalid
	}
	if _, err := ParseBlockedPorts(config.BlockedPorts); err != nil {
		return err
	}
	for _, raw := range []string{config.GeoIPURL, config.GeoSiteURL, config.SpeedtestDownloadURL, config.SpeedtestUploadURL} {
		if raw != "" && !validHTTPS(raw) {
			return ErrInvalid
		}
	}
	for _, hash := range []string{config.GeoIPHash, config.GeoSiteHash} {
		if hash != "" {
			value, err := hex.DecodeString(hash)
			if err != nil || len(value) != 32 {
				return ErrInvalid
			}
		}
	}
	validation := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{}, Providers: config.Providers, Torrent: config.Torrent, Telemetry: config.Telemetry, Updates: config.Updates}
	if err := validation.Validate(); err != nil {
		return ErrInvalid
	}
	return nil
}
func validHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}

func GlobalWire(config GlobalConfig, native map[string]any) ([]wire.PortRange, map[string]string, error) {
	ports, err := ParseBlockedPorts(config.BlockedPorts)
	if err != nil {
		return nil, nil, err
	}
	assets := []map[string]string{}
	hashes := map[string]string{}
	for _, asset := range []struct{ file, url, hash string }{{"geosite.dat", config.GeoSiteURL, config.GeoSiteHash}} {
		if asset.url != "" {
			assets = append(assets, map[string]string{"file": asset.file, "url": asset.url})
			if asset.hash != "" {
				hashes[asset.file] = asset.hash
			}
		}
	}
	if len(assets) > 0 {
		policy := map[string]any{"assets": assets}
		if config.GeodataIntervalHours != nil {
			policy["interval_hours"] = *config.GeodataIntervalHours
		} else if config.GeodataCron != "" {
			policy["cron"] = config.GeodataCron
		}
		native["geodata"] = policy
	} else {
		delete(native, "geodata")
	}
	return ports, hashes, nil
}

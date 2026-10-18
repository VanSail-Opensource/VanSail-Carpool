package xnode

import (
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// CapabilityError keeps incompatible policy distinct from transport outages.
type CapabilityError struct{ Capability string }

func (e *CapabilityError) Error() string { return "XNODE_UPGRADE_REQUIRED: " + e.Capability }
func (e *CapabilityError) Unwrap() error { return ErrUnavailable }

func configurationCapabilities(config wire.Config, advertised []string) error {
	required := []string{}
	for _, certificate := range config.Certificates {
		if certificate.Config.CertMode == "file" && certificate.Config.Domain == "" {
			required = append(required, "node_file_certificates_v1")
		}
	}
	if config.Firewall.SourceGeoIP != nil {
		required = append(required, "source_geoip_firewall_v1")
		if config.Firewall.SourceGeoIP.Reference {
			required = append(required, "source_geoip_stream_v1")
		}
	}
	if len(config.BlockedDomains) > 0 {
		required = append(required, "shared_domain_blocks_v1")
	}
	for _, user := range config.Users {
		if len(user.TransitCredentials) > 0 {
			required = append(required, "transit_credentials_v1")
			break
		}
	}
	var native struct {
		Geodata struct {
			IntervalHours *int `json:"interval_hours"`
		} `json:"geodata"`
	}
	if json.Unmarshal(config.Xray, &native) == nil && native.Geodata.IntervalHours != nil {
		required = append(required, "geodata_interval_v1")
	}
	if config.DNS.Redirect {
		required = append(required, "xray_scoped_dns_v1")
	}
	if len(config.Users) > 0 || len(config.MemberOutbounds) > 0 {
		required = append(required, "traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards")
	}
	tags, err := wire.AuxiliaryInboundTags(config.Xray)
	if err != nil {
		return ErrInvalid
	}
	if len(tags) > 0 {
		required = append(required, "native_auxiliary_inbounds_v1")
	}
	supported := map[string]bool{}
	for _, name := range advertised {
		supported[name] = true
	}
	for _, name := range required {
		if !supported[name] {
			return &CapabilityError{Capability: name}
		}
	}
	return nil
}

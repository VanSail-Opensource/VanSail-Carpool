package xnode

import wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"

func configurationCapabilities(config wire.Config, advertised []string) error {
	required := []string{}
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
			return ErrUnavailable
		}
	}
	return nil
}

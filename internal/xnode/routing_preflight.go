package xnode

import (
	"context"
	"fmt"
)

func (s *Service) preflightRouting(ctx context.Context, candidate, old RoutingPlan) error {
	if s.master == nil {
		return ErrUnavailable
	}
	nodes, err := s.routingPlanNodes(candidate, old)
	if err != nil {
		return err
	}
	service := s.withCompileReads()
	service.routingOverrides = map[int64]RoutingPlan{candidate.ProxyUserID: candidate}
	for _, node := range nodes {
		hello, err := s.master.AgentBuild(node)
		if err != nil {
			return err
		}
		if !containsCapability(hello.Capabilities, "transit_credentials_v1") {
			return fmt.Errorf("%w: node %s requires transit_credentials_v1", ErrUnavailable, node)
		}
		config, err := service.Compile(ctx, node)
		if err != nil {
			return fmt.Errorf("node %s route preflight: %w", node, err)
		}
		if err = configurationCapabilities(config, hello.Capabilities); err != nil {
			return err
		}
		if containsCapability(hello.Capabilities, "source_geoip_stream_v1") {
			if err = s.master.ensureCountry(ctx, node, config.Firewall.SourceGeoIP); err != nil {
				return err
			}
			config = referenceCountry(config)
		}
		if _, err = s.master.Command(ctx, node, "", "config.validate", config); err != nil {
			return err
		}
	}
	return nil
}
func containsCapability(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

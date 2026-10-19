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
			return fmt.Errorf("node %s: %w", node, &CapabilityError{Capability: "transit_credentials_v1"})
		}
		config, err := service.Compile(ctx, node)
		if err != nil {
			return fmt.Errorf("node %s route preflight: %w", node, err)
		}
		if config.Firewall.SourceGeoIP != nil {
			config, err = s.master.prepareCountryReference(ctx, node, config, hello.Capabilities)
			if err != nil {
				return fmt.Errorf("node %s country asset preflight: %w", node, err)
			}
		}
		if err = configurationCapabilities(config, hello.Capabilities); err != nil {
			return err
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

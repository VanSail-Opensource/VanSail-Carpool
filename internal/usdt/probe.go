package usdt

import "context"

type CheckResult struct {
	Network   string `json:"network"`
	Success   bool   `json:"success"`
	ErrorCode string `json:"errorCode"`
}

// Check probes configured servers through the same quota groups as the scanners.
func (s *Service) Check(ctx context.Context, network string) (CheckResult, error) {
	c, err := s.config(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	spec, ok := Network(network)
	if !ok {
		return CheckResult{}, ErrConfiguration
	}
	result := CheckResult{Network: network, ErrorCode: ErrUnavailable.Error()}
	for _, n := range c.Networks {
		if n.Network == network {
			for _, endpoint := range n.Endpoints {
				reader := &scanner{client: newClient(ctx, s.executor, endpoint, spec)}
				err := reader.check(ctx)
				reader.client.close()
				if err == nil {
					return CheckResult{Network: network, Success: true}, nil
				}
				result.ErrorCode = ErrorCode(err)
			}
		}
	}
	return result, nil
}

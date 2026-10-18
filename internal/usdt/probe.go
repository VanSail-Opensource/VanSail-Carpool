package usdt

import "context"

type CheckResult struct {
	Network   string `json:"network"`
	Token     string `json:"token"`
	Rail      string `json:"rail"`
	Success   bool   `json:"success"`
	ErrorCode string `json:"errorCode"`
}

// Check probes configured servers through the shared scanner queue.
func (s *Service) Check(ctx context.Context, network string) (CheckResult, error) {
	c, err := s.config(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	spec, ok := Network(network)
	if !ok {
		return CheckResult{}, ErrConfiguration
	}
	result := CheckResult{Network: spec.Network, Token: spec.Token, Rail: spec.Rail, ErrorCode: ErrUnavailable.Error()}
	for _, n := range c.Networks {
		if n.Network == spec.Network {
			for _, endpoint := range n.Endpoints {
				reader := &scanner{client: newClient(ctx, s.executor, endpoint, spec)}
				err := reader.check(ctx)
				reader.client.close()
				if err == nil {
					return CheckResult{Network: spec.Network, Token: spec.Token, Rail: spec.Rail, Success: true}, nil
				}
				result.ErrorCode = ErrorCode(err)
			}
		}
	}
	return result, nil
}

package iplookup

import "context"

func (s *EnhancedService) reportBlock(ctx context.Context, r *Report, c Config) AbuseBlock {
	if r.LiveDetails != nil && r.LiveDetails.Block.Status != "" && r.LiveDetails.Block.Status != "skipped" {
		return r.LiveDetails.Block
	}
	return s.live.block(ctx, r.IP, c)
}

// abuseBlockClean reports a successful AbuseIPDB check-block of the same
// prefix with zero reported addresses. The /24 is then already known clean,
// so the later subnet scan skips its AbuseIPDB item instead of re-evaluating
// every neighbor address.
func abuseBlockClean(r *Report, prefix string) bool {
	if r.LiveDetails == nil {
		return false
	}
	block := r.LiveDetails.Block
	return block.Prefix == prefix && block.Status == "success" && block.ReportedAddresses != nil && *block.ReportedAddresses == 0
}

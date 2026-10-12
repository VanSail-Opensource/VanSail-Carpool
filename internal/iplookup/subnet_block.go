package iplookup

import "context"

func (s *EnhancedService) reportBlock(ctx context.Context, r *Report, c Config) AbuseBlock {
	if r.LiveDetails != nil && r.LiveDetails.Block.Status != "" && r.LiveDetails.Block.Status != "skipped" {
		return r.LiveDetails.Block
	}
	return s.live.block(ctx, r.IP, c)
}

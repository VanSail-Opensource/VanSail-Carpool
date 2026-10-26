package xnode

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func (s *Service) hostPriceLabels(ctx context.Context, user string, proxies []subscription.Proxy) (map[string]string, error) {
	out := map[string]string{}
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return nil, err
	}
	multipliers := map[string]int64{}
	for _, node := range nodes {
		multipliers[node.ID] = node.MultiplierFP
	}
	prices := map[int64]int64{}
	if r, ok := s.paygRepository(); ok && user != "" {
		accounts, e := r.XNodePAYGAccounts(ctx, user, "")
		if e != nil {
			return nil, e
		}
		byGroup := map[string]int64{}
		for _, a := range accounts {
			price, found := byGroup[a.GroupID]
			if !found {
				price = a.UnitPriceMinor
				base := s.repo
				if reads, ok := base.(*compileReads); ok {
					base = reads.Repository
				}
				if pricing, ok := base.(interface {
					HostPAYGRate(context.Context, string, string, time.Time) (int64, error)
				}); ok {
					price, e = pricing.HostPAYGRate(ctx, user, a.GroupID, s.now().UTC())
					if e != nil {
						return nil, e
					}
				}
				byGroup[a.GroupID] = price
			}
			prices[a.ProxyUserID] = price
		}
	}
	for _, p := range proxies {
		if price, ok := prices[p.PrincipalID]; ok && p.PrincipalID != 0 {
			out[p.ID] = fmt.Sprintf("%d.%02d TXB/GB", price/100, price%100)
			continue
		}
		rate := multipliers[p.NodeID]
		if rate <= 0 {
			rate = MultiplierScale
		}
		value := strconv.FormatFloat(float64(rate)/float64(MultiplierScale), 'f', -1, 64)
		if !strings.Contains(value, ".") {
			value += ".0"
		}
		out[p.ID] = value + "x"
	}
	return out, nil
}

// Active is the durable routing phase reached after the deployment receipts.
// Pending/revoking documents do not produce an applied-route badge.
func (s *Service) annotateHostRoutes(ctx context.Context, user string, proxies []subscription.Proxy) ([]subscription.Proxy, error) {
	r, ok := s.routingRepository()
	if !ok {
		return proxies, nil
	}
	i, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return nil, err
	}
	plan, err := r.XNodeRoutingPlan(ctx, i.ProxyUserID)
	if isMissing(err) {
		return proxies, nil
	}
	if err != nil {
		return nil, err
	}
	if plan.Phase != "active" {
		return proxies, nil
	}
	doc, err := s.routingDocument(plan)
	if err != nil {
		return nil, err
	}
	result := append([]subscription.Proxy(nil), proxies...)
	for index := range result {
		p := &result[index]
		for _, choice := range doc.Choices {
			if choice.InboundEndpointID == p.ID || choice.InboundEndpointID == "" && choice.InboundNodeID == p.NodeID {
				p.CustomRoute = true
				break
			}
		}
	}
	return result, nil
}

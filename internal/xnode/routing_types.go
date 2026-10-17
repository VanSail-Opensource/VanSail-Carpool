package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"time"
)

const RoutingTemplatesKey = "xnode.routing_templates"
const RoutingOperationKind = "xnode_routing"

type RoutingTemplate struct {
	ID    string             `json:"id"`
	Name  string             `json:"name"`
	Match routingmatch.Match `json:"match"`
}
type RoutingTemplateConfig struct {
	Revision  int64             `json:"revision"`
	Templates []RoutingTemplate `json:"templates"`
}
type RoutingChoice struct {
	ID             string `json:"id"`
	InboundNodeID  string `json:"inboundNodeId"`
	TemplateID     string `json:"templateId"`
	OutboundNodeID string `json:"outboundNodeId"`
}
type RoutingDocument struct {
	LegacyNodes  []string        `json:"legacyNodes,omitempty"`
	Choices      []RoutingChoice `json:"choices"`
	Seed         string          `json:"seed"`
	Dependencies string          `json:"dependencies"`
}
type RoutingPlan struct {
	ProxyUserID    int64           `json:"-"`
	Revision       int64           `json:"revision"`
	Phase          string          `json:"phase"`
	OperationID    string          `json:"operationId,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	Choices        []RoutingChoice `json:"choices"`
	Cipher         string          `json:"-"`
	PreviousCipher string          `json:"-"`
	LegacyCipher   string          `json:"-"`
}
type RoutingView struct {
	Plan      RoutingPlan         `json:"plan"`
	Templates []RoutingTemplate   `json:"templates"`
	Nodes     []RoutingNode       `json:"nodes"`
	Paths     []routingmatch.Path `json:"paths"`
}
type RoutingNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type routingRepository interface {
	XNodeUnmanagedRoutes(context.Context, time.Time) ([]Customization, error)
	ReplayXNodeRouting(context.Context, string, string, string) (model.OperationReceipt, bool, error)
	XNodeRoutingPlan(context.Context, int64) (RoutingPlan, error)
	XNodeRoutingPlans(context.Context) ([]RoutingPlan, error)
	QueueXNodeRouting(context.Context, RoutingPlan, providerops.CreateInput, time.Time) (model.OperationReceipt, error)
	AdvanceXNodeRouting(context.Context, int64, string, string, string, time.Time) error
	WakeXNodeRouting(context.Context, time.Time) error
}

func (s *Service) routingRepository() (routingRepository, bool) {
	base := s.repo
	for {
		switch r := base.(type) {
		case *compileReads:
			base = r.Repository
		case candidateRepository:
			base = r.Repository
		default:
			value, ok := base.(routingRepository)
			return value, ok
		}
	}
}

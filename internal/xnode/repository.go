package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type Repository interface {
	XNodeNodes(context.Context) ([]Node, error)
	XNodeNode(context.Context, string) (Node, error)
	XNodeNodeByToken(context.Context, []byte) (Node, error)
	SaveXNodeNode(context.Context, Node) error
	DeleteXNodeNode(context.Context, string) error
	RotateXNodeToken(context.Context, string, []byte) error
	BindXNodeAgent(context.Context, string, string) error
	AdvanceXNodeRevision(context.Context, string, uint64) (uint64, error)
	AcknowledgeXNodeState(context.Context, string, uint64, bool) error
	SetXNodeRate(context.Context, string, string, uint64, int64) error
	XNodeConfigurations(context.Context) ([]Configuration, error)
	XNodeConfiguration(context.Context, string) (Configuration, error)
	SaveXNodeConfiguration(context.Context, Configuration) error
	DeleteXNodeConfiguration(context.Context, string) error
	XNodeSnippets(context.Context) ([]Snippet, error)
	XNodeSnippet(context.Context, string) (Snippet, error)
	SaveXNodeSnippet(context.Context, Snippet) error
	DeleteXNodeSnippet(context.Context, string) error
	XNodeGroups(context.Context) ([]Group, error)
	SaveXNodeGroup(context.Context, Group) error
	DeleteXNodeGroup(context.Context, string) error
	XNodeEndpoints(context.Context) ([]Endpoint, error)
	SaveXNodeEndpoint(context.Context, Endpoint) error
	DeleteXNodeEndpoint(context.Context, string) error
	XNodeIdentities(context.Context) ([]Identity, error)
	XNodeIdentity(context.Context, int64) (Identity, error)
	XNodeIdentityByUser(context.Context, string) (Identity, error)
	XNodeIdentityBySubscription(context.Context, []byte) (Identity, error)
	SaveXNodeIdentity(context.Context, Identity) (Identity, error)
	SetXNodeAccessDisabled(context.Context, int64, bool) error
	XNodeServiceBindings(context.Context, int64) ([]ServiceBinding, error)
	SaveXNodeServiceBinding(context.Context, ServiceBinding) error
	XNodePeriod(context.Context, string) (Period, error)
	SaveXNodePeriod(context.Context, Period) error
	XNodePeriods(context.Context, int64) ([]Period, error)
	XNodeClosingPeriods(context.Context, int) ([]Period, error)
	TrackXNodePeriodNode(context.Context, string, string) error
	XNodePeriodNodes(context.Context, string) ([]PeriodNode, error)
	SealXNodePeriodNode(context.Context, string, string, uint64) error
	ReopenXNodePeriod(context.Context, string) error
	RecordXNodeTraffic(context.Context, string, string, uint64, string, wire.TrafficReport) error
	XNodeReportSequence(context.Context, string) (uint64, error)
	XNodeUsage(context.Context, int64, time.Time, time.Time) ([]DailyUsage, error)
	XNodeLegacyUsage(context.Context, int64) (LegacyUsage, error)
	SyncXNodeLegacyUsage(context.Context, LegacyUsage) error
	SaveXNodeLegacyHistory(context.Context, []LegacyHistory) error
	XNodeLegacyHistory(context.Context, int64, time.Time, time.Time) ([]LegacyHistory, error)
	XNodeCustomizations(context.Context, string) ([]Customization, error)
	SaveXNodeCustomization(context.Context, Customization) error
	XNodeMigration(context.Context) (MigrationState, error)
	SaveXNodeMigration(context.Context, MigrationState) error
	SaveXNodeMigrationNode(context.Context, MigrationNode) error
	UserByID(context.Context, string) (model.User, error)
	UserByUsername(context.Context, string) (model.User, error)
	UserByTelegramID(context.Context, int64) (model.User, error)
	DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error)
	ListPurchases(context.Context, string) ([]model.Purchase, error)
	ContinuityEntitlement(context.Context, string, time.Time) (*model.Purchase, error)
	UserDisabledSquads(context.Context, string) ([]string, error)
	XNodeUserRestricted(context.Context, string) (bool, error)
}

type Settings interface {
	Optional(context.Context, string) (string, error)
	PutXNodeGlobal(context.Context, string, string) error
}

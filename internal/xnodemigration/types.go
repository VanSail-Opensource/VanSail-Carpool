// Package xnodemigration isolates the temporary Remnawave import and bridge.
package xnodemigration

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"sync"
	"time"
)

type SourceConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}
type MonitoringMapping struct {
	LegacyUserID int64  `json:"legacyUserId,string"`
	ServiceName  string `json:"serviceName"`
}
type PreviewInput struct {
	Source                SourceConfig        `json:"source"`
	SourceBackupReference string              `json:"sourceBackupReference"`
	Monitoring            []MonitoringMapping `json:"monitoring"`
}
type Snapshot struct {
	TakenAt       time.Time                    `json:"takenAt"`
	Profiles      []remnawave.MigrationProfile `json:"profiles"`
	Nodes         []Node                       `json:"nodes"`
	Groups        []Group                      `json:"groups"`
	Hosts         []json.RawMessage            `json:"hosts"`
	HostTemplates map[string]json.RawMessage   `json:"hostTemplates,omitempty"`
	Accounts      []Account                    `json:"accounts"`
	Monitoring    []MonitoringMapping          `json:"monitoring"`
}
type Node struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	CountryCode  string                  `json:"countryCode"`
	ProviderName string                  `json:"providerName"`
	MultiplierFP int64                   `json:"multiplierFp"`
	Enabled      bool                    `json:"enabled"`
	ProfileID    string                  `json:"profileId"`
	Inbounds     []remnawave.NodeInbound `json:"inbounds"`
}
type Group struct {
	ID       string                  `json:"id"`
	Name     string                  `json:"name"`
	Inbounds []remnawave.NodeInbound `json:"inbounds"`
}
type Account struct {
	ServiceAccess *xnode.ServiceAccess `json:"serviceAccess,omitempty"`
	ServiceName   string               `json:"serviceName,omitempty"`
	GroupIDs      []string             `json:"groupIds"`
	ExpiresAt     *time.Time           `json:"expiresAt"`
	PanelUserID   string               `json:"panelUserId"`
	LegacyID      int64                `json:"legacyId"`
	Username      string               `json:"username"`
	UUID          string               `json:"uuid"`
	Trojan        string               `json:"trojan"`
	Chacha        string               `json:"chacha"`
	UsedBytes     int64                `json:"usedBytes"`
	LifetimeBytes int64                `json:"lifetimeBytes"`
	LastResetAt   *time.Time           `json:"lastResetAt"`
	Stats         remnawave.UserStats  `json:"stats"`
	Disabled      bool                 `json:"disabled"`
	TelegramID    *int64               `json:"telegramId,omitempty"`
}
type Issue struct {
	ResourceID string `json:"resourceId"`
	Code       string `json:"code"`
	Blocking   bool   `json:"blocking"`
}
type Preview struct {
	Digest    string  `json:"digest"`
	Nodes     int     `json:"nodes"`
	Groups    int     `json:"groups"`
	Accounts  int     `json:"accounts"`
	Endpoints int     `json:"endpoints"`
	Issues    []Issue `json:"issues"`
}
type Source interface {
	EndpointTopology(context.Context, SourceConfig) (Snapshot, error)
	Capture(context.Context, SourceConfig, []model.User, []MonitoringMapping) (Snapshot, error)
	SetNodeDisabled(context.Context, SourceConfig, string, bool) error
	SyncUsage(context.Context, SourceConfig, []model.User) error
	FinalAccounts(context.Context, SourceConfig) ([]Node, []Account, error)
	WithProvisioningFence(context.Context, func(context.Context) error) error
	NodeAccounts(context.Context, SourceConfig, string) ([]Account, error)
}
type Repository interface {
	xnode.Repository
	MigrationPanelAccounts(context.Context) ([]model.User, error)
	ImportXNodeMigrationNode(context.Context, xnode.Node, xnode.MigrationNode) error
}
type Backups interface {
	Run(context.Context) (model.BackupRun, error)
}
type Panel interface {
	Now() time.Time
	VerifyRelease(context.Context, string) (xnode.ReleaseEvidence, error)
	Candidate(context.Context, string) (wire.Config, error)
	VerifyClientEndpoints(context.Context, string) error
	ImportIdentity(context.Context, model.User, int64, string, string, string, ...bool) (xnode.Identity, error)
	ImportServiceIdentity(context.Context, string, int64, string, string, string, ...xnode.ServiceAccess) (xnode.Identity, error)
	SyncLegacyUsage(context.Context, xnode.Identity, xnode.LegacyUsage) error
}
type Controller interface {
	Command(context.Context, string, string, string, any) (json.RawMessage, error)
	Sync(context.Context, string) error
	WaitApplied(context.Context, string) error
	Flush(context.Context, string, int64, bool) (wire.TrafficBarrier, error)
	DeferOperation(model.OutboxJob, func(context.Context) error) error
}
type Service struct {
	repo    Repository
	source  Source
	panel   Panel
	master  Controller
	vault   *secret.Vault
	backups Backups
	mu      sync.Mutex
}

func New(repo Repository, source Source, panel Panel, master Controller, vault *secret.Vault, backups Backups) *Service {
	return &Service{repo: repo, source: source, panel: panel, master: master, vault: vault, backups: backups}
}
func (s *Service) Status(ctx context.Context) (xnode.MigrationState, error) {
	return s.repo.XNodeMigration(ctx)
}

package xnode

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type Service struct {
	routingRuntime       *routingRuntime
	compilation          *validationCache
	routingOverrides     map[int64]RoutingPlan
	routingAssets        *routingAssets
	policyAssets         *policyAssets
	localCompileRevision atomic.Int64
	repo                 Repository
	settings             Settings
	vault                *secret.Vault
	publicURL            *url.URL
	validationQueue      *upstreamqueue.Queue
	master               *Master
	now                  func() time.Time
	mu                   sync.Mutex
	templateMu           sync.Mutex
	legacyProxies        func(context.Context, Identity) ([]subscription.Proxy, error)
	legacySync           func(context.Context) error
}

func NewService(repo Repository, settings Settings, vault *secret.Vault, publicURL *url.URL, validationQueue *upstreamqueue.Queue) *Service {
	service := &Service{repo: repo, settings: settings, vault: vault, publicURL: publicURL, validationQueue: validationQueue, now: time.Now, compilation: &validationCache{}}
	service.policyAssets = &policyAssets{publish: func() { service.localCompileRevision.Add(1) }}
	service.routingAssets = &routingAssets{publish: func() { service.localCompileRevision.Add(1) }}
	service.routingRuntime = &routingRuntime{peers: map[string]routingPeer{}, legacyBefore: service.now()}
	return service
}

// SetLegacyProxies attaches the temporary, queued subscription bridge at bootstrap.
func (s *Service) SetLegacyProxies(source func(context.Context, Identity) ([]subscription.Proxy, error)) {
	s.legacyProxies = source
}
func (s *Service) Now() time.Time                                 { return s.now().UTC() }
func (s *Service) SetLegacySync(sync func(context.Context) error) { s.legacySync = sync }
func (s *Service) Global(ctx context.Context) (GlobalConfig, error) {
	raw, err := s.settings.Optional(ctx, GlobalSettingKey)
	if err != nil {
		return GlobalConfig{}, err
	}
	return DecodeGlobal(raw)
}
func (s *Service) Nodes(ctx context.Context) ([]Node, error) { return s.repo.XNodeNodes(ctx) }
func (s *Service) Node(ctx context.Context, id string) (Node, NodeSettings, error) {
	node, err := s.repo.XNodeNode(ctx, id)
	if err != nil {
		return node, NodeSettings{}, err
	}
	var settings NodeSettings
	err = s.open("xnode.node."+id, node.SettingsCipher, &settings)
	normalizeNodeSettings(&settings)
	settings.Firewall.Rules = nil
	settings.Firewall.BanSeconds = 0
	return node, settings, err
}

func normalizeNodeSettings(settings *NodeSettings) {
	if settings.DDNS == nil {
		settings.DDNS = []wire.DDNSJob{}
	}
	if settings.Certificates == nil {
		settings.Certificates = map[string]wire.Certificate{}
	}
	if settings.InboundCertificates == nil {
		settings.InboundCertificates = map[string]string{}
	}
}
func (s *Service) Configurations(ctx context.Context) ([]Configuration, error) {
	items, err := s.repo.XNodeConfigurations(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		raw, err := s.vault.Decrypt("xnode.config."+items[i].ID, items[i].DocumentCipher)
		if err != nil {
			return nil, err
		}
		items[i].Document = json.RawMessage(raw)
	}
	return items, nil
}
func (s *Service) Snippets(ctx context.Context) ([]Snippet, error) {
	items, err := s.repo.XNodeSnippets(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		raw, err := s.vault.Decrypt("xnode.snippet."+items[i].ID, items[i].DocumentCipher)
		if err != nil {
			return nil, err
		}
		items[i].Document = json.RawMessage(raw)
	}
	return items, nil
}
func (s *Service) open(key, cipher string, target any) error {
	raw, err := s.vault.Decrypt(key, cipher)
	if err != nil {
		return err
	}
	return wire.Decode([]byte(raw), target)
}
func (s *Service) seal(key string, value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return s.vault.Encrypt(key, string(raw))
}

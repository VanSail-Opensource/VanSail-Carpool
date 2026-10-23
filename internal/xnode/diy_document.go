package xnode

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

type WhitelistPolicy struct {
	Enabled bool     `json:"enabled"`
	IPs     []string `json:"ips"`
}

type WhitelistView struct {
	WhitelistPolicy
	Quota        int      `json:"quota"`
	Phase        string   `json:"phase"`
	PendingNodes []string `json:"pendingNodes"`
}

type SplitProfile struct {
	ID                 string `json:"id"`
	UploadEndpointID   string `json:"uploadEndpointId"`
	DownloadEndpointID string `json:"downloadEndpointId"`
	Mode               string `json:"mode"`
}

// DIYSettings persists references and desired policy, never generated secrets.
type DIYSettings struct {
	Version       int                              `json:"version"`
	Overrides     subscription.ConnectionOverrides `json:"overrides"`
	Whitelist     WhitelistPolicy                  `json:"whitelist"`
	SplitProfiles []SplitProfile                   `json:"splitProfiles"`
}

type diyReader interface {
	XNodeDIY(context.Context, string) (DIYDocument, error)
}

func (s *Service) readDIYSettings(ctx context.Context, user string) (DIYDocument, DIYSettings, error) {
	settings := DIYSettings{Version: 2, Whitelist: WhitelistPolicy{IPs: []string{}}, SplitProfiles: []SplitProfile{}}
	doc := DIYDocument{UserID: user}
	repo, ok := s.repo.(diyReader)
	if !ok {
		return doc, settings, nil
	}
	doc, err := repo.XNodeDIY(ctx, user)
	if err != nil || doc.Cipher == "" {
		return doc, settings, err
	}
	var raw json.RawMessage
	if err = s.open("xnode.diy."+user, doc.Cipher, &raw); err != nil {
		return doc, settings, err
	}
	settings, _, err = decodeDIYSettings(raw)
	if settings.Whitelist.IPs == nil {
		settings.Whitelist.IPs = []string{}
	}
	if settings.SplitProfiles == nil {
		settings.SplitProfiles = []SplitProfile{}
	}
	return doc, settings, err
}

func (r *compileReads) XNodeDIY(ctx context.Context, user string) (DIYDocument, error) {
	if repo, ok := r.Repository.(diyReader); ok {
		return repo.XNodeDIY(ctx, user)
	}
	return DIYDocument{UserID: user}, nil
}

func (r *compileReads) XNodeWhitelistQuota(ctx context.Context, purchase string) (int, error) {
	if repo, ok := r.Repository.(interface {
		XNodeWhitelistQuota(context.Context, string) (int, error)
	}); ok {
		return repo.XNodeWhitelistQuota(ctx, purchase)
	}
	return 5, nil
}

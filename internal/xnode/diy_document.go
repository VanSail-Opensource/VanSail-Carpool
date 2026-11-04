package xnode

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

type SplitProfile struct {
	ID                 string `json:"id"`
	UploadEndpointID   string `json:"uploadEndpointId"`
	DownloadEndpointID string `json:"downloadEndpointId"`
	Mode               string `json:"mode"`
}

// DIYSettings persists references and desired policy, never generated secrets.
type DIYSettings struct {
	Version   int                              `json:"version"`
	Overrides subscription.ConnectionOverrides `json:"overrides"`

	SplitProfiles []SplitProfile `json:"splitProfiles"`
}

type diyReader interface {
	XNodeDIY(context.Context, string) (DIYDocument, error)
}

func (s *Service) readDIYSettings(ctx context.Context, user string) (DIYDocument, DIYSettings, error) {
	settings := DIYSettings{Version: 3, SplitProfiles: []SplitProfile{}}
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

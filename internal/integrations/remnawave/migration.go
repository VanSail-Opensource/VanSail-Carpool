package remnawave

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"net/url"
)

// MigrationProfile mirrors the documented configuration export. Config contains
// server secrets and must remain in the isolated, encrypted migration workflow.
type MigrationProfile struct {
	UUID     string          `json:"uuid"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
	Inbounds []NodeInbound   `json:"inbounds"`
}

func (c *Client) MigrationHosts(ctx context.Context) ([]json.RawMessage, error) {
	var envelope struct {
		Response []json.RawMessage `json:"response"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/hosts", nil, nil, &envelope); err != nil {
		return nil, err
	}
	return envelope.Response, nil
}
func (c *Client) MigrationProfiles(ctx context.Context) ([]MigrationProfile, error) {
	var envelope struct {
		Response struct {
			Total    int                `json:"total"`
			Profiles []MigrationProfile `json:"configProfiles"`
		} `json:"response"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/config-profiles", nil, nil, &envelope); err != nil {
		return nil, err
	}
	if envelope.Response.Total != len(envelope.Response.Profiles) {
		return nil, errors.New("incomplete configuration profile export")
	}
	// The documented computed export expands upstream snippet references. Raw
	// profiles can contain placeholders that native Xray does not understand.
	for i, profile := range envelope.Response.Profiles {
		if _, err := uuid.Parse(profile.UUID); err != nil {
			return nil, err
		}
		var computed struct {
			Response MigrationProfile `json:"response"`
		}
		if err := c.do(ctx, http.MethodGet, "/api/config-profiles/"+url.PathEscape(profile.UUID)+"/computed-config", nil, nil, &computed); err != nil {
			return nil, err
		}
		if computed.Response.UUID != profile.UUID || !json.Valid(computed.Response.Config) {
			return nil, errors.New("computed configuration identity mismatch")
		}
		envelope.Response.Profiles[i] = computed.Response
	}
	return envelope.Response.Profiles, nil
}
func (c *Client) SetMigrationNodeDisabled(ctx context.Context, id string, disabled bool) error {
	if _, err := uuid.Parse(id); err != nil {
		return err
	}
	action := "enable"
	if disabled {
		action = "disable"
	}
	var envelope struct {
		Response Node `json:"response"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/nodes/"+url.PathEscape(id)+"/actions/"+action, nil, nil, &envelope); err != nil {
		return err
	}
	if envelope.Response.UUID != id || envelope.Response.IsDisabled != disabled {
		return errors.New("node ownership transition was not acknowledged")
	}
	return nil
}

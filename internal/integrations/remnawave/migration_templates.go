package remnawave

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

// MigrationHostTemplates reads only the documented Xray templates referenced by
// hosts. The caller executes this inside the existing upstream queue.
func (c *Client) MigrationHostTemplates(ctx context.Context, hosts []json.RawMessage) (map[string]json.RawMessage, error) {
	result := map[string]json.RawMessage{}
	for _, raw := range hosts {
		var host struct {
			Template *string `json:"xrayJsonTemplateUuid"`
		}
		if json.Unmarshal(raw, &host) != nil {
			return nil, errors.New("invalid host template reference")
		}
		if host.Template == nil || result[*host.Template] != nil {
			continue
		}
		id := *host.Template
		if _, err := uuid.Parse(id); err != nil {
			return nil, err
		}
		var envelope struct {
			Response struct {
				UUID     string          `json:"uuid"`
				Type     string          `json:"templateType"`
				Document json.RawMessage `json:"templateJson"`
			} `json:"response"`
		}
		if err := c.do(ctx, http.MethodGet, "/api/subscription-templates/"+url.PathEscape(id), nil, nil, &envelope); err != nil {
			return nil, err
		}
		if envelope.Response.UUID != id || envelope.Response.Type != "XRAY_JSON" || !json.Valid(envelope.Response.Document) {
			return nil, errors.New("source template identity or format mismatch")
		}
		result[id] = envelope.Response.Document
	}
	return result, nil
}

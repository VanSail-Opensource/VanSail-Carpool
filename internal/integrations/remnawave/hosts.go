package remnawave

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// ListHosts reads the documented host inventory for subscription migration.
func (c *Client) ListHosts(ctx context.Context) ([]Host, error) {
	var envelope struct {
		Response []Host `json:"response"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/hosts", nil, nil, &envelope); err != nil {
		return nil, err
	}
	for _, host := range envelope.Response {
		if _, err := uuid.Parse(host.UUID); err != nil {
			return nil, errors.New("remnawave host response contains an invalid UUID")
		}
	}
	return envelope.Response, nil
}

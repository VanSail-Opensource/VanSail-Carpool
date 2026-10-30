package emby

import (
	"context"
	"net/http"
)

// DeleteUser uses documented DELETE /Users/{Id}. Missing targets are reconciled
// by the queued adapter, so retry after a lost successful response stays safe.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, "/Users/"+id, nil, nil)
}

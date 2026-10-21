package subscription

import (
	"bytes"
	"errors"
)

// EmptyHTTPStatus validates the response-only template. It is never a client
// format/default, never renders a document, and defaults to a final 403 status.
func EmptyHTTPStatus(template Template) (int, error) {
	status := template.StatusCode
	if status == 0 {
		status = 403
	}
	document := bytes.TrimSpace(template.Document)
	if template.Format != "empty-http" || status < 200 || status > 599 || len(document) > 0 && !bytes.Equal(document, []byte("null")) {
		return 0, errors.New("invalid empty HTTP response")
	}
	return status, nil
}

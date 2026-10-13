package browserauth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type queuedTransport struct {
	queue     *upstreamqueue.Queue
	transport http.RoundTripper
}

// QueuedHTTPClient routes token and JWKS requests through the existing Telegram provider queue.
func QueuedHTTPClient(queue *upstreamqueue.Queue) *http.Client {
	return &http.Client{Transport: &queuedTransport{queue: queue, transport: http.DefaultTransport}, Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func (t *queuedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return upstreamqueue.Do(request.Context(), t.queue, func(ctx context.Context) (*http.Response, error) {
		response, err := t.transport.RoundTrip(request.Clone(ctx))
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		if err != nil {
			return nil, err
		}
		if len(raw) > 1<<20 {
			return nil, errors.New("Telegram OAuth response exceeds size limit")
		}
		response.Body = io.NopCloser(bytes.NewReader(raw))
		response.ContentLength = int64(len(raw))
		return response, nil
	})
}

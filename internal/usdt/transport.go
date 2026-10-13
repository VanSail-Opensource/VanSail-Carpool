package usdt

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const transferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
const maxRPCResponseBytes = 2 * 1024 * 1024

type rpcReply struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
type client struct {
	executor  *Executor
	endpoint  Endpoint
	spec      NetworkSpec
	http      *http.Client
	lifecycle context.Context
	id        atomic.Uint64
	mu        sync.Mutex
	ws        *socket
	wake      chan struct{}
}

func newClient(ctx context.Context, e *Executor, endpoint Endpoint, spec NetworkSpec) *client {
	return &client{executor: e, endpoint: endpoint, spec: spec, lifecycle: ctx, wake: make(chan struct{}, 1), http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *client) headers() http.Header {
	h := http.Header{"Accept": []string{"application/json"}}
	if c.endpoint.APIKey != "" {
		switch c.spec.Family {
		case "tron":
			h.Set("TRON-PRO-API-KEY", c.endpoint.APIKey)
		case "ton":
			h.Set("X-API-Key", c.endpoint.APIKey)
		default:
			h.Set("Authorization", "Bearer "+c.endpoint.APIKey)
		}
	}
	return h
}

func (c *client) request(ctx context.Context, method, rawURL string, payload any, output any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return ErrConfiguration
		}
	}
	return c.executor.do(ctx, c.endpoint.Group, func(callCtx context.Context) error {
		r, err := http.NewRequestWithContext(callCtx, method, rawURL, bytes.NewReader(body))
		if err != nil {
			return ErrRPC
		}
		r.Header = c.headers()
		if payload != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(r)
		if err != nil {
			return ErrRPC
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == 429 {
			return &Pause{Code: "USDT_RPC_RATE_LIMITED", Until: retryAfter(resp.Header.Get("Retry-After"))}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ErrRPC
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxRPCResponseBytes+1))
		if err != nil || len(data) > maxRPCResponseBytes {
			return ErrRPC
		}
		if json.Unmarshal(data, output) != nil {
			return ErrRPC
		}
		if reply, ok := output.(*rpcReply); ok {
			return rpcFailure(*reply)
		}
		return nil
	})
}

func retryAfter(raw string) time.Time {
	now := time.Now().UTC()
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds > 0 {
		return now.Add(time.Duration(min(seconds, 86400)) * time.Second)
	}
	if until, err := http.ParseTime(raw); err == nil && until.After(now) {
		return until
	}
	return now.Add(30 * time.Second)
}

func (c *client) rpc(ctx context.Context, method string, params any, out any) error {
	id := c.id.Add(1)
	payload := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	var reply rpcReply
	if strings.HasPrefix(c.endpoint.RPCURL, "ws") {
		sock, err := c.connection(ctx)
		if err != nil {
			return err
		}
		err = c.executor.do(ctx, c.endpoint.Group, func(callCtx context.Context) error { return sock.call(callCtx, id, payload, &reply) })
		if err != nil {
			return err
		}
	} else if err := c.request(ctx, http.MethodPost, c.endpoint.RPCURL, payload, &reply); err != nil {
		return err
	}
	return decodeReply(reply, id, out)
}

func decodeReply(reply rpcReply, id uint64, out any) error {
	if reply.ID != id || reply.Error != nil || len(reply.Result) == 0 || string(reply.Result) == "null" {
		return ErrRPC
	}
	if json.Unmarshal(reply.Result, out) != nil {
		return ErrRPC
	}
	return nil
}

func rpcFailure(reply rpcReply) error {
	if reply.Error == nil {
		return nil
	}
	if reply.Error.Code == 429 {
		return &Pause{Code: "USDT_RPC_RATE_LIMITED", Until: time.Now().UTC().Add(30 * time.Second)}
	}
	return ErrRPC
}

func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	base, err := url.Parse(c.endpoint.RPCURL)
	if err != nil {
		return ErrConfiguration
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	q := base.Query()
	for key, values := range query {
		q[key] = values
	}
	base.RawQuery = q.Encode()
	return c.request(ctx, http.MethodGet, base.String(), nil, out)
}

func (c *client) post(ctx context.Context, path string, payload, out any) error {
	base, err := url.Parse(c.endpoint.RPCURL)
	if err != nil {
		return ErrConfiguration
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	return c.request(ctx, http.MethodPost, base.String(), payload, out)
}

func (c *client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws != nil {
		c.ws.close()
		<-c.ws.readDone
		c.ws = nil
	}
	c.http.CloseIdleConnections()
}

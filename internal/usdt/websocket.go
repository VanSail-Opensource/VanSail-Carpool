package usdt

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type socket struct {
	conn     *websocket.Conn
	mu       sync.Mutex
	writeMu  sync.Mutex
	pending  map[uint64]chan rpcReply
	done     chan struct{}
	readDone chan struct{}
	once     sync.Once
	wake     chan struct{}
}

func (c *client) connection(ctx context.Context) (*socket, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws != nil {
		select {
		case <-c.ws.done:
			<-c.ws.readDone
			c.ws = nil
		default:
			return c.ws, nil
		}
	}
	endpoint := c.endpoint.WSURL
	if endpoint == "" {
		endpoint = c.endpoint.RPCURL
	}
	var conn *websocket.Conn
	err := c.executor.do(ctx, func(callCtx context.Context) error {
		connected, resp, err := (&websocket.Dialer{HandshakeTimeout: 15 * time.Second}).DialContext(callCtx, endpoint, c.headers())
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			return ErrRPC
		}
		conn = connected
		return nil
	})
	if err != nil {
		return nil, err
	}
	s := &socket{conn: conn, pending: map[uint64]chan rpcReply{}, done: make(chan struct{}), readDone: make(chan struct{}), wake: c.wake}
	conn.SetReadLimit(maxRPCResponseBytes)
	c.ws = s
	go s.read(c.lifecycle)
	return s, nil
}

func (s *socket) call(ctx context.Context, id uint64, payload any, out *rpcReply) error {
	replies := make(chan rpcReply, 1)
	s.mu.Lock()
	s.pending[id] = replies
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, id); s.mu.Unlock() }()
	s.writeMu.Lock()
	err := s.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	if err == nil {
		err = s.conn.WriteJSON(payload)
	}
	s.writeMu.Unlock()
	if err != nil {
		s.close()
		return ErrRPC
	}
	select {
	case reply := <-replies:
		*out = reply
		return rpcFailure(reply)
	case <-ctx.Done():
		s.close()
		return ctx.Err()
	case <-s.done:
		return ErrRPC
	}
}

func (s *socket) read(ctx context.Context) {
	defer close(s.readDone)
	stop := context.AfterFunc(ctx, s.close)
	defer stop()
	defer s.close()
	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return
		}
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		var message struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(raw, &message) != nil {
			return
		}
		if message.ID > 0 {
			var reply rpcReply
			if json.Unmarshal(raw, &reply) != nil {
				return
			}
			s.mu.Lock()
			ch := s.pending[reply.ID]
			s.mu.Unlock()
			if ch != nil {
				select {
				case ch <- reply:
				default:
				}
			}
		} else if message.Method != "" {
			select {
			case s.wake <- struct{}{}:
			default:
			}
		}
	}
}

func (s *socket) close() { s.once.Do(func() { close(s.done); _ = s.conn.Close() }) }

// subscribe uses the provider's native protocol; notifications only wake the
// durable scanner and never directly authorize a payment.
func (c *client) subscribe(ctx context.Context, addresses []string) error {
	s, err := c.connection(ctx)
	if err != nil {
		return err
	}
	methods := []any{}
	if c.spec.Family == "evm" {
		topics := []string{}
		for _, address := range addresses {
			topics = append(topics, evmTopic(address))
		}
		methods = append(methods, []any{"logs", map[string]any{"address": c.spec.Token, "topics": []any{transferTopic, nil, topics}}})
	} else {
		for _, address := range addresses {
			methods = append(methods, []any{map[string]any{"mentions": []string{address}}, map[string]any{"commitment": "finalized"}})
		}
	}
	for _, params := range methods {
		id := c.id.Add(1)
		method := "eth_subscribe"
		if c.spec.Family == "solana" {
			method = "logsSubscribe"
		}
		var reply rpcReply
		if err := c.executor.do(ctx, func(callCtx context.Context) error {
			return s.call(callCtx, id, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}, &reply)
		}); err != nil {
			return err
		}
		var subscription any
		if err := decodeReply(reply, id, &subscription); err != nil {
			return err
		}
	}
	return nil
}

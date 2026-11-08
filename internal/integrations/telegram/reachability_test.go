package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSendChatActionUsesNumericPrivateIdentityAndTypingOnly(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.URL.Path != "/bot123:fixture/sendChatAction" || len(body) != 2 || body["chat_id"] != float64(8485399326) || body["action"] != "typing" {
			t.Error("wrong private action payload")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()
	client, err := NewClient("123:fixture", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := client.SendChatAction(context.Background(), 8485399326); err != nil || !ok {
		t.Fatalf("action = %v, %v", ok, err)
	}
	for _, id := range []int64{0, -1, -100123, 1 << 52} {
		if _, err := client.SendChatAction(context.Background(), id); err == nil {
			t.Fatal("invalid private ID accepted")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("invalid identities reached transport")
	}
}

func TestReachabilityClassifierRequiresExactTypedMethodAndResponse(t *testing.T) {
	api := func(method string, status int, description string) *APIError {
		return &APIError{Method: method, HTTPStatus: status, ErrorCode: status, Description: description}
	}
	for _, test := range []struct {
		name    string
		success bool
		err     error
		want    string
	}{
		{"positive", true, nil, ReachabilityReachable},
		{"false result", false, nil, ReachabilityUnknown},
		{"blocked", false, api("sendChatAction", 403, "Forbidden: bot was blocked by the user"), ReachabilityBlocked},
		{"deactivated", false, api("sendChatAction", 403, "Forbidden: user is deactivated"), ReachabilityDeactivated},
		{"unreachable", false, api("sendChatAction", 400, "Bad Request: chat not found"), ReachabilityUnreachable},
		{"cannot initiate", false, api("sendChatAction", 403, "Forbidden: bot can't initiate conversation with a user"), ReachabilityUnreachable},
		{"timeout", false, context.DeadlineExceeded, ReachabilityUnknown},
		{"transport text", false, errors.New("Forbidden: bot was blocked by the user"), ReachabilityUnknown},
		{"throttled", false, api("sendChatAction", 429, "Forbidden: user is deactivated"), ReachabilityUnknown},
		{"server error", false, api("sendChatAction", 503, "Bad Request: chat not found"), ReachabilityUnknown},
		{"wrong method", false, api("getChat", 403, "Forbidden: bot was blocked by the user"), ReachabilityUnknown},
		{"new description", false, api("sendChatAction", 403, "Forbidden: user state changed"), ReachabilityUnknown},
		{"mixed positive and failure", true, api("sendChatAction", 403, "Forbidden: bot was blocked by the user"), ReachabilityUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyChatAction(test.success, test.err); got != test.want {
				t.Fatalf("class=%s, want %s", got, test.want)
			}
		})
	}
	wrongStatus := api("sendChatAction", 403, "Forbidden: bot was blocked by the user")
	wrongStatus.HTTPStatus = 200
	if ClassifyChatAction(false, wrongStatus) != ReachabilityUnknown {
		t.Fatal("HTTP/API contradiction accepted")
	}
	wrongStatus.HTTPStatus = 403
	wrongStatus.RetryAfter = 10
	if ClassifyChatAction(false, wrongStatus) != ReachabilityUnknown {
		t.Fatal("throttling metadata became account evidence")
	}
}

func TestPrivateChatIdentityAndUnknownBoundaries(t *testing.T) {
	for _, test := range []struct {
		name string
		chat ChatInfo
		err  error
		want string
	}{
		{"private", ChatInfo{ID: 42, Type: "private"}, nil, ReachabilityPrivate},
		{"foreign ID", ChatInfo{ID: 43, Type: "private"}, nil, ReachabilityUnknown},
		{"group", ChatInfo{ID: 42, Type: "group"}, nil, ReachabilityUnknown},
		{"missing result", ChatInfo{}, nil, ReachabilityUnknown},
		{"deactivated", ChatInfo{}, &APIError{Method: "getChat", HTTPStatus: 403, ErrorCode: 403, Description: "Forbidden: user is deactivated"}, ReachabilityDeactivated},
		{"unreachable", ChatInfo{}, &APIError{Method: "getChat", HTTPStatus: 400, ErrorCode: 400, Description: "Bad Request: chat not found"}, ReachabilityUnreachable},
		{"partial contradictory result", ChatInfo{ID: 42, Type: "private"}, &APIError{Method: "getChat", HTTPStatus: 400, ErrorCode: 400, Description: "Bad Request: chat not found"}, ReachabilityUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyPrivateChat(42, test.chat, test.err); got != test.want {
				t.Fatalf("class=%s, want %s", got, test.want)
			}
		})
	}
	if !BotIdentityUncertain(&APIError{Method: "getChat", HTTPStatus: 401, ErrorCode: 401}) || BotIdentityUncertain(&APIError{Method: "getChat", HTTPStatus: 400, ErrorCode: 400}) || BotIdentityUncertain(context.DeadlineExceeded) {
		t.Fatal("bot identity/configuration classification widened")
	}
}

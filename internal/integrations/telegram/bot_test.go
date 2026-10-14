package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

func TestSendMarkdownV2MessageDecodesMessageResult(t *testing.T) {
	t.Parallel()
	const token = "123:token"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		if request.URL.Path != "/bot"+token+"/sendMessage" {
			t.Errorf("path = %s", request.URL.Path)
		}
		var body struct {
			Text            string `json:"text"`
			ParseMode       string `json:"parse_mode"`
			ReplyParameters *struct {
				MessageID                int64 `json:"message_id"`
				AllowSendingWithoutReply bool  `json:"allow_sending_without_reply"`
			} `json:"reply_parameters"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body.Text != "*paid*" || body.ParseMode != "MarkdownV2" {
			t.Errorf("request = %+v", body)
		}
		if body.ReplyParameters == nil || body.ReplyParameters.MessageID != 9 || !body.ReplyParameters.AllowSendingWithoutReply {
			t.Errorf("reply parameters = %+v", body.ReplyParameters)
		}
		_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
	}))
	defer server.Close()

	client, err := NewClient(token, WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.SendMarkdownV2Message(context.Background(), -100, 9, "*paid*"); err != nil {
		t.Fatalf("SendMarkdownV2Message() error = %v", err)
	}
}

func TestSendMarkdownV2MessageLimitsMarkupAtTransport(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		var payload struct {
			Text      string `json:"text"`
			ParseMode string `json:"parse_mode"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.ParseMode != "MarkdownV2" || utf8.RuneCountInString(payload.Text) > telegramformat.MessageLimit ||
			!strings.HasSuffix(payload.Text, `*\.\.\.`) {
			t.Errorf("unsafe bounded payload: %q", payload.Text)
		}
		_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
	}))
	defer server.Close()
	client, err := NewClient("123:token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendMarkdownV2Message(context.Background(), 42, 0,
		"✨ *"+strings.Repeat("a", telegramformat.MessageLimit)+"*"); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownEntityRejectionRetriesAsPlainText(t *testing.T) {
	t.Parallel()
	const token = "123:token"
	for _, method := range []string{"sendMessage", "editMessageText"} {
		t.Run(method, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				defer func() { _ = request.Body.Close() }()
				attempt := calls.Add(1)
				if request.URL.Path != "/bot"+token+"/"+method {
					t.Errorf("path = %s", request.URL.Path)
				}
				var payload struct {
					Text      string `json:"text"`
					ParseMode string `json:"parse_mode"`
				}
				if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
					t.Errorf("decode request: %v", err)
				}
				if attempt == 1 {
					if payload.ParseMode != "MarkdownV2" {
						t.Errorf("first parse mode = %q", payload.ParseMode)
					}
					writer.WriteHeader(http.StatusBadRequest)
					_, _ = writer.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: can't parse entities"}`))
					return
				}
				if payload.ParseMode != "" || payload.Text != "Prize_A!" {
					t.Errorf("fallback payload = %+v", payload)
				}
				_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
			}))
			defer server.Close()
			client, err := NewClient(token, WithBaseURL(server.URL), WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}
			if method == "sendMessage" {
				_, err = client.PublishMarkdownV2Message(context.Background(), 42, `*Prize\_A\!*`)
			} else {
				err = client.EditMarkdownV2Message(context.Background(), 42, 7, `*Prize\_A\!*`)
			}
			if err != nil || calls.Load() != 2 {
				t.Fatalf("fallback = %v after %d calls", err, calls.Load())
			}
		})
	}
}

func TestMarkdownSendDoesNotRetryOtherTelegramErrors(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		calls.Add(1)
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
	}))
	defer server.Close()
	client, err := NewClient("123:token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.PublishMarkdownV2Message(context.Background(), 42, "*notice*")
	if err == nil || calls.Load() != 1 {
		t.Fatalf("blocked delivery = %v after %d calls", err, calls.Load())
	}
}

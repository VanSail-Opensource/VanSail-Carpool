package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClientHelpers(t *testing.T) {
	t.Parallel()
	const token = "123:super-secret"
	requests := make(chan capturedRequest, 7)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requests <- capturedRequest{path: request.URL.Path, body: body}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/bot" + token + "/createChatInviteLink":
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"invite_link":"https://t.me/+invite","creator":{"id":1,"is_bot":true,"first_name":"Bot"},"creates_join_request":true,"is_primary":false,"is_revoked":false}}`))
		case "/bot" + token + "/getChatMember":
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"status":"restricted","user":{"id":42,"is_bot":false,"first_name":"Ada"},"is_member":true}}`))
		case "/bot" + token + "/createInvoiceLink":
			_, _ = writer.Write([]byte(`{"ok":true,"result":"https://t.me/$invoice"}`))
		case "/bot" + token + "/getStarTransactions":
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"transactions":[{"id":"charge","amount":25,"date":1,"source":{"type":"user","transaction_type":"invoice_payment","user":{"id":42,"is_bot":false,"first_name":"Ada"},"invoice_payload":"order-1"}},{"id":"charge","amount":-25,"date":2,"receiver":{"type":"user","transaction_type":"invoice_payment","user":{"id":42,"is_bot":false,"first_name":"Ada"},"invoice_payload":"order-1"}}]}}`))
		default:
			_, _ = writer.Write([]byte(`{"ok":true,"result":true}`))
		}
	}))
	defer server.Close()

	client, err := NewClient(token, WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()
	invite, err := client.CreateJoinRequestInvite(ctx, "-100123", "tx-42", time.Unix(1800000000, 0))
	if err != nil || invite.InviteLink == "" {
		t.Fatalf("CreateJoinRequestInvite() = %#v, %v", invite, err)
	}
	request := <-requests
	if request.path != "/bot"+token+"/createChatInviteLink" || request.body["creates_join_request"] != true {
		t.Fatalf("invite request = %#v", request)
	}
	if _, found := request.body["member_limit"]; found {
		t.Fatal("join-request invite must not set member_limit")
	}

	member, err := client.GetChatMember(ctx, "@group", 42)
	if err != nil || !member.Present() {
		t.Fatalf("GetChatMember() = %#v, %v", member, err)
	}
	<-requests
	if err := client.DeclineJoinRequest(ctx, "@group", 42); err != nil {
		t.Fatalf("DeclineJoinRequest() error = %v", err)
	}
	request = <-requests
	if request.path != "/bot"+token+"/declineChatJoinRequest" || request.body["user_id"] != float64(42) {
		t.Fatalf("decline request = %#v", request)
	}

	link, err := client.CreateStarsInvoiceLink(ctx, StarsInvoiceRequest{
		Title: "TXB", Description: "VanSail Carpool balance", Payload: "order-1", Label: "TXB", Amount: 25,
	})
	if err != nil || link == "" {
		t.Fatalf("CreateStarsInvoiceLink() = %q, %v", link, err)
	}
	request = <-requests
	if request.body["currency"] != "XTR" || request.body["provider_token"] != "" {
		t.Fatalf("invoice request = %#v", request.body)
	}
	prices, ok := request.body["prices"].([]any)
	if !ok || len(prices) != 1 {
		t.Fatalf("invoice prices = %#v", request.body["prices"])
	}

	if err := client.AnswerPreCheckoutQuery(ctx, "query-1", true, ""); err != nil {
		t.Fatalf("AnswerPreCheckoutQuery() error = %v", err)
	}
	request = <-requests
	if request.body["pre_checkout_query_id"] != "query-1" || request.body["ok"] != true {
		t.Fatalf("pre-checkout request = %#v", request.body)
	}

	transactions, err := client.GetStarTransactions(ctx, 0, 10)
	if err != nil || len(transactions) != 2 || transactions[0].Source == nil || transactions[1].Receiver == nil || transactions[1].Receiver.InvoicePayload != "order-1" {
		t.Fatalf("GetStarTransactions() = %#v, %v", transactions, err)
	}
	request = <-requests
	if request.body["limit"] != float64(10) {
		t.Fatalf("Stars transactions request = %#v", request.body)
	}
	if err := client.RefundStarPayment(ctx, 42, "charge"); err != nil {
		t.Fatalf("RefundStarPayment() error = %v", err)
	}
	request = <-requests
	if request.body["user_id"] != float64(42) || request.body["telegram_payment_charge_id"] != "charge" {
		t.Fatalf("Stars refund request = %#v", request.body)
	}
}

func TestClientAPIErrorDoesNotLeakToken(t *testing.T) {
	t.Parallel()
	const token = "123:never-log-this"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"ok":false,"error_code":400,"description":"bad request"}`))
	}))
	defer server.Close()
	client, err := NewClient(token, WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetChatMember(context.Background(), "@group", 42)
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.ErrorCode != 400 {
		t.Fatalf("GetChatMember() error = %v", err)
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("error leaks a credential or URL: %v", err)
	}
}

func TestClientTransportErrorDoesNotLeakToken(t *testing.T) {
	t.Parallel()
	const token = "123:never-log-this"
	transport := doerFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: "https://api.telegram.org/bot" + token + "/getChatMember", Err: errors.New("dial failed")}
	})
	client, err := NewClient(token, WithHTTPClient(transport))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetChatMember(context.Background(), "@group", 42)
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("GetChatMember() error = %v", err)
	}
}

func TestUpdateDTOs(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"update_id":7,"chat_join_request":{"chat":{"id":-1,"type":"supergroup"},"from":{"id":42,"is_bot":false,"first_name":"Ada"},"user_chat_id":42,"date":1},"message":{"message_id":2,"chat":{"id":42,"type":"private"},"date":1,"successful_payment":{"currency":"XTR","total_amount":25,"invoice_payload":"order-1","telegram_payment_charge_id":"charge","provider_payment_charge_id":"provider"}}}`)
	var update Update
	if err := json.Unmarshal(raw, &update); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if update.ChatJoinRequest == nil || update.ChatJoinRequest.From.ID != 42 || update.Message == nil || update.Message.SuccessfulPayment == nil {
		t.Fatalf("update = %#v", update)
	}
}

func TestVerifyWebhookSecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provided string
		expected string
		want     bool
	}{
		{name: "match", provided: "secret_1", expected: "secret_1", want: true},
		{name: "mismatch", provided: "secret_2", expected: "secret_1"},
		{name: "empty", provided: "", expected: ""},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := VerifyWebhookSecret(test.provided, test.expected); got != test.want {
				t.Fatalf("VerifyWebhookSecret() = %v, want %v", got, test.want)
			}
		})
	}
}

type capturedRequest struct {
	path string
	body map[string]any
}

type doerFunc func(*http.Request) (*http.Response, error)

func (function doerFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

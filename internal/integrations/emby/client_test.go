package emby

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

func TestClientWireContract(t *testing.T) {
	t.Parallel()
	const token = "server-api-token"
	const userOne = "11111111111141118111111111111111"
	const userTwo = "22222222-2222-4222-8222-222222222222"
	requests := make(chan capturedRequest, 8)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		var body map[string]any
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		requests <- capturedRequest{Method: request.Method, Path: request.URL.Path, Token: request.Header.Get("X-Emby-Token"), Body: body}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/Users":
			_, _ = writer.Write([]byte(`[{"Name":"ada","Id":"` + userOne + `","Policy":{"EnableRemoteAccess":true,"FutureField":"kept"}}]`))
		case "/Users/New":
			_, _ = writer.Write([]byte(`{"Name":"river","Id":"` + userTwo + `","Policy":{}}`))
		case "/Users/" + userOne:
			_, _ = writer.Write([]byte(`{"Name":"ada","Id":"` + userOne + `","Policy":{"EnableRemoteAccess":true}}`))
		case "/Library/SelectableMediaFolders":
			_, _ = writer.Write([]byte(`[{"Name":"Movies","Id":"folder-1","SubFolders":[]}]`))
		case "/Localization/ParentalRatings":
			_, _ = writer.Write([]byte(`[{"Name":"PG-13","Value":13}]`))
		default:
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, token, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()

	user, exists, err := client.FindUserByName(ctx, "ada")
	if err != nil || !exists || user.ID != userOne {
		t.Fatalf("FindUserByName() = (%+v, %v, %v)", user, exists, err)
	}
	if _, exists, err := client.FindUserByName(ctx, "ADA"); err != nil || exists {
		t.Fatalf("FindUserByName(case mismatch) = (%v, %v)", exists, err)
	}
	created, err := client.CreateUser(ctx, "river")
	if err != nil || created.ID != userTwo {
		t.Fatalf("CreateUser() = (%+v, %v)", created, err)
	}
	loaded, err := client.GetUser(ctx, userOne)
	if err != nil || loaded.Name != "ada" {
		t.Fatalf("GetUser() = (%+v, %v)", loaded, err)
	}
	if err := client.SetPassword(ctx, userOne, []byte("old"), []byte("new")); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	policy := domain.HardenPolicy(loaded.Policy, domain.Preferences{DisabledLibraryIDs: []string{"folder-1"}})
	if err := client.UpdatePolicy(ctx, userOne, policy); err != nil {
		t.Fatalf("UpdatePolicy() error = %v", err)
	}
	folders, err := client.ListSelectableFolders(ctx)
	if err != nil || len(folders) != 1 || folders[0].ID != "folder-1" {
		t.Fatalf("ListSelectableFolders() = (%+v, %v)", folders, err)
	}
	ratings, err := client.ListParentalRatings(ctx)
	if err != nil || len(ratings) != 1 || ratings[0].Value != 13 {
		t.Fatalf("ListParentalRatings() = (%+v, %v)", ratings, err)
	}

	want := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/Users"}, {http.MethodGet, "/Users"}, {http.MethodPost, "/Users/New"},
		{http.MethodGet, "/Users/" + userOne}, {http.MethodPost, "/Users/" + userOne + "/Password"},
		{http.MethodPost, "/Users/" + userOne + "/Policy"}, {http.MethodGet, "/Library/SelectableMediaFolders"},
		{http.MethodGet, "/Localization/ParentalRatings"},
	}
	for index, expected := range want {
		request := <-requests
		if request.Method != expected.method || request.Path != expected.path || request.Token != token {
			t.Fatalf("request %d = %+v, want %s %s with token", index, request, expected.method, expected.path)
		}
		if request.Path == "/Users/New" && request.Body["Name"] != "river" {
			t.Fatalf("create body = %#v", request.Body)
		}
		if request.Path == "/Users/"+userOne+"/Password" {
			if request.Body["Id"] != userOne || request.Body["CurrentPw"] != "old" || request.Body["NewPw"] != "new" || request.Body["ResetPassword"] != false {
				t.Fatalf("password body = %#v", request.Body)
			}
		}
		if request.Path == "/Users/"+userOne+"/Policy" {
			if request.Body["EnableRemoteAccess"] != true || request.Body["EnableAllFolders"] != true || request.Body["EnableVideoPlaybackTranscoding"] != false {
				t.Fatalf("policy body = %#v", request.Body)
			}
		}
	}
}

func TestClientErrorClassification(t *testing.T) {
	t.Parallel()
	const userID = "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name         string
		status       int
		wantNotFound bool
		wantTerminal bool
	}{
		{name: "not found", status: http.StatusNotFound, wantNotFound: true, wantTerminal: true},
		{name: "bad request", status: http.StatusBadRequest, wantTerminal: true},
		{name: "rate limited", status: http.StatusTooManyRequests},
		{name: "server error", status: http.StatusBadGateway},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(`{"Message":"provider failure"}`))
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "token", WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			_, err = client.GetUser(context.Background(), userID)
			if err == nil || client.IsNotFound(err) != test.wantNotFound || client.IsTerminal(err) != test.wantTerminal {
				t.Fatalf("error = %v, notFound=%v terminal=%v", err, client.IsNotFound(err), client.IsTerminal(err))
			}
			var apiError *APIError
			if !errors.As(err, &apiError) || apiError.HTTPStatus != test.status {
				t.Fatalf("APIError = %#v", apiError)
			}
		})
	}
}

func TestSetPasswordRedactsAnEchoedProviderSecret(t *testing.T) {
	t.Parallel()
	const secret = "correct horse battery staple"
	const userID = "11111111-1111-4111-8111-111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"Message":"password ` + secret + ` is invalid"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "token", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	err = client.SetPassword(context.Background(), userID, nil, []byte(secret))
	if err == nil {
		t.Fatal("SetPassword() error = nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("SetPassword() error retained plaintext: %q", err)
	}
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.HTTPStatus != http.StatusBadRequest || apiError.Message != http.StatusText(http.StatusBadRequest) {
		t.Fatalf("SetPassword() API error = %#v", apiError)
	}
}

func TestGetUserRejectsMismatchedResponseIdentity(t *testing.T) {
	t.Parallel()
	const requestedID = "11111111-1111-4111-8111-111111111111"
	const returnedID = "22222222-2222-4222-8222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Name":"other","Id":"` + returnedID + `","Policy":{}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "token", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetUser(context.Background(), requestedID); err == nil {
		t.Fatal("GetUser(mismatched response Id) error = nil")
	}
}

func TestClientValidationAndIncompletePolicy(t *testing.T) {
	t.Parallel()
	const missingPolicyID = "33333333-3333-4333-8333-333333333333"
	constructorTests := []struct {
		name  string
		url   string
		token string
	}{
		{name: "relative URL", url: "/emby", token: "token"},
		{name: "unsupported scheme", url: "ftp://emby.example", token: "token"},
		{name: "URL credentials", url: "https://user:pass@emby.example", token: "token"},
		{name: "URL query", url: "https://emby.example?token=bad", token: "token"},
		{name: "empty token", url: "https://emby.example"},
	}
	for _, test := range constructorTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewClient(test.url, test.token); err == nil {
				t.Fatal("NewClient() error = nil")
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/Users/"+missingPolicyID {
			_, _ = writer.Write([]byte(`{"Name":"ada","Id":"` + missingPolicyID + `"}`))
			return
		}
		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "token", WithHTTPClient(nil), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := client.FindUserByName(ctx, " "); err == nil {
		t.Fatal("FindUserByName(empty) error = nil")
	}
	if _, err := client.CreateUser(ctx, " "); err == nil {
		t.Fatal("CreateUser(empty) error = nil")
	}
	if _, err := client.GetUser(ctx, "bad/id"); err == nil {
		t.Fatal("GetUser(invalid id) error = nil")
	}
	if _, err := client.GetUser(ctx, missingPolicyID); err == nil {
		t.Fatal("GetUser(missing policy) error = nil")
	}
	if err := client.SetPassword(ctx, "user", nil, nil); err == nil {
		t.Fatal("SetPassword(empty) error = nil")
	}
	if err := client.UpdatePolicy(ctx, "user", nil); err == nil {
		t.Fatal("UpdatePolicy(nil) error = nil")
	}
	if err := client.do(nil, http.MethodGet, "/Users", nil, nil); err == nil {
		t.Fatal("do(nil context) error = nil")
	}
	if got := (&APIError{HTTPStatus: 418, Message: "teapot"}).Error(); got == "" {
		t.Fatal("APIError.Error() is empty")
	}
	if mapped := mapUser(userDTO{}); mapped.Policy == nil {
		t.Fatal("mapUser() left a nil policy map")
	}
}

type capturedRequest struct {
	Method string
	Path   string
	Token  string
	Body   map[string]any
}

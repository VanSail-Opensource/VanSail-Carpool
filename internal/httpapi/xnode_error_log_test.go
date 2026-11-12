package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestXNodeErrorRedactsSubscriptionBearerPaths(t *testing.T) {
	const bearer = "private-subscription-bearer"
	for _, path := range []string{
		"/api/v1/subscriptions/" + bearer,
		"/api/v1/subscriptions/" + bearer + "/metadata",
		"/subscription/" + bearer,
		"/api/v1/admin/xnode/nodes",
	} {
		t.Run(path, func(t *testing.T) {
			var logs bytes.Buffer
			server := &Server{deps: Dependencies{Logger: slog.New(slog.NewJSONHandler(&logs, nil))}}
			response := httptest.NewRecorder()
			server.xnodeError(response, httptest.NewRequest(http.MethodGet, path, nil), xnode.ErrUnavailable)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatal(response.Code)
			}
			var entry struct {
				Path string `json:"path"`
				Code string `json:"error_code"`
			}
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(logs.String(), bearer) || entry.Path != redactedRequestPath(path) || entry.Code != "XNODE_UNAVAILABLE" {
				t.Fatal("native failure log retained bearer path or lost safe diagnostics", logs.String())
			}
		})
	}
}

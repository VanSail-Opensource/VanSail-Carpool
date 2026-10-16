package remnawave

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMigrationReadsReferencedClientTemplatesOnceAndChecksIdentity(t *testing.T) {
	const id = "373f14bc-089a-4c3a-91c3-3421e7c83367"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/api/subscription-templates/"+id {
			t.Errorf("undocumented request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"response":{"uuid":%q,"templateType":"XRAY_JSON","templateJson":{"outbounds":[{"tag":"direct","protocol":"freedom"}]}}}`, id)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	host := json.RawMessage(`{"xrayJsonTemplateUuid":"` + id + `"}`)
	result, err := client.MigrationHostTemplates(context.Background(), []json.RawMessage{host, host})
	if err != nil || len(result) != 1 || calls != 1 {
		t.Fatalf("template export=%v,%v,calls=%d", result, err, calls)
	}
}

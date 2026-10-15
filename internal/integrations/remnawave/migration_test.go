package remnawave

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMigrationProfilesUseComputedSnippetExpansionAndVerifyIdentity(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "computed", true: "wrong-profile"}[mismatch], func(t *testing.T) {
			calls := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path)
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer migration-secret" {
					t.Error("incorrect protected migration request")
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/config-profiles" {
					_ = json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"total": 1, "configProfiles": []any{map[string]any{"uuid": id, "config": map[string]any{"outbounds": []any{map[string]any{"snippet": "shared"}}}}}}})
					return
				}
				if r.URL.Path != "/api/config-profiles/"+id+"/computed-config" {
					t.Errorf("unexpected endpoint %s", r.URL.Path)
				}
				actual := id
				if mismatch {
					actual = "22222222-2222-4222-8222-222222222222"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"uuid": actual, "name": "resolved", "config": map[string]any{"inbounds": []any{}, "outbounds": []any{map[string]any{"tag": "direct", "protocol": "freedom"}}}}})
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "migration-secret", WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}
			profiles, err := client.MigrationProfiles(context.Background())
			if mismatch {
				if err == nil {
					t.Fatal("foreign computed profile accepted")
				}
				return
			}
			if err != nil || len(calls) != 2 || len(profiles) != 1 || profiles[0].Name != "resolved" {
				t.Fatalf("computed export=%+v,%v;calls=%v", profiles, err, calls)
			}
			var document map[string]any
			_ = json.Unmarshal(profiles[0].Config, &document)
			outbounds := document["outbounds"].([]any)
			if outbounds[0].(map[string]any)["protocol"] != "freedom" {
				t.Fatal("raw snippet placeholder exported")
			}
		})
	}
}

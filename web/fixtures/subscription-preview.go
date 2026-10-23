//go:build ignore

// Manual browser fixture: go run ./web/fixtures/subscription-preview.go
package main

import (
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/diy", func(w http.ResponseWriter, r *http.Request) {
		var input subscription.ConnectionOverrides
		if !decode(w, r, &input) {
			return
		}
		proxies := []subscription.Proxy{
			{ID: "tls", Name: "TLS host", Protocol: "vless", Address: "tls.example", Port: 443, Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"}, Stream: map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"serverName": "admin.example", "fingerprint": "chrome"}}},
			{ID: "xhttp", Name: "XHTTP host", Protocol: "vless", Address: "xhttp.example", Port: 443, Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"}, Stream: map[string]any{"network": "xhttp", "security": "tls", "tlsSettings": map[string]any{"serverName": "admin.example", "fingerprint": "chrome"}, "xhttpSettings": map[string]any{"path": "/admin", "mode": "auto"}}},
			{ID: "ss", Name: "SS host", Protocol: "shadowsocks", Address: "ss.example", Port: 443, Account: map[string]any{"method": "aes-128-gcm", "password": "synthetic"}, Stream: map[string]any{"network": "tcp"}},
		}
		if r.URL.Query().Get("mode") == "stream-one" {
			proxies[1].Stream["xhttpSettings"].(map[string]any)["mode"] = "stream-one"
		}
		if r.URL.Query().Has("empty") {
			proxies = nil
		}
		input = subscription.ActiveTuningFor(input, proxies)
		errors := subscription.ValidateTuning(input)
		if len(errors) == 0 {
			for _, proxy := range proxies {
				proxy.Tuning = &input
				out, err := proxy.Outbound()
				if err != nil {
					errors["overrides"] = "xrayValidation"
					break
				}
				raw, err := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": []any{out}})
				validation := xnode.ValidateClientConfigurationDocument(raw)
				if err != nil || !validation.Valid {
					log.Printf("synthetic DIY parser: %s: %+v", proxy.ID, validation)
					errors["overrides"] = "xrayValidation"
					break
				}
			}
		}
		respond(w, map[string]any{"fields": subscription.TuningFieldsFor(proxies), "valid": len(errors) == 0, "fieldErrors": errors, "paths": []any{}, "compatibility": subscription.TuningCompatibility(input, proxies)})
	})
	mux.HandleFunc("/selection", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Config    xnode.SubscriptionTemplateConfig `json:"config"`
			Format    string                           `json:"format"`
			UserAgent string                           `json:"userAgent"`
		}
		if !decode(w, r, &input) {
			return
		}
		selection, validation := xnode.PreviewSubscriptionSelection(input.Config, input.Format, input.UserAgent)
		respond(w, map[string]any{"selection": selection, "validation": validation})
	})
	mux.HandleFunc("/template", func(w http.ResponseWriter, r *http.Request) {
		var input subscription.Template
		if !decode(w, r, &input) {
			return
		}
		var service xnode.Service
		raw, validation, err := service.PreviewSubscriptionTemplate(input)
		if err != nil {
			http.Error(w, "fixture preview unavailable", 422)
			return
		}
		kind, status := "document", 200
		if input.Format == "empty-http" {
			kind = "empty-http"
			if value, err := subscription.EmptyHTTPStatus(input); err == nil {
				status = value
			}
		}
		respond(w, map[string]any{"preview": string(raw), "validation": validation, "responseKind": kind, "statusCode": status})
	})
	server := http.Server{Addr: "127.0.0.1:5175", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "http://127.0.0.1:5173" {
			http.Error(w, "fixture origin required", 403)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		mux.ServeHTTP(w, r)
	}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	log.Fatal(server.ListenAndServe())
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 5<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		http.Error(w, "invalid fixture input", 422)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Print("fixture response closed")
	}
}

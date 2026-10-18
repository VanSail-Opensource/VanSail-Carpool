package xnode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"strings"
	"testing"
)

func TestSubscriptionSelectionPrecedenceAndGoRegexValidation(t *testing.T) {
	config := DefaultSubscriptionTemplates()
	config.UserAgentRules = []SubscriptionUserAgentRule{{Pattern: "(?i)stash", TemplateID: "default-xray"}, {Pattern: ".*", TemplateID: "default-sing-box"}}
	for _, tc := range []struct{ format, agent, want, template, source string }{
		{"native", "Stash", "native", "", "explicit"},
		{"base64", "Stash", "base64", "", "explicit"},
		{"", "Stash/3", "xray", "default-xray", "regex"},
		{"", "mihomo", "sing-box", "default-sing-box", "regex"},
	} {
		selected, err := SelectSubscription(config, tc.format, tc.agent)
		if err != nil || selected.Format != tc.want || selected.TemplateID != tc.template || selected.Source != tc.source {
			t.Fatal(tc, selected, err)
		}
	}
	config.UserAgentRules = nil
	for _, tc := range []struct{ agent, format string }{{"Stash/3", "stash"}, {"sing-box/1", "sing-box"}, {"mihomo", "clash"}, {"unknown", "base64"}} {
		selected, err := SelectSubscription(config, "", tc.agent)
		if err != nil || selected.Format != tc.format {
			t.Fatal(selected, err)
		}
	}
	for _, pattern := range []string{"(?=stash)", `(stash)\1`, "[", ""} {
		config.UserAgentRules = []SubscriptionUserAgentRule{{Pattern: pattern, TemplateID: "default-stash"}}
		_, validation := PreviewSubscriptionSelection(config, "", "Stash")
		if validation.Valid {
			t.Fatal("non-Go or empty regex accepted", pattern)
		}
	}
	config.UserAgentRules = []SubscriptionUserAgentRule{{Pattern: ".*", TemplateID: "missing"}}
	if _, err := SelectSubscription(config, "", "Stash"); err == nil {
		t.Fatal("dangling template accepted")
	}
	config.UserAgentRules = nil
	config.Formats["stash"] = false
	if _, err := SelectSubscription(config, "", "Stash"); err == nil {
		t.Fatal("disabled format bypassed")
	}
}

type selectionSettings struct {
	cacheTestSettings
	raw string
}

func (s selectionSettings) Optional(_ context.Context, key string) (string, error) {
	if key == SubscriptionTemplatesKey {
		return s.raw, nil
	}
	return "", nil
}

func selectionProxy() subscription.Proxy {
	return subscription.Proxy{ID: "host", Name: "Host", Protocol: "vless", Address: "proxy.example", Port: 443, Account: map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000"}, Stream: map[string]any{"network": "tcp", "security": "reality", "realitySettings": map[string]any{"publicKey": "synthetic-public-key", "shortId": "abcd", "serverName": "example.com"}}}
}

func TestExactRegexTemplateOverridesRetainedAssignmentAndBase64Compatibility(t *testing.T) {
	config := DefaultSubscriptionTemplates()
	custom := subscription.DefaultTemplate("clash")
	custom.ID = "regex-template"
	custom.Name = "Regex"
	var doc map[string]any
	_ = json.Unmarshal(custom.Document, &doc)
	doc["mode"] = "global"
	custom.Document, _ = json.Marshal(doc)
	config.Templates = append(config.Templates, custom)
	config.Assignments["member"] = map[string]string{"clash": "default-clash"}
	raw, _ := json.Marshal(config)
	service := NewService(nil, selectionSettings{raw: string(raw)}, nil, nil, nil)
	proxy := selectionProxy()
	selected := SubscriptionSelection{Format: "clash", TemplateID: custom.ID, Source: "regex"}
	rendered, err := service.renderSelectedSubscription(context.Background(), "member", selected, []subscription.Proxy{proxy})
	if err != nil || !strings.Contains(string(rendered), "mode: global") {
		t.Fatal(string(rendered), err)
	}
	rendered, err = service.renderOwnedSubscription(context.Background(), "member", "clash", []subscription.Proxy{proxy})
	if err != nil || strings.Contains(string(rendered), "mode: global") {
		t.Fatal("assignment bypassed", string(rendered), err)
	}
	native, err := renderSubscription("native", []subscription.Proxy{proxy})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := renderSubscription("base64", []subscription.Proxy{proxy})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil || string(decoded) != string(native) || !strings.HasPrefix(string(native), "vless://") {
		t.Fatal("native/Base64 contract changed", err)
	}
	for _, format := range []string{"clash", "stash"} {
		rendered, err := renderSubscription(format, []subscription.Proxy{proxy})
		if err != nil || !strings.Contains(string(rendered), "support-x25519mlkem768: true") {
			t.Fatal(format, string(rendered), err)
		}
	}
}

func TestLegacySubscriptionPolicyKeepsReferencesAndDisabledNativePolicy(t *testing.T) {
	config := DefaultSubscriptionTemplates()
	config.Revision = 7
	delete(config.Formats, "base64")
	delete(config.Formats, "stash")
	config.Formats["native"] = false
	config.Assignments["retained-member"] = map[string]string{"xray": "default-xray"}
	raw, _ := json.Marshal(config)
	decoded, err := DecodeSubscriptionTemplates(string(raw))
	if err != nil || decoded.Revision != 7 || decoded.Formats["base64"] || decoded.Assignments["retained-member"]["xray"] != "default-xray" {
		t.Fatal(decoded, err)
	}
}

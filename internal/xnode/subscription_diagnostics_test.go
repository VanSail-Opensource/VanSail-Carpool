package xnode

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func TestSubscriptionMetadataReportsExclusionsAndTemplateFailures(t *testing.T) {
	config := DefaultSubscriptionTemplates()
	for index := range config.Templates {
		if config.Templates[index].Format == "stash" {
			config.Templates[index].Document = json.RawMessage(`{"invalid":"template"}`)
		}
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(nil, selectionSettings{raw: string(raw)}, nil, nil, nil)
	good := selectionProxy()
	unsupported := selectionProxy()
	unsupported.Name, unsupported.ID = "Unsupported transport", "unsupported"
	unsupported.Stream = map[string]any{"network": "unsupported"}
	metadata := SubscriptionMetadata{Active: true}
	if err := s.projectSubscriptionFormats(context.Background(), "member", []subscription.Proxy{good, unsupported}, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"sing-box", "clash"} {
		if !metadata.Formats[format] || metadata.FormatDiagnostics[format].ExcludedEndpoints != 1 || metadata.FormatDiagnostics[format].Code != "SUBSCRIPTION_ENDPOINTS_EXCLUDED" {
			t.Fatal("compatible endpoints were silently disabled", format, metadata)
		}
	}
	if metadata.Formats["stash"] || metadata.FormatDiagnostics["stash"].Code != "SUBSCRIPTION_FORMAT_RENDER_FAILED" {
		t.Fatal("bad template was silently disabled", metadata)
	}
	if err := s.projectSubscriptionFormats(context.Background(), "member", []subscription.Proxy{unsupported}, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Formats["sing-box"] || metadata.FormatDiagnostics["sing-box"].Code != "SUBSCRIPTION_FORMAT_NO_ENDPOINTS" {
		t.Fatal("protocol exclusion was not reported", metadata)
	}
}

func TestSelectionPreviewUsesTheSelectedLiveRenderer(t *testing.T) {
	config := DefaultSubscriptionTemplates()
	for index := range config.Templates {
		if config.Templates[index].Format == "stash" {
			config.Templates[index].Document = json.RawMessage(`{"invalid":"template"}`)
		}
	}
	_, validation := PreviewSubscriptionSelection(config, "", "Stash/3")
	if validation.Valid || validation.ErrorCode != "SUBSCRIPTION_FORMAT_RENDER_FAILED" {
		t.Fatal("preview accepted a template live downloads cannot render", validation)
	}
	_, validation = PreviewSubscriptionSelection(config, "clash", "Stash/3")
	if !validation.Valid {
		t.Fatal("explicit format did not override detection", validation)
	}
}

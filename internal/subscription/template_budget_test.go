package subscription

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestTemplateExpansionBudgetCountsRepeatedEscapedNames(t *testing.T) {
	root := map[string]any{"proxies": []any{map[string]any{"SubscriptionProxies": map[string]any{}}}, "selectors": []any{
		[]any{map[string]any{"SubscriptionNames": map[string]any{}}},
		[]any{map[string]any{"SubscriptionNames": map[string]any{}}},
	}}
	proxies := []any{map[string]any{"name": "<quoted>\"", "type": "vless"}}
	names := []any{"<quoted>\""}
	size, err := templateExpansionSize(root, proxies, names, 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = templateExpansionSize(root, proxies, names, size-1); !errors.Is(err, errTemplateExpansionLimit) {
		t.Fatal("repeated insertion escaped the byte allowance", err)
	}
	expanded, err := expandTemplate(root, proxies, names, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(expanded)
	if err != nil || len(raw) > size {
		t.Fatal("preflight underestimated escaped output", len(raw), size, err)
	}
}

func TestTemplateSmallPreviewCannotAmplifyLiveNameInsertion(t *testing.T) {
	markers := make([]string, 1000)
	for i := range markers {
		markers[i] = `{"SubscriptionNames":{}}`
	}
	document := `{"outbounds":[{"SubscriptionProxies":{}},{"type":"selector","tag":"proxy","outbounds":[` + strings.Join(markers, ",") + `]}],"route":{"final":"proxy"}}`
	template := Template{Format: "sing-box", Document: json.RawMessage(document)}
	if _, err := PreviewTemplate(template); err != nil {
		t.Fatal("small single-endpoint preview should be valid", err)
	}
	proxies := make([]Proxy, 64)
	for i := range proxies {
		proxies[i] = templateProxy(fmt.Sprint(i), strings.Repeat("<", 64)+fmt.Sprint(i))
	}
	raw, err := RenderTemplate(template, proxies)
	if !errors.Is(err, errTemplateExpansionLimit) || raw != nil {
		t.Fatal("amplified fleet must fail during preflight", len(raw), err)
	}
	if string(template.Document) != document {
		t.Fatal("preflight rewrote administrator source")
	}
}

func TestTemplateXrayProfilesShareExpansionAllowance(t *testing.T) {
	markers := make([]string, 3000)
	for i := range markers {
		markers[i] = `{"SubscriptionNames":{}}`
	}
	document := `{"outbounds":[{"SubscriptionProxies":{}}],"routing":{"balancers":[{"tag":"select","selector":[` + strings.Join(markers, ",") + `]}]}}`
	proxies := make([]Proxy, 80)
	for i := range proxies {
		proxies[i] = templateProxy(fmt.Sprint(i), "Endpoint")
	}
	raw, err := RenderTemplate(Template{Format: "xray", Document: json.RawMessage(document)}, proxies)
	if !errors.Is(err, errTemplateExpansionLimit) || raw != nil {
		t.Fatal("selectable profiles must share the expansion allowance", len(raw), err)
	}
}

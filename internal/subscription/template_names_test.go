package subscription

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/ghodss/yaml"
)

func renderedNameEntries(t *testing.T, format string, proxies []Proxy) (map[string]string, []any) {
	t.Helper()
	raw, err := RenderTemplate(DefaultTemplate(format), proxies)
	if err != nil {
		t.Fatal(err)
	}
	if yamlFormat(format) {
		raw, err = yaml.YAMLToJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	var root map[string]any
	if err = json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	key, tag, refs := "outbounds", "tag", "outbounds"
	var groups []any
	if yamlFormat(format) {
		key, tag, refs = "proxies", "name", "proxies"
		groups = root["proxy-groups"].([]any)
	} else {
		groups = root[key].([]any)
	}
	names, seen := map[string]string{}, map[string]bool{}
	for _, raw := range root[key].([]any) {
		item := raw.(map[string]any)
		if uuid, ok := item["uuid"].(string); ok {
			name := item[tag].(string)
			if seen[name] {
				t.Fatal("duplicate rendered name", name)
			}
			seen[name], names[uuid] = true, name
		}
	}
	return names, groups[0].(map[string]any)[refs].([]any)
}

func TestTemplateNamesPreserveDuplicatePAYGCredentialsAndMappedSelectors(t *testing.T) {
	for _, format := range []string{"sing-box", "clash", "stash"} {
		for _, mapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/mapped=%v", format, mapped), func(t *testing.T) {
				proxies := []Proxy{templateProxy("combo", "Tokyo"), templateProxy("payg-first", "Tokyo"), templateProxy("payg-second", "Tokyo")}
				for index := range proxies {
					proxies[index].Account = map[string]any{"id": fmt.Sprintf("550e8400-e29b-41d4-a716-%012d", index+1), "encryption": "none"}
					if mapped {
						key, tag := "singbox", "tag"
						if yamlFormat(format) {
							key, tag = "mihomo", "name"
							if format == "stash" {
								key = "stash"
							}
						}
						proxies[index].MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{key: []any{map[string]any{"op": "set", "to": tag, "value": "Mapped Tokyo"}}}}}
					}
				}
				names, refs := renderedNameEntries(t, format, proxies)
				if len(names) != len(proxies) || len(refs) != len(proxies) {
					t.Fatal("credentials or selector entries lost", names, refs)
				}
				for index, proxy := range proxies {
					if refs[index] != names[proxy.Account["id"].(string)] || proxy.Name != "Tokyo" {
						t.Fatal("selector mismatch or inventory label mutated")
					}
				}
				reordered := slices.Clone(proxies)
				slices.Reverse(reordered)
				again, _ := renderedNameEntries(t, format, reordered)
				for uuid, name := range names {
					if again[uuid] != name {
						t.Fatal("export order changed a credential's name")
					}
				}
			})
		}
	}
}

func TestTemplateNamesReservePoliciesGroupsAndExistingSuffixes(t *testing.T) {
	for _, format := range []string{"sing-box", "clash", "stash"} {
		t.Run(format, func(t *testing.T) {
			first := templateProxy("payg-first", "Tokyo")
			proxies := []Proxy{first, templateProxy("payg-second", "Tokyo"), templateProxy("ordinary", "Tokyo ["+first.Tag()[6:14]+"]"), templateProxy("selector", "proxy"), templateProxy("direct", "direct"), templateProxy("builtin", "DIRECT")}
			for index := range proxies {
				proxies[index].Account = map[string]any{"id": fmt.Sprintf("550e8400-e29b-41d4-a716-%012d", index+1), "encryption": "none"}
			}
			names, _ := renderedNameEntries(t, format, proxies)
			if len(names) != len(proxies) || names[proxies[2].Account["id"].(string)] != proxies[2].Name {
				t.Fatal("existing unique suffix was replaced", names)
			}
		})
	}
}

func TestTemplateNamesKeepInvalidAdministratorDocumentsVisible(t *testing.T) {
	for _, document := range []string{
		`{"outbounds":[{"SubscriptionProxies":{}},{"type":"direct","tag":"direct"},{"type":"direct","tag":"direct"}]}`,
		`{"outbounds":[{"SubscriptionProxies":{}},{"type":"selector","tag":"proxy","outbounds":["missing"]}]}`,
	} {
		if _, err := RenderTemplate(Template{Format: "sing-box", Document: json.RawMessage(document)}, []Proxy{templateProxy("payg", "direct")}); err == nil {
			t.Fatal("invalid administrator template accepted")
		}
	}
}

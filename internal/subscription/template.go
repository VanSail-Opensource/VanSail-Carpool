package subscription

import (
	"encoding/json"
	"errors"

	"github.com/ghodss/yaml"
)

// Template is an administrator-owned client document with structural insertion
// markers. It never evaluates expressions or interpolates credential strings.
type Template struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Format     string          `json:"format"`
	Document   json.RawMessage `json:"document,omitempty"`
	StatusCode int             `json:"statusCode,omitempty"`
}

func DefaultTemplate(format string) Template {
	doc := map[string]string{
		"xray":     `{"inbounds":[],"outbounds":[{"SubscriptionProxies":{}},{"tag":"direct","protocol":"freedom"}],"routing":{"rules":[]}}`,
		"sing-box": `{"outbounds":[{"type":"selector","tag":"proxy","outbounds":[{"SubscriptionNames":{}}]},{"SubscriptionProxies":{}},{"type":"direct","tag":"direct"}],"route":{"final":"proxy","auto_detect_interface":true}}`,
		"stash":    `{"proxies":[{"SubscriptionProxies":{}}],"proxy-groups":[{"name":"proxy","type":"select","proxies":[{"SubscriptionNames":{}}]}],"rules":["MATCH,proxy"]}`,
		"clash":    `{"proxies":[{"SubscriptionProxies":{}}],"proxy-groups":[{"name":"proxy","type":"select","proxies":[{"SubscriptionNames":{}}]}],"rules":["MATCH,proxy"]}`,
	}[format]
	return Template{ID: "default-" + format, Name: format, Format: format, Document: json.RawMessage(doc)}
}

// RenderTemplate expands only the caller's eligible proxies. Xray uses one
// independently selectable configuration per endpoint; other clients use a fleet.
func RenderTemplate(template Template, proxies []Proxy) ([]byte, error) {
	if template.Format == "empty-http" {
		_, err := EmptyHTTPStatus(template)
		return []byte{}, err
	}
	if template.StatusCode != 0 {
		return nil, errors.New("HTTP status is only valid for an empty response")
	}
	if len(proxies) > 4096 {
		return nil, errors.New("subscription has too many endpoints")
	}
	budget := len(template.Document)
	if _, err := templateRoot(template); err != nil {
		return nil, err
	}
	var err error
	proxies, _, err = CompatibleProxies(template.Format, proxies)
	if err != nil {
		return nil, err
	}
	if template.Format == "xray" {
		configs := []any{}
		for _, proxy := range proxies {
			current := template
			if len(proxy.XrayTemplate) > 0 {
				current.Document = proxy.XrayTemplate
			}
			root, err := templateRoot(current)
			if err != nil {
				return nil, err
			}
			out, err := proxy.Outbound()
			if err != nil {
				return nil, err
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			budget += len(current.Document) + len(encoded) + len(proxy.Name) + 256
			if budget > 8<<20 {
				return nil, errors.New("rendered subscription exceeds 8 MiB")
			}
			value, err := expandTemplate(root, []any{out}, []any{proxy.Tag()}, 0)
			if err != nil {
				return nil, err
			}
			root = value.(map[string]any)
			root["remarks"] = proxy.Name
			if err = validateRenderedTemplate(template.Format, root); err != nil {
				return nil, err
			}
			configs = append(configs, root)
		}
		raw, err := json.Marshal(configs)
		if err == nil && len(raw) > 8<<20 {
			return nil, errors.New("rendered subscription exceeds 8 MiB")
		}
		return raw, err
	}
	root, err := templateRoot(template)
	if err != nil {
		return nil, err
	}
	values, names := []any{}, []any{}
	for _, proxy := range proxies {
		var out map[string]any
		if yamlFormat(template.Format) {
			out, err = proxy.yamlProxy(template.Format)
		} else {
			out, err = proxy.SingBox()
		}
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		budget += len(encoded) + len(proxy.Name) + 256
		if budget > 8<<20 {
			return nil, errors.New("rendered subscription exceeds 8 MiB")
		}
		values = append(values, out)
		nameKey := "tag"
		if yamlFormat(template.Format) {
			nameKey = "name"
		}
		name, ok := out[nameKey].(string)
		if !ok || name == "" {
			return nil, errors.New("mapped proxy name is required")
		}
		names = append(names, name)
	}
	value, err := expandTemplate(root, values, names, 0)
	if err != nil {
		return nil, err
	}
	if err = validateRenderedTemplate(template.Format, value.(map[string]any)); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if yamlFormat(template.Format) {
		raw, err = yaml.JSONToYAML(raw)
	}
	if err == nil && len(raw) > 8<<20 {
		return nil, errors.New("rendered subscription exceeds 8 MiB")
	}
	return raw, err
}

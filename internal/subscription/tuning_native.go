package subscription

import (
	"fmt"
	"net/url"
)

func (p Proxy) tunedNativeLink() (string, error) {
	base := p
	base.Tuning = nil
	raw, err := base.NativeLink()
	if err != nil {
		return "", err
	}
	if p.Protocol != "vless" && p.Protocol != "trojan" {
		return raw, nil
	}
	link, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query := link.Query()
	values, err := p.Tuning.values()
	if err != nil {
		return "", err
	}
	for _, spec := range allTuningSpecs() {
		value, exists := values[spec.Key]
		key := spec.Mappings["native"]
		if exists && key != "" && tuningReason(p, spec, "native", value, values) == "" {
			query.Set(key, fmt.Sprint(value))
		}
	}
	link.RawQuery = query.Encode()
	return link.String(), nil
}

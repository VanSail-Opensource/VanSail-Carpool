package subscription

import (
	"crypto/tls"
	"maps"
	"slices"
	"sync"

	xraytls "github.com/xtls/xray-core/transport/internet/tls"
)

type TuningField struct {
	Key          string   `json:"key"`
	Group        string   `json:"group"`
	Kind         string   `json:"kind"`
	Minimum      int64    `json:"minimum"`
	Maximum      int64    `json:"maximum"`
	Enum         []string `json:"enum,omitempty"`
	Unit         string   `json:"unit"`
	Advanced     bool     `json:"advanced"`
	Restrictions []string `json:"restrictions,omitempty"`
}
type tuningSpec struct {
	TuningField
	Context  string
	Mappings map[string]string
}

var cachedTuningSpecs = sync.OnceValue(func() []tuningSpec {
	result := slices.Clone(tuningSpecs)
	for index := range result {
		spec := &result[index]
		switch spec.Key {
		case "fingerprint", "downloadFingerprint":
			set := map[string]bool{"unsafe": true}
			for name := range xraytls.PresetFingerprints {
				set[name] = true
			}
			for name := range xraytls.ModernFingerprints {
				set[name] = true
			}
			for name := range xraytls.OtherFingerprints {
				set[name] = true
			}
			spec.Enum = slices.Sorted(maps.Keys(set))
		case "cipherSuites", "downloadCipherSuites":
			for _, suite := range append(tls.CipherSuites(), tls.InsecureCipherSuites()...) {
				if !slices.Contains(suite.SupportedVersions, tls.VersionTLS13) {
					spec.Enum = append(spec.Enum, suite.Name)
				}
			}
		}
		if spec.Key == "mark" || spec.Key == "downloadMark" {
			spec.Restrictions = []string{"linux"}
		}
		if spec.Key == "tcpMptcp" || spec.Key == "downloadTcpMptcp" {
			spec.Restrictions = []string{"go121"}
		}
		if spec.Key == "tcpKeepAliveIdle" || spec.Key == "tcpKeepAliveInterval" {
			spec.Restrictions = []string{"singbox113"}
		}
		if spec.Key == "curvePreferences" {
			spec.Restrictions = []string{"singboxCurves"}
		}
		if spec.Key == "interface" {
			spec.Restrictions = []string{"stashInterface"}
		}
		if spec.Group == "shadowsocks" {
			spec.Restrictions = append(spec.Restrictions, "stashUotVersions")
		}
		if spec.Context == "uot" || spec.Context == "sager-mux" {
			spec.Restrictions = append(spec.Restrictions, "serverCapability")
		}
		if spec.Context == "xhttp" || spec.Group == "download" {
			spec.Restrictions = append(spec.Restrictions, "stashXhttpVersions")
		}
	}
	return result
})

func allTuningSpecs() []tuningSpec { return cachedTuningSpecs() }

func TuningFields() []TuningField {
	items := []TuningField{}
	for _, spec := range allTuningSpecs() {
		items = append(items, spec.TuningField)
	}
	return items
}

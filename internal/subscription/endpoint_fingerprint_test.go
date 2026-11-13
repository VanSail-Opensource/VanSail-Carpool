package subscription

import (
	"encoding/json"
	"net/url"
	"slices"
	"testing"
)

func TestEndpointDownloadFingerprintUsesCurrentCatalogAndNativeBuilder(t *testing.T) {
	p := endpointReviewProxy()
	enabled, fingerprint := true, "firefox"
	p.EndpointTuning = &EndpointOptions{DownloadEnabled: &enabled, DownloadFingerprint: &fingerprint}
	fields := EndpointFields(p)
	index := slices.IndexFunc(fields, func(field TuningField) bool { return field.Key == "downloadFingerprint" })
	if index < 0 || !slices.Contains(fields[index].Enum, fingerprint) {
		t.Fatal("download fingerprint has no selectable current choices")
	}
	if failures := ValidateEndpointOptions(*p.EndpointTuning, p); len(failures) != 0 {
		t.Fatal("supported download fingerprint failed native validation", failures)
	}
	out := buildXMUXReview(t, p)
	down := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "downloadSettings")
	if text(object(down, "tlsSettings"), "fingerprint") != fingerprint {
		t.Fatal("native download fingerprint was lost")
	}
	for _, format := range []string{"clash", "stash"} {
		out, err := p.endpointDocument(format)
		if err != nil || text(object(object(out, "xhttp-opts"), "download-settings"), "client-fingerprint") != fingerprint {
			t.Fatal(format, out, err)
		}
	}
	raw, err := p.NativeLink()
	if err != nil {
		t.Fatal(err)
	}
	link, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var extra map[string]any
	if err = json.Unmarshal([]byte(link.Query().Get("extra")), &extra); err != nil || text(object(object(extra, "downloadSettings"), "tlsSettings"), "fingerprint") != fingerprint {
		t.Fatal("native URI lost structured download fingerprint", extra, err)
	}
	fingerprint = "not-a-supported-fingerprint"
	if failures := ValidateEndpointOptions(*p.EndpointTuning, p); failures["downloadFingerprint"] != "invalid" {
		t.Fatal("unknown download fingerprint was accepted", failures)
	}
}

func TestEndpointUnsupportedDownloadFingerprintIsOmittedPerClient(t *testing.T) {
	p := endpointReviewProxy()
	enabled, fingerprint := true, "hellochrome_120"
	p.EndpointTuning = &EndpointOptions{DownloadEnabled: &enabled, DownloadFingerprint: &fingerprint}
	if failures := ValidateEndpointOptions(*p.EndpointTuning, p); len(failures) != 0 {
		t.Fatal("pinned native fingerprint should be valid", failures)
	}
	for _, format := range []string{"clash", "stash"} {
		compat := endpointClientCompatibility(p, format)
		if !compat.Supported || !slices.Contains(compat.OmittedOptions, "downloadFingerprint") || !slices.Contains(compat.Reasons, "fingerprint") {
			t.Fatal("optional download incompatibility was not explained", format, compat)
		}
		out, err := p.endpointDocument(format)
		if err != nil || object(object(out, "xhttp-opts"), "download-settings")["client-fingerprint"] != nil {
			t.Fatal("unsupported inherited/projected fingerprint remained in client output", format, out, err)
		}
	}
	if *p.EndpointTuning.DownloadFingerprint != fingerprint {
		t.Fatal("client omission changed saved administrator choice")
	}
}

func TestEndpointDownloadFingerprintOmissionsFollowEffectiveInheritance(t *testing.T) {
	for _, item := range []struct {
		name      string
		partial   bool
		typedMain bool
		child     string
		disabled  bool
		streamOne bool
		plain     bool
		wantOmit  bool
	}{
		{name: "new download inherits raw primary", wantOmit: true},
		{name: "partial download inherits raw primary", partial: true, wantOmit: true},
		{name: "new download inherits typed primary", typedMain: true, wantOmit: true},
		{name: "explicit child takes precedence", partial: true, child: "firefox"},
		{name: "disabled child is inactive", partial: true, disabled: true},
		{name: "stream-one child is inactive", partial: true, streamOne: true},
		{name: "selected child security has no fingerprint", plain: true},
	} {
		t.Run(item.name, func(t *testing.T) {
			p := endpointReviewProxy()
			fingerprint, enabled := "hellochrome_120", !item.disabled
			p.EndpointTuning = &EndpointOptions{}
			object(p.Stream, "tlsSettings")["fingerprint"] = fingerprint
			if item.typedMain {
				object(p.Stream, "tlsSettings")["fingerprint"] = "chrome"
				p.EndpointTuning.Fingerprint = &fingerprint
			}
			if item.partial {
				down := map[string]any{"address": "down.example"}
				if item.child != "" {
					down["tlsSettings"] = map[string]any{"fingerprint": item.child}
				}
				object(p.Stream, "xhttpSettings")["extra"] = map[string]any{"downloadSettings": down}
			} else {
				p.EndpointTuning.DownloadEnabled = &enabled
			}
			if item.disabled {
				p.EndpointTuning.DownloadEnabled = &enabled
			}
			if item.streamOne {
				object(p.Stream, "xhttpSettings")["mode"] = "stream-one"
			}
			if item.plain {
				security := "none"
				p.EndpointTuning.DownloadSecurity = &security
			}
			before, err := json.Marshal([]any{p.Stream, p.EndpointTuning})
			if err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"clash", "stash"} {
				compat := endpointClientCompatibility(p, format)
				if !compat.Supported || slices.Contains(compat.OmittedOptions, "downloadFingerprint") != item.wantOmit {
					t.Fatal("download metadata disagrees with effective inheritance", format, compat)
				}
				out, err := p.endpointDocument(format)
				down := object(object(out, "xhttp-opts"), "download-settings")
				if err != nil || text(down, "client-fingerprint") != item.child || out["client-fingerprint"] != nil {
					t.Fatal("unsupported fingerprint retained or explicit child lost", format, out, err)
				}
				if (item.disabled || item.streamOne) && down != nil {
					t.Fatal("inactive download reappeared", format, down)
				}
			}
			out, err := p.Outbound()
			down := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "downloadSettings")
			wantNative := fingerprint
			if item.child != "" {
				wantNative = item.child
			} else if item.disabled || item.streamOne || item.plain {
				wantNative = ""
			}
			if err != nil || text(object(down, "tlsSettings"), "fingerprint") != wantNative {
				t.Fatal("client omission changed native download semantics", out, err)
			}
			after, err := json.Marshal([]any{p.Stream, p.EndpointTuning})
			if err != nil || string(before) != string(after) {
				t.Fatal("inherited omission rewrote administrator source", err)
			}
		})
	}
}

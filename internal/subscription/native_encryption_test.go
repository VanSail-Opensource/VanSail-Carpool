package subscription

import (
	"errors"
	"net/url"
	"testing"
)

func TestVLESSEncryptionSurvivesNativeExportAndUnsupportedClientIsolation(t *testing.T) {
	for _, projection := range []string{"plain", "member", "administrator"} {
		t.Run(projection, func(t *testing.T) {
			proxy := templateProxy("encrypted", "Encrypted VLESS")
			// The public encryption material belongs to the administrator. Export
			// must preserve its exact value, including URL-reserved characters.
			const encryption = "mlkem768x25519plus.native.0rtt.public+key/segment="
			proxy.Account["encryption"] = encryption
			switch projection {
			case "member":
				proxy.Tuning = &ConnectionOverrides{}
			case "administrator":
				proxy.EndpointTuning = &EndpointOptions{}
			}
			link, err := proxy.NativeLink()
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(link)
			if err != nil || parsed.Query().Get("encryption") != encryption {
				t.Fatal("administrator encryption was lost", link, err)
			}
			if _, err = proxy.SingBox(); !errors.Is(err, ErrUnsupported) {
				t.Fatal("unsupported encryption produced a broken sing-box endpoint", err)
			}
			plain := templateProxy("plain", "Plain VLESS")
			eligible, excluded, err := CompatibleProxies("sing-box", []Proxy{proxy, plain})
			if err != nil || excluded != 1 || len(eligible) != 1 || eligible[0].ID != plain.ID {
				t.Fatal("unsupported encryption disabled compatible peers", eligible, excluded, err)
			}
			for _, format := range []string{"clash", "stash"} {
				out, err := proxy.yamlProxy(format)
				if err != nil || out["encryption"] != encryption {
					t.Fatal("supported YAML encryption lost", out, err)
				}
			}
			if proxy.Account["encryption"] != encryption {
				t.Fatal("export mutated administrator material")
			}
		})
	}
}

func TestNativeVLESSAbsentEncryptionRetainsNoneDefault(t *testing.T) {
	proxy := templateProxy("default", "Default")
	delete(proxy.Account, "encryption")
	link, err := proxy.NativeLink()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil || parsed.Query().Get("encryption") != "none" {
		t.Fatal(link, err)
	}
}

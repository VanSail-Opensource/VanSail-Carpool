package xnode

import (
	"slices"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func splitCatalogProxy(id, mode string) subscription.Proxy {
	return subscription.Proxy{
		ID:         id,
		Name:       id,
		NodeID:     "node",
		InboundTag: "xhttp",
		Protocol:   "vless",
		Address:    id + ".example",
		Port:       443,
		Account:    map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f"},
		Stream: map[string]any{
			"network": "xhttp",
			"xhttpSettings": map[string]any{
				"path": "/shared",
				"mode": mode,
			},
		},
	}
}

func TestSplitHostCatalogKeepsPairModesAndDeduplicatesCompatibilityModes(t *testing.T) {
	hosts := splitHostCatalog([]subscription.Proxy{
		splitCatalogProxy("upload", "auto"),
		splitCatalogProxy("download-one", "packet-up"),
		splitCatalogProxy("download-two", "stream-up"),
	})
	if len(hosts) != 3 {
		t.Fatalf("split hosts = %d, want 3", len(hosts))
	}
	var upload *DIYSplitHost
	for index := range hosts {
		if hosts[index].ID == "upload" {
			upload = &hosts[index]
			break
		}
	}
	if upload == nil {
		t.Fatal("upload host missing")
	}
	if !slices.Equal(upload.DownloadIDs, []string{"download-one", "download-two"}) {
		t.Fatalf("download ids = %v", upload.DownloadIDs)
	}
	if !slices.Equal(upload.Modes, []string{"auto", "packet-up", "stream-up"}) {
		t.Fatalf("compatibility modes = %v", upload.Modes)
	}
	if len(upload.DownloadOptions) != 2 || upload.DownloadOptions[0].DownloadEndpointID != "download-one" || upload.DownloadOptions[1].DownloadEndpointID != "download-two" {
		t.Fatalf("download options = %+v", upload.DownloadOptions)
	}
	for _, option := range upload.DownloadOptions {
		if !slices.Equal(option.Modes, []string{"auto", "packet-up", "stream-up"}) {
			t.Fatalf("pair %s modes = %v", option.DownloadEndpointID, option.Modes)
		}
	}
}

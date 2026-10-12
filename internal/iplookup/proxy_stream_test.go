package iplookup

import (
	"context"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

// generatedInventoryReader generates a sparse, unique FireHOL-shaped IPv4 feed
// in small batches, without allocating its complete body in the test fixture.
type generatedInventoryReader struct {
	next, total, bytes int
	pending            []byte
}

func generatedInventoryIP(n int) netip.Addr {
	v := uint32(n*5) & 0xffffff
	return netip.AddrFrom4([4]byte{11, byte(v >> 16), byte(v >> 8), byte(v)})
}

func (r *generatedInventoryReader) Read(out []byte) (int, error) {
	if len(r.pending) == 0 {
		if r.next == r.total {
			return 0, io.EOF
		}
		for len(r.pending) < 32<<10 && r.next < r.total {
			ip := generatedInventoryIP(r.next).As4()
			for i, octet := range ip {
				if i > 0 {
					r.pending = append(r.pending, '.')
				}
				r.pending = strconv.AppendInt(r.pending, int64(octet), 10)
			}
			r.pending = append(r.pending, '/', '3', '2', '\n')
			r.next++
		}
	}
	n := copy(out, r.pending)
	r.pending = r.pending[n:]
	r.bytes += n
	return n, nil
}

func TestThreeMillionEntryInventoryStreamsCompletely(t *testing.T) {
	s := NewEnhancedService(nil, nil, nil)
	reader := &generatedInventoryReader{total: 3000000}
	s.inventoryStream = func(ctx context.Context, _ string, _ Config, emit func(string)) error {
		return readInventory(ctx, io.MultiReader(strings.NewReader("# FireHOL-style source\n10.0.0.0/8\n"), reader), emit)
	}
	updates := 0
	started := time.Now()
	index, result := s.collectSource(context.Background(), ProxySource{ID: "firehol", Category: ProxyDatabaseCategory}, DefaultConfig(), false, func(ProxySourceResult) { updates++ })
	if result.Status != "success" || index.count() != reader.total || result.Processed != reader.total || reader.bytes <= maxProxyBody {
		t.Fatalf("truncated inventory: %+v bytes=%d", result, reader.bytes)
	}
	if len(s.dns) != 0 || result.DNSFailed != 0 || updates > int(time.Since(started)/(250*time.Millisecond))+6 {
		t.Fatalf("literal IPs entered DNS or emitted per-entry progress: updates=%d", updates)
	}
	if !index.contains(generatedInventoryIP(0)) || !index.contains(generatedInventoryIP(reader.total-1)) || index.contains(generatedInventoryIP(reader.total)) {
		t.Fatal("large index membership incorrect")
	}
	// Even sparse lists must retain at least 70% less data than 3 million
	// netip.Prefix keys alone (32 bytes each), before map overhead or strings.
	var retained uint64
	for _, bitmap := range index.v4 {
		if bitmap != nil {
			retained += bitmap.GetSizeInBytes()
		}
	}
	if retained > uint64(reader.total*32*3/10) {
		t.Fatalf("retained bitmap too large: %d", retained)
	}
	t.Logf("entries=%d input_bytes=%d compressed_bytes=%d progress_updates=%d", index.count(), reader.bytes, retained, updates)
}

func TestInventoryStreamingCancellationAndBounds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	seen := 0
	err := readInventory(ctx, strings.NewReader("11.0.0.1\n11.0.0.2\n"), func(string) { seen++; cancel() })
	if err != context.Canceled || seen != 1 {
		t.Fatalf("cancellation: %v entries=%d", err, seen)
	}
	for _, body := range []string{"11.0.0.1" + strings.Repeat(" ", maxInventoryLine), "proxies:\n" + strings.Repeat("x\n", maxProxyBody/2)} {
		if code := ErrorCode(readInventory(context.Background(), strings.NewReader(body), func(string) {})); code != "IP_PROXY_PARSE" {
			t.Fatalf("bound not reported: %s", code)
		}
	}
}

func TestStructuredInventoryPreservesLongDocumentLines(t *testing.T) {
	body := `{"proxies":[{"server":"edge.example.net"}],"padding":"` + strings.Repeat("x", maxInventoryLine+1) + `"}`
	seen := []string{}
	err := readInventory(context.Background(), strings.NewReader(body), func(value string) { seen = append(seen, value) })
	if err != nil || len(seen) != 1 || seen[0] != "edge.example.net" {
		t.Fatalf("long structured line lost supported host: %v error=%v", seen, err)
	}
}

func TestInventorySizeLimitDoesNotAcceptTruncatedPrefix(t *testing.T) {
	first := "11.0.0.1\n"
	seen := []string{}
	reader := io.MultiReader(strings.NewReader(first), strings.NewReader("2001:ac8:16::/48\n"))
	err := readInventoryBounded(context.Background(), reader, int64(len(first)+14), func(value string) { seen = append(seen, value) })
	if ErrorCode(err) != "IP_PROXY_LIMIT" || len(seen) != 1 || seen[0] != "11.0.0.1" {
		t.Fatalf("accepted truncated CIDR: %v error=%v", seen, err)
	}
}

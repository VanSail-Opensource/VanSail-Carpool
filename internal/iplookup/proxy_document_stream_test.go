package iplookup

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestStreamingDocumentsExceedFallbackWithoutObjectTree(t *testing.T) {
	for _, format := range []string{"json", "yaml", "html", "base64"} {
		t.Run(format, func(t *testing.T) {
			var input strings.Builder
			switch format {
			case "json":
				input.WriteString("{\"proxies\":[")
			case "yaml":
				input.WriteString("proxies:\n")
			case "html":
				input.WriteString("<table>")
			}
			const entries = 18000
			for i := 0; i < entries; i++ {
				switch format {
				case "json":
					if i > 0 {
						input.WriteByte(',')
					}
					fmt.Fprintf(&input, "{\"server\":\"11.%d.%d.1\",\"name\":\"%s\"}", i>>8, i&255, strings.Repeat("n", 50))
				case "yaml":
					fmt.Fprintf(&input, "  - name: %s\n    server: 11.%d.%d.1\n", strings.Repeat("n", 50), i>>8, i&255)
				case "html":
					fmt.Fprintf(&input, "<tr><td>11.%d.%d.1</td><td>%s</td></tr>", i>>8, i&255, strings.Repeat("n", 50))
				case "base64":
					fmt.Fprintf(&input, "socks5://%s@11.%d.%d.1:1080\n", strings.Repeat("x", 50), i>>8, i&255)
				}
			}
			switch format {
			case "json":
				input.WriteString("]}")
			case "html":
				input.WriteString("</table>")
			case "yaml":
				input.WriteString("proxy-groups:\n  - name: group\n")
			}
			raw := input.String()
			if format == "base64" {
				raw = base64.RawURLEncoding.EncodeToString([]byte(raw))
			}
			if len(raw) <= maxComplexInventoryBody {
				t.Fatal("fixture did not exercise streaming")
			}
			count := 0
			if err := readInventory(context.Background(), strings.NewReader(raw), func(string) { count++ }); err != nil || count != entries {
				t.Fatalf("entries=%d error=%v", count, err)
			}
		})
	}
}

func TestComplexDocumentFallbackIsBounded(t *testing.T) {
	raw := "configuration:\n" + strings.Repeat("  unrelated: value\n", 100000)
	if err := readInventory(context.Background(), strings.NewReader(raw), func(string) {}); err == nil {
		t.Fatal("unbounded complex fallback accepted")
	}
}

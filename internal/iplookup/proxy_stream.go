package iplookup

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/netip"
	"strings"
)

const maxPlainInventoryBody = 128 << 20
const maxInventoryLine = 1 << 20

// readInventory streams endpoint lines and buffers only bounded structured or
// encoded documents. emit receives canonical public hosts or masked prefixes.
func readInventory(ctx context.Context, reader io.Reader, emit func(string)) error {
	return readInventoryBounded(ctx, reader, maxPlainInventoryBody, emit)
}

func readInventoryBounded(ctx context.Context, reader io.Reader, bodyLimit int64, emit func(string)) error {
	limited := &io.LimitedReader{R: reader, N: bodyLimit + 1}
	scanner := bufio.NewScanner(limited)
	// Structured JSON and encoded subscriptions may occupy one long line.
	// Plain-line limits apply after format detection; documents retain 16 MiB.
	scanner.Buffer(make([]byte, 64<<10), maxProxyBody+1)
	started := false
	var document bytes.Buffer
	buffered := false
	for scanner.Scan() {
		// Never accept a final token cut mid-CIDR by a size limit or broken
		// transport: its shortened prefix length could flag unrelated IPs.
		if limited.N <= 0 {
			return &CodeError{"IP_PROXY_LIMIT"}
		}
		if scanner.Err() != nil {
			return &CodeError{"IP_PROXY_PARSE"}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if !started {
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "*") {
				continue
			}
			started = true
			buffered = inventoryDocument(line)
		}
		if buffered {
			if document.Len()+len(rawLine)+1 > maxProxyBody {
				return &CodeError{"IP_PROXY_PARSE"}
			}
			document.WriteString(rawLine)
			document.WriteByte('\n')
			continue
		}
		if len(rawLine) > maxInventoryLine {
			return &CodeError{"IP_PROXY_PARSE"}
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "*") {
			continue
		}
		// This hot path avoids regexp, YAML/base64 detection and host slices.
		if value := proxyInventoryEntry(line); value != "" && !strings.Contains(line, ",") && !strings.Contains(line, "://") {
			emit(value)
		} else if err := walkInventory([]byte(line), emit); err != nil {
			return err
		}
	}
	if limited.N <= 0 {
		return &CodeError{"IP_PROXY_LIMIT"}
	}
	if err := scanner.Err(); err != nil {
		return &CodeError{"IP_PROXY_PARSE"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if buffered {
		return walkInventory(document.Bytes(), emit)
	}
	return nil
}

func inventoryDocument(line string) bool {
	if _, err := netip.ParseAddr(strings.Trim(line, "[]")); err == nil {
		return false
	}
	if _, err := netip.ParsePrefix(line); err == nil {
		return false
	}
	lower := strings.ToLower(line)
	if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "proxies:") || strings.Contains(line, "server:") || strings.HasPrefix(lower, "<") {
		return true
	}
	if strings.HasPrefix(line, "[") && endpointHost(line) == "" {
		return true
	}
	if strings.Contains(line, ",") || strings.HasPrefix(lower, "remote ") || strings.Contains(line, "://") {
		return false
	}
	return proxyInventoryEntry(line) == ""
}

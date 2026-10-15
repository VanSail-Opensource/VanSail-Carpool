package iplookup

import (
	"encoding/base64"
	"io"
	"strings"
)

// base64InventoryReader normalizes the four existing subscription encodings
// without retaining a full encoded and decoded document simultaneously.
type base64InventoryReader struct{ source io.Reader }

func (r base64InventoryReader) Read(data []byte) (int, error) {
	for {
		n, err := r.source.Read(data)
		out := 0
		for _, c := range data[:n] {
			switch c {
			case ' ', '\t', '\r', '\n', '=':
				continue
			case '-':
				c = '+'
			case '_':
				c = '/'
			}
			data[out] = c
			out++
		}
		if out > 0 || err != nil {
			return out, err
		}
	}
}

func isBase64Inventory(prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if len(prefix) < 8 || strings.Contains(prefix, ":") {
		return false
	}
	for _, c := range prefix {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=_-\r\n\t ", c) {
			return false
		}
	}
	// Decimal-encoded subscriptions retain the bounded legacy adapter.
	first := strings.Fields(prefix)
	if len(first) > 0 && len(first[0]) <= 7 && strings.Trim(first[0], "0123456789") == "" {
		return false
	}
	return true
}

func decodedInventory(reader io.Reader) io.Reader {
	return base64.NewDecoder(base64.RawStdEncoding, base64InventoryReader{source: reader})
}

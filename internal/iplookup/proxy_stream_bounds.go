package iplookup

import (
	"bufio"
	"bytes"
	"io"
)

type inventoryBodyReader struct{ limited *io.LimitedReader }

func (r inventoryBodyReader) Read(data []byte) (int, error) {
	n, err := r.limited.Read(data)
	if r.limited.N <= 0 {
		return n, &CodeError{"IP_PROXY_LIMIT"}
	}
	return n, err
}

// Only newline-terminated tokens survive a truncated body. In particular, a
// shortened /48 must never become a valid but much broader /4 positive match.
func inventoryLineSplit(limited *io.LimitedReader) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, bytes.TrimSuffix(data[:i], []byte{'\r'}), nil
		}
		if atEOF {
			if limited.N <= 0 {
				return 0, nil, &CodeError{"IP_PROXY_LIMIT"}
			}
			if len(data) > 0 {
				return len(data), bytes.TrimSuffix(data, []byte{'\r'}), nil
			}
		}
		return 0, nil, nil
	}
}

package jsonlimit

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type oneByteReader struct{ io.Reader }

func (r oneByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}

func TestReaderBoundsAcrossChunks(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		limits    Limits
		ok        bool
	}{
		{"array", "[{},{}]", Limits{ArrayItems: 2}, true},
		{"array overflow", "[{},{},{}]", Limits{ArrayItems: 2}, false},
		{"nested", "[[[0]]]", Limits{Depth: 2}, false},
		{"nodes", "[1,2,3]", Limits{Nodes: 3}, false},
		{"string", "{\"key\":\"12345\"}", Limits{StringBytes: 4}, false},
		{"quoted syntax", "[\"[},\\\"x\",\"ok\"]", Limits{ArrayItems: 2}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, chunked := range []bool{false, true} {
				var source io.Reader = strings.NewReader(tc.raw)
				if chunked {
					source = oneByteReader{source}
				}
				var value any
				err := json.NewDecoder(NewReader(source, tc.limits)).Decode(&value)
				if (err == nil) != tc.ok {
					t.Fatalf("chunked=%v err=%v", chunked, err)
				}
			}
		})
	}
}

func TestDecodeSliceStopsBeforeExcessElement(t *testing.T) {
	values, err := DecodeSlice[int]([]byte("[1,2]"), 2)
	if err != nil || len(values) != 2 {
		t.Fatal(values, err)
	}
	if _, err := DecodeSlice[int]([]byte("[1,2,{}]"), 2); err != ErrLimit {
		t.Fatalf("excess element decoded: %v", err)
	}
	if _, err := DecodeSlice[int]([]byte("[1] {}"), 2); err == nil {
		t.Fatal("trailing document accepted")
	}
}

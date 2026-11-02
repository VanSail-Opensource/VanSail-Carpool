package database

import "testing"

func TestProviderCompletionMergePreservesExactEvidence(t *testing.T) {
	for _, tc := range []struct{ document, patch, want string }{
		{`{"identity":9007199254740993,"receipt":{"id":"paid","pending":true}}`,
			`{"receipt":{"pending":null,"ready":true}}`,
			`{"identity":9007199254740993,"receipt":{"id":"paid","ready":true}}`},
		{`{"nodes":["old"],"reference":"keep"}`, `{"nodes":["new"]}`, `{"nodes":["new"],"reference":"keep"}`},
		{`{"a":{"old":1}}`, `{"a":null}`, `{}`},
		{`{"a":3}`, `{"a":{"new":9007199254740995}}`, `{"a":{"new":9007199254740995}}`},
	} {
		got, err := mergeProviderResult(tc.document, tc.patch)
		if err != nil || got != tc.want {
			t.Fatalf("completion merge = %s, %v; want %s", got, err, tc.want)
		}
	}
}

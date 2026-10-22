package xnode

import (
	"reflect"
	"testing"
)

func TestHostBypassRequiresExactManagedOrigin(t *testing.T) {
	trusted := []string{"8.8.8.8", "2001:4860:4860::8888"}
	answers := []string{"8.8.8.8", "::ffff:8.8.8.8", "2001:4860:4860::8888", "1.1.1.1", "10.0.0.1"}
	want := []string{"2001:4860:4860::8888", "8.8.8.8"}
	if got := verifiedHostOrigins(answers, trusted); !reflect.DeepEqual(got, want) {
		t.Fatalf("origin resolution: %v", got)
	}
	if got := verifiedHostOrigins([]string{"1.1.1.1"}, trusted); len(got) != 0 {
		t.Fatal("unverified CDN address bypassed detection", got)
	}
	if got := verifiedHostOrigins(answers, nil); len(got) != 0 {
		t.Fatal("removed ownership retained DNS exemption", got)
	}
}

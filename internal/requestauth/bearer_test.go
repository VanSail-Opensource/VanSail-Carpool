package requestauth

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBearerRetainsExistingRequestBudgets(t *testing.T) {
	v, err := New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	v.replays.maxNonces = 2
	for i := 0; i < 2; i++ {
		if err = v.CheckBearer(httptest.NewRequest("GET", "/", nil), "token", false); err != nil {
			t.Fatal(err)
		}
	}
	if err = v.CheckBearer(httptest.NewRequest("GET", "/", nil), "token", false); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate bypassed: %v", err)
	}
	v.maxBody = 4
	if err = v.CheckBearer(httptest.NewRequest("POST", "/", strings.NewReader("large")), "other", false); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("body limit bypassed: %v", err)
	}
}

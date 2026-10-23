package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSquadProductRequestDecodesTypedProfile(t *testing.T) {
	request := httptest.NewRequest("PUT", "/api/v1/admin/squad-products/squad-1", strings.NewReader(`{"groupId":"squad-1","name":"ignored","description":"extra","profile":{"type":"international_network","portMbps":null,"countryCode":"sg","upstreamCarriers":["A, B"]},"priceTxbMinor":"100","visible":true}`))
	response := httptest.NewRecorder()
	var input squadProductRequest
	if err := decodeJSON(response, request, &input); err != nil {
		t.Fatalf("decodeJSON() error = %v", err)
	}
	if input.Profile == nil || input.Profile.CountryCode != "sg" || len(input.Profile.UpstreamCarriers) != 1 {
		t.Fatalf("decoded profile = %+v", input.Profile)
	}
}

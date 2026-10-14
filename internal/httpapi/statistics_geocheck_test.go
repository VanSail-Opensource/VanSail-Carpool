package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	productstats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
	"github.com/go-chi/chi/v5"
)

const httpGeocheckNodeUUID = "373f14bc-089a-4c3a-91c3-3421e7c83367"

func TestStatisticsNodeGeocheckReturnsImageOnlyOrUnavailable(t *testing.T) {
	service := productstats.NewService(nil, httpGeocheckProvider{})
	if err := service.RefreshGeochecks(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("RefreshGeochecks(): %v", err)
	}
	waitForHTTPGeocheck(t, service)
	server := &Server{deps: Dependencies{Statistics: service}}
	response := httptest.NewRecorder()
	server.statisticsNodeGeocheck(response, geocheckRequest(httpGeocheckNodeUUID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var image model.StatisticsNodeGeocheck
	if err := json.NewDecoder(response.Body).Decode(&image); err != nil {
		t.Fatalf("decode image response: %v", err)
	}
	if image.NodeUUID != httpGeocheckNodeUUID || image.Image.Data != "PHN2Zy8+" {
		t.Fatalf("image response = %#v", image)
	}

	response = httptest.NewRecorder()
	server.statisticsNodeGeocheck(response, geocheckRequest("missing"))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unavailable status = %d, want %d", response.Code, http.StatusNotFound)
	}
	var failure apiError
	if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
		t.Fatalf("decode unavailable response: %v", err)
	}
	if failure.Code != "NODE_GEOCHECK_UNAVAILABLE" {
		t.Fatalf("unavailable code = %q", failure.Code)
	}
}

func geocheckRequest(nodeUUID string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/statistics/nodes/"+nodeUUID+"/geocheck", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("nodeUuid", nodeUUID)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func waitForHTTPGeocheck(t *testing.T, service *productstats.Service) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, ok := service.NodeGeocheck(httpGeocheckNodeUUID); ok {
			return
		}
		select {
		case <-deadline:
			t.Fatal("Geocheck image was not cached")
		case <-ticker.C:
		}
	}
}

type httpGeocheckProvider struct{}

func (httpGeocheckProvider) Digest(context.Context, time.Time, time.Time) (productstats.Digest, error) {
	return productstats.Digest{}, nil
}
func (httpGeocheckProvider) Traffic(context.Context, time.Time, time.Time) (productstats.Traffic, error) {
	return productstats.Traffic{}, nil
}
func (httpGeocheckProvider) UsageSnapshotForRollover(context.Context, string, time.Time, time.Time) (rollover.UsageSnapshot, error) {
	return rollover.UsageSnapshot{}, nil
}
func (httpGeocheckProvider) Nodes(context.Context) ([]productstats.Node, error) {
	return []productstats.Node{{UUID: httpGeocheckNodeUUID}}, nil
}
func (httpGeocheckProvider) RequestNodeGeocheck(context.Context, string) (string, error) {
	return "job", nil
}
func (httpGeocheckProvider) NodeGeocheckResult(context.Context, string) (productstats.GeocheckResult, error) {
	return productstats.GeocheckResult{Completed: true, Success: true, NodeUUID: httpGeocheckNodeUUID,
		Image: &productstats.GeocheckImage{Format: "svg", MediaType: "image/svg+xml", Encoding: "base64", Data: "PHN2Zy8+"}}, nil
}
func (httpGeocheckProvider) Hosts(context.Context) ([]productstats.Host, error)     { return nil, nil }
func (httpGeocheckProvider) UpdateHostRemark(context.Context, string, string) error { return nil }

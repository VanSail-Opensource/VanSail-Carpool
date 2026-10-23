package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

const connectivityTestHost = "80000000-0000-4000-8000-000000000001"
const connectivityOtherHost = "80000000-0000-4000-8000-000000000002"

func connectivityTestAttempt(index int, host string, at time.Time) connectivity.Attempt {
	return connectivity.Attempt{
		ID:    fmt.Sprintf("90000000-0000-4000-8000-%012d", index),
		RunID: "90000000-0000-4000-8000-999999999999", HostUUID: host,
		ConfigHash: strings.Repeat("a", 64), ProxyUserID: 41, Trigger: "manual", StartedAt: at,
	}
}

func seedConnectivityAttempt(t *testing.T, store *Store, attempt connectivity.Attempt) {
	t.Helper()
	if err := store.BeginConnectivityAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("begin attempt %s: %v", attempt.ID, err)
	}
}

func connectivityOutcome() connectivity.Outcome {
	latency, status := 4.25, 204
	return connectivity.Outcome{Status: "connected", LatencyMS: &latency, HTTPStatus: &status}
}

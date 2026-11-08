package iplookup

import (
	"encoding/json"
	"testing"
)

func TestRetiredScanFieldsDoNotReappearFromStoredDocuments(t *testing.T) {
	config, err := DecodeConfig(`{"proxySources":[{"enabled":true,"url":"http://127.0.0.1/private"}],"probePorts":[443],"dnsResolvers":["127.0.0.1:53"]}`)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("invalid config")
	}
	for _, key := range []string{"proxySources", "probePorts", "dnsResolvers"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("retired setting %s retained", key)
		}
	}
	report, err := DecodeReport([]byte(`{"id":"historical","ip":"8.8.8.8","status":"processing","proxy":{"status":"success"},"subnet":{"prefix":"8.8.8.0/24"},"ports":[{"port":443}],"_checkpoint":{"enhanced":{"inventoryDone":true,"liveDone":true,"phase":"subnet"}}}`))
	if err != nil || report.Checkpoint == nil || report.Checkpoint.Enrichment == nil || !report.Checkpoint.Enrichment.LiveDone {
		t.Fatalf("legacy live recovery lost: %+v %v", report, err)
	}
	raw, err = json.Marshal(PublicReport(report))
	if err != nil {
		t.Fatal(err)
	}
	fields = nil
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("invalid report")
	}
	for _, key := range []string{"proxy", "subnet", "ports", "_checkpoint"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("retired projection %s retained", key)
		}
	}
}

func TestQualityProgressWaitsForRetainedNetworkEnrichment(t *testing.T) {
	r := Report{Status: "processing", Checkpoint: &Checkpoint{Stages: []Stage{{ID: "ipapi", Status: "success"}}, Enrichment: &EnrichmentCheckpoint{}}}
	progress := CheckProgress(r)
	if progress.Phase != "live" || progress.Completed >= progress.Total {
		t.Fatalf("enrichment prematurely complete: %+v", progress)
	}
	r.Checkpoint.Enrichment.LiveDone = true
	progress = CheckProgress(r)
	if progress.Completed != progress.Total {
		t.Fatalf("finished enrichment missing: %+v", progress)
	}
}

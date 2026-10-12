package iplookup

import "testing"

func TestCurrentSettingsReconcileOnlyUnattemptedStages(t *testing.T) {
	r := Report{Checkpoint: &Checkpoint{Stages: []Stage{{ID: "abuseipdb", Status: "success", Attempted: true, Complete: true}, {ID: "ipapi", Status: "queued"}, {ID: "maxmind", Status: "queued"}, {ID: "scamalytics", Status: "disabled"}}}}
	c := Config{Providers: []ProviderConfig{{ID: "maxmind", Enabled: true}, {ID: "ipapi", Enabled: false}, {ID: "scamalytics", Enabled: true}, {ID: "abuseipdb", Enabled: true}}}
	reconcilePendingStages(&r, c)
	if r.Checkpoint.Stages[0].ID != "abuseipdb" || !r.Checkpoint.Stages[0].Attempted || !r.Checkpoint.Stages[0].Complete {
		t.Fatal("completed evidence changed")
	}
	for _, stage := range r.Checkpoint.Stages {
		if stage.ID == "ipapi" && stage.Status != "disabled" {
			t.Fatal("disabled pending provider still queued")
		}
		if stage.ID == "scamalytics" && stage.Status != "disabled" {
			t.Fatal("newly enabled source added to existing run")
		}
	}
	if r.Checkpoint.Stages[2].ID != "maxmind" {
		t.Fatal("current pending order not applied")
	}
}

package subscription

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestEndpointChildXMUXOverridesOnlyOpposingInheritedChoice(t *testing.T) {
	for _, connections := range []bool{false, true} {
		parent, own, other := "maxConnections", "maxConcurrency", "maxConnections"
		if connections {
			parent, own, other = "maxConcurrency", "maxConnections", "maxConcurrency"
		}
		for _, explicit := range []bool{false, true} {
			child := map[string]any{"hMaxRequestTimes": 99}
			if explicit {
				child[own] = 8
			}
			p := xmuxReviewProxy(parent, 16, child, EndpointOptions{})
			before, _ := json.Marshal(p.Stream)
			out := buildXMUXReview(t, p)
			down := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "downloadSettings")
			mux := object(effectiveXHTTP(object(down, "xhttpSettings")), "xmux")
			if explicit && (fmt.Sprint(mux[own]) != "8" || xmuxMaximum(mux[other]) != 0) || !explicit && fmt.Sprint(mux[parent]) != "16" {
				t.Fatal("child mode did not override only inherited opponent", mux)
			}
			if fmt.Sprint(mux["cMaxReuseTimes"]) != "21" || fmt.Sprint(mux["hMaxRequestTimes"]) != "99" || fmt.Sprint(mux["hKeepAlivePeriod"]) != "37" {
				t.Fatal("unrelated XMUX fields were lost", mux)
			}
			after, _ := json.Marshal(p.Stream)
			if string(before) != string(after) {
				t.Fatal("child merge mutated stored source")
			}
		}
	}
}

func TestEndpointExplicitZeroDoesNotEraseOpposingInheritedXMUX(t *testing.T) {
	zero := &TuningRange{Min: 0, Max: 0}
	p := xmuxReviewProxy("maxConcurrency", 16, nil, EndpointOptions{XmuxMaxConnections: zero})
	out := buildXMUXReview(t, p)
	mux := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "xmux")
	if fmt.Sprint(mux["maxConcurrency"]) != "16" || fmt.Sprint(mux["maxConnections"]) != "0" {
		t.Fatal("explicit zero erased an inherited choice", mux)
	}
	p = xmuxReviewProxy("maxConnections", 16, map[string]any{"maxConcurrency": 0}, EndpointOptions{})
	out = buildXMUXReview(t, p)
	down := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "downloadSettings")
	mux = object(effectiveXHTTP(object(down, "xhttpSettings")), "xmux")
	if fmt.Sprint(mux["maxConnections"]) != "16" || fmt.Sprint(mux["maxConcurrency"]) != "0" {
		t.Fatal("child explicit zero changed inherited opposing mode", mux)
	}
}

func TestEndpointMemberXMUXOverlayUsesSameSafeChoiceSemantics(t *testing.T) {
	p := xmuxReviewProxy("maxConnections", 16, nil, EndpointOptions{})
	positive := &TuningRange{Min: 8, Max: 8}
	p.Tuning = &ConnectionOverrides{XmuxMaxConcurrency: positive}
	out := buildXMUXReview(t, p)
	mux := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "xmux")
	if fmt.Sprint(mux["maxConcurrency"]) != "8" || xmuxMaximum(mux["maxConnections"]) != 0 {
		t.Fatal("member choice broke the inherited XMUX document", mux)
	}
	p.Tuning.XmuxMaxConnections = positive
	if errors := ValidateTuning(*p.Tuning); errors["xmuxMaxConnections"] != "conflict" {
		t.Fatal("member explicit pair bypassed existing allowlist validation", errors)
	}
	out = buildXMUXReview(t, p)
	mux = object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "xmux")
	if fmt.Sprint(mux["maxConnections"]) != "16" || xmuxMaximum(mux["maxConcurrency"]) != 0 {
		t.Fatal("cached member pair did not retain valid inherited policy", mux)
	}
}

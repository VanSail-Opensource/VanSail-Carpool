package iplookup

import (
	"io"
	"strings"
	"time"
)

type trojanReader interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
}

func observeTrojan(client trojanReader, key, negotiated string, started time.Time, now func() time.Time, milestone func(int) error) probeOutcome {
	out := probeOutcome{status: "success"}
	deadlines := []int{10}
	switch key {
	case "trojan_short":
		deadlines = []int{150}
	case "trojan_incomplete":
		deadlines = trojanMilestones
	case "trojan_overbuffer":
		deadlines = []int{20}
	case "trojan_close":
		deadlines = []int{35}
	}
	response := make([]byte, 4096)
	for _, seconds := range deadlines {
		if err := milestone(seconds); err != nil {
			out.status = "error"
			out.err = err
			return out
		}
		if err := client.SetReadDeadline(started.Add(time.Duration(seconds) * time.Second)); err != nil {
			out.status = "error"
			out.err = err
			return out
		}
		n, err := client.Read(response)
		if n > 0 {
			text := string(response[:n])
			if negotiated == "h2" && strings.HasPrefix(text, "HTTP/1.") {
				out.findings = append(out.findings, ProbeFinding{Protocol: "trojan", Confidence: "observed", Evidence: "alpn_http_mismatch"})
			}
			if key == "trojan_close" {
				received := now()
				if err := client.SetReadDeadline(received.Add(35 * time.Second)); err != nil {
					out.err = err
					return out
				}
				_, closeErr := client.Read(response)
				elapsed := now().Sub(received)
				if closeErr == io.EOF && elapsed >= 29*time.Second && elapsed <= 31*time.Second {
					out.findings = append(out.findings, ProbeFinding{Protocol: "trojan", Confidence: "observed", Evidence: "close_30_seconds"})
				}
			}
			return out
		}
		if err == io.EOF {
			return out
		}
		if err != nil {
			if timeout, ok := err.(interface{ Timeout() bool }); !ok || !timeout.Timeout() {
				out.result = "unavailable"
				return out
			}
		}
	}
	if key == "trojan_short" || key == "trojan_incomplete" || key == "trojan_overbuffer" {
		out.findings = append(out.findings, ProbeFinding{Protocol: "trojan", Confidence: "observed", Evidence: key + "_timing"})
	}
	return out
}

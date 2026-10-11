package iplookup

import (
	"io"
	"reflect"
	"testing"
	"time"
)

type probeTimeout struct{}

func (probeTimeout) Error() string { return "observation deadline" }
func (probeTimeout) Timeout() bool { return true }

type simulatedObservation struct {
	now       time.Time
	deadlines []time.Time
}

func (s *simulatedObservation) SetReadDeadline(at time.Time) error {
	s.now = at
	s.deadlines = append(s.deadlines, at)
	return nil
}
func (s *simulatedObservation) Read([]byte) (int, error) { return 0, probeTimeout{} }

type countingProbeWriter struct {
	bytes  int
	writes int
}

func (w *countingProbeWriter) Write(b []byte) (int, error) {
	w.bytes += len(b)
	w.writes++
	return len(b), nil
}

func TestTrojanFullObservationWindowsWithoutSleeping(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want []int
	}{{"trojan_incomplete", []int{150, 595, 605}}, {"trojan_short", []int{150}}, {"trojan_overbuffer", []int{20}}} {
		t.Run(tc.key, func(t *testing.T) {
			start := time.Unix(100, 0)
			stream := &simulatedObservation{now: start}
			milestones := []int{}
			result := observeTrojan(stream, tc.key, "h2", start, func() time.Time { return stream.now }, func(s int) error { milestones = append(milestones, s); return nil })
			if !reflect.DeepEqual(milestones, tc.want) {
				t.Fatalf("milestones %v", milestones)
			}
			if result.status != "success" || len(result.findings) != 1 || result.findings[0].Confidence != "suspected" {
				t.Fatalf("unexpected classification: %+v", result)
			}
		})
	}
}
func TestTrojanRepeatedHeaderIsFullyStreamed(t *testing.T) {
	writer := &countingProbeWriter{}
	if err := writeTrojanRequest(writer, "trojan_overbuffer"); err != nil {
		t.Fatal(err)
	}
	if writer.bytes != 1140036 || writer.writes != 12001 {
		t.Fatalf("truncated request: bytes=%d writes=%d", writer.bytes, writer.writes)
	}
}

type immediateEOF struct{}

func (immediateEOF) SetReadDeadline(time.Time) error { return nil }
func (immediateEOF) Read([]byte) (int, error)        { return 0, io.EOF }
func TestTrojanPeerClosureIsNotConfirmedProxy(t *testing.T) {
	result := observeTrojan(immediateEOF{}, "trojan_incomplete", "h2", time.Now(), time.Now, func(int) error { return nil })
	if len(result.findings) != 0 || result.status != "success" {
		t.Fatalf("ordinary closure misclassified: %+v", result)
	}
}

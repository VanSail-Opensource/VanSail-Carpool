package xnode

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/klauspost/compress/zstd"
)

func TestComputeRejectsMalformedBoundedResultUploads(t *testing.T) {
	writer, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	bomb := writer.EncodeAll(bytes.Repeat([]byte(" "), compute.MaxResultBytes+1), nil)
	writer.Close()
	chunk := func(raw []byte) wire.ComputeChunk {
		return wire.ComputeChunk{Index: 0, Total: 1, ResultHash: compute.Digest(raw), Data: base64.StdEncoding.EncodeToString(raw)}
	}
	malformed := chunk([]byte("invalid zstd"))
	invalidBase64 := malformed
	invalidBase64.Data = "!"
	oversized := malformed
	oversized.Total = 17
	wrongHash := malformed
	wrongHash.ResultHash = strings.Repeat("0", 64)
	first := malformed
	first.Total = 2
	duplicate := first
	duplicate.Data = base64.StdEncoding.EncodeToString([]byte("conflicting bytes"))
	for _, test := range []struct {
		name   string
		chunks []wire.ComputeChunk
	}{
		{"malformed compression", []wire.ComputeChunk{malformed}},
		{"invalid encoding", []wire.ComputeChunk{invalidBase64}},
		{"too many chunks", []wire.ComputeChunk{oversized}},
		{"wrong digest", []wire.ComputeChunk{wrongHash}},
		{"conflicting duplicate", []wire.ComputeChunk{first, duplicate}},
		{"decompression limit", []wire.ComputeChunk{chunk(bomb)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			attempt := &computeAttempt{chunks: make(chan wire.ComputeChunk, 24), lease: time.Now().Add(time.Minute)}
			for _, c := range test.chunks {
				attempt.chunks <- c
			}
			if _, err := newComputePool(nil).collect(ctx, attempt, &agentPeer{ctx: ctx}); err == nil {
				t.Fatal("malformed upload accepted")
			}
		})
	}
}

func TestComputeStaleAttemptsAndConnectionsCannotClaimOrUpload(t *testing.T) {
	m := NewMaster(&Service{}, nil)
	ctx := context.Background()
	peer := &agentPeer{nodeID: "node", ctx: ctx, computePool: true}
	m.peers[peer.nodeID] = peer
	m.live[peer.nodeID] = liveSnapshot{receivedAt: time.Now(), report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	signal := wire.ComputeSignal{BatchID: "batch", Attempt: 2, InputHash: strings.Repeat("a", 64)}
	a := &computeAttempt{offer: wire.ComputeAnnouncement{Signal: signal}, claimed: make(chan *agentPeer, 1), chunks: make(chan wire.ComputeChunk, 24)}
	m.compute.attempts[signal.BatchID] = a
	old := signal
	old.Attempt = 1
	if err := m.receiveCompute(ctx, peer, wire.New("compute.claim", old.BatchID, 0, old)); err != nil {
		t.Fatal(err)
	}
	if err := m.receiveCompute(ctx, peer, wire.New("compute.result", old.BatchID, 0, wire.ComputeChunk{Signal: old})); err != nil {
		t.Fatal(err)
	}
	if a.winner != nil || len(a.chunks) != 0 {
		t.Fatal("stale attempt changed execution")
	}
	superseded := &agentPeer{nodeID: "node", ctx: ctx, computePool: true}
	if err := m.receiveCompute(ctx, superseded, wire.New("compute.claim", signal.BatchID, 0, signal)); err != ErrConflict {
		t.Fatal("old connection accepted", err)
	}
	if err := m.receiveCompute(ctx, peer, wire.New("compute.claim", signal.BatchID, 0, signal)); err != nil || a.winner != peer || len(a.claimed) != 1 {
		t.Fatal("current claimant rejected", err)
	}
}

func TestComputeConcurrentClaimsAwardExactlyOneNode(t *testing.T) {
	m := computeMasterFixture(t)
	m.SetComputePressure(true)
	first := computeNodeFixture(t, m, "first-racer", 0, false)
	second := computeNodeFixture(t, m, "second-racer", 0, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := m.Compute(ctx, compute.Task{ID: "anonymous", Kind: "forecast_quantiles_v1", Samples: []float64{2, 4}}, func(compute.Result) error { return nil })
	if err != nil || first.Load()+second.Load() != 1 {
		t.Fatal("nonexclusive grant", err, first.Load(), second.Load())
	}
}

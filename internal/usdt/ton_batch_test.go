package usdt

import (
	"errors"
	"testing"
)

func TestTONBatchedTraceSelectsExactSenderMessage(t *testing.T) {
	spec, _ := Network("ton")
	first, trace, wallet, owner := tonProofFixture()
	second := first
	second.TxHash = "second-sender-hash"
	sender := trace.Transactions["sender"]
	sender.Hash = second.TxHash
	sender.Out = []tonMessage{{Hash: "second-message", Destination: wallet}}
	receiver := trace.Transactions["receiver"]
	receiver.Hash, receiver.In.Hash = "second-receive-hash", "second-message"
	trace.Transactions["second-sender"] = sender
	trace.Transactions["second-receiver"] = receiver

	// Go map iteration is unordered. Repeated reads of the same trace must
	// resolve each indexed transfer to its own receiving event every time.
	for range 64 {
		for _, tc := range []struct {
			item        tonTransfer
			hash, event string
		}{
			{first, "receive-hash", "message"},
			{second, "second-receive-hash", "second-message"},
		} {
			transfer, err := decodeTONTransfer(spec, tc.item, trace, wallet, owner)
			if err != nil || transfer.TxHash != tc.hash || transfer.EventID != tc.event || transfer.AtomicAmount != tc.item.Amount {
				t.Fatalf("batched sender/event mismatch: %+v, %v", transfer, err)
			}
		}
	}

	delete(trace.Transactions, "second-receiver")
	if _, err := decodeTONTransfer(spec, second, trace, wallet, owner); !errors.Is(err, ErrProof) {
		t.Fatalf("another transfer was accepted without the exact receiving event: %v", err)
	}
	receiver.In.Content.Body = "invalid"
	trace.Transactions["second-receiver"] = receiver
	if _, err := decodeTONTransfer(spec, second, trace, wallet, owner); !errors.Is(err, ErrProof) {
		t.Fatalf("exact sender match bypassed body verification: %v", err)
	}
}

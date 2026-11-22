package database

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestOutboxRetainsBlockedResourceOrderingAcrossCandidateBatches(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert := func(id, payload string) {
		t.Helper()
		if err := s.EnqueueOutbox(ctx, "batch_ordering", payload, now); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}
	insert("active", `{"userId":"owner-a"}`)
	first, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || first == nil {
		t.Fatal(first, err)
	}
	// A full first page is blocked by the active account. Its first item also
	// reserves a chat used by a later account, which must stay blocked even
	// when the candidate query advances to another page.
	for i := range 65 {
		insert(fmt.Sprint(i), fmt.Sprintf(`{"userId":"owner-a","chatId":"shared","sequence":%d}`, i))
	}
	insert("shared-successor", `{"userId":"owner-b","chatId":"shared"}`)
	insert("independent", `{"userId":"owner-c"}`)
	job, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || job == nil || job.Payload != `{"userId":"owner-c"}` {
		t.Fatalf("independent work behind occupied pages did not advance: %+v %v", job, err)
	}
	job, err = s.ClaimOutboxJob(ctx, now)
	if err != nil || job != nil {
		t.Fatalf("multi-resource successor overtook its blocked predecessor: %+v %v", job, err)
	}
	if err := s.CompleteOutboxClaim(ctx, *first, nil, now); err != nil {
		t.Fatal(err)
	}
	job, err = s.ClaimOutboxJob(ctx, now)
	if err != nil || job == nil {
		t.Fatalf("released predecessor did not resume in order: %+v %v", job, err)
	}
	var predecessor struct {
		UserID   string `json:"userId"`
		ChatID   string `json:"chatId"`
		Sequence int    `json:"sequence"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &predecessor); err != nil || predecessor.UserID != "owner-a" || predecessor.ChatID != "shared" || predecessor.Sequence != 0 {
		t.Fatalf("released predecessor order changed: %+v %v", predecessor, err)
	}
}

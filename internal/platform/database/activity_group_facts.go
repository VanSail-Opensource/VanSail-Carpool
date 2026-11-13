package database

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	groupMessageFactBufferCapacity = 4096
	groupMessageFactBatchSize      = 512
)

// ErrGroupMessageFactBufferFull indicates that the bounded webhook buffer
// cannot accept another distinct fact before the scheduler flushes it.
var ErrGroupMessageFactBufferFull = errors.New("group message fact buffer is full")

type groupMessageFactKey struct {
	chatID    int64
	messageID int64
}

type groupMessageFact struct {
	key       groupMessageFactKey
	localDate string
	createdAt time.Time
}

// BufferGroupMessageFact accepts a webhook fact without waiting for PostgreSQL.
func (s *Store) BufferGroupMessageFact(chatID, messageID int64, localDate string, now time.Time) error {
	fact, err := newGroupMessageFact(chatID, messageID, localDate, now)
	if err != nil {
		return err
	}
	s.groupFactsMu.Lock()
	defer s.groupFactsMu.Unlock()
	if _, exists := s.groupFacts[fact.key]; exists {
		return nil
	}
	if len(s.groupFacts) >= groupMessageFactBufferCapacity {
		return ErrGroupMessageFactBufferFull
	}
	if s.groupFacts == nil {
		s.groupFacts = make(map[groupMessageFactKey]groupMessageFact)
	}
	s.groupFacts[fact.key] = fact
	return nil
}

// FlushGroupMessageFacts persists one buffered snapshot in a single transaction.
func (s *Store) FlushGroupMessageFacts(ctx context.Context) error {
	// Only flushers share this lock. Buffer admission and unrelated writes keep
	// progressing while PostgreSQL is slow or one fact is locked.
	s.groupFactsFlushMu.Lock()
	defer s.groupFactsFlushMu.Unlock()
	s.groupFactsMu.Lock()
	snapshot := make([]groupMessageFact, 0, len(s.groupFacts))
	for _, fact := range s.groupFacts {
		snapshot = append(snapshot, fact)
	}
	s.groupFactsMu.Unlock()
	if len(snapshot) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin group message fact flush: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Stable conflict order also keeps concurrent stores from locking the same
	// deduplication keys in opposite orders.
	slices.SortFunc(snapshot, func(a, b groupMessageFact) int {
		if order := cmp.Compare(a.key.chatID, b.key.chatID); order != 0 {
			return order
		}
		return cmp.Compare(a.key.messageID, b.key.messageID)
	})
	for start := 0; start < len(snapshot); start += groupMessageFactBatchSize {
		batch := snapshot[start:min(start+groupMessageFactBatchSize, len(snapshot))]
		chats, messages := make([]int64, len(batch)), make([]int64, len(batch))
		dates, stamps := make([]string, len(batch)), make([]string, len(batch))
		for i, fact := range batch {
			chats[i], messages[i] = fact.key.chatID, fact.key.messageID
			dates[i], stamps[i] = fact.localDate, stamp(fact.createdAt)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_group_message_raw_events(chat_id,message_id,local_date,created_at)
			SELECT * FROM unnest($1::bigint[],$2::bigint[],$3::text[],$4::text[])
			ORDER BY 1,2 ON CONFLICT(chat_id,message_id) DO NOTHING`, chats, messages, dates, stamps); err != nil {
			return fmt.Errorf("flush group message facts: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit group message fact flush: %w", err)
	}

	s.groupFactsMu.Lock()
	for _, fact := range snapshot {
		if current, exists := s.groupFacts[fact.key]; exists && current == fact {
			delete(s.groupFacts, fact.key)
		}
	}
	s.groupFactsMu.Unlock()
	return nil
}

func newGroupMessageFact(chatID, messageID int64, localDate string, now time.Time) (groupMessageFact, error) {
	localDate = strings.TrimSpace(localDate)
	if chatID == 0 || messageID <= 0 || localDate == "" {
		return groupMessageFact{}, ErrConflict
	}
	if _, err := time.Parse(time.DateOnly, localDate); err != nil {
		return groupMessageFact{}, ErrConflict
	}
	return groupMessageFact{key: groupMessageFactKey{chatID: chatID, messageID: messageID}, localDate: localDate, createdAt: now.UTC()}, nil
}

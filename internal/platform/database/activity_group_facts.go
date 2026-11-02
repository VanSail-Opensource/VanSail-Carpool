package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const groupMessageFactBufferCapacity = 4096

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

const insertGroupMessageFact = `INSERT INTO activity_group_message_raw_events(chat_id,message_id,local_date,created_at)
	VALUES($1,$2,$3,$4) ON CONFLICT(chat_id,message_id) DO NOTHING`

// RecordGroupMessageFact deduplicates every non-command message observed in
// the configured Telegram group without requiring a local user identity.
func (s *Store) RecordGroupMessageFact(ctx context.Context, chatID, messageID int64, localDate string, now time.Time) error {
	fact, err := newGroupMessageFact(chatID, messageID, localDate, now)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.db.ExecContext(ctx, insertGroupMessageFact, fact.key.chatID, fact.key.messageID, fact.localDate, stamp(fact.createdAt))
	return err
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
	s.groupFactsMu.Lock()
	snapshot := make([]groupMessageFact, 0, len(s.groupFacts))
	for _, fact := range s.groupFacts {
		snapshot = append(snapshot, fact)
	}
	s.groupFactsMu.Unlock()
	if len(snapshot) == 0 {
		return nil
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin group message fact flush: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, fact := range snapshot {
		if _, err := tx.ExecContext(ctx, insertGroupMessageFact, fact.key.chatID, fact.key.messageID, fact.localDate, stamp(fact.createdAt)); err != nil {
			return fmt.Errorf("flush group message fact: %w", err)
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

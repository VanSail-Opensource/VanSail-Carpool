package statistics

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Service) loadLastGood(ctx context.Context) error {
	s.mu.RLock()
	loaded := !s.snapshot.RemoteGeneratedAt.IsZero() || !s.snapshot.DatabaseGeneratedAt.IsZero()
	s.mu.RUnlock()
	if loaded {
		return nil
	}
	remoteBytes, remoteAt, remoteErr := s.repository.LoadStatisticsPartition(ctx, remotePartition)
	databaseBytes, databaseAt, databaseErr := s.repository.LoadStatisticsPartition(ctx, databasePartition)
	remoteMissing := errors.Is(remoteErr, ErrPartitionNotFound)
	databaseMissing := errors.Is(databaseErr, ErrPartitionNotFound)
	var remote model.RemoteStatistics
	var database model.DatabaseStatistics
	if remoteErr == nil {
		remote, remoteErr = decodePartition[model.RemoteStatistics](remoteBytes)
	}
	if databaseErr == nil {
		database, databaseErr = decodePartition[model.DatabaseStatistics](databaseBytes)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Another first-load or refresh may have published while storage was read.
	// Its newer values also supersede errors from this delayed cache read.
	if !s.snapshot.RemoteGeneratedAt.IsZero() || !s.snapshot.DatabaseGeneratedAt.IsZero() {
		return nil
	}
	if remoteErr == nil {
		s.snapshot.Remote, s.snapshot.RemoteGeneratedAt = remote, remoteAt
	}
	if databaseErr == nil {
		s.snapshot.Database, s.snapshot.DatabaseGeneratedAt = database, databaseAt
	}
	s.snapshot.GeneratedAt = s.snapshot.DatabaseGeneratedAt
	if s.snapshot.RemoteGeneratedAt.After(s.snapshot.GeneratedAt) {
		s.snapshot.GeneratedAt = s.snapshot.RemoteGeneratedAt
	}
	s.snapshot.StalePartitions = stalePartitions(remoteErr, databaseErr)
	if !s.snapshot.RemoteGeneratedAt.IsZero() || !s.snapshot.DatabaseGeneratedAt.IsZero() {
		return nil
	}
	// Absence before the first refresh is normal; corrupt or unreadable caches
	// still report an error when neither partition can be served.
	if remoteMissing {
		remoteErr = nil
	}
	if databaseMissing {
		databaseErr = nil
	}
	return errors.Join(remoteErr, databaseErr)
}

func (s *Service) saveRemote(ctx context.Context, value model.RemoteStatistics, now time.Time) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.repository.SaveStatisticsPartition(ctx, remotePartition, payload, now); err != nil {
		return err
	}
	s.mu.Lock()
	s.snapshot.Remote, s.snapshot.RemoteGeneratedAt = value, now
	s.mu.Unlock()
	return nil
}

func (s *Service) saveDatabase(ctx context.Context, value model.DatabaseStatistics, now time.Time) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.repository.SaveStatisticsPartition(ctx, databasePartition, payload, now); err != nil {
		return err
	}
	s.mu.Lock()
	s.snapshot.Database, s.snapshot.DatabaseGeneratedAt = value, now
	s.mu.Unlock()
	return nil
}

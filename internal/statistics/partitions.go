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
	if remoteErr != nil && !errors.Is(remoteErr, ErrPartitionNotFound) {
		return remoteErr
	}
	if databaseErr != nil && !errors.Is(databaseErr, ErrPartitionNotFound) {
		return databaseErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if remoteErr == nil {
		remote, err := decodePartition[model.RemoteStatistics](remoteBytes)
		if err != nil {
			return err
		}
		s.snapshot.Remote, s.snapshot.RemoteGeneratedAt = remote, remoteAt
	}
	if databaseErr == nil {
		database, err := decodePartition[model.DatabaseStatistics](databaseBytes)
		if err != nil {
			return err
		}
		s.snapshot.Database, s.snapshot.DatabaseGeneratedAt = database, databaseAt
	}
	if remoteAt.After(databaseAt) {
		s.snapshot.GeneratedAt = remoteAt
	} else {
		s.snapshot.GeneratedAt = databaseAt
	}
	return nil
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

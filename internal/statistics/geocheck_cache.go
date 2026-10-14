package statistics

import (
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const (
	geocheckTTL           = 6 * time.Hour
	geocheckRetryInterval = 30 * time.Minute
	geocheckPollInterval  = 5 * time.Second
	geocheckTimeout       = 75 * time.Second
	geocheckWorkers       = 4
)

type geocheckEntry struct {
	image      model.StatisticsNodeGeocheck
	attempted  time.Time
	inProgress bool
}

type geocheckCache struct {
	mu      sync.RWMutex
	entries map[string]geocheckEntry
}

// RefreshGeochecks starts bounded background jobs for nodes whose image is due.
func (s *Service) RefreshGeochecks(ctx context.Context, now time.Time) error {
	now = normalizedNow(now)
	nodes, err := s.provider.Nodes(ctx)
	if err != nil {
		return err
	}
	due := s.geocheck.markDue(nodes, now)
	if len(due) > 0 {
		go s.runGeochecks(ctx, due)
	}
	return nil
}

// NodeGeocheck returns the current in-memory image for one node.
func (s *Service) NodeGeocheck(nodeUUID string) (model.StatisticsNodeGeocheck, bool) {
	return s.geocheck.get(nodeUUID)
}

func (c *geocheckCache) markDue(nodes []Node, now time.Time) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]geocheckEntry, len(nodes))
	}
	known := make(map[string]struct{}, len(nodes))
	due := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeUUID := strings.TrimSpace(node.UUID)
		if nodeUUID == "" {
			continue
		}
		known[nodeUUID] = struct{}{}
		entry := c.entries[nodeUUID]
		if entry.inProgress || (!entry.image.CheckedAt.IsZero() && now.Sub(entry.image.CheckedAt) < geocheckTTL) || (!entry.attempted.IsZero() && now.Sub(entry.attempted) < geocheckRetryInterval) {
			continue
		}
		entry.attempted = now
		entry.inProgress = true
		c.entries[nodeUUID] = entry
		due = append(due, nodeUUID)
	}
	for nodeUUID := range c.entries {
		if _, exists := known[nodeUUID]; !exists {
			delete(c.entries, nodeUUID)
		}
	}
	return due
}

func (c *geocheckCache) get(nodeUUID string) (model.StatisticsNodeGeocheck, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[strings.TrimSpace(nodeUUID)]
	if !ok || entry.image.CheckedAt.IsZero() {
		return model.StatisticsNodeGeocheck{}, false
	}
	return entry.image, true
}

func (c *geocheckCache) finish(nodeUUID string, image model.StatisticsNodeGeocheck, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, exists := c.entries[nodeUUID]
	if !exists {
		return
	}
	entry.inProgress = false
	if ok {
		entry.image = image
	}
	c.entries[nodeUUID] = entry
}

func (s *Service) runGeochecks(ctx context.Context, nodeUUIDs []string) {
	jobs := make(chan string)
	var workers sync.WaitGroup
	count := min(geocheckWorkers, len(nodeUUIDs))
	for range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for nodeUUID := range jobs {
				s.runGeocheck(ctx, nodeUUID)
			}
		}()
	}
	for index, nodeUUID := range nodeUUIDs {
		select {
		case jobs <- nodeUUID:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			for _, pending := range nodeUUIDs[index:] {
				s.geocheck.finish(pending, model.StatisticsNodeGeocheck{}, false)
			}
			return
		}
	}
	close(jobs)
	workers.Wait()
}

func (s *Service) runGeocheck(ctx context.Context, nodeUUID string) {
	ctx, cancel := context.WithTimeout(ctx, geocheckTimeout)
	defer cancel()
	jobID, err := s.provider.RequestNodeGeocheck(ctx, nodeUUID)
	if err != nil {
		s.geocheck.finish(nodeUUID, model.StatisticsNodeGeocheck{}, false)
		return
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			s.geocheck.finish(nodeUUID, model.StatisticsNodeGeocheck{}, false)
			return
		case <-timer.C:
			result, pollErr := s.provider.NodeGeocheckResult(ctx, jobID)
			if pollErr != nil || (result.Completed && result.Failed) {
				s.geocheck.finish(nodeUUID, model.StatisticsNodeGeocheck{}, false)
				return
			}
			if result.Completed {
				image, valid := validGeocheckImage(nodeUUID, result, time.Now().UTC())
				s.geocheck.finish(nodeUUID, image, valid)
				return
			}
			timer.Reset(geocheckPollInterval)
		}
	}
}

func validGeocheckImage(nodeUUID string, result GeocheckResult, checkedAt time.Time) (model.StatisticsNodeGeocheck, bool) {
	if !result.Success || result.Image == nil || !strings.EqualFold(nodeUUID, result.NodeUUID) {
		return model.StatisticsNodeGeocheck{}, false
	}
	image := result.Image
	if image.Format != "svg" || image.MediaType != "image/svg+xml" || image.Encoding != "base64" || strings.TrimSpace(image.Data) == "" {
		return model.StatisticsNodeGeocheck{}, false
	}
	if _, err := base64.StdEncoding.DecodeString(image.Data); err != nil {
		return model.StatisticsNodeGeocheck{}, false
	}
	return model.StatisticsNodeGeocheck{NodeUUID: nodeUUID, CheckedAt: checkedAt, Image: model.StatisticsNodeGeocheckImage{
		Format: image.Format, MediaType: image.MediaType, Encoding: image.Encoding, Data: image.Data,
	}}, true
}

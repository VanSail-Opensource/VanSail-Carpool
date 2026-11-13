package billing

import (
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PaymentChannelCache keeps provider-discovered channels out of durable storage.
type PaymentChannelCache struct {
	mu       sync.RWMutex
	profiles map[string]paymentChannelSnapshot
}

type paymentChannelSnapshot struct {
	updatedAt time.Time
	channels  []model.PaymentChannel
}

// NewPaymentChannelCache creates an empty process-local channel cache.
func NewPaymentChannelCache() *PaymentChannelCache {
	return &PaymentChannelCache{profiles: make(map[string]paymentChannelSnapshot)}
}

// Replace publishes only the discovered revision, without overwriting a newer
// completed probe. Reads additionally require an exact durable revision match.
func (c *PaymentChannelCache) Replace(profileID string, updatedAt time.Time, channels []model.PaymentChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if updatedAt.IsZero() || c.profiles[profileID].updatedAt.After(updatedAt) {
		return
	}
	c.profiles[profileID] = paymentChannelSnapshot{updatedAt: updatedAt, channels: append([]model.PaymentChannel(nil), channels...)}
}

// Remove forgets only the specified revision. A late failure or deletion cannot
// erase a discovery published for a replacement configuration of the same ID.
func (c *PaymentChannelCache) Remove(profileID string, updatedAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.profiles[profileID].updatedAt.Equal(updatedAt) {
		delete(c.profiles, profileID)
	}
}

// Revision captures the entry to invalidate when deleting a profile. It never
// authorizes a rail; callers still need the current durable profile revision.
func (c *PaymentChannelCache) Revision(profileID string) time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.profiles[profileID].updatedAt
}

// PaymentProfileChannels returns an isolated snapshot for one profile.
func (c *PaymentChannelCache) PaymentProfileChannels(profileID string, updatedAt time.Time) []model.PaymentChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snapshot := c.profiles[profileID]
	if updatedAt.IsZero() || !snapshot.updatedAt.Equal(updatedAt) {
		return nil
	}
	return append([]model.PaymentChannel(nil), snapshot.channels...)
}

// PaymentProfileRails returns the discovered direct trade types for settings.
func (c *PaymentChannelCache) PaymentProfileRails(profileID string, updatedAt time.Time) []string {
	channels := c.PaymentProfileChannels(profileID, updatedAt)
	rails := make([]string, 0, len(channels))
	for _, channel := range channels {
		rails = append(rails, channel.Rail)
	}
	return rails
}

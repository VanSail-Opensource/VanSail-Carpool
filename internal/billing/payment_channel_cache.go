package billing

import (
	"sync"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PaymentChannelCache keeps provider-discovered channels out of durable storage.
type PaymentChannelCache struct {
	mu       sync.RWMutex
	profiles map[string][]model.PaymentChannel
}

// NewPaymentChannelCache creates an empty process-local channel cache.
func NewPaymentChannelCache() *PaymentChannelCache {
	return &PaymentChannelCache{profiles: make(map[string][]model.PaymentChannel)}
}

// Replace atomically publishes one profile's latest discovery result.
func (c *PaymentChannelCache) Replace(profileID string, channels []model.PaymentChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.profiles[profileID] = append([]model.PaymentChannel(nil), channels...)
}

// Remove forgets a deleted or unavailable profile.
func (c *PaymentChannelCache) Remove(profileID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.profiles, profileID)
}

// PaymentProfileChannels returns an isolated snapshot for one profile.
func (c *PaymentChannelCache) PaymentProfileChannels(profileID string) []model.PaymentChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]model.PaymentChannel(nil), c.profiles[profileID]...)
}

// PaymentProfileRails returns the discovered direct trade types for settings.
func (c *PaymentChannelCache) PaymentProfileRails(profileID string) []string {
	channels := c.PaymentProfileChannels(profileID)
	rails := make([]string, 0, len(channels))
	for _, channel := range channels {
		rails = append(rails, channel.Rail)
	}
	return rails
}

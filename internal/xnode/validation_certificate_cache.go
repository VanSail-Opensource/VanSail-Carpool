package xnode

import (
	"sync"
	"time"
)

var validationMaterial struct {
	sync.Mutex
	certificate, key []string
	until            time.Time
}

// Stable synthetic material lets account/rule validation reuse unchanged
// listener sections. It never leaves the listener-free analysis copy.
func cachedValidationCertificate() ([]string, []string, error) {
	validationMaterial.Lock()
	defer validationMaterial.Unlock()
	if len(validationMaterial.certificate) == 0 || !time.Now().Before(validationMaterial.until) {
		certificate, key, err := validationCertificate()
		if err != nil {
			return nil, nil, err
		}
		validationMaterial.certificate, validationMaterial.key = certificate, key
		validationMaterial.until = time.Now().Add(30 * time.Minute)
	}
	return append([]string(nil), validationMaterial.certificate...), append([]string(nil), validationMaterial.key...), nil
}

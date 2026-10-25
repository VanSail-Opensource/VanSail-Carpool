// Package xraydiagnostics applies the panel's credential-safe Xray log policy.
package xraydiagnostics

import (
	"sync"

	xraylog "github.com/xtls/xray-core/common/log"
)

var once sync.Once

type discard struct{}

func (discard) Handle(xraylog.Message) {}

// Quiet disables raw core diagnostics before configuration building. Callers
// still receive validation/probe errors through their existing typed contracts.
func Quiet() {
	once.Do(func() { xraylog.RegisterHandler(discard{}) })
}

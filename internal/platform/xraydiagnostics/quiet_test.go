package xraydiagnostics

import (
	"sync"
	"testing"

	xraylog "github.com/xtls/xray-core/common/log"
)

type credentialDiagnostic struct{}

func (credentialDiagnostic) String() string { panic("raw diagnostic must not be formatted") }

func TestQuietDoesNotFormatCredentialDiagnostics(t *testing.T) {
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			Quiet()
			xraylog.Record(credentialDiagnostic{})
		})
	}
	group.Wait()
}

package xnode_test

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestSlowOutboundDNSDoesNotHoldNativeMutationLock(t *testing.T) {
	// This test replaces the process resolver and therefore stays nonparallel.
	f := controlFixture(t, true)
	started := make(chan struct{})
	var once sync.Once
	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	defer func() { net.DefaultResolver = previous }()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	done := make(chan error, 1)
	go func() {
		_, err := f.service.SaveOwnedOutbounds(ctx, f.user.ID, f.node.ID, 0,
			json.RawMessage(`[{"tag":"slow","protocol":"socks","settings":{"servers":[{"address":"slow.example.invalid","port":1080}]}}]`))
		done <- err
	}()
	defer func() { cancel(); <-done }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("outbound validation never reached queued DNS")
	}
	writeDone := make(chan error, 1)
	go func() {
		_, err := f.service.SaveSnippet(ctx, xnode.Snippet{Name: "independent", Kind: "routing", Document: json.RawMessage(`[]`)})
		writeDone <- err
	}()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-timer.C:
		cancel()
		<-writeDone
		t.Fatal("queued DNS held the native mutation lock")
	}
	rows, err := f.store.XNodeCustomizations(context.Background(), f.node.ID)
	if err != nil || len(rows) != 0 {
		t.Fatal("unfinished DNS published routing state", rows, err)
	}
}

package xnode_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestSlowGlobalAssetDoesNotHoldNativeMutationLock(t *testing.T) {
	// This test replaces the process resolver and therefore stays nonparallel.
	f := controlFixture(t, true)
	before, err := f.service.Global(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	candidate := before
	candidate.GeoSiteURL = "https://slow.example.invalid/geosite.dat"
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
		done <- f.service.SaveGlobal(ctx, f.user.ID, candidate)
	}()
	defer func() { cancel(); <-done }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("global validation never reached queued DNS")
	}
	writeDone := make(chan error, 1)
	go func() {
		_, err := f.service.SaveSnippet(ctx, xnode.Snippet{Name: "independent", Kind: "routing", Document: []byte(`[]`)})
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
	got, err := f.service.Global(context.Background())
	if err != nil || got.GeoSiteURL != before.GeoSiteURL {
		t.Fatal("unfinished asset validation published global state", got.GeoSiteURL, err)
	}
}

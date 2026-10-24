package xnode

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type heldDeployment struct {
	sent    chan string
	waiting chan struct{}
	release chan struct{}
}

func (m *heldDeployment) Sync(_ context.Context, node string) error {
	m.sent <- node
	return nil
}

func (m *heldDeployment) WaitApplied(ctx context.Context, node string) error {
	if node != "first" {
		return nil
	}
	close(m.waiting)
	select {
	case <-m.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestDeploymentPublishesEveryNodeBeforeHeldAcknowledgement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	m := &heldDeployment{sent: make(chan string, 2), waiting: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		done <- syncDeploymentNodes(ctx, m, []string{"first", "second"})
	}()
	defer func() { cancel(); <-finished }()
	select {
	case <-m.waiting:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-m.sent:
			if got != want {
				t.Fatalf("published %q, want %q", got, want)
			}
		default:
			t.Fatalf("%s did not receive the phase before waiting for first", want)
		}
	}
	select {
	case err := <-done:
		t.Fatalf("deployment completed without the first acknowledgement: %v", err)
	default:
	}
	close(m.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestDeploymentCancellationIdentifiesPendingNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := &heldDeployment{sent: make(chan string, 2), waiting: make(chan struct{}), release: make(chan struct{})}
	err := syncDeploymentNodes(ctx, m, []string{"first", "second"})
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "apply node first") {
		t.Fatalf("pending barrier lost cancellation or node context: %v", err)
	}
}

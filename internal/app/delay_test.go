package app

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestDelayedMessageCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := delayedMessage(ctx, time.Hour, func(time.Time) tea.Msg {
		t.Error("canceled delay invoked its callback")
		return nil
	})
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	cancel()
	select {
	case msg := <-done:
		if msg != nil {
			t.Fatalf("canceled delay returned %T", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled delay kept waiting")
	}
	if got := delayedMessage(context.Background(), 0, func(time.Time) tea.Msg { return "ready" })(); got != "ready" {
		t.Fatalf("active delay returned %v", got)
	}
}

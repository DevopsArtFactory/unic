package app

import (
	"context"
	"sync"
	"time"
)

// commandDeadline bounds every background AWS command so a stalled request
// cannot pin the TUI in its loading state indefinitely.
const commandDeadline = 90 * time.Second

// commandLifecycle owns the context shared by background commands. It is held
// by pointer on the Model so every value copy sees the same generation:
// commands resolve their context at run time via Current, startLoading renews
// the generation (cancelling superseded work), and navigation that abandons a
// screen cancels outright.
type commandLifecycle struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func newCommandLifecycle() *commandLifecycle {
	return &commandLifecycle{}
}

// Current returns the active command context, reviving a fresh one when the
// previous generation was cancelled or expired. Commands that run without a
// preceding renewal (refreshes, polls) therefore never start dead.
func (l *commandLifecycle) Current() context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ctx == nil || l.ctx.Err() != nil {
		l.ctx, l.cancel = context.WithTimeout(context.Background(), commandDeadline)
	}
	return l.ctx
}

// Renew cancels the previous generation and starts a new deadline-bound one.
// Called when a new load supersedes whatever was still in flight.
func (l *commandLifecycle) Renew() context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		l.cancel()
	}
	l.ctx, l.cancel = context.WithTimeout(context.Background(), commandDeadline)
	return l.ctx
}

// CancelAll cancels the active generation without starting a new one, so
// in-flight work stops when the user abandons a screen or quits.
func (l *commandLifecycle) CancelAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		l.cancel()
	}
}

// commandContext is the entry point command closures use at run time.
func (m Model) commandContext() context.Context {
	if m.commands == nil {
		return context.Background()
	}
	return m.commands.Current()
}

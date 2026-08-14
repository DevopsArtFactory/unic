package app

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	gen    int
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

// Renew cancels the previous generation and starts a new deadline-bound one,
// returning the new generation id. Called when a new load supersedes whatever
// was still in flight; scheduled commands are bound to the returned id so a
// queued command can never silently migrate into a later generation.
func (l *commandLifecycle) Renew() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		l.cancel()
	}
	l.gen++
	l.ctx, l.cancel = context.WithTimeout(context.Background(), commandDeadline)
	return l.gen
}

// CancelAll cancels the active generation and advances the generation counter
// without starting new work, so abandonment stays distinguishable from the
// fresh context a later poll or refresh intentionally revives.
func (l *commandLifecycle) CancelAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		l.cancel()
	}
	l.gen++
}

// CurrentGen returns the active generation id.
func (l *commandLifecycle) CurrentGen() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.gen
}

// BindCmd pins a command to the given generation: the command body runs only
// while its generation is still current, and its resulting message is wrapped
// so delivery can be dropped when the generation has moved on since.
func (l *commandLifecycle) BindCmd(gen int, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		if l.CurrentGen() != gen {
			// Superseded or abandoned before it ever ran: skip the AWS call.
			return nil
		}
		return genBoundMsg{gen: gen, msg: cmd()}
	}
}

// commandContext is the entry point command closures use at run time.
func (m Model) commandContext() context.Context {
	if m.commands == nil {
		return context.Background()
	}
	return m.commands.Current()
}

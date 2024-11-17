// Package parallel provides a simpler version of errgroup where:
// - all goroutines are started immediately
// - no concurrency limit
// - runners receive a context
// - the context is canceled without passing 'cause'.
//
// The last differece is the main purpose of this package. When using errgroup,
// the context is canceled with a context.WithCancelCause, which cause libraries like
// net/http to return the causal error in other workers, rather than
// context.Canceled, which can create confusing log output and error handling.
package parallel

import (
	"context"
	"sync"
)

// Group is a collection of goroutines working on subtasks that are part of the
// same overall task with the same parent context.
type Group struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lock     sync.Mutex
	firstErr error
}

func NewGroup(ctx context.Context) *Group {
	innerCtx, cancel := context.WithCancel(ctx)
	return &Group{ctx: innerCtx, cancel: cancel}
}

// Go calls the given function in a new goroutine immediately.
//
// The derived Context is canceled the first time a function passed to Go
// returns a non-nil error or the first time Wait returns, whichever occurs
// first.
//
// The context is canceled with a simpel context.WithCancel, not a
// context.WithCancelCause.
//
// The error will be returned by Wait.
func (g *Group) Go(f func(ctx context.Context) error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := f(g.ctx); err != nil {
			g.handleErr(err)
		}
	}()
}

func (g *Group) handleErr(err error) {
	g.lock.Lock()
	if g.firstErr == nil {
		g.firstErr = err
	}
	g.lock.Unlock()
	g.cancel()
}

func (g *Group) Wait() error {
	g.wg.Wait()
	if g.firstErr != nil {
		return g.firstErr
	}

	// if no error was returned, the input context was canceled (or timed out),
	// so we return the context error.
	return g.ctx.Err()
}

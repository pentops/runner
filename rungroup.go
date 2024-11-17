package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/pentops/log.go/log"
	"golang.org/x/sync/errgroup"
)

const (
	LogLineGroupStarted                         = "Run group triggered"
	LogLineGroupExited                          = "Run group exited"
	LogLineGroupExitedWithError                 = "Run group exited with error"
	LogLineRunnerStarted                        = "Runner started"
	LogLineRunnerExited                         = "Runner exited"
	LogLineRunnerExitedWithError                = "Runner exited with error"
	LogLineRunnerExitedWithContextCanceledError = "Runner exited with context canceled"
)

type Group struct {
	name            string
	logger          log.Logger
	cancelOnSignals []os.Signal

	running   bool
	isWaiting bool

	errGroup     *errgroup.Group
	runners      []*runner
	controlMutex sync.Mutex
	runContext   context.Context

	holdOpen chan struct{}
}

type runner struct {
	name    string
	f       func(ctx context.Context) error
	stopped chan struct{}
}

type option func(*Group)

func WithLogger(logger log.Logger) option {
	return func(g *Group) {
		g.logger = logger
	}
}

func WithName(name string) option {
	return func(g *Group) {
		g.name = name
	}
}

// WithCancelOnSignals will cancel the context when any of the given signals
// are received. If no signals are given, the default signals are used:
// os.Interrupt, os.Kill, syscall.SIGTERM
func WithCancelOnSignals(signals ...os.Signal) option {
	if len(signals) == 0 {
		signals = []os.Signal{
			os.Interrupt,
			os.Kill,
			os.Signal(syscall.SIGTERM),
		}
	}
	return func(g *Group) {
		g.cancelOnSignals = signals
	}
}

func NewGroup(options ...option) *Group {
	gg := &Group{
		logger: log.DefaultLogger,
	}
	for _, option := range options {
		option(gg)
	}
	return gg
}

// Add registers a function to run when the group is triggered with Run or Start.
// If the group is already running, the function will be started immediately and
// added to the pool.
func (gg *Group) Add(name string, f func(ctx context.Context) error) {
	gg.controlMutex.Lock()
	defer gg.controlMutex.Unlock()

	if gg.isWaiting {
		panic("group is already waiting")
	}

	runner := &runner{name: name, f: f}
	gg.runners = append(gg.runners, runner)
	if gg.running {
		gg.startRunner(gg.runContext, runner)
	}

}

func (gg *Group) startRunner(ctx context.Context, rr *runner) {
	rr.stopped = make(chan struct{})
	ctx = log.WithField(ctx, "runner", rr.name)
	gg.errGroup.Go(func() error {
		gg.logger.Info(ctx, LogLineRunnerStarted)
		err := rr.f(ctx)
		close(rr.stopped)
		if err == nil {
			gg.logger.Info(ctx, LogLineRunnerExited)
			return nil
		}
		if errors.Is(err, context.Canceled) {
			gg.logger.Debug(ctx, LogLineRunnerExitedWithContextCanceledError)
			return nil
		}
		gg.logger.Error(log.WithError(ctx, err), LogLineRunnerExitedWithError)
		return err
	})
}

// Start starts the runners in the group in the background.
// Errors are not returned until Wait is called
// Runners are tied to the passed in context
func (gg *Group) Start(ctx context.Context) error {
	if gg.name != "" {
		ctx = log.WithField(ctx, "runGroup", gg.name)
	}

	if len(gg.cancelOnSignals) > 0 {
		ctx, _ = signal.NotifyContext(ctx, gg.cancelOnSignals...)
	}

	// Hold the lock until we have
	// - Created all pending runners
	// - Marked as running
	gg.controlMutex.Lock()
	defer gg.controlMutex.Unlock()
	if gg.running {
		return fmt.Errorf("group already triggered")
	}
	gg.running = true
	gg.errGroup, ctx = errgroup.WithContext(ctx)
	gg.runContext = ctx

	// Forces at least one worker to keep the group open, until 'Wait' is
	// called, allowing runners to be added after the group has started.
	gg.holdOpen = make(chan struct{})
	gg.errGroup.Go(func() error {
		<-gg.holdOpen
		return nil
	})

	for _, rr := range gg.runners {
		rr := rr
		gg.startRunner(ctx, rr)
	}

	gg.logger.Info(ctx, LogLineGroupStarted)
	return nil
}

// Run runs the runners in the group until all have exited.
// If any function returns an error, the context passed to each is canceled.
// Once a group is triggered with Run, no more functions can be added
func (gg *Group) Run(ctx context.Context) error {
	err := gg.Start(ctx)
	if err != nil {
		return err
	}
	return gg.Wait()
}

// Wait waits for all runners to exit. If any runner returns an error, the first
// error is returned.
// Once Wait is called, no more runners can be added to the group
func (gg *Group) Wait() error {
	gg.controlMutex.Lock()
	defer gg.controlMutex.Unlock()
	if gg.isWaiting {
		return fmt.Errorf("group is already waiting")
	}

	gg.isWaiting = true
	close(gg.holdOpen)

	go func() {
		<-gg.runContext.Done()
		waiting := sync.Map{}

		for _, rr := range gg.runners {
			waiting.Store(rr.name, struct{}{})
			<-rr.stopped
			waiting.Delete(rr)
			waiting.Range(func(key, value interface{}) bool {
				rr := key.(string)
				gg.logger.Debug(gg.runContext, "Waiting for runner "+rr)
				return true
			})

		}
		gg.logger.Info(gg.runContext, "All runners exited")
	}()

	firstError := gg.errGroup.Wait()
	if firstError != nil {
		gg.logger.Error(gg.runContext, LogLineGroupExitedWithError)
	} else {
		gg.logger.Info(gg.runContext, LogLineGroupExited)
	}

	return firstError
}

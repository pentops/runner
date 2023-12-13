package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pentops/log.go/log"
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
	Name         string
	runners      []*runner
	controlMutex sync.Mutex
	triggered    bool
	logger       log.Logger
}

type runner struct {
	name string
	f    func(ctx context.Context) error
	err  error
}

type option func(*Group)

func WithLogger(logger log.Logger) option {
	return func(g *Group) {
		g.logger = logger
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

func (gg *Group) Add(name string, f func(ctx context.Context) error) error {
	if gg.triggered {
		return fmt.Errorf("cannot add runners after the group is triggered")
	}
	gg.controlMutex.Lock()
	defer gg.controlMutex.Unlock()
	if gg.triggered {
		return fmt.Errorf("cannot add runners after the group is triggered")
	}
	gg.runners = append(gg.runners, &runner{name: name, f: f})
	return nil
}

// Run runs the runners in the group until all have exited.
// If any function returns an error, the context passed to each is canceled.
// Once a group is triggered with Run, no more functions can be added
func (gg *Group) Run(ctx context.Context) error {
	gg.controlMutex.Lock()
	defer gg.controlMutex.Unlock()
	if gg.triggered {
		return fmt.Errorf("group already triggered")
	}
	if gg.Name != "" {
		ctx = log.WithField(ctx, "runGroup", gg.Name)
	}
	gg.triggered = true
	gg.logger.Info(ctx, LogLineGroupStarted)

	ctx, cancel := context.WithCancel(ctx)

	var firstError error
	errorMutex := sync.Mutex{}

	wg := sync.WaitGroup{}
	for _, rr := range gg.runners {
		wg.Add(1)
		ctx := log.WithField(ctx, "runner", rr.name)
		go func(ctx context.Context, rr *runner) {
			defer wg.Done()
			gg.logger.Info(ctx, LogLineRunnerStarted)
			err := rr.f(ctx)
			rr.err = err
			if err != nil {
				errorMutex.Lock()
				if firstError == nil {
					firstError = err
					cancel()
				}
				errorMutex.Unlock()
				if errors.Is(err, context.Canceled) {
					gg.logger.Info(ctx, LogLineRunnerExitedWithContextCanceledError)
				} else {
					gg.logger.Error(log.WithError(ctx, err), LogLineRunnerExitedWithError)
				}
			} else {
				gg.logger.Info(ctx, LogLineRunnerExited)
			}
		}(ctx, rr)
	}

	wg.Wait()

	if firstError != nil {
		gg.logger.Error(ctx, LogLineGroupExitedWithError)
	} else {
		gg.logger.Info(ctx, LogLineGroupExited)
	}

	// In case a runner ran a sub-thread (which is not recommended), we need to
	// make sure that the context is canceled. Also because the linter
	// complained.
	cancel()

	return firstError
}

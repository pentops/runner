package runner

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/pentops/log.go/log"
)

type logEntry struct {
	level   string
	message string
	fields  map[string]any
}

type logEntries []logEntry

func (ll *logEntries) Logger(t testing.TB) log.LogFunc {
	return log.LogFunc(func(level, message string, attrs []slog.Attr) {
		t.Log(level, message, attrs)
		fields := make(map[string]any)
		for _, attr := range attrs {
			fields[attr.Key] = attr.Value.Any()
		}
		*ll = append(*ll, logEntry{level, message, fields})
	})
}

func (ll logEntries) Assert(t testing.TB, want map[string][]logEntry) {
	t.Helper()

	gotByRunner := make(map[string][]logEntry)
	for _, entry := range ll {
		runner, ok := entry.fields["runner"].(string)
		if !ok {
			runner = "root"
		}
		gotByRunner[runner] = append(gotByRunner[runner], entry)
	}

	for runner, wantEntries := range want {
		gotEntries, ok := gotByRunner[runner]
		if !ok {
			t.Errorf("Runner %v not found", runner)
			continue
		}
		if len(gotEntries) != len(wantEntries) {
			t.Errorf("Expected %v entries for runner %v, got %v", len(wantEntries), runner, len(gotEntries))
			continue
		}
		for idx, wantEntry := range wantEntries {
			gotEntry := gotEntries[idx]
			if gotEntry.level != wantEntry.level {
				t.Errorf("Expected level %v for runner %v, got %v", wantEntry.level, runner, gotEntry.level)
			}
			if gotEntry.message != wantEntry.message {
				t.Errorf("Expected message %v for runner %v, got %v", wantEntry.message, runner, gotEntry.message)
			}
		}
	}

}

func TestHappyPath(t *testing.T) {
	entries := &logEntries{}
	logger := log.NewCallbackLogger(entries.Logger(t))

	// Create a new group
	g := NewGroup(WithLogger(logger))

	// Add a runner to the group
	g.Add("t1", func(ctx context.Context) error {
		// Do something
		return nil
	})

	g.Add("t2", func(ctx context.Context) error {
		// Do something
		return nil
	})

	// Run the group
	err := g.Run(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	entries.Assert(t, map[string][]logEntry{
		"t1": {
			{level: "INFO", message: LogLineRunnerStarted},
			{level: "INFO", message: LogLineRunnerExited},
		},
		"t2": {
			{level: "INFO", message: LogLineRunnerStarted},
			{level: "INFO", message: LogLineRunnerExited},
		},
		"root": {
			{level: "INFO", message: LogLineGroupStarted},
			{level: "INFO", message: LogLineGroupExited},
		},
	})
}

func TestContextCancelOnErrors(t *testing.T) {

	entries := &logEntries{}
	logger := log.NewCallbackLogger(entries.Logger(t))
	logger.SetLevel(slog.LevelDebug)

	// Create a new group
	g := NewGroup(WithLogger(logger))

	// Add a runner to the group
	g.Add("t1", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	exitError := errors.New("Exit")
	g.Add("t2", func(ctx context.Context) error {
		// Do something
		return exitError
	})

	// Run the group
	err := g.Run(context.Background())
	if !errors.Is(err, exitError) {
		t.Errorf("Expected exit error, got %v", err)
	}

	entries.Assert(t, map[string][]logEntry{
		"t1": {
			{level: "INFO", message: LogLineRunnerStarted},
			{level: "DEBUG", message: LogLineRunnerExitedWithContextCanceledError},
		},
		"t2": {
			{level: "INFO", message: LogLineRunnerStarted},
			{level: "ERROR", message: LogLineRunnerExitedWithError},
		},
		"root": {
			{level: "INFO", message: LogLineGroupStarted},
			{level: "ERROR", message: LogLineGroupExitedWithError},
		},
	})

}

func TestMultipleErrors(t *testing.T) {

	entries := &logEntries{}
	logger := log.NewCallbackLogger(entries.Logger(t))

	// Create a new group
	g := NewGroup(WithLogger(logger))

	// Add a runner to the group
	g.Add("t1", func(ctx context.Context) error {
		return errors.New("Error 1")
	})

	g.Add("t2", func(ctx context.Context) error {
		return errors.New("Error 2")
	})

	g.Add("t3", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	// Run the group
	err := g.Run(context.Background())
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	entries.Assert(t, map[string][]logEntry{
		"t1": {
			{level: "INFO", message: LogLineRunnerStarted},
			{level: "ERROR", message: LogLineRunnerExitedWithError},
		},
		"t2": {
			{level: "INFO", message: LogLineRunnerStarted},
			{level: "ERROR", message: LogLineRunnerExitedWithError},
		},
		"root": {
			{level: "INFO", message: LogLineGroupStarted},
			{level: "ERROR", message: LogLineGroupExitedWithError},
		},
	})

}

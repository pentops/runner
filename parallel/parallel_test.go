package parallel

import (
	"context"
	"fmt"
	"testing"
)

func TestParallel(t *testing.T) {
	ctx := context.Background()
	group := NewGroup(ctx)

	var run1, run2 bool

	group.Go(func(ctx context.Context) error {
		run1 = true
		return nil
	})
	group.Go(func(ctx context.Context) error {
		run2 = true
		return nil
	})

	err := group.Wait()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !run1 {
		t.Errorf("callback 1 should have run")
	}
	if !run2 {
		t.Errorf("callback 2 should have run")
	}
}

func TestErr(t *testing.T) {
	ctx := context.Background()
	group := NewGroup(ctx)

	var run1, run2 bool

	var ctl1 = make(chan struct{})

	testErr := fmt.Errorf("test err")

	group.Go(func(ctx context.Context) error {
		run1 = true
		<-ctl1
		return testErr
	})
	group.Go(func(ctx context.Context) error {
		run2 = true
		<-ctx.Done()
		return nil
	})

	close(ctl1)

	err := group.Wait()
	if err == nil {
		t.Errorf("expected error")
	}

	if err != testErr {
		t.Errorf("unexpected error: %v", err)
	}

	if !run1 {
		t.Errorf("callback 1 should have run")
	}
	if !run2 {
		t.Errorf("callback 2 should have run")
	}

}

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	group := NewGroup(ctx)

	var run1, run2 bool

	group.Go(func(ctx context.Context) error {
		run1 = true
		<-ctx.Done()
		return nil
	})
	group.Go(func(ctx context.Context) error {
		run2 = true
		<-ctx.Done()
		return nil
	})

	cancel()

	err := group.Wait()
	if err == nil {
		t.Errorf("expected error")
	}

	if err != context.Canceled {
		t.Errorf("unexpected error: %v", err)
	}

	if !run1 {
		t.Errorf("callback 1 should have run")
	}
	if !run2 {
		t.Errorf("callback 2 should have run")
	}

}

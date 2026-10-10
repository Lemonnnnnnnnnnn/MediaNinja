package tasks

import (
	"MediaNinja/core/application"
	"MediaNinja/core/config"
	"MediaNinja/core/events"
	"context"
	"testing"
	"time"
)

func request() application.Request {
	return application.Request{URL: "https://example.com", Options: *config.New()}
}
func awaitState(t *testing.T, m *Manager, id, state string) Task {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, _ := m.Get(id)
		if task.State == state {
			return task
		}
		time.Sleep(time.Millisecond)
	}
	task, _ := m.Get(id)
	t.Fatalf("wanted %s, got %s", state, task.State)
	return Task{}
}
func TestCancelRunningAndQueued(t *testing.T) {
	started := make(chan struct{}, 2)
	m := New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		started <- struct{}{}
		o(events.Event{Type: "progress", File: "a", Completed: 4})
		<-ctx.Done()
		return application.Result{}, ctx.Err()
	})
	defer m.Close()
	first, err := m.Submit(request())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	second, err := m.Submit(request())
	if err != nil {
		t.Fatal(err)
	}
	if !m.Cancel(second.ID) || !m.Cancel(first.ID) {
		t.Fatal("cancel failed")
	}
	awaitState(t, m, first.ID, "canceled")
	awaitState(t, m, second.ID, "canceled")
	snapshot, _ := m.Get(first.ID)
	snapshot.Progress["a"] = events.Event{Completed: 999}
	actual, _ := m.Get(first.ID)
	if actual.Progress["a"].Completed != 4 {
		t.Fatal("mutable progress leaked")
	}
	select {
	case <-started:
		t.Fatal("queued canceled task executed")
	default:
	}
}
func TestFailureAndValidation(t *testing.T) {
	m := New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		return application.Result{}, context.DeadlineExceeded
	})
	defer m.Close()
	r := request()
	r.Options.Concurrency = 0
	if _, err := m.Submit(r); err == nil {
		t.Fatal("invalid request accepted")
	}
	task, err := m.Submit(request())
	if err != nil {
		t.Fatal(err)
	}
	failed := awaitState(t, m, task.ID, "failed")
	if failed.Error == "" {
		t.Fatal("failure lost")
	}
}

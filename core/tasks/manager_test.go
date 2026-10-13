package tasks

import (
	"MediaNinja/core/application"
	"MediaNinja/core/crawler"
	"MediaNinja/core/config"
	"MediaNinja/core/events"
	"context"
	"errors"
	"os"
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
	}, "")
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
	}, "")
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

// partialRunner 模拟一个部分失败的任务：a 成功、b 失败，并发出统计事件。
func partialRunner(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
	o(events.Event{Type: "plan", Total: 2})
	o(events.Event{Type: "file_completed", File: "a"})
	o(events.Event{Type: "file_failed", File: "b", Error: "boom"})
	return application.Result{Files: []crawler.FileResult{{File: "a"}, {File: "b", Error: "boom"}}}, errors.New("one file failed")
}

func TestStatsAndPartialRetry(t *testing.T) {
	m := New(partialRunner, "")
	defer m.Close()
	task, err := m.Submit(request())
	if err != nil {
		t.Fatal(err)
	}
	failed := awaitState(t, m, task.ID, "failed")
	if failed.Stats.Total != 2 || failed.Stats.Completed != 1 || failed.Stats.Failed != 1 {
		t.Fatalf("stats lost: %+v", failed.Stats)
	}
	if _, err := m.Retry(task.ID, "nope"); err != ErrInvalidScope {
		t.Fatalf("invalid scope accepted: %v", err)
	}
	partial, err := m.Retry(task.ID, "failed")
	if err != nil {
		t.Fatal(err)
	}
	if partial.RetryOf != task.ID {
		t.Fatal("retry source lost")
	}
	if len(partial.Request.SkipFiles) != 1 || partial.Request.SkipFiles[0] != "a" {
		t.Fatalf("skip files wrong: %v", partial.Request.SkipFiles)
	}
	awaitState(t, m, partial.ID, "failed")
	full, err := m.Retry(task.ID, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Request.SkipFiles) != 0 {
		t.Fatalf("full retry must not skip files: %v", full.Request.SkipFiles)
	}
	awaitState(t, m, full.ID, "failed")
}

func TestRetryGuards(t *testing.T) {
	started := make(chan struct{})
	m := New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		started <- struct{}{}
		<-ctx.Done()
		return application.Result{}, ctx.Err()
	}, "")
	defer m.Close()
	task, err := m.Submit(request())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	if _, err := m.Retry(task.ID, "all"); err != ErrTaskActive {
		t.Fatalf("active task retried: %v", err)
	}
	if err := m.Remove(task.ID); err != ErrTaskActive {
		t.Fatalf("active task removed: %v", err)
	}
	m.Cancel(task.ID)
	awaitState(t, m, task.ID, "canceled")
	if _, err := m.Retry(task.ID, "failed"); err != ErrNothingFailed {
		t.Fatalf("empty failed set retried: %v", err)
	}
	if _, err := m.Retry("missing", "all"); err != ErrTaskNotFound {
		t.Fatalf("missing task retried: %v", err)
	}
}

func TestRemoveAndClearFinished(t *testing.T) {
	m := New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		return application.Result{}, nil
	}, "")
	defer m.Close()
	first, _ := m.Submit(request())
	second, _ := m.Submit(request())
	awaitState(t, m, first.ID, "completed")
	awaitState(t, m, second.ID, "completed")
	if err := m.Remove(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(first.ID); err != ErrTaskNotFound {
		t.Fatalf("double remove: %v", err)
	}
	if removed := m.ClearFinished(); removed != 1 {
		t.Fatalf("cleared %d records", removed)
	}
	if tasks := m.List(); len(tasks) != 0 {
		t.Fatalf("records left: %d", len(tasks))
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	m := New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		o(events.Event{Type: "plan", Total: 1})
		o(events.Event{Type: "file_completed", File: "a"})
		return application.Result{Files: []crawler.FileResult{{File: "a"}}}, nil
	}, path)
	task, err := m.Submit(request())
	if err != nil {
		t.Fatal(err)
	}
	awaitState(t, m, task.ID, "completed")
	m.Close()

	reopened := New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		return application.Result{}, nil
	}, path)
	defer reopened.Close()
	restored, ok := reopened.Get(task.ID)
	if !ok || restored.State != "completed" {
		t.Fatalf("record not restored: %+v", restored)
	}
	if restored.Stats.Completed != 1 {
		t.Fatalf("stats not persisted: %+v", restored.Stats)
	}
	retry, err := reopened.Retry(task.ID, "all")
	if err != nil {
		t.Fatal(err)
	}
	if retry.RetryOf != task.ID {
		t.Fatal("retry across restart lost source")
	}
	awaitState(t, reopened, retry.ID, "completed")
	if err := reopened.Remove(task.ID); err != nil {
		t.Fatal(err)
	}
	reopened.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	afterRemove := New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		return application.Result{}, nil
	}, path)
	defer afterRemove.Close()
	if _, ok := afterRemove.Get(task.ID); ok {
		t.Fatal("removed record resurrected")
	}
}

// Package tasks manages in-memory jobs independently of their HTTP transport.
package tasks

import (
	"MediaNinja/core/application"
	"MediaNinja/core/events"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Runner func(context.Context, application.Request, events.Observer) (application.Result, error)
type Task struct {
	LastEvent events.Event            `json:"last_event"`
	ID        string                  `json:"id"`
	State     string                  `json:"state"`
	Created   time.Time               `json:"created"`
	Updated   time.Time               `json:"updated"`
	Request   application.Request     `json:"request"`
	Progress  map[string]events.Event `json:"progress"`
	Result    application.Result      `json:"result"`
	Error     string                  `json:"error,omitempty"`
}
type job struct {
	task   Task
	ctx    context.Context
	cancel context.CancelFunc
}
type Manager struct {
	mu     sync.Mutex
	jobs   map[string]*job
	queue  chan *job
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	runner Runner
}

func New(runner Runner) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{jobs: map[string]*job{}, queue: make(chan *job, 32), ctx: ctx, cancel: cancel, runner: runner}
	m.wg.Add(1)
	go m.work()
	return m
}
func (m *Manager) Close() {
	m.cancel()
	m.mu.Lock()
	for _, j := range m.jobs {
		j.cancel()
		if j.task.State == "queued" {
			j.task.State = "canceled"
			j.task.Updated = time.Now()
		}
	}
	m.mu.Unlock()
	m.wg.Wait()
}
func (m *Manager) Submit(r application.Request) (Task, error) {
	if err := r.Validate(); err != nil {
		return Task{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx.Err() != nil {
		return Task{}, fmt.Errorf("task service closed")
	}

	// Bound retained history without preventing further downloads.
	if len(m.jobs) >= 200 {
		var oldest *job
		for _, candidate := range m.jobs {
			if terminal(candidate.task.State) && (oldest == nil || candidate.task.Created.Before(oldest.task.Created)) {
				oldest = candidate
			}
		}
		if oldest == nil {
			return Task{}, fmt.Errorf("task history is full")
		}
		delete(m.jobs, oldest.task.ID)
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Task{}, err
	}
	ctx, cancel := context.WithCancel(m.ctx)
	now := time.Now()
	r.Names = append([]string(nil), r.Names...)
	j := &job{ctx: ctx, cancel: cancel, task: Task{ID: hex.EncodeToString(id[:]), State: "queued", Created: now, Updated: now, Request: r, Progress: map[string]events.Event{}, Result: application.Result{Files: nil}}}
	select {
	case m.queue <- j:
		m.jobs[j.task.ID] = j
		return clone(j.task), nil
	default:
		cancel()
		return Task{}, fmt.Errorf("task queue is full")
	}
}
func clone(t Task) Task {
	t.Request.Names = append([]string(nil), t.Request.Names...)
	p := map[string]events.Event{}
	for k, v := range t.Progress {
		p[k] = v
	}
	t.Progress = p
	t.Result.Files = append(t.Result.Files[:0:0], t.Result.Files...)
	return t
}
func (m *Manager) List() []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Task, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, clone(j.task))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}
func (m *Manager) Get(id string) (Task, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Task{}, false
	}
	return clone(j.task), true
}
func terminal(s string) bool { return s == "completed" || s == "failed" || s == "canceled" }
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return false
	}
	if !terminal(j.task.State) {
		j.cancel()
		if j.task.State == "queued" {
			j.task.State = "canceled"
		} else {
			j.task.State = "canceling"
		}
		j.task.Updated = time.Now()
	}
	return true
}
func (m *Manager) work() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case j := <-m.queue:
			m.mu.Lock()
			if j.ctx.Err() != nil {
				j.task.State = "canceled"
				j.task.Updated = time.Now()
				m.mu.Unlock()
				continue
			}
			j.task.State = "running"
			j.task.Updated = time.Now()
			m.mu.Unlock()
			result, err := m.runner(j.ctx, j.task.Request, func(e events.Event) {
				m.mu.Lock()
				defer m.mu.Unlock()
				j.task.LastEvent = e
				if e.Type == "progress" {
					j.task.Progress[e.File] = e
				}
				j.task.Updated = time.Now()
			})
			m.mu.Lock()
			j.task.Result = result
			j.task.State = "completed"
			if err != nil {
				j.task.State = "failed"
				j.task.Error = err.Error()
			}
			if j.ctx.Err() != nil {
				j.task.State = "canceled"
			}
			j.task.Updated = time.Now()
			m.mu.Unlock()
			j.cancel()
		}
	}
}

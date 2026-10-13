// Package tasks manages in-memory jobs independently of their HTTP transport.
package tasks

import (
	"MediaNinja/core/application"
	"MediaNinja/core/events"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// 任务服务的可预期错误，HTTP 层据此映射状态码。
var (
	ErrTaskNotFound  = errors.New("task not found")
	ErrTaskActive    = errors.New("task has not finished")
	ErrNothingFailed = errors.New("task has no failed files to retry")
	ErrInvalidScope  = errors.New("scope must be all or failed")
)

type Runner func(context.Context, application.Request, events.Observer) (application.Result, error)

// Stats 汇总一次任务的文件计数：plan 事件给出总数，文件事件累加完成与失败。
type Stats struct {
	Total     int64 `json:"total"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

type Task struct {
	LastEvent events.Event            `json:"last_event"`
	ID        string                  `json:"id"`
	State     string                  `json:"state"`
	Created   time.Time               `json:"created"`
	Updated   time.Time               `json:"updated"`
	Request   application.Request     `json:"request"`
	Progress  map[string]events.Event `json:"progress"`
	Stats     Stats                   `json:"stats"`
	Result    application.Result      `json:"result"`
	Error     string                  `json:"error,omitempty"`
	RetryOf   string                  `json:"retry_of,omitempty"`
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
	// statePath 为空表示仅内存运行；否则终态任务持久化到该文件。
	statePath string
}

// New 创建任务管理器。statePath 非空时从文件加载终态记录，并在记录变化时落盘。
func New(runner Runner, statePath string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{jobs: map[string]*job{}, queue: make(chan *job, 32), ctx: ctx, cancel: cancel, runner: runner, statePath: statePath}
	for _, task := range loadState(statePath) {
		if !terminal(task.State) {
			continue
		}
		m.jobs[task.ID] = &job{task: task}
	}
	m.wg.Add(1)
	go m.work()
	return m
}
func (m *Manager) Close() {
	m.cancel()
	m.mu.Lock()
	for _, j := range m.jobs {
		if j.cancel != nil {
			j.cancel()
		}
		if j.task.State == "queued" {
			j.task.State = "canceled"
			j.task.Updated = time.Now()
		}
	}
	m.persistLocked()
	m.mu.Unlock()
	m.wg.Wait()
}

// Submit 校验并入队一个新任务。
func (m *Manager) Submit(r application.Request) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.submitLocked(r, "")
}

// Retry 以源任务为模板入队一个新任务。scope=all 原样重跑；scope=failed
// 跳过源任务中已成功收集的文件，仅重新下载失败的部分。
func (m *Manager) Retry(id, scope string) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	if !terminal(j.task.State) {
		return Task{}, ErrTaskActive
	}
	if scope == "" {
		scope = "all"
	}
	if scope != "all" && scope != "failed" {
		return Task{}, ErrInvalidScope
	}
	request := cloneTask(j.task).Request
	if scope == "failed" {
		skip := make([]string, 0, len(j.task.Result.Files))
		failed := 0
		for _, file := range j.task.Result.Files {
			if file.Error == "" {
				skip = append(skip, file.File)
			} else {
				failed++
			}
		}
		if failed == 0 {
			return Task{}, ErrNothingFailed
		}
		request.SkipFiles = skip
	} else {
		request.SkipFiles = nil
	}
	return m.submitLocked(request, id)
}
func terminal(s string) bool { return s == "completed" || s == "failed" || s == "canceled" }

// submitLocked 入队任务；调用方必须持有 m.mu。retryOf 非空时记录重试来源。
func (m *Manager) submitLocked(r application.Request, retryOf string) (Task, error) {
	if err := r.Validate(); err != nil {
		return Task{}, err
	}
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
		m.persistLocked()
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Task{}, err
	}
	ctx, cancel := context.WithCancel(m.ctx)
	now := time.Now()
	j := &job{ctx: ctx, cancel: cancel, task: Task{ID: hex.EncodeToString(id[:]), State: "queued", Created: now, Updated: now, Request: r, Progress: map[string]events.Event{}, Result: application.Result{Files: nil}, RetryOf: retryOf}}
	select {
	case m.queue <- j:
		m.jobs[j.task.ID] = j
		return cloneTask(j.task), nil
	default:
		cancel()
		return Task{}, fmt.Errorf("task queue is full")
	}
}

// cloneTask 返回任务的深拷贝，避免调用方持有可变内部状态。
func cloneTask(t Task) Task {
	if t.Request.Batch != nil {
		batch := *t.Request.Batch
		batch.Names = append([]string(nil), t.Request.Batch.Names...)
		t.Request.Batch = &batch
	}
	t.Request.SkipFiles = append([]string(nil), t.Request.SkipFiles...)
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
		out = append(out, cloneTask(j.task))
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
	return cloneTask(j.task), true
}

// Remove 删除一条终态记录；运行中的任务须先取消。
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return ErrTaskNotFound
	}
	if !terminal(j.task.State) {
		return ErrTaskActive
	}
	delete(m.jobs, id)
	m.persistLocked()
	return nil
}

// ClearFinished 清空全部终态记录，返回删除数量。
func (m *Manager) ClearFinished() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for id, j := range m.jobs {
		if terminal(j.task.State) {
			delete(m.jobs, id)
			removed++
		}
	}
	if removed > 0 {
		m.persistLocked()
	}
	return removed
}
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
		m.persistLocked()
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
				switch e.Type {
				case "progress", "file_completed", "file_failed":
					// Progress 保留每个文件的最后一个事件，前端据此区分
					// 下载中 / 已完成 / 失败。
					j.task.Progress[e.File] = e
				}
				switch e.Type {
				case "plan":
					j.task.Stats.Total += e.Total
				case "file_completed":
					j.task.Stats.Completed++
				case "file_failed":
					j.task.Stats.Failed++
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
			m.persistLocked()
			m.mu.Unlock()
			j.cancel()
		}
	}
}

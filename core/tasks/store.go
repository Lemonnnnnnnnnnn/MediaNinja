package tasks

import (
	"encoding/json"
	"os"
	"sort"
)

// loadState 从磁盘读取终态任务记录；文件缺失或损坏时返回空切片，
// 由调用方决定跳过（无状态运行）。
func loadState(path string) []Task {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil
	}
	return tasks
}

// persistLocked 把全部终态任务原子写入状态文件；调用方必须持有 m.mu。
// core 不向 stdout 打印日志，持久化失败时保持静默，内存状态不受影响。
func (m *Manager) persistLocked() {
	if m.statePath == "" {
		return
	}
	tasks := make([]Task, 0, len(m.jobs))
	for _, j := range m.jobs {
		if terminal(j.task.State) {
			tasks = append(tasks, cloneTask(j.task))
		}
	}
	sort.Slice(tasks, func(i, k int) bool { return tasks[i].Created.After(tasks[k].Created) })
	data, err := json.Marshal(tasks)
	if err != nil {
		return
	}
	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	os.Rename(tmp, m.statePath)
}

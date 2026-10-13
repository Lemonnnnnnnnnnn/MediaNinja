export interface Config {
  URL: string; ProxyURL: string; Concurrency: number; OutputDir: string
  MaxRetries: number; RetryDelay: number; MaxPages: number; MaxVideos: number
}
export interface ExecutionEvent {
  type: string; message?: string; file?: string; completed?: number
  total?: number; unit?: 'bytes' | 'segments'; error?: string; time: string
}
export type TaskState = 'queued' | 'running' | 'canceling' | 'completed' | 'failed' | 'canceled'
export interface BatchSiteInfo { id: string; label: string; summary: string }
export interface SessionInfo { protocol: number; batch_sites: BatchSiteInfo[] }
export interface BatchRequest { site: string; names: string[] }
export interface DownloadRequest { url?: string; batch?: BatchRequest; options?: Partial<Config> }
export interface TaskStats { total: number; completed: number; failed: number }
export type RetryScope = 'all' | 'failed'
export interface Task {
  id: string; state: TaskState; created: string; updated: string
  request: { url?: string; batch?: BatchRequest }; progress: Record<string, ExecutionEvent>
  stats: TaskStats; last_event: ExecutionEvent
  result: { files: { file: string; error?: string }[] | null }; error?: string; retry_of?: string
}
export const isActive = (task: Task) => ['queued', 'running', 'canceling'].includes(task.state)
export const isTerminal = (task: Task) => !isActive(task)

// taskSummary 汇总任务文件计数：plan 事件给出总数，结果列表给出最终成败。
export function taskSummary(task: Task) {
  const files = task.result.files ?? []
  const failedFiles = files.filter(file => file.error).length
  return {
    total: Math.max(task.stats.total, files.length),
    completed: task.stats.completed || files.length - failedFiles,
    failed: task.stats.failed || failedFiles,
  }
}
// 文件级进度分类：progress 是每个文件的最后一个事件，据此区分下载中/失败/完成。
export type FileGroup = 'active' | 'failed' | 'done'
export function fileGroup(event: ExecutionEvent): FileGroup {
  if (event.type === 'file_failed') return 'failed'
  if (event.type === 'file_completed') return 'done'
  return 'active'
}
export class ApiError extends Error { constructor(public status: number, message: string) { super(message) } }

// The same client runs in a browser or a desktop WebView; it has no React/Tauri dependency.
export class MediaNinjaClient {
  constructor(private baseUrl = '') {}
  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(`${this.baseUrl}${path}`, { credentials: 'same-origin', ...init })
    if (!response.ok) {
      const body = await response.json().catch(() => null) as { error?: string } | null
      throw new ApiError(response.status, body?.error ?? `Request failed (${response.status})`)
    }
    return response.status === 204 ? undefined as T : response.json() as Promise<T>
  }
  bootstrap() { return this.request<SessionInfo>('/api/session') }
  config() { return this.request<Config>('/api/config') }
  saveConfig(config: Config) { return this.request<Config>('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(config) }) }
  tasks() { return this.request<Task[]>('/api/tasks') }
  submit(request: DownloadRequest) { return this.request<Task>('/api/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request) }) }
  cancel(id: string) { return this.request<void>(`/api/tasks/${encodeURIComponent(id)}/cancel`, { method: 'POST' }) }
  retry(id: string, scope: RetryScope = 'all') {
    return this.request<Task>(`/api/tasks/${encodeURIComponent(id)}/retry`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ scope }) })
  }
  remove(id: string) { return this.request<void>(`/api/tasks/${encodeURIComponent(id)}`, { method: 'DELETE' }) }
  clearFinished() { return this.request<{ removed: number }>('/api/tasks', { method: 'DELETE' }) }
  subscribe(onTasks: (tasks: Task[]) => void, onConnection: (connected: boolean) => void) {
    const source = new EventSource(`${this.baseUrl}/api/events`)
    source.addEventListener('tasks', event => {
      try { onTasks(JSON.parse((event as MessageEvent<string>).data) as Task[]); onConnection(true) }
      catch { onConnection(false) }
    })
    source.onerror = () => onConnection(false)
    return () => source.close()
  }
}

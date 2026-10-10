export interface Config {
  URL: string; ProxyURL: string; Concurrency: number; OutputDir: string
  MaxRetries: number; RetryDelay: number; MaxPages: number; MaxVideos: number
}
export interface ExecutionEvent {
  type: string; message?: string; file?: string; completed?: number
  total?: number; unit?: 'bytes' | 'segments'; error?: string; time: string
}
export type TaskState = 'queued' | 'running' | 'canceling' | 'completed' | 'failed' | 'canceled'
export interface DownloadRequest { url?: string; names?: string[]; options?: Partial<Config> }
export interface Task {
  id: string; state: TaskState; created: string; updated: string
  request: DownloadRequest; progress: Record<string, ExecutionEvent>
  last_event: ExecutionEvent; result: { files: { file: string; error?: string }[] | null }; error?: string
}
export const isActive = (task: Task) => ['queued', 'running', 'canceling'].includes(task.state)
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
  bootstrap() { return this.request<{ protocol: number }>('/api/session') }
  config() { return this.request<Config>('/api/config') }
  saveConfig(config: Config) { return this.request<Config>('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(config) }) }
  tasks() { return this.request<Task[]>('/api/tasks') }
  submit(request: DownloadRequest) { return this.request<Task>('/api/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request) }) }
  cancel(id: string) { return this.request<void>(`/api/tasks/${encodeURIComponent(id)}/cancel`, { method: 'POST' }) }
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

import { useState } from 'react'
import { ChevronDown, ChevronUp, RotateCcw, Square, Trash2 } from 'lucide-react'
import { fileGroup, isActive, isTerminal, taskSummary, type ExecutionEvent, type RetryScope, type Task } from '@medianinja/api-client'
import { Button } from '@/components/ui/button'

const labels = { queued: 'QUEUED', running: 'DOWNLOADING', canceling: 'CANCELING', completed: 'COLLECTED', failed: 'FAILED', canceled: 'CANCELED' }

function fileName(event: ExecutionEvent) {
  return event.file?.split(/[\\/]/).pop() ?? event.file ?? ''
}
function FileRow({ event }: { event: ExecutionEvent }) {
  const done = event.type === 'file_completed'
  const failed = event.type === 'file_failed'
  const pct = event.total ? Math.min(100, Math.round(((event.completed ?? 0) / event.total) * 100)) : 0
  return <div className={`file-progress${done ? ' is-done' : ''}${failed ? ' is-failed' : ''}`}>
    <div><span title={event.file}>{fileName(event)}</span><span>{done ? '✓' : failed ? '✗' : event.unit === 'segments' ? `${event.completed ?? 0} / ${event.total || '?'} segments` : `${((event.completed ?? 0) / 1048576).toFixed(1)}${event.total ? ` / ${(event.total / 1048576).toFixed(1)}` : ''} MB`}</span></div>
    {!done && !failed && <div className={`file-bar${event.total ? '' : ' indeterminate'}`}>{event.total ? <span style={{ width: `${pct}%` }} /> : <span />}</div>}
    {failed && event.error && <p className="file-error">{event.error}</p>}
  </div>
}

export function TaskCard({ task, onCancel, onRetry, onDelete, collapsible = false, defaultCollapsed = false }: {
  task: Task
  onCancel: (id: string) => Promise<void>
  onRetry?: (id: string, scope: RetryScope) => Promise<void>
  onDelete?: (id: string) => Promise<void>
  collapsible?: boolean
  defaultCollapsed?: boolean
}) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed)
  const [showDone, setShowDone] = useState(false)
  const [canceling, setCanceling] = useState(false)
  const [busy, setBusy] = useState(false)
  const title = task.request.url ?? task.request.batch?.names.join(', ') ?? 'Download'
  const summary = taskSummary(task)
  const collected = summary.completed + summary.failed
  const pct = summary.total ? Math.round((collected / summary.total) * 100) : 0
  const rows = Object.values(task.progress)
  const activeFiles = rows.filter(event => fileGroup(event) === 'active')
  const failedFiles = rows.filter(event => fileGroup(event) === 'failed')
  const doneFiles = rows.filter(event => fileGroup(event) === 'done')
  const partial = summary.failed > 0 && summary.completed > 0
  const terminal = isTerminal(task)
  const act = async (run: () => Promise<void>) => { setBusy(true); try { await run() } finally { setBusy(false) } }
  return <article className={`task-card task-${task.state}`}>
    <div className="task-top">
      {collapsible && <button className="task-fold" aria-expanded={!collapsed} aria-label={collapsed ? 'Expand record' : 'Collapse record'} onClick={() => setCollapsed(value => !value)}>{collapsed ? <ChevronDown /> : <ChevronUp />}</button>}
      <span className="task-state">{labels[task.state]}</span>
      {summary.total > 0 && <span className="task-stats">{collected} / {summary.total} FILES · {pct}%{summary.failed > 0 ? ` · ${summary.failed} FAILED` : ''}</span>}
      <time dateTime={task.created}>{new Date(task.created).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</time>
      <div className="task-actions">
        {isActive(task) && <Button variant="outline" size="sm" disabled={canceling} onClick={async () => { setCanceling(true); try { await onCancel(task.id) } finally { setCanceling(false) } }}><Square />{canceling ? 'STOPPING…' : 'STOP'}</Button>}
        {terminal && onRetry && <Button variant="outline" size="sm" disabled={busy} onClick={() => act(() => onRetry(task.id, 'all'))}><RotateCcw />{partial ? 'RETRY ALL' : 'RETRY'}</Button>}
        {terminal && onRetry && partial && <Button variant="outline" size="sm" disabled={busy} onClick={() => act(() => onRetry(task.id, 'failed'))}><RotateCcw />RETRY FAILED</Button>}
        {terminal && onDelete && <Button variant="ghost" size="icon" disabled={busy} aria-label="Delete record" onClick={() => act(() => onDelete(task.id))}><Trash2 /></Button>}
      </div>
    </div>
    {collapsed
      ? <p className="task-detail">{summary.completed} / {summary.total} files{summary.failed > 0 ? ` · ${summary.failed} failed` : ''}</p>
      : <>
        <h3>{title}</h3>
        {summary.total > 0 && <div className="task-total" role="progressbar" aria-label="Overall file progress" aria-valuemin={0} aria-valuemax={summary.total} aria-valuenow={collected}><div className="fill" data-state={task.state} style={{ width: `${pct}%` }} /></div>}
        {[...activeFiles, ...failedFiles].map(event => <FileRow key={event.file} event={event} />)}
        {doneFiles.length > 0 && (showDone
          ? <>{doneFiles.map(event => <FileRow key={event.file} event={event} />)}<button className="text-action" onClick={() => setShowDone(false)}>HIDE COLLECTED ↘</button></>
          : <button className="task-unfold" onClick={() => setShowDone(true)}>{doneFiles.length} COLLECTED ↗</button>)}
        {!!task.result.files?.length && <p className="task-detail">{task.result.files.filter(file => !file.error).length} / {task.result.files.length} files collected</p>}
        {task.last_event?.message && isActive(task) && <p className="task-detail">{task.last_event.message}</p>}
        {task.error && <p className="task-error">{task.error}</p>}
      </>}
  </article>
}

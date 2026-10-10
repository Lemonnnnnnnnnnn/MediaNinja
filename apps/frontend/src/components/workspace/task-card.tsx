import { useState } from 'react'
import { Square } from 'lucide-react'
import { isActive, type Task } from '@medianinja/api-client'
import { Button } from '@/components/ui/button'

const labels = { queued: 'QUEUED', running: 'DOWNLOADING', canceling: 'CANCELING', completed: 'COLLECTED', failed: 'FAILED', canceled: 'CANCELED' }
export function TaskCard({ task, onCancel }: { task: Task; onCancel: (id: string) => Promise<void> }) {
  const [canceling, setCanceling] = useState(false)
  const title = task.request.url ?? task.request.names?.join(', ') ?? 'Download'
  return <article className={`task-card task-${task.state}`}>
    <div className="task-top"><span className="task-state">{labels[task.state]}</span><time dateTime={task.created}>{new Date(task.created).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</time>{['queued', 'running'].includes(task.state) && <Button variant="outline" size="sm" disabled={canceling} onClick={async () => { setCanceling(true); try { await onCancel(task.id) } finally { setCanceling(false) } }}><Square />{canceling ? 'STOPPING…' : 'STOP'}</Button>}</div>
    <h3>{title}</h3>
    {Object.values(task.progress).map(progress => <div className="file-progress" key={progress.file}><div><span title={progress.file}>{progress.file?.split(/[\\/]/).pop()}</span><span>{progress.unit === 'segments' ? `${progress.completed ?? 0} / ${progress.total || '?'} segments` : `${((progress.completed ?? 0) / 1048576).toFixed(1)}${progress.total ? ` / ${(progress.total / 1048576).toFixed(1)}` : ''} MB`}</span></div>{!!progress.total && <progress aria-label={`Progress for ${progress.file}`} max={progress.total} value={progress.completed ?? 0} />}</div>)}
    {!!task.result.files?.length && <p className="task-detail">{task.result.files.filter(file => !file.error).length} / {task.result.files.length} files collected</p>}
    {task.last_event?.message && isActive(task) && <p className="task-detail">{task.last_event.message}</p>}
    {task.error && <p className="task-error">{task.error}</p>}
  </article>
}

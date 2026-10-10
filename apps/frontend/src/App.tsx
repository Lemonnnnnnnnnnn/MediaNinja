import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ArrowDownToLine, ArrowUpRight, Check, CircleAlert, FolderOpen, History, LoaderCircle, Settings2, Wifi, WifiOff } from 'lucide-react'
import { isActive, MediaNinjaClient, type Config, type Task } from '@medianinja/api-client'
import { createHost } from '@medianinja/platform'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { FloatingPanel } from '@/components/workspace/floating-panel'
import { SettingsForm } from '@/components/workspace/settings-form'
import { TaskCard } from '@/components/workspace/task-card'

const api = new MediaNinjaClient()
const host = createHost()
const errorMessage = (error: unknown) => error instanceof Error ? error.message : typeof error === 'string' ? error : 'Something went wrong. Please try again.'
const readPanel = () => location.hash === '#/settings' ? 'settings' : location.hash === '#/history' ? 'history' : null

export function App() {
  const [panel, setPanel] = useState<'settings' | 'history' | null>(readPanel)
  const [config, setConfig] = useState<Config | null>(null)
  const [draft, setDraft] = useState<Config | null>(null)
  const [tasks, setTasks] = useState<Task[]>([])
  const [connected, setConnected] = useState(false)
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [mode, setMode] = useState<'url' | 'names'>('url')
  const [target, setTarget] = useState('')
  const [busy, setBusy] = useState(false)
  const [filter, setFilter] = useState<'active' | 'all' | 'failed'>('active')
  const targetInput = useRef<HTMLInputElement>(null)
  const previous = useRef<Map<string, Task['state']>>(new Map())
  const updateTasks = (next: Task[]) => {
    previous.current = new Map(next.map(task => [task.id, task.state]))
    setTasks(next)
  }
  useEffect(() => {
    const update = () => { setPanel(readPanel()); setMessage(''); setError('') }
    window.addEventListener('hashchange', update)
    return () => window.removeEventListener('hashchange', update)
  }, [])
  useEffect(() => {
    let alive = true
    let unsubscribe = () => {}
    const update = (next: Task[]) => {
      if (!alive) return
      for (const task of next) {
        const old = previous.current.get(task.id)
        if (old && ['queued', 'running', 'canceling'].includes(old) && ['completed', 'failed'].includes(task.state)) {
          void host.notify('MediaNinja', task.state === 'completed' ? 'Your download is complete.' : 'Download failed. Open the task for details.').catch(() => {})
        }
      }
      previous.current = new Map(next.map(task => [task.id, task.state]))
      setTasks(next)
    }
    void (async () => {
      try {
        await api.bootstrap()
        const [settings, initial] = await Promise.all([api.config(), api.tasks()])
        if (!alive) return
        setConfig(settings); setDraft(settings); update(initial)
        unsubscribe = api.subscribe(update, value => { if (alive) setConnected(value) })
      } catch (error) { if (alive) setError(errorMessage(error)) }
      finally { if (alive) setLoading(false) }
    })()
    return () => { alive = false; unsubscribe() }
  }, [])
  const active = tasks.filter(isActive)
  const history = tasks.filter(task => !isActive(task))
  const completed = tasks.filter(task => task.state === 'completed').length
  const failed = tasks.filter(task => task.state === 'failed')
  const visibleTasks = filter === 'active' ? active : filter === 'failed' ? failed : tasks
  const navigate = (next: 'settings' | 'history' | null) => { location.hash = next ? `/${next}` : '/downloads' }
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!config || busy) return
    setBusy(true); setError(''); setMessage('')
    try {
      const input = target.trim()
      await api.submit(mode === 'url' ? { url: input, options: config } : { names: input.split(/[,，]/).map(name => name.trim()).filter(Boolean), options: config })
      updateTasks(await api.tasks()); setTarget(''); setFilter('active'); setMessage('Download added to the queue.')
    } catch (error) { setError(errorMessage(error)) } finally { setBusy(false) }
  }
  const cancel = async (id: string) => {
    try { await api.cancel(id); updateTasks(await api.tasks()) }
    catch (error) { setError(errorMessage(error)) }
  }
  const save = async (event: FormEvent) => {
    event.preventDefault()
    if (!draft) return
    setBusy(true); setError(''); setMessage('')
    try {
      const saved = await api.saveConfig(draft)
      setConfig(saved); setDraft(saved); setMessage('Settings saved. New downloads will use them.')
    } catch (error) { setError(errorMessage(error)) } finally { setBusy(false) }
  }
  const selectDirectory = async () => {
    try { const selected = await host.selectDirectory(); if (selected) setDraft(current => current && { ...current, OutputDir: selected }) }
    catch (error) { setError(errorMessage(error)) }
  }
  const notices = <>{error && <div role="alert" className="notice notice-error"><CircleAlert aria-hidden="true" />{error}</div>}{message && <div role="status" className="notice notice-success"><Check aria-hidden="true" />{message}</div>}</>

  return <main className="canvas">
    <div inert={panel ? true : undefined}>
    <div className="canvas-grid" aria-hidden="true" />
    <a className="wordmark" href="#/downloads" aria-label="MediaNinja home"><span className="brand-mark">m<span>↘</span></span><span>MEDIA<br />NINJA<span className="brand-period">.</span></span></a>
    <div className="canvas-tools">
      <span className={`connection ${connected ? 'is-connected' : ''}`} role="status">{connected ? <Wifi /> : <WifiOff />}<span>{connected ? 'ENGINE ONLINE' : loading ? 'CONNECTING' : 'RECONNECTING'}</span></span>
      <Button variant="outline" onClick={() => navigate('history')} aria-label="Open session history"><History /><span>History</span><span className="tool-count">{history.length.toString().padStart(2, '0')}</span></Button>
      <Button variant="outline" onClick={() => navigate('settings')} aria-label="Open settings"><Settings2 /><span>Settings</span></Button>
    </div>
    <div className="workspace-heading"><h1>DOWNLOAD WORKSPACE</h1><span>{active.length} active / {completed} completed / {failed.length} failed</span></div>
    {!panel && notices}
    <div className="workbench">
      <section className="download-station" aria-labelledby="capture-title">
        <div className="station-title"><h2 id="capture-title">NEW DOWNLOAD</h2><span>01 ↘</span></div>
        <form className="capture-box" onSubmit={submit}>
          <div className="mode-switch" role="group" aria-label="Download source">{(['url', 'names'] as const).map(value => <button key={value} type="button" aria-pressed={mode === value} onClick={() => setMode(value)}>{value === 'url' ? 'PAGE URL' : 'RULE34 / BATCH'}</button>)}</div>
          <label className="target-label" htmlFor="download-target">{mode === 'url' ? 'PAGE URL' : 'CREATOR NAMES / KEYWORDS'}</label>
          <Input ref={targetInput} id="download-target" required type={mode === 'url' ? 'url' : 'text'} value={target} onChange={event => setTarget(event.target.value)} placeholder={mode === 'url' ? 'https://…' : 'creator, keyword…'} />
          <p className="input-hint">{mode === 'url' ? 'A video, image or supported media page.' : 'Separate creators or keywords with commas.'}</p>
          <Button className="capture-submit" disabled={busy || !config || !connected}>{busy ? <LoaderCircle className="animate-spin" /> : <ArrowDownToLine />}<span>{busy ? 'ADDING…' : 'QUEUE DOWNLOAD'}</span></Button>
          <div className="destination-heading"><label htmlFor="output-summary">OUTPUT DIRECTORY</label><button type="button" onClick={() => navigate('settings')}>EDIT <ArrowUpRight /></button></div>
          <div className="output-summary" id="output-summary"><FolderOpen aria-hidden="true" /><span title={config?.OutputDir}>{config?.OutputDir ?? 'Connecting to the engine…'}</span></div>
          {host.kind === 'desktop' && config && <button className="text-action" type="button" onClick={() => void host.openOutputDirectory(config.OutputDir).catch(error => setError(errorMessage(error)))}>OPEN DIRECTORY ↗</button>}
        </form>
        <div className="engine-snapshot"><div className="snapshot-heading"><h3>ENGINE SETTINGS</h3><button onClick={() => navigate('settings')} aria-label="Edit engine settings"><Settings2 /></button></div><dl><div><dt>Concurrent files</dt><dd>{config?.Concurrency ?? '—'}</dd></div><div><dt>Maximum attempts</dt><dd>{config?.MaxRetries ?? '—'}</dd></div><div><dt>Retry delay</dt><dd>{config ? `${config.RetryDelay}s` : '—'}</dd></div><div><dt>Connection</dt><dd>{config ? config.ProxyURL ? 'Proxy' : 'Direct' : '—'}</dd></div>{mode === 'names' && <><div><dt>Rule34 list pages</dt><dd>{config?.MaxPages ?? '—'}</dd></div><div><dt>Video limit</dt><dd>{config ? config.MaxVideos || 'Unlimited' : '—'}</dd></div></>}</dl></div>
      </section>
      <section className="queue-area" aria-labelledby="queue-title">
        <div className="queue-heading"><h2 id="queue-title"><span className="live-dot" /> DOWNLOADS <span className="queue-count">{tasks.length.toString().padStart(2, '0')}</span></h2><span className="queue-order">QUEUED TASKS RUN IN ORDER</span></div>
        <div className="queue-filters" role="group" aria-label="Filter downloads">{([{ id: 'active', label: 'Active', count: active.length }, { id: 'all', label: 'This session', count: tasks.length }, { id: 'failed', label: 'Failed', count: failed.length }] as const).map(item => <button key={item.id} aria-pressed={filter === item.id} onClick={() => setFilter(item.id)}>{item.label}<span>{item.count.toString().padStart(2, '0')}</span></button>)}</div>
        <div className="queue-content">{loading ? <div className="empty-queue"><LoaderCircle className="animate-spin" /><h3>Connecting to the engine…</h3></div> : visibleTasks.length ? <div className="task-list">{visibleTasks.map(task => <TaskCard key={task.id} task={task} onCancel={cancel} />)}</div> : <div className="empty-queue"><span className="empty-symbol" aria-hidden="true">[ {filter === 'failed' ? '✓' : '↓'} ]</span><h3>{filter === 'failed' ? 'NO FAILED DOWNLOADS' : filter === 'all' ? 'NO DOWNLOADS YET' : 'NO ACTIVE DOWNLOADS'}</h3><p>{filter === 'failed' ? 'Failed tasks will appear here.' : 'Add a page URL or a creator batch to start a download.'}</p>{filter !== 'failed' && <Button variant="outline" size="sm" onClick={() => targetInput.current?.focus()}>ADD A DOWNLOAD <ArrowUpRight /></Button>}</div>}</div>
        <div className="queue-bottom"><span>{completed} completed · {failed.length} failed</span><button onClick={() => navigate('history')}>SESSION HISTORY <History /></button></div>
      </section>
    </div>
    <footer className="canvas-footer"><span>{host.kind === 'desktop' ? 'CLOSE TO TRAY · QUIT TO STOP DOWNLOADS' : 'DOWNLOADS CONTINUE AFTER CLOSING THIS PAGE'}</span><span>SETTINGS & HISTORY ARE SAVED FOR THIS SESSION ONLY</span></footer>
    </div>
    {panel && <FloatingPanel title={panel === 'settings' ? 'SETTINGS' : 'SESSION HISTORY'} label={panel === 'settings' ? 'CONTROL ROOM / SETTINGS' : 'ARCHIVE / THIS SESSION'} onClose={() => navigate(null)}>
      {notices}
      {panel === 'settings' ? draft ? <SettingsForm draft={draft} onChange={setDraft} onSave={save} busy={busy} canSelectDirectory={host.canSelectDirectory} onSelectDirectory={selectDirectory} onOpenDirectory={host.kind === 'desktop' && config ? () => void host.openOutputDirectory(config.OutputDir).catch(error => setError(errorMessage(error))) : undefined} /> : <p className="panel-note">Settings will appear when the engine connects.</p> : <><p className="panel-note">Completed, failed and canceled downloads from this session. Records clear when the engine restarts.</p>{history.length ? <div className="task-list">{history.map(task => <TaskCard key={task.id} task={task} onCancel={cancel} />)}</div> : <div className="history-empty"><span aria-hidden="true">[ 00 ]</span><h3>NO FINISHED DOWNLOADS</h3><p>Completed, failed and canceled tasks will appear here.</p></div>}</>}
    </FloatingPanel>}
  </main>
}

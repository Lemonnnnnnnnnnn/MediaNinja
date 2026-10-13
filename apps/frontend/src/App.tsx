import { useEffect, useRef, useState, type FormEvent, type RefObject } from 'react'
import { ArrowUpRight, Check, ChevronDown, ChevronUp, CircleAlert, History, LoaderCircle, Settings2, Wifi, WifiOff } from 'lucide-react'
import { isActive, isTerminal, MediaNinjaClient, type BatchSiteInfo, type Config, type RetryScope, type Task } from '@medianinja/api-client'
import { createHost } from '@medianinja/platform'
import { Button } from '@/components/ui/button'
import { DownloadForm } from '@/components/workspace/download-form'
import { BatchPage } from '@/pages/batch-page'
import { FloatingPanel } from '@/components/workspace/floating-panel'
import { SettingsForm } from '@/components/workspace/settings-form'
import { TaskCard } from '@/components/workspace/task-card'
import { createAutosave, type SaveState } from '@/lib/autosave'
import { settingsIssue } from '@/lib/settings'

const api = new MediaNinjaClient()
const host = createHost()
const errorMessage = (error: unknown) => error instanceof Error ? error.message : typeof error === 'string' ? error : 'Something went wrong. Please try again.'
const readPage = () => location.hash.startsWith('#/batch') ? 'batch' : 'downloads'
const readPanel = () => location.hash.includes('/settings') ? 'settings' : location.hash.endsWith('/history') ? 'history' : null
const readSettingsTab = () => location.hash.endsWith('/parsers') ? 'parsers' : 'general'
const stateRank = { running: 0, queued: 1, canceling: 2, failed: 3, canceled: 4, completed: 5 }
const byQueueOrder = (a: Task, b: Task) => stateRank[a.state] - stateRank[b.state] || +new Date(b.updated) - +new Date(a.updated)

export function App() {
  const [panel, setPanel] = useState<'settings' | 'history' | null>(readPanel)
  const [page, setPage] = useState<'downloads' | 'batch'>(readPage)
  const [settingsTab, setSettingsTab] = useState<'general' | 'parsers'>(readSettingsTab)
  const [config, setConfig] = useState<Config | null>(null)
  const [draft, setDraft] = useState<Config | null>(null)
  const draftRef = useRef(draft)
  draftRef.current = draft
  const [saveState, setSaveState] = useState<SaveState>({ status: 'saved' })
  const [settingsSaver] = useState(() => createAutosave<Config>(value => api.saveConfig(value), (saved, current) => {
    setConfig(saved)
    if (current) setDraft(saved)
  }, setSaveState))
  useEffect(() => () => settingsSaver.cancel(), [settingsSaver])
  const changeSettings = (next: Config) => {
    setDraft(next)
    settingsSaver.stage(next, settingsIssue(next))
  }
  const [tasks, setTasks] = useState<Task[]>([])
  const [batchSites, setBatchSites] = useState<BatchSiteInfo[]>([])
  const [batchSite, setBatchSite] = useState('')
  const [connected, setConnected] = useState(false)
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [targets, setTargets] = useState({ downloads: '', batch: '' })
  const target = targets[page]
  const setTarget = (value: string) => setTargets(current => ({ ...current, [page]: value }))
  const [busy, setBusy] = useState(false)
  const [filter, setFilter] = useState<'active' | 'all' | 'failed'>('all')
  const [showFinished, setShowFinished] = useState(false)
  const targetInput = useRef<HTMLInputElement>(null)
  const previous = useRef<Map<string, Task['state']>>(new Map())
  const updateTasks = (next: Task[]) => {
    previous.current = new Map(next.map(task => [task.id, task.state]))
    setTasks(next)
  }
  useEffect(() => {
    const update = () => { void settingsSaver.flush(); setPage(readPage()); setPanel(readPanel()); setSettingsTab(readSettingsTab()); setMessage(''); setError('') }
    window.addEventListener('hashchange', update)
    return () => window.removeEventListener('hashchange', update)
  }, [settingsSaver])
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
        const [session, settings, initial] = await Promise.all([api.bootstrap(), api.config(), api.tasks()])
        if (!alive) return
        setBatchSites(session.batch_sites ?? [])
        setBatchSite(session.batch_sites?.[0]?.id ?? '')
        setConfig(settings); setDraft(settings); update(initial)
        unsubscribe = api.subscribe(update, value => { if (alive) setConnected(value) })
      } catch (error) { if (alive) setError(errorMessage(error)) }
      finally { if (alive) setLoading(false) }
    })()
    return () => { alive = false; unsubscribe() }
  }, [])
  const isBatchTask = (task: Task) => !!task.request.batch
  const pageTasks = tasks.filter(task => page === 'batch' ? isBatchTask(task) : !isBatchTask(task))
  const ordered = [...pageTasks].sort(byQueueOrder)
  const active = ordered.filter(isActive)
  const finished = ordered.filter(isTerminal)
  const history = tasks.filter(isTerminal)
  const completed = pageTasks.filter(task => task.state === 'completed').length
  const failed = pageTasks.filter(task => task.state === 'failed')
  const visibleTasks = filter === 'active' ? active : filter === 'failed' ? failed : ordered
  const navigate = (next: 'settings' | 'history' | null, tab: 'general' | 'parsers' = 'general') => {
    if (panel === 'settings') void settingsSaver.flush()
    location.hash = `/${page}${next ? `/${next}${next === 'settings' && tab === 'parsers' ? '/parsers' : ''}` : ''}`
  }
  const refresh = async () => updateTasks(await api.tasks())
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!config || busy) return
    setBusy(true); setError(''); setMessage('')
    try {
      const state = await settingsSaver.flush()
      if (state.status !== 'saved') throw new Error(state.message ?? 'Settings have not been saved. Check settings before starting a download.')
      const options = await api.config()
      setConfig(options)
      const input = target.trim()
      if (page === 'batch' && !batchSite) throw new Error('No batch source is available.')
      await api.submit(page === 'downloads'
        ? { url: input, options }
        : { batch: { site: batchSite, names: input.split(/[,，]/).map(name => name.trim()).filter(Boolean) }, options })
      await refresh(); setTarget(''); setFilter('all'); setMessage('Download added to the queue.')
    } catch (error) { setError(errorMessage(error)) } finally { setBusy(false) }
  }
  const cancel = async (id: string) => {
    try { await api.cancel(id); await refresh() }
    catch (error) { setError(errorMessage(error)) }
  }
  const retry = async (id: string, scope: RetryScope) => {
    try { await api.retry(id, scope); await refresh(); setMessage(scope === 'failed' ? 'Retrying the failed files.' : 'Retry added to the queue.') }
    catch (error) { setError(errorMessage(error)) }
  }
  const removeRecord = async (id: string) => {
    try { await api.remove(id); await refresh() }
    catch (error) { setError(errorMessage(error)) }
  }
  const clearHistory = async () => {
    if (!window.confirm('Clear all finished download records?')) return
    try { const { removed } = await api.clearFinished(); await refresh(); setMessage(`Cleared ${removed} finished records.`) }
    catch (error) { setError(errorMessage(error)) }
  }
  const selectDirectory = async () => {
    try {
      const selected = await host.selectDirectory()
      if (selected && draftRef.current) { changeSettings({ ...draftRef.current, OutputDir: selected }); await settingsSaver.flush() }
    }
    catch (error) { setError(errorMessage(error)) }
  }
  const notices = <>{error && <div role="alert" className="notice notice-error"><CircleAlert aria-hidden="true" />{error}</div>}{message && <div role="status" className="notice notice-success"><Check aria-hidden="true" />{message}</div>}</>
  const taskActions = { onCancel: cancel, onRetry: retry, onDelete: removeRecord }

  return <main className="canvas">
    <div inert={panel ? true : undefined}>
    <div className="canvas-grid" aria-hidden="true" />
    <a className="wordmark" href="#/downloads" aria-label="MediaNinja home"><img className="brand-mark" src="/logo.png" width="44" height="44" alt="" /><span>MEDIA<br />NINJA<span className="brand-period">.</span></span></a>
    <div className="canvas-tools">
      <span className={`connection ${connected ? 'is-connected' : ''}`} role="status">{connected ? <Wifi /> : <WifiOff />}<span>{connected ? 'ENGINE ONLINE' : loading ? 'CONNECTING' : 'RECONNECTING'}</span></span>
      <Button variant="outline" onClick={() => navigate('history')} aria-label="Open session history"><History /><span>History</span><span className="tool-count">{history.length.toString().padStart(2, '0')}</span></Button>
      <Button variant="outline" onClick={() => navigate('settings')} aria-label="Open settings"><Settings2 /><span>Settings</span></Button>
    </div>
    <div className="workspace-heading"><div className="workspace-pages"><a href="#/downloads" aria-current={page === 'downloads' ? 'page' : undefined}>PAGE DOWNLOAD</a><a href="#/batch" aria-current={page === 'batch' ? 'page' : undefined}>BATCH</a></div><h1 className="sr-only">{page === 'batch' ? 'Batch downloads' : 'Page downloads'}</h1><span>{active.length} active / {completed} completed / {failed.length} failed</span></div>
    {!panel && notices}
    <div className="workbench">
      <section className="download-station" aria-labelledby="capture-title">
        {page === 'batch' ? <BatchPage sites={batchSites} site={batchSite} onSiteChange={setBatchSite} config={config} target={target} onTargetChange={setTarget} onSubmit={submit} busy={busy} connected={connected} inputRef={targetInput} onSettings={() => navigate('settings')} onParserSettings={() => navigate('settings', 'parsers')} onOpenDirectory={host.kind === 'desktop' && config ? () => void host.openOutputDirectory(config.OutputDir).catch(error => setError(errorMessage(error))) : undefined} /> : <><div className="station-title"><h2 id="capture-title">NEW DOWNLOAD</h2><span>01 ↘</span></div><DownloadForm config={config} target={target} onTargetChange={setTarget} onSubmit={submit} busy={busy} connected={connected} inputRef={targetInput} onSettings={() => navigate('settings')} onOpenDirectory={host.kind === 'desktop' && config ? () => void host.openOutputDirectory(config.OutputDir).catch(error => setError(errorMessage(error))) : undefined} /></>}
        <div className="engine-snapshot"><div className="snapshot-heading"><h3>ENGINE SETTINGS</h3><button onClick={() => navigate('settings')} aria-label="Edit engine settings"><Settings2 /></button></div><dl><div><dt>Concurrent files</dt><dd>{config?.Concurrency ?? '—'}</dd></div><div><dt>Maximum attempts</dt><dd>{config?.MaxRetries ?? '—'}</dd></div><div><dt>Retry delay</dt><dd>{config ? `${config.RetryDelay}s` : '—'}</dd></div><div><dt>Connection</dt><dd>{config ? config.ProxyURL ? 'Proxy' : 'Direct' : '—'}</dd></div></dl></div>
      </section>
      <section className="queue-area" aria-labelledby="queue-title">
        <div className="queue-heading"><h2 id="queue-title"><span className="live-dot" /> {page === 'batch' ? 'BATCH DOWNLOADS' : 'PAGE DOWNLOADS'} <span className="queue-count">{pageTasks.length.toString().padStart(2, '0')}</span></h2><span className="queue-order">QUEUED TASKS RUN IN ORDER</span></div>
        <div className="queue-filters" role="group" aria-label="Filter downloads">{([{ id: 'active', label: 'Active', count: active.length }, { id: 'all', label: 'This session', count: pageTasks.length }, { id: 'failed', label: 'Failed', count: failed.length }] as const).map(item => <button key={item.id} aria-pressed={filter === item.id} onClick={() => setFilter(item.id)}>{item.label}<span>{item.count.toString().padStart(2, '0')}</span></button>)}</div>
        <div className="queue-content">{loading ? <div className="empty-queue"><LoaderCircle className="animate-spin" /><h3>Connecting to the engine…</h3></div>
          : filter === 'all' ? <>
            {active.length > 0 && <div className="task-list">{active.map(task => <TaskCard key={task.id} task={task} {...taskActions} />)}</div>}
            {finished.length > 0 && <section className="finished-group">
              <button className="finished-toggle" aria-expanded={showFinished} onClick={() => setShowFinished(value => !value)}>FINISHED <span className="queue-count">{finished.length.toString().padStart(2, '0')}</span>{showFinished ? <ChevronUp /> : <ChevronDown />}</button>
              {showFinished && <div className="task-list">{finished.map(task => <TaskCard key={task.id} task={task} collapsible defaultCollapsed {...taskActions} />)}</div>}
            </section>}
            {pageTasks.length === 0 && <EmptyQueue filter={filter} page={page} inputRef={targetInput} />}
          </>
          : visibleTasks.length ? <div className="task-list">{visibleTasks.map(task => <TaskCard key={task.id} task={task} {...taskActions} />)}</div>
          : <EmptyQueue filter={filter} page={page} inputRef={targetInput} />}
        </div>
        <div className="queue-bottom"><span>{completed} completed · {failed.length} failed</span><button onClick={() => navigate('history')}>SESSION HISTORY <History /></button></div>
      </section>
    </div>
    <footer className="canvas-footer"><span>{host.kind === 'desktop' ? 'CLOSE TO TRAY · QUIT TO STOP DOWNLOADS' : 'DOWNLOADS CONTINUE AFTER CLOSING THIS PAGE'}</span><span>SETTINGS LAST FOR THIS SESSION · HISTORY PERSISTS ON THIS MACHINE</span></footer>
    </div>
    {panel && <FloatingPanel title={panel === 'settings' ? 'SETTINGS' : 'DOWNLOAD HISTORY'} label={panel === 'settings' ? 'CONTROL ROOM / SETTINGS' : 'ARCHIVE / SAVED RECORDS'} onClose={() => navigate(null)}>
      {notices}
      {panel === 'settings' ? draft ? <SettingsForm tab={settingsTab} onTabChange={tab => navigate('settings', tab)} draft={draft} onChange={changeSettings} saveState={saveState} onCommit={() => void settingsSaver.flush()} onRetry={() => { changeSettings(draft); void settingsSaver.flush() }} canSelectDirectory={host.canSelectDirectory} onSelectDirectory={selectDirectory} onOpenDirectory={host.kind === 'desktop' && config ? () => void host.openOutputDirectory(config.OutputDir).catch(error => setError(errorMessage(error))) : undefined} sites={batchSites} /> : <p className="panel-note">Settings will appear when the engine connects.</p> : <><p className="panel-note">Completed, failed and canceled downloads. Records persist on this machine and survive engine restarts.</p>{history.length ? <><div className="history-heading"><span>{history.length.toString().padStart(2, '0')} RECORDS</span><button className="text-action" onClick={clearHistory}>CLEAR ALL</button></div><div className="task-list">{history.map(task => <TaskCard key={task.id} task={task} collapsible defaultCollapsed {...taskActions} />)}</div></> : <div className="history-empty"><span aria-hidden="true">[ 00 ]</span><h3>NO FINISHED DOWNLOADS</h3><p>Completed, failed and canceled tasks will appear here.</p></div>}</>}
    </FloatingPanel>}
  </main>
}

function EmptyQueue({ filter, page, inputRef }: { filter: 'active' | 'all' | 'failed'; page: 'downloads' | 'batch'; inputRef: RefObject<HTMLInputElement | null> }) {
  return <div className="empty-queue"><span className="empty-symbol" aria-hidden="true">[ {filter === 'failed' ? '✓' : '↓'} ]</span><h3>{filter === 'failed' ? 'NO FAILED DOWNLOADS' : filter === 'all' ? 'NO DOWNLOADS YET' : 'NO ACTIVE DOWNLOADS'}</h3><p>{filter === 'failed' ? 'Failed tasks will appear here.' : page === 'batch' ? 'Add names or keywords to start a batch.' : 'Add a page URL to start a download.'}</p>{filter !== 'failed' && <Button variant="outline" size="sm" onClick={() => inputRef.current?.focus()}>ADD A DOWNLOAD <ArrowUpRight /></Button>}</div>
}

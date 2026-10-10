import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { ArrowDownToLine, Check, CircleAlert, FolderOpen, HardDriveDownload, History, LoaderCircle, Plus, Settings2, ShieldCheck, Square, Wifi, WifiOff } from 'lucide-react'
import { isActive, MediaNinjaClient, type Config, type Task } from '@medianinja/api-client'
import { createHost } from '@medianinja/platform'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

const api = new MediaNinjaClient()
const host = createHost()
const stateLabels = { queued: '等待执行', running: '下载中', canceling: '正在取消', completed: '已完成', failed: '下载失败', canceled: '已取消' }
const errorMessage = (error: unknown) => error instanceof Error ? error.message : typeof error === 'string' ? error : '操作未完成，请重试'
function Field({ label, children, hint }: { label: string; children: ReactNode; hint?: string }) {
  return <label className="grid gap-2 text-sm"><span className="font-medium">{label}</span>{children}{hint && <span className="text-xs leading-5 text-muted-foreground">{hint}</span>}</label>
}
function TaskCard({ task, onCancel }: { task: Task; onCancel: (id: string) => Promise<void> }) {
  const [canceling, setCanceling] = useState(false)
  const title = task.request.url ?? task.request.names?.join('、') ?? '下载任务'
  return <article className="rounded-xl border bg-white p-5">
    <div className="flex items-start justify-between gap-4">
      <div className="min-w-0"><div className="mb-2 flex items-center gap-2"><span className={cn('rounded-md px-2 py-1 text-xs', task.state === 'completed' ? 'bg-emerald-50 text-emerald-700' : task.state === 'failed' ? 'bg-red-50 text-red-700' : 'bg-muted text-muted-foreground')}>{stateLabels[task.state]}</span><span className="text-xs text-muted-foreground">{new Date(task.created).toLocaleString()}</span></div><h3 className="break-all text-sm font-medium">{title}</h3></div>
      {['queued', 'running'].includes(task.state) && <Button variant="outline" size="sm" disabled={canceling} onClick={async () => { setCanceling(true); try { await onCancel(task.id) } finally { setCanceling(false) } }}><Square />取消</Button>}
    </div>
    {Object.values(task.progress).map(progress => <div key={progress.file} className="mt-4"><div className="mb-2 flex justify-between gap-3 text-xs text-muted-foreground"><span className="truncate" title={progress.file}>{progress.file?.split(/[\\/]/).pop()}</span><span className="shrink-0">{progress.unit === 'segments' ? `${progress.completed ?? 0} / ${progress.total ?? '?'} 片段` : `${((progress.completed ?? 0) / 1048576).toFixed(1)}${progress.total ? ` / ${(progress.total / 1048576).toFixed(1)}` : ''} MB`}</span></div>{!!progress.total && <progress className="h-1.5 w-full" max={progress.total} value={progress.completed ?? 0} />}</div>)}
    {!!task.result.files?.length && <p className="mt-3 text-xs text-muted-foreground">成功 {task.result.files.filter(file => !file.error).length} / {task.result.files.length} 个文件</p>}
    {task.last_event?.message && isActive(task) && <p className="mt-3 break-all text-xs text-muted-foreground">{task.last_event.message}</p>}
    {task.error && <p className="mt-3 break-all rounded-lg bg-red-50 p-3 text-xs leading-5 text-red-700">{task.error}</p>}
  </article>
}
export function App() {
  const [page, setPage] = useState(location.hash.slice(2) || 'downloads')
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
  const previous = useRef<Map<string, Task['state']>>(new Map())
  useEffect(() => { const update = () => setPage(location.hash.slice(2) || 'downloads'); window.addEventListener('hashchange', update); return () => window.removeEventListener('hashchange', update) }, [])
  useEffect(() => {
    let alive = true
    let unsubscribe = () => {}
    const update = (next: Task[]) => {
      if (!alive) return
      for (const task of next) {
        const old = previous.current.get(task.id)
        if (old && ['queued', 'running', 'canceling'].includes(old) && ['completed', 'failed'].includes(task.state)) void host.notify('MediaNinja', task.state === 'completed' ? '下载任务已完成' : '下载任务失败，请查看任务详情').catch(() => {})
      }
      previous.current = new Map(next.map(task => [task.id, task.state])); setTasks(next)
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
  const submit = async (event: FormEvent) => {
    event.preventDefault(); if (!config || busy) return
    setBusy(true); setError(''); setMessage('')
    try {
      const input = target.trim()
      await api.submit(mode === 'url' ? { url: input, options: config } : { names: input.split(/[,，]/).map(name => name.trim()).filter(Boolean), options: config })
      setTasks(await api.tasks()); setTarget(''); setMessage('任务已加入下载队列')
    } catch (error) { setError(errorMessage(error)) } finally { setBusy(false) }
  }
  const cancel = async (id: string) => { try { await api.cancel(id); setTasks(await api.tasks()) } catch (error) { setError(errorMessage(error)) } }
  const save = async (event: FormEvent) => {
    event.preventDefault(); if (!draft) return
    setBusy(true); setError(''); setMessage('')
    try { const saved = await api.saveConfig(draft); setConfig(saved); setDraft(saved); setMessage('已保存，后续任务将使用新配置') } catch (error) { setError(errorMessage(error)) } finally { setBusy(false) }
  }
  const selectDirectory = async () => { try { const selected = await host.selectDirectory(); if (selected) setDraft(current => current && { ...current, OutputDir: selected }) } catch (error) { setError(errorMessage(error)) } }
  return <div className="min-h-screen lg:grid lg:grid-cols-[224px_1fr]">
    <aside className="border-b bg-[#eef1e9] px-5 py-6 lg:fixed lg:inset-y-0 lg:w-56 lg:border-r lg:border-b-0">
      <a href="#/downloads" className="flex items-center gap-3"><span className="flex size-9 items-center justify-center rounded-xl bg-primary text-white"><ArrowDownToLine className="size-5" /></span><span className="font-semibold tracking-tight">MediaNinja<span className="block text-[10px] font-normal tracking-[0.2em] text-muted-foreground">YOUR MEDIA, COLLECTED</span></span></a>
      <p className="mt-9 mb-3 text-[10px] tracking-[0.15em] text-muted-foreground">工作空间</p>
      <nav className="flex gap-1 lg:flex-col">{[{id:'downloads',label:'下载任务',icon:HardDriveDownload},{id:'history',label:'会话历史',icon:History},{id:'settings',label:'配置',icon:Settings2}].map(item => <a key={item.id} href={`#/${item.id}`} className={cn('flex items-center gap-3 rounded-lg px-3 py-3 text-sm',page === item.id ? 'bg-white font-medium shadow-sm' : 'text-muted-foreground hover:bg-white/50')}><item.icon className="size-4" />{item.label}{item.id === 'downloads' && active.length > 0 && <span className="ml-auto rounded bg-accent px-1.5 text-xs">{active.length}</span>}</a>)}</nav>
      <div className="mt-6 rounded-lg border border-[#dbe3d5] p-3 text-xs leading-5 text-muted-foreground lg:absolute lg:right-5 lg:bottom-6 lg:left-5"><div className="mb-1 flex items-center gap-2 font-medium text-foreground"><ShieldCheck className="size-3.5" />{host.kind === 'desktop' ? 'Windows 桌面端' : '本地 Web'}</div>{host.kind === 'desktop' ? '关闭窗口后保持后台下载，可从托盘退出。' : '文件保存在运行服务的电脑上，关闭浏览器不会停止下载。'}</div>
    </aside>
    <main className="min-w-0 lg:col-start-2">
      <header className="flex items-center justify-between border-b bg-white/60 px-6 py-4 lg:px-10"><span className="text-xs text-muted-foreground">工作空间 / {page === 'settings' ? '配置' : page === 'history' ? '会话历史' : '下载任务'}</span><span className="flex items-center gap-2 text-xs text-muted-foreground">{connected ? <Wifi className="size-3.5 text-emerald-600" /> : <WifiOff className="size-3.5" />}{connected ? '服务已连接' : loading ? '正在连接' : '等待重连'}</span></header>
      <div className="mx-auto max-w-6xl px-6 py-8 lg:px-10 lg:py-10">
        <div className="mb-8 flex items-start justify-between gap-4"><div><p className="mb-2 text-xs tracking-[0.12em] text-muted-foreground">{page === 'settings' ? 'PREFERENCES' : page === 'history' ? 'SESSION HISTORY' : 'DOWNLOAD MANAGER'}</p><h1 className="text-3xl font-semibold tracking-tight">{page === 'settings' ? '让下载按你的习惯运行。' : page === 'history' ? '查看本次运行的下载记录。' : '收集媒体，从一个链接开始。'}</h1><p className="mt-3 text-sm text-muted-foreground">{page === 'settings' ? '配置只影响新任务，正在执行的任务保持原有设置。' : page === 'history' ? '任务记录暂存于内存，重启服务后清空。' : '解析页面、批量下载，在后台完成收集。'}</p></div>{host.kind === 'desktop' && config && <Button variant="outline" onClick={() => void host.openOutputDirectory(config.OutputDir).catch(error => setError(errorMessage(error)))}><FolderOpen />打开目录</Button>}</div>
        {error && <div role="alert" className="mb-5 flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700"><CircleAlert className="mt-0.5 size-4 shrink-0" />{error}</div>}
        {message && <div role="status" className="mb-5 flex items-center gap-2 rounded-lg bg-emerald-50 p-3 text-sm text-emerald-700"><Check className="size-4" />{message}</div>}
        {loading ? <div className="flex items-center gap-3 py-16 text-muted-foreground"><LoaderCircle className="size-5 animate-spin" />正在连接下载引擎…</div> : page === 'settings' && draft ? <Card><CardHeader><CardTitle>下载配置</CardTitle><p className="mt-2 text-xs text-muted-foreground">当前版本配置用于本次运行，重启后恢复启动参数。</p></CardHeader><CardContent><form onSubmit={save} className="grid gap-6">
          <Field label="输出目录" hint={host.kind === 'browser' ? '填写服务所在机器的目录，而不是远程浏览器的本地目录。' : '使用目录选择器选择本机下载位置。'}><div className="flex gap-2"><Input required value={draft.OutputDir} onChange={event => setDraft({...draft,OutputDir:event.target.value})} />{host.canSelectDirectory && <Button type="button" variant="outline" onClick={() => void selectDirectory()}><FolderOpen />选择目录</Button>}</div></Field>
          <div className="grid gap-5 sm:grid-cols-3">{[{key:'Concurrency',label:'下载并发数',min:1,max:64},{key:'MaxRetries',label:'最大尝试次数',min:1,max:20},{key:'RetryDelay',label:'重试间隔（秒）',min:0,max:300}].map(field => <Field key={field.key} label={field.label}><Input required type="number" min={field.min} max={field.max} value={draft[field.key as keyof Config]} onChange={event => setDraft({...draft,[field.key]:Number(event.target.value)})} /></Field>)}</div>
          <Field label="HTTP 代理" hint="留空为直连。示例：http://127.0.0.1:7890"><Input value={draft.ProxyURL} placeholder="http://127.0.0.1:7890" onChange={event => setDraft({...draft,ProxyURL:event.target.value})} /></Field>
          <div className="grid gap-5 sm:grid-cols-2"><Field label="Rule34 列表页数"><Input required type="number" min={1} value={draft.MaxPages} onChange={event => setDraft({...draft,MaxPages:Number(event.target.value)})} /></Field><Field label="Rule34 视频上限" hint="0 表示不限制。"><Input required type="number" min={0} value={draft.MaxVideos} onChange={event => setDraft({...draft,MaxVideos:Number(event.target.value)})} /></Field></div><div><Button disabled={busy}>{busy ? <LoaderCircle className="animate-spin" /> : <Check />}保存配置</Button></div>
        </form></CardContent></Card> : page === 'history' ? <div className="grid gap-4">{history.length ? history.map(task => <TaskCard key={task.id} task={task} onCancel={cancel} />) : <Empty text="还没有完成的任务" detail="已完成、失败和取消的任务会显示在这里。" />}</div> : <>
          <div className="mb-6 grid gap-4 sm:grid-cols-3">{[{label:'进行中的任务',value:active.length},{label:'已完成任务',value:tasks.filter(task => task.state === 'completed').length},{label:'需要关注',value:tasks.filter(task => task.state === 'failed').length}].map(stat => <div key={stat.label} className="rounded-xl border bg-white px-5 py-4"><p className="text-xs text-muted-foreground">{stat.label}</p><p className="mt-2 text-2xl font-semibold">{stat.value.toString().padStart(2,'0')}</p></div>)}</div>
          <Card className="mb-8"><CardHeader><CardTitle className="flex items-center gap-2"><Plus className="size-4" />新建下载</CardTitle></CardHeader><CardContent><form onSubmit={submit} className="grid gap-5"><div className="flex gap-2">{(['url','names'] as const).map(value => <Button key={value} type="button" variant={mode === value ? 'default' : 'outline'} size="sm" onClick={() => setMode(value)}>{value === 'url' ? '页面链接' : 'Rule34 作者 / 关键词'}</Button>)}</div><Field label={mode === 'url' ? '目标链接' : '作者名或关键词'}><Input required type={mode === 'url' ? 'url' : 'text'} value={target} onChange={event => setTarget(event.target.value)} placeholder={mode === 'url' ? '粘贴视频页、图片页或列表页链接' : '多个名字使用逗号分隔'} /></Field><div className="flex flex-wrap items-center justify-between gap-4"><p className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground"><FolderOpen className="size-4 shrink-0" /><span className="break-all">{config?.OutputDir ?? '尚未连接服务'}</span></p><Button disabled={busy || !config || !connected}>{busy ? <LoaderCircle className="animate-spin" /> : <ArrowDownToLine />}加入下载队列</Button></div></form></CardContent></Card>
          <div className="mb-4 flex items-center justify-between"><h2 className="font-semibold">正在进行 <span className="ml-2 text-xs font-normal text-muted-foreground">{active.length} 个任务</span></h2><a href="#/history" className="text-xs text-muted-foreground hover:text-primary">查看会话历史 →</a></div><div className="grid gap-4">{active.length ? active.map(task => <TaskCard key={task.id} task={task} onCancel={cancel} />) : <Empty text="下载队列空闲" detail="添加一个链接，任务会自动开始执行。" />}</div>
        </>}
        <p className="mt-8 text-xs leading-5 text-muted-foreground">MediaNinja · {host.kind === 'desktop' ? '窗口关闭后隐藏到托盘；退出应用将停止任务。' : '关闭页面后下载继续；停止服务将取消任务。'}</p>
      </div>
    </main>
  </div>
}
function Empty({text,detail}:{text:string;detail:string}){return <div className="rounded-xl border border-dashed bg-white/40 px-6 py-12 text-center"><HardDriveDownload className="mx-auto mb-4 size-7 text-[#9baa99]" /><p className="text-sm font-medium">{text}</p><p className="mt-2 text-xs text-muted-foreground">{detail}</p></div>}

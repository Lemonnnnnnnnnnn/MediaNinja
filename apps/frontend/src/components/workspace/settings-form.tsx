import type { ReactNode } from 'react'
import { Check, CircleAlert, FolderOpen, LoaderCircle } from 'lucide-react'
import type { BatchSiteInfo, Config } from '@medianinja/api-client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs } from '@/components/ui/tabs'
import type { SaveState } from '@/lib/autosave'

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <label className="settings-field"><span>{label}</span>{children}{hint && <small>{hint}</small>}</label>
}
export function SettingsForm({ tab, onTabChange, draft, onChange, saveState, onCommit, onRetry, canSelectDirectory, onSelectDirectory, onOpenDirectory, sites }: {
  tab: 'general' | 'parsers'; onTabChange: (tab: 'general' | 'parsers') => void
  draft: Config; onChange: (config: Config) => void; saveState: SaveState; onCommit: () => void; onRetry: () => void
  canSelectDirectory: boolean; onSelectDirectory: () => Promise<void>; onOpenDirectory?: () => void
  sites: BatchSiteInfo[]
}) {
  const number = (key: 'Concurrency' | 'MaxRetries' | 'RetryDelay' | 'MaxPages' | 'MaxVideos', value: string) => onChange({ ...draft, [key]: value === '' ? NaN : Number(value) })
  const numberValue = (value: number) => Number.isFinite(value) ? value : ''
  return <form noValidate onSubmit={event => { event.preventDefault(); onCommit() }} onBlur={event => { if (event.target.tagName === 'INPUT') onCommit() }} className="settings-form">
    <p className="panel-note">Changes save when you leave a field or press Enter, and apply to new downloads. Existing tasks keep their settings. Settings persist on this machine and survive engine restarts.</p>
    <div className={`settings-status settings-status-${saveState.status}`} role={saveState.status === 'error' ? 'alert' : 'status'}>
      {saveState.status === 'saving' ? <LoaderCircle className="animate-spin" /> : saveState.status === 'saved' || saveState.status === 'editing' ? <Check /> : <CircleAlert />}
      <span>{saveState.status === 'editing' ? 'CHANGES APPLY WHEN YOU LEAVE THIS FIELD' : saveState.status === 'saving' ? 'SAVING CHANGES…' : saveState.status === 'saved' ? 'ALL CHANGES APPLIED' : saveState.status === 'invalid' ? saveState.message : `Changes could not be saved. ${saveState.message}`}</span>
      {saveState.status === 'error' && <Button type="button" variant="outline" size="sm" onClick={onRetry}>Retry</Button>}
    </div>
    <Tabs id="settings" label="Settings categories" items={[{ id: 'general', label: 'GENERAL' }, { id: 'parsers', label: 'PARSERS' }]} value={tab} onChange={onTabChange} />
    {tab === 'general' && <div id="settings-panel-general" role="tabpanel" aria-labelledby="settings-tab-general" tabIndex={0}>
    <section className="settings-unit" aria-labelledby="storage-title"><div className="unit-heading"><span>01 / STORAGE</span><h3 id="storage-title">DOWNLOAD LOCATION</h3></div>
      <Field label="OUTPUT DIRECTORY" hint="A folder on the machine running MediaNinja."><div className="directory-field"><Input required value={draft.OutputDir} onChange={event => onChange({ ...draft, OutputDir: event.target.value })} />{canSelectDirectory && <Button type="button" variant="outline" onClick={() => void onSelectDirectory()}><FolderOpen />Browse</Button>}</div></Field>
      {onOpenDirectory && <button className="text-action" type="button" onClick={onOpenDirectory}>OPEN SAVED DIRECTORY ↗</button>}
    </section>
    <section className="settings-unit" aria-labelledby="engine-title"><div className="unit-heading"><span>02 / DOWNLOAD ENGINE</span><h3 id="engine-title">CONNECTION & RETRIES</h3></div>
      <div className="settings-columns">{([{ key: 'Concurrency', label: 'CONCURRENT DOWNLOADS', min: 1, max: 64 }, { key: 'MaxRetries', label: 'MAXIMUM ATTEMPTS', min: 1, max: 20 }, { key: 'RetryDelay', label: 'RETRY DELAY / SECONDS', min: 0, max: 300 }] as const).map(field => <Field key={field.key} label={field.label}><Input required type="number" min={field.min} max={field.max} value={numberValue(draft[field.key])} onChange={event => number(field.key, event.target.value)} /></Field>)}</div>
      <Field label="HTTP PROXY" hint="Leave blank for a direct connection."><Input value={draft.ProxyURL} placeholder="http://127.0.0.1:7890" onChange={event => onChange({ ...draft, ProxyURL: event.target.value })} /></Field>
    </section>
    </div>}
    {tab === 'parsers' && <div id="settings-panel-parsers" role="tabpanel" aria-labelledby="settings-tab-parsers" tabIndex={0}>
    <section className="settings-unit parser-unit" aria-labelledby="parser-title"><div className="unit-heading"><span>BATCH / PARSER OPTIONS</span><h3 id="parser-title">BATCH SOURCES</h3></div>
      {sites.map(site => <div className="parser-card" key={site.id}><div className="parser-card-heading"><h4>{site.label}</h4><span>BATCH SOURCE</span></div><p>{site.summary}</p><div className="settings-columns two-columns"><Field label="LIST PAGES"><Input required type="number" min={1} value={numberValue(draft.MaxPages)} onChange={event => number('MaxPages', event.target.value)} /></Field><Field label="ITEM LIMIT" hint="Per name. 0 means unlimited."><Input required type="number" min={0} value={numberValue(draft.MaxVideos)} onChange={event => number('MaxVideos', event.target.value)} /></Field></div></div>)}
      {!sites.length && <div className="parser-card"><p>No batch sources are registered.</p></div>}
      <div className="parser-defaults"><span>SINGLE PAGE PARSERS / BUILT-IN DEFAULTS</span><p>Telegraph · DDYS · NTDM · Yingshi · Pornhub · XifanACG</p><small>These parsers currently have no custom settings.</small></div>
    </section>
    </div>}
  </form>
}

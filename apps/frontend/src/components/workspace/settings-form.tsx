import type { FormEvent, ReactNode } from 'react'
import { Check, FolderOpen, LoaderCircle } from 'lucide-react'
import type { Config } from '@medianinja/api-client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs } from '@/components/ui/tabs'

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <label className="settings-field"><span>{label}</span>{children}{hint && <small>{hint}</small>}</label>
}
export function SettingsForm({ tab, onTabChange, draft, onChange, onSave, busy, canSelectDirectory, onSelectDirectory, onOpenDirectory }: {
  tab: 'general' | 'parsers'; onTabChange: (tab: 'general' | 'parsers') => void
  draft: Config; onChange: (config: Config) => void; onSave: (event: FormEvent) => void; busy: boolean
  canSelectDirectory: boolean; onSelectDirectory: () => Promise<void>; onOpenDirectory?: () => void
}) {
  const number = (key: 'Concurrency' | 'MaxRetries' | 'RetryDelay' | 'MaxPages' | 'MaxVideos', value: string) => onChange({ ...draft, [key]: Number(value) })
  return <form onSubmit={onSave} className="settings-form">
    <p className="panel-note">Changes apply to new downloads. Settings are kept for this session and reset when the engine restarts.</p>
    <Tabs id="settings" label="Settings categories" items={[{ id: 'general', label: 'GENERAL' }, { id: 'parsers', label: 'PARSERS' }]} value={tab} onChange={onTabChange} />
    {tab === 'general' && <div id="settings-panel-general" role="tabpanel" aria-labelledby="settings-tab-general" tabIndex={0}>
    <section className="settings-unit" aria-labelledby="storage-title"><div className="unit-heading"><span>01 / STORAGE</span><h3 id="storage-title">DOWNLOAD LOCATION</h3></div>
      <Field label="OUTPUT DIRECTORY" hint="A folder on the machine running MediaNinja."><div className="directory-field"><Input required value={draft.OutputDir} onChange={event => onChange({ ...draft, OutputDir: event.target.value })} />{canSelectDirectory && <Button type="button" variant="outline" onClick={() => void onSelectDirectory()}><FolderOpen />Browse</Button>}</div></Field>
      {onOpenDirectory && <button className="text-action" type="button" onClick={onOpenDirectory}>OPEN SAVED DIRECTORY ↗</button>}
    </section>
    <section className="settings-unit" aria-labelledby="engine-title"><div className="unit-heading"><span>02 / DOWNLOAD ENGINE</span><h3 id="engine-title">CONNECTION & RETRIES</h3></div>
      <div className="settings-columns">{([{ key: 'Concurrency', label: 'CONCURRENT DOWNLOADS', min: 1, max: 64 }, { key: 'MaxRetries', label: 'MAXIMUM ATTEMPTS', min: 1, max: 20 }, { key: 'RetryDelay', label: 'RETRY DELAY / SECONDS', min: 0, max: 300 }] as const).map(field => <Field key={field.key} label={field.label}><Input required type="number" min={field.min} max={field.max} value={draft[field.key]} onChange={event => number(field.key, event.target.value)} /></Field>)}</div>
      <Field label="HTTP PROXY" hint="Leave blank for a direct connection."><Input value={draft.ProxyURL} placeholder="http://127.0.0.1:7890" onChange={event => onChange({ ...draft, ProxyURL: event.target.value })} /></Field>
    </section>
    </div>}
    {tab === 'parsers' && <div id="settings-panel-parsers" role="tabpanel" aria-labelledby="settings-tab-parsers" tabIndex={0}>
    <section className="settings-unit parser-unit" aria-labelledby="parser-title"><div className="unit-heading"><span>PARSERS / CUSTOM OPTIONS</span><h3 id="parser-title">PARSER SETTINGS</h3></div>
      <div className="parser-card"><div className="parser-card-heading"><h4>RULE34 VIDEO</h4><span>CUSTOM OPTIONS</span></div><p>Limits for creator pages and keyword batches.</p><div className="settings-columns two-columns"><Field label="LIST PAGES"><Input required type="number" min={1} value={draft.MaxPages} onChange={event => number('MaxPages', event.target.value)} /></Field><Field label="VIDEO LIMIT" hint="Per creator or keyword. 0 means unlimited."><Input required type="number" min={0} value={draft.MaxVideos} onChange={event => number('MaxVideos', event.target.value)} /></Field></div></div>
      <div className="parser-defaults"><span>OTHER PARSERS / BUILT-IN DEFAULTS</span><p>Telegraph · DDYS · NTDM · Yingshi · Pornhub · XifanACG</p><small>These parsers currently have no custom settings.</small></div>
    </section>
    </div>}
    <div className="settings-save"><span>SAVES ALL TABS · EXISTING TASKS KEEP THEIR SETTINGS.</span><Button disabled={busy}>{busy ? <LoaderCircle className="animate-spin" /> : <Check />}{busy ? 'SAVING…' : 'SAVE SETTINGS'}</Button></div>
  </form>
}

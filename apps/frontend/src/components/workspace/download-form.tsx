import { ArrowDownToLine, ArrowUpRight, FolderOpen, LoaderCircle } from 'lucide-react'
import type { Config } from '@medianinja/api-client'
import type { FormEvent, RefObject } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export interface DownloadFormProps {
  config: Config | null
  target: string
  onTargetChange: (value: string) => void
  onSubmit: (event: FormEvent) => void
  busy: boolean
  connected: boolean
  inputRef: RefObject<HTMLInputElement | null>
  onSettings: () => void
  onOpenDirectory?: () => void
}

export function DownloadForm({ config, target, onTargetChange, onSubmit, busy, connected, inputRef, onSettings, onOpenDirectory, batch = false }: DownloadFormProps & { batch?: boolean }) {
  return <form className="capture-box" onSubmit={onSubmit}>
    <label className="target-label" htmlFor="download-target">{batch ? 'NAMES / KEYWORDS' : 'PAGE URL'}</label>
    <Input ref={inputRef} id="download-target" required type={batch ? 'text' : 'url'} value={target} onChange={event => onTargetChange(event.target.value)} placeholder={batch ? 'creator, keyword…' : 'https://…'} />
    <p className="input-hint">{batch ? 'Separate names or keywords with commas.' : 'A video, image or supported media page.'}</p>
    <Button className="capture-submit" disabled={busy || !config || !connected}>{busy ? <LoaderCircle className="animate-spin" /> : <ArrowDownToLine />}<span>{busy ? 'ADDING…' : batch ? 'QUEUE BATCH' : 'QUEUE DOWNLOAD'}</span></Button>
    <div className="destination-heading"><label htmlFor="output-summary">OUTPUT DIRECTORY</label><button type="button" onClick={onSettings}>EDIT <ArrowUpRight /></button></div>
    <div className="output-summary" id="output-summary"><FolderOpen aria-hidden="true" /><span title={config?.OutputDir}>{config?.OutputDir ?? 'Connecting to the engine…'}</span></div>
    {onOpenDirectory && <button className="text-action" type="button" onClick={onOpenDirectory}>OPEN DIRECTORY ↗</button>}
  </form>
}

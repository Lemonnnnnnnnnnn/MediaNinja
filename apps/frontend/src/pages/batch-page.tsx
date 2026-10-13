import { DownloadForm, type DownloadFormProps } from '@/components/workspace/download-form'
import type { BatchSiteInfo } from '@medianinja/api-client'

export interface BatchPageProps extends DownloadFormProps {
  sites: BatchSiteInfo[]
  site: string
  onSiteChange: (site: string) => void
  onParserSettings: () => void
}

export function BatchPage({ sites, site, onSiteChange, onParserSettings, config, ...form }: BatchPageProps) {
  const selected = sites.find(item => item.id === site) ?? sites[0]
  return <>
    <div className="station-title"><h2 id="capture-title">BATCH / SOURCES</h2><span>02 ↘</span></div>
    {sites.length > 1
      ? <div className="source-picker" role="radiogroup" aria-label="Batch source">{sites.map(item => <button key={item.id} role="radio" aria-checked={item.id === selected?.id} onClick={() => onSiteChange(item.id)}>{item.label}</button>)}</div>
      : sites.length === 1 && <p className="source-label">SOURCE / {selected.label}</p>}
    <DownloadForm {...form} config={config} batch />
    <div className="batch-options"><div className="snapshot-heading"><h3>BATCH LIMITS</h3><button onClick={onParserSettings}>EDIT ↗</button></div>
      <dl><div><dt>List pages per name</dt><dd>{config?.MaxPages ?? '—'}</dd></div><div><dt>Items per name</dt><dd>{config ? config.MaxVideos || 'Unlimited' : '—'}</dd></div></dl>
      <p>{selected?.summary}</p>
    </div>
  </>
}

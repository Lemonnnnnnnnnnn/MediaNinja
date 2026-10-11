import { DownloadForm, type DownloadFormProps } from '@/components/workspace/download-form'

export function BatchPage(props: DownloadFormProps & { onParserSettings: () => void }) {
  return <>
    <div className="station-title"><h2 id="capture-title">RULE34 / BATCH</h2><span>02 ↘</span></div>
    <DownloadForm {...props} batch />
    <div className="batch-options"><div className="snapshot-heading"><h3>RULE34 LIMITS</h3><button onClick={props.onParserSettings}>EDIT ↗</button></div>
      <dl><div><dt>List pages per name</dt><dd>{props.config?.MaxPages ?? '—'}</dd></div><div><dt>Videos per name</dt><dd>{props.config ? props.config.MaxVideos || 'Unlimited' : '—'}</dd></div></dl>
      <p>Creator pages are tried first. Missing creators fall back to keyword search.</p>
    </div>
  </>
}

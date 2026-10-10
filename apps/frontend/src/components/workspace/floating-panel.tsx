import { useEffect, useRef, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function FloatingPanel({ title, label, onClose, children }: { title: string; label: string; onClose: () => void; children: ReactNode }) {
  const panel = useRef<HTMLDivElement>(null)
  const close = useRef(onClose)
  close.current = onClose
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    panel.current?.focus()
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); close.current() }
      if (event.key !== 'Tab') return
      const elements = Array.from(panel.current?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), a[href], [tabindex="0"]') ?? []).filter(element => element.getClientRects().length > 0)
      const first = elements[0], last = elements.at(-1)
      if (!first) { event.preventDefault(); panel.current?.focus(); return }
      if (event.shiftKey && (document.activeElement === first || document.activeElement === panel.current)) { event.preventDefault(); last?.focus() }
      else if (!event.shiftKey && (document.activeElement === last || document.activeElement === panel.current)) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', keydown)
    return () => { document.body.style.overflow = overflow; document.removeEventListener('keydown', keydown); previous?.focus() }
  }, [])
  return <div className="panel-backdrop" onClick={event => { if (event.target === event.currentTarget) onClose() }}>
    <div className="floating-panel" role="dialog" aria-modal="true" aria-labelledby="panel-title" tabIndex={-1} ref={panel}>
      <div className="panel-titlebar"><span>{label}</span><Button variant="ghost" size="icon" onClick={onClose} aria-label="Close panel"><X /></Button></div>
      <div className="panel-body"><h2 id="panel-title">{title}</h2>{children}</div>
      <div className="panel-bottom"><span>YOUR WORKSPACE. YOUR PARAMETERS.</span><span>ESC TO CLOSE ↗</span></div>
    </div>
  </div>
}

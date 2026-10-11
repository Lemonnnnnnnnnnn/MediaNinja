import type { KeyboardEvent } from 'react'

export function Tabs<T extends string>({ id, label, items, value, onChange }: {
  id: string; label: string; items: readonly { id: T; label: string }[]; value: T; onChange: (value: T) => void
}) {
  const keydown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    let next: number
    switch (event.key) {
      case 'ArrowRight': next = (index + 1) % items.length; break
      case 'ArrowLeft': next = (index + items.length - 1) % items.length; break
      case 'Home': next = 0; break
      case 'End': next = items.length - 1; break
      default: return
    }
    event.preventDefault()
    onChange(items[next].id)
    document.getElementById(`${id}-tab-${items[next].id}`)?.focus()
  }
  return <div className="settings-tabs" role="tablist" aria-label={label}>{items.map((item, index) => <button key={item.id} id={`${id}-tab-${item.id}`} type="button" role="tab" aria-selected={value === item.id} aria-controls={`${id}-panel-${item.id}`} tabIndex={value === item.id ? 0 : -1} onClick={() => onChange(item.id)} onKeyDown={event => keydown(event, index)}>{item.label}</button>)}</div>
}

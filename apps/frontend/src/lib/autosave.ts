export type SaveState = { status: 'saved' | 'editing' | 'saving' | 'invalid' | 'error'; message?: string }

// Serialize writes so a slow response cannot overwrite a newer edit.
export function createAutosave<T>(save: (value: T) => Promise<T>, onSaved: (value: T, current: boolean) => void, onState: (state: SaveState) => void) {
  let pending: { value: T; revision: number } | null = null
  let staged: { value: T; revision: number; issue?: string } | null = null
  let revision = 0
  let running: Promise<void> | undefined
  let state: SaveState = { status: 'saved' }
  const update = (next: SaveState) => { state = next; onState(next) }
  const flush = async () => {
    if (staged) {
      if (staged.issue) {
        update({ status: 'invalid', message: staged.issue })
      } else {
        pending = { value: staged.value, revision: staged.revision }
        staged = null
        update({ status: 'saving' })
      }
    }
    if (!running && pending) {
      running = (async () => {
        while (pending) {
          const write = pending
          pending = null
          try {
            const saved = await save(write.value)
            const current = write.revision === revision
            onSaved(saved, current)
            if (current) update({ status: 'saved' })
          } catch (error) {
            if (write.revision === revision) update({ status: 'error', message: error instanceof Error ? error.message : String(error) })
          }
        }
      })()
      try { await running } finally { running = undefined }
    } else if (running) await running
    return state
  }
  const stage = (value: T, issue?: string) => {
    revision++
    pending = null
    staged = { value, revision, issue }
    update({ status: 'editing' })
  }
  return {
    stage,
    flush,
    cancel() { pending = null; staged = null; revision++ },
  }
}

export interface HostCapabilities {
  kind: 'browser' | 'desktop'
  canSelectDirectory: boolean
  selectDirectory(): Promise<string | null>
  openOutputDirectory(path: string): Promise<void>
  notify(title: string, message: string): Promise<void>
}
const browserHost: HostCapabilities = {
  kind: 'browser', canSelectDirectory: false,
  selectDirectory: async () => null,
  openOutputDirectory: async () => { throw new Error('请在文件管理器中打开输出目录') },
  notify: async () => {},
}
export function createHost(): HostCapabilities {
  if (!('__TAURI_INTERNALS__' in window)) return browserHost
  // Tauri-specific code is kept behind this boundary; UI components use only the contract.
  const invoke = async <T>(command: string, args?: Record<string, unknown>): Promise<T> => {
    const { invoke } = await import('@tauri-apps/api/core')
    return invoke<T>(command, args)
  }
  return {
    kind: 'desktop', canSelectDirectory: true,
    selectDirectory: () => invoke<string | null>('select_output_directory'),
    openOutputDirectory: (path) => invoke<void>('open_output_directory', { path }),
    notify: (title, message) => invoke<void>('notify_task', { title, message }),
  }
}

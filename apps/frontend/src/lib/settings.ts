import type { Config } from '@medianinja/api-client'

export function settingsIssue(config: Config): string | undefined {
  if (!config.OutputDir.trim()) return 'Enter a download directory.'
  for (const [key, min, max, label] of [
    ['Concurrency', 1, 64, 'Concurrent downloads'],
    ['MaxRetries', 1, 20, 'Maximum attempts'],
    ['RetryDelay', 0, 300, 'Retry delay'],
    ['MaxPages', 1, Number.MAX_SAFE_INTEGER, 'List pages'],
    ['MaxVideos', 0, Number.MAX_SAFE_INTEGER, 'Video limit'],
  ] as const) {
    const value = config[key]
    if (!Number.isSafeInteger(value) || value < min || value > max) return `${label} must be a whole number ${max === Number.MAX_SAFE_INTEGER ? `at least ${min}` : `between ${min} and ${max}`}.`
  }
  if (config.ProxyURL) {
    try {
      const proxy = new URL(config.ProxyURL)
      if (proxy.protocol !== 'http:' || !proxy.hostname) throw new Error()
    } catch { return 'Enter a valid HTTP proxy URL, or leave it blank.' }
  }
}

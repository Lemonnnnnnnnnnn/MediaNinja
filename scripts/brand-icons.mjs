import { spawnSync } from 'node:child_process'
import { copyFileSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
if (!process.env.npm_execpath) throw new Error('Run through pnpm brand:icons')
const output = resolve(root, '.cache/brand-icons')
const result = spawnSync(process.execPath, [process.env.npm_execpath, '--filter', '@medianinja/desktop', 'exec', 'tauri', 'icon', resolve(root, 'assets/brand/medianinja-logo-v2.png'), '--output', output], { cwd: root, stdio: 'inherit' })
if (result.error) throw result.error
if (result.status !== 0) process.exit(result.status ?? 1)
for (const [source, destination] of [
  ['32x32.png', 'apps/desktop/src-tauri/icons/32x32.png'],
  ['128x128.png', 'apps/desktop/src-tauri/icons/128x128.png'],
  ['128x128@2x.png', 'apps/desktop/src-tauri/icons/icon.png'],
  ['icon.ico', 'apps/desktop/src-tauri/icons/icon.ico'],
  ['128x128@2x.png', 'apps/frontend/public/logo.png'],
  ['icon.ico', 'apps/frontend/public/favicon.ico'],
]) {
  const path = resolve(root, destination)
  mkdirSync(dirname(path), { recursive: true })
  copyFileSync(resolve(output, source), path)
}
console.log('Desktop and web icons updated from assets/brand/medianinja-logo-v2.png')

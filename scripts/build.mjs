import { spawnSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, resolve, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
function run(command, args, env = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', env: { ...process.env, ...env } })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}
if (!process.env.npm_execpath) throw new Error('Run this script through pnpm build:web or pnpm build:sidecar')
run(process.execPath, [process.env.npm_execpath, '--filter', '@medianinja/frontend', 'build'])
const ui = resolve(root, 'server/ui')
const assets = resolve(ui, 'assets')
if (relative(root, assets).startsWith('..') || dirname(assets) !== ui) throw new Error('Invalid generated asset directory')
if (existsSync(assets)) rmSync(assets, { recursive: true, force: true })
cpSync(resolve(root, 'apps/frontend/dist'), ui, { recursive: true })
if (process.argv[2] === 'sidecar') {
  if (process.platform !== 'win32') throw new Error('Desktop release currently supports Windows only')
  const rust = spawnSync('rustc', ['-vV'], { encoding: 'utf8' })
  if (rust.status !== 0) throw new Error('Rust toolchain is required for desktop builds')
  const target = rust.stdout.match(/^host: (.+)$/m)?.[1]
  if (!target?.endsWith('pc-windows-msvc')) throw new Error('Use a Windows MSVC Rust target')
  const output = resolve(root, `apps/desktop/src-tauri/binaries/medianinja-core-${target}.exe`)
  mkdirSync(dirname(output), { recursive: true })
  run('go', ['build', '-o', output, '.'], { CGO_ENABLED: '0', GOOS: 'windows', GOARCH: target.startsWith('aarch64') ? 'arm64' : 'amd64' })
  console.log(`Go sidecar ready: ${output}`)
} else {
  run('go', ['build', '-o', resolve(root, 'MediaNinja.exe'), '.'])
}

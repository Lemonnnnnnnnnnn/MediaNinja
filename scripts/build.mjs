import { spawnSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, resolve, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { checkVersion } from './version.mjs'
import { prepareFFmpeg } from './ffmpeg.mjs'
import { removeGenerated } from './lib/release.mjs'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
function run(command, args, env = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', env: { ...process.env, ...env } })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}
if (!process.env.npm_execpath) throw new Error('Run this script through pnpm build:web or pnpm build:sidecar')
const version = checkVersion()
const commitResult = spawnSync('git', ['rev-parse', '--short=12', 'HEAD'], { cwd: root, encoding: 'utf8' })
if (commitResult.status !== 0) throw new Error('Git checkout is required to identify the build')
const commit = commitResult.stdout.trim()
const date = new Date(process.env.SOURCE_DATE_EPOCH ? Number(process.env.SOURCE_DATE_EPOCH) * 1000 : Date.now()).toISOString()
const ldflags = `-s -w -X MediaNinja/internal/buildinfo.Version=${version} -X MediaNinja/internal/buildinfo.Commit=${commit} -X MediaNinja/internal/buildinfo.Date=${date}`
const ffmpeg = process.platform === 'win32' ? await prepareFFmpeg() : null
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
  if (target !== 'x86_64-pc-windows-msvc') throw new Error('Bundled desktop release currently supports Windows x64 only')
  const output = resolve(root, `apps/desktop/src-tauri/binaries/medianinja-core-${target}.exe`)
  mkdirSync(dirname(output), { recursive: true })
  cpSync(resolve(ffmpeg, 'ffmpeg.exe'), resolve(dirname(output), `ffmpeg-${target}.exe`))
  cpSync(resolve(ffmpeg, 'ffmpeg.exe'), resolve(dirname(output), 'ffmpeg.exe'))
  const notices = resolve(root, 'apps/desktop/src-tauri/resources/ffmpeg')
  removeGenerated(notices, resolve(root, 'apps/desktop/src-tauri/resources'))
  mkdirSync(notices, { recursive: true })
  cpSync(ffmpeg, notices, { recursive: true })
  rmSync(resolve(notices, 'ffmpeg.exe'))
  run('go', ['build', '-ldflags', ldflags, '-o', output, '.'], { CGO_ENABLED: '0', GOOS: 'windows', GOARCH: 'amd64' })
  console.log(`Go sidecar ready: ${output}`)
} else {
  run('go', ['build', '-ldflags', ldflags, '-o', resolve(root, 'MediaNinja.exe'), '.'], { CGO_ENABLED: '0' })
  if (ffmpeg) {
    cpSync(resolve(ffmpeg, 'ffmpeg.exe'), resolve(root, 'ffmpeg.exe'))
    const notices = resolve(root, 'licenses/ffmpeg')
    removeGenerated(notices, resolve(root, 'licenses'))
    mkdirSync(notices, { recursive: true })
    cpSync(ffmpeg, notices, { recursive: true })
    rmSync(resolve(notices, 'ffmpeg.exe'))
  }
}

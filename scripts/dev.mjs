import { spawn, spawnSync } from 'node:child_process'
import { mkdirSync, existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createInterface } from 'node:readline'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const executable = resolve(root, '.cache', process.platform === 'win32' ? 'medianinja-dev.exe' : 'medianinja-dev')
mkdirSync(dirname(executable), { recursive: true })
const build = spawnSync('go', ['build', '-o', executable, '.'], { cwd: root, stdio: 'inherit' })
if (build.status !== 0) process.exit(build.status ?? 1)
const bundledFFmpeg = resolve(root, '.cache/ffmpeg/windows-x64/ffmpeg.exe')
const backend = spawn(executable, ['serve', '--desktop', '--listen', '127.0.0.1:0'], { cwd: root, stdio: ['pipe', 'pipe', 'inherit'], env: { ...process.env, ...(process.platform === 'win32' && existsSync(bundledFFmpeg) && !process.env.MEDIANINJA_FFMPEG ? { MEDIANINJA_FFMPEG: bundledFFmpeg } : {}) } })
let frontend
let stopping = false
function stop(code = 0) {
  if (stopping) return
  stopping = true
  backend.stdin.end(); frontend?.kill()
  if (backend.exitCode !== null || backend.signalCode !== null) process.exit(code)
  const timeout = setTimeout(() => { backend.kill(); process.exit(code) }, 7000)
  timeout.unref()
  backend.once('exit', () => process.exit(code))
}
backend.on('error', error => { console.error(error); stop(1) })
backend.on('exit', code => { if (!stopping) { console.error('Go service stopped'); stop(code ?? 1) } })
const lines = createInterface({ input: backend.stdout })
lines.once('line', line => {
  const info = JSON.parse(line)
  if (info.protocol !== 1) throw new Error('Unsupported Go service protocol')
  console.log(`Go API: ${info.url}`)
  frontend = spawn(process.execPath, [resolve(root, 'apps/frontend/node_modules/vite/bin/vite.js'), '--host', '127.0.0.1'], { cwd: resolve(root, 'apps/frontend'), stdio: 'inherit', env: { ...process.env, MEDIANINJA_BACKEND_URL: info.url } })
  frontend.on('error', error => { console.error(error); stop(1) })
  frontend.on('exit', code => { if (!stopping) stop(code ?? 0) })
})
process.on('SIGINT', () => stop()); process.on('SIGTERM', () => stop())

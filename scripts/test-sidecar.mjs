import assert from 'node:assert/strict'
import { spawn, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createInterface } from 'node:readline'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const host = spawnSync('rustc', ['-vV'], { encoding: 'utf8' }).stdout.match(/^host: (.+)$/m)?.[1]
const executable = resolve(root, `apps/desktop/src-tauri/binaries/medianinja-core-${host}.exe`)
const directory = mkdtempSync(resolve(tmpdir(), 'medianinja-sidecar-'))
const child = spawn(executable, ['serve', '--desktop', '--output', directory], { stdio: ['pipe', 'pipe', 'inherit'] })
const lines = createInterface({ input: child.stdout })
let exited = false
child.once('exit', () => { exited = true })
try {
  const ready = await Promise.race([
    new Promise((resolve, reject) => { lines.once('line', line => resolve(JSON.parse(line))); child.once('error', reject); child.once('exit', () => reject(new Error('Backend exited before ready'))) }),
    new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('Startup timeout')), 10000); timer.unref() }),
  ])
  assert.equal(ready.protocol, 1)
  const session = await fetch(`${ready.url}/api/session`)
  assert.equal(session.status, 200)
  const cookie = session.headers.get('set-cookie').split(';')[0]
  const tasks = await fetch(`${ready.url}/api/tasks`, { headers: { Cookie: cookie } })
  assert.deepEqual(await tasks.json(), [])
  const ui = await fetch(ready.url)
  assert.equal(ui.status, 200)
  assert.match(await ui.text(), /assets\/index-/)
  const exit = new Promise((resolve, reject) => {
    child.once('exit', code => code === 0 ? resolve() : reject(new Error(`Exit code ${code}`)))
    const timer = setTimeout(() => reject(new Error('Backend did not stop after parent pipe closed')), 7000); timer.unref()
  })
  child.stdin.end()
  await exit
  console.log('Sidecar startup, React assets, API session and parent-lifetime shutdown passed.')
} finally {
  if (!exited) { child.kill(); await new Promise(resolve => child.once('exit', resolve)) }
  lines.close()
  // This temporary directory is created and owned by this test, never an input path.
  const temporaryRoot = resolve(tmpdir())
  if (dirname(directory) !== temporaryRoot || !directory.startsWith(resolve(temporaryRoot, 'medianinja-sidecar-'))) throw new Error('Invalid test directory')
  rmSync(directory, { recursive: true, force: true })
}

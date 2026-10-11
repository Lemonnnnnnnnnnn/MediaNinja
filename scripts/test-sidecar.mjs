import assert from 'node:assert/strict'
import { spawn, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, readFileSync } from 'node:fs'
import { createServer } from 'node:http'
import { setTimeout as delay } from 'node:timers/promises'
import { tmpdir } from 'node:os'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createInterface } from 'node:readline'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const host = spawnSync('rustc', ['-vV'], { encoding: 'utf8' }).stdout.match(/^host: (.+)$/m)?.[1]
const executable = resolve(root, `apps/desktop/src-tauri/binaries/medianinja-core-${host}.exe`)
const directory = mkdtempSync(resolve(tmpdir(), 'medianinja-sidecar-'))
const segment = readFileSync(resolve(root, 'scripts/fixtures/remux.ts'))
let fixtureURL
const fixture = createServer((request, response) => {
  if (request.url === '/telegra.ph') response.end(`<header><h1>Fixture</h1></header><img src="${fixtureURL}/video.m3u8">`)
  else if (request.url === '/video.m3u8') response.end(`#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\n${fixtureURL}/segment.ts\n#EXT-X-ENDLIST\n`)
  else if (request.url === '/segment.ts') response.end(segment)
  else { response.writeHead(404); response.end() }
})
await new Promise((resolve, reject) => { fixture.once('error', reject); fixture.listen(0, '127.0.0.1', resolve) })
fixtureURL = `http://127.0.0.1:${fixture.address().port}`
// Verify a real task without any system FFmpeg or explicit override available.
const child = spawn(executable, ['serve', '--desktop', '--output', directory], { stdio: ['pipe', 'pipe', 'inherit'], env: { ...process.env, PATH: '', MEDIANINJA_FFMPEG: '' } })
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
  const submit = await fetch(`${ready.url}/api/tasks`, { method: 'POST', headers: { Cookie: cookie, 'Content-Type': 'application/json' }, body: JSON.stringify({ url: `${fixtureURL}/telegra.ph` }) })
  assert.equal(submit.status, 202)
  const queued = await submit.json()
  const deadline = Date.now() + 15000
  let task
  do {
    task = await (await fetch(`${ready.url}/api/tasks/${queued.id}`, { headers: { Cookie: cookie } })).json()
    if (['completed', 'failed', 'canceled'].includes(task.state)) break
    await delay(100)
  } while (Date.now() < deadline)
  assert.equal(task.state, 'completed', task.error ?? 'Bundled FFmpeg task did not complete')
  const mp4 = readFileSync(resolve(directory, 'Fixture/images/001.m3u8.mp4'))
  assert.equal(mp4.toString('ascii', 4, 8), 'ftyp')
  const exit = new Promise((resolve, reject) => {
    child.once('exit', code => code === 0 ? resolve() : reject(new Error(`Exit code ${code}`)))
    const timer = setTimeout(() => reject(new Error('Backend did not stop after parent pipe closed')), 7000); timer.unref()
  })
  child.stdin.end()
  await exit
  console.log('Sidecar startup, React assets, API session, real bundled-FFmpeg task without PATH, and parent-lifetime shutdown passed.')
} finally {
  if (!exited) { child.kill(); await new Promise(resolve => child.once('exit', resolve)) }
  lines.close()
  await new Promise(resolve => fixture.close(resolve))
  // This temporary directory is created and owned by this test, never an input path.
  const temporaryRoot = resolve(tmpdir())
  if (dirname(directory) !== temporaryRoot || !directory.startsWith(resolve(temporaryRoot, 'medianinja-sidecar-'))) throw new Error('Invalid test directory')
  rmSync(directory, { recursive: true, force: true })
}

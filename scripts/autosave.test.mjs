import test from 'node:test'
import assert from 'node:assert/strict'
import { createAutosave } from '../apps/frontend/src/lib/autosave.ts'
import { settingsIssue } from '../apps/frontend/src/lib/settings.ts'

const config = { URL: '', ProxyURL: '', Concurrency: 5, OutputDir: 'downloads', MaxRetries: 3, RetryDelay: 5, MaxPages: 1, MaxVideos: 0 }

test('typing only stages changes; blur or download flush saves the latest value', async () => {
  const writes = []
  const saver = createAutosave(async value => { writes.push(value); return value }, () => {}, () => {})
  saver.stage(1); saver.stage(2)
  assert.deepEqual(writes, [])
  assert.equal((await saver.flush()).status, 'saved')
  assert.deepEqual(writes, [2])
  saver.stage(3)
  await saver.flush()
  assert.deepEqual(writes, [2, 3])
})

test('slow saves are serialized and do not replace newer input', async () => {
  const writes = []; const responses = []; const releases = []
  const saver = createAutosave(value => new Promise(resolve => { writes.push(value); releases.push(() => resolve(value)) }), (value, current) => responses.push([value, current]), () => {})
  saver.stage('old')
  const saving = saver.flush()
  saver.stage('intermediate'); saver.stage('latest')
  const latestSaving = saver.flush()
  assert.deepEqual(writes, ['old'])
  releases.shift()()
  await Promise.resolve(); await Promise.resolve()
  assert.deepEqual(writes, ['old', 'latest'])
  releases.shift()()
  await Promise.all([saving, latestSaving])
  assert.deepEqual(responses, [['old', false], ['latest', true]])
  saver.cancel()
})

test('invalid input cancels pending writes, including while an older save is in flight', async () => {
  let resolveWrite; const states = []
  const saver = createAutosave(value => new Promise(resolve => { resolveWrite = () => resolve(value) }), () => {}, state => states.push(state.status))
  saver.stage('old')
  const saving = saver.flush()
  saver.stage('', 'Directory is required')
  const invalid = saver.flush()
  resolveWrite()
  assert.equal((await saving).status, 'invalid')
  assert.equal((await invalid).status, 'invalid')
  assert.equal(states.at(-1), 'invalid')
  saver.cancel()
})

test('failed writes stay visibly unsaved and can be retried', async () => {
  let failing = true
  const saver = createAutosave(async value => { if (failing) throw new Error('Offline'); return value }, () => {}, () => {})
  saver.stage(config)
  assert.deepEqual(await saver.flush(), { status: 'error', message: 'Offline' })
  failing = false
  saver.stage(config)
  assert.equal((await saver.flush()).status, 'saved')
})

test('typing during an older save does not commit the new draft until blur', async () => {
  const writes = []; const applied = []
  let finish
  const saver = createAutosave(value => { writes.push(value); return value === 'old' ? new Promise(resolve => { finish = () => resolve(value) }) : Promise.resolve(value) }, (value, current) => applied.push([value, current]), () => {})
  saver.stage('old')
  const saving = saver.flush()
  saver.stage('still typing')
  finish()
  assert.equal((await saving).status, 'editing')
  assert.deepEqual(writes, ['old'])
  assert.deepEqual(applied, [['old', false]])
  assert.equal((await saver.flush()).status, 'saved')
  assert.deepEqual(writes, ['old', 'still typing'])
})

test('settings validation permits clearing numbers without sending NaN, rejects out-of-range and unsupported proxies', () => {
  assert.equal(settingsIssue(config), undefined)
  for (const changes of [{ Concurrency: NaN }, { MaxRetries: 0 }, { RetryDelay: 301 }, { MaxPages: 1.5 }, { MaxVideos: -1 }, { OutputDir: '' }, { ProxyURL: 'http://' }, { ProxyURL: 'socks5://localhost:7890' }]) assert.ok(settingsIssue({ ...config, ...changes }))
  assert.equal(settingsIssue({ ...config, ProxyURL: 'http://127.0.0.1:7890', MaxVideos: 0, RetryDelay: 0 }), undefined)
})

import assert from 'node:assert/strict'
import test from 'node:test'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import { verifyDownload, lockKey } from './ffmpeg.mjs'
import { removeGenerated } from './lib/release.mjs'

test('dependency corruption is rejected before extracting or executing', () => {
  const directory = mkdtempSync(resolve(tmpdir(), 'medianinja-download-test-'))
  try {
    const path = resolve(directory, 'archive')
    writeFileSync(path, 'original')
    const expected = createHash('sha256').update('original').digest('hex')
    verifyDownload(path, expected)
    writeFileSync(path, 'corrupted')
    assert.throws(() => verifyDownload(path, expected), /SHA256 mismatch/)
    assert.throws(() => verifyDownload(path, 'invalid'), /SHA256 mismatch/)
  } finally { removeGenerated(directory, tmpdir()) }
})
test('changing the source, compiler or configuration invalidates the build key', () => {
  const original = { source: '1', toolchain: '2', configure: ['--disable-gpl'] }
  for (const update of [{ source: '2' }, { toolchain: '3' }, { configure: ['--enable-gpl'] }]) assert.notEqual(lockKey(original), lockKey({ ...original, ...update }))
})
test('generated cleanup cannot delete a parent directory', () => {
  assert.throws(() => removeGenerated(tmpdir(), tmpdir()), /Unsafe/)
  assert.throws(() => removeGenerated(resolve(tmpdir(), '..'), tmpdir()), /Unsafe/)
})

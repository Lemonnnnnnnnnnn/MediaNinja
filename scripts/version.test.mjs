import assert from 'node:assert/strict'
import test from 'node:test'
import { validVersion, updateCargo, checkVersion } from './version.mjs'

test('reject unsafe versions and non-SemVer leading zeros', () => {
  for (const version of ['1.2.3', '0.1.0-beta.1']) assert.equal(validVersion(version), true)
  for (const version of [undefined, 'v1.2.3', '01.2.3', '1.2', '1.2.3-beta.01', '1.2.3;echo bad', '1.2.3\n']) assert.equal(validVersion(version), false)
})
test('update only the product package, preserving dependency versions and CRLF', () => {
  const cargo = '[package]\r\nname = "medianinja-desktop"\r\nversion = "0.1.0"\r\n\r\n[dependencies]\r\nserde = "1"\r\n'
  assert.equal(updateCargo(cargo, '0.2.0'), cargo.replace('version = "0.1.0"', 'version = "0.2.0"'))
  const lock = '[[package]]\nname = "serde"\nversion = "1.0.0"\n\n[[package]]\nname = "medianinja-desktop"\nversion = "0.1.0"\n'
  assert.equal(updateCargo(lock, '0.2.0', true), lock.replace('version = "0.1.0"', 'version = "0.2.0"'))
  assert.throws(() => updateCargo('[package]\nname = "bad"', '0.2.0'), /not found/)
})
test('release tag must match the single product version', () => {
  const version = checkVersion()
  assert.equal(checkVersion(`v${version}`), version)
  assert.throws(() => checkVersion('v999.999.999'), /must be/)
})

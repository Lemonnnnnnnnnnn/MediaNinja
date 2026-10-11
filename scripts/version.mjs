import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { root, readJSON } from './lib/release.mjs'

export function validVersion(value) {
  return /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/.test(value)
    && !(value.split('-').slice(1).join('-').split('.').some(part => /^0\d+$/.test(part)))
}
export function updateCargo(text, version, lock = false) {
  const pattern = lock
    ? /(\[\[package\]\]\r?\nname = "medianinja-desktop"\r?\nversion = ")[^"]+("\r?\n)/
    : /(\[package\][\s\S]*?\r?\nversion = ")[^"]+("\r?\n)/
  if (!pattern.test(text)) throw new Error('Desktop package version not found')
  return text.replace(pattern, (_, before, after) => `${before}${version}${after}`)
}
export function checkVersion(tag) {
  const version = readJSON('package.json').version
  if (!validVersion(version)) throw new Error(`Invalid root version: ${version}`)
  if (readJSON('apps/desktop/src-tauri/tauri.conf.json').version !== version) throw new Error('Tauri version differs from package.json')
  for (const [path, lock] of [['apps/desktop/src-tauri/Cargo.toml', false], ['apps/desktop/src-tauri/Cargo.lock', true]]) {
    const text = readFileSync(resolve(root, path), 'utf8')
    if (updateCargo(text, version, lock) !== text) throw new Error(`${path} version differs from package.json`)
  }
  if (tag && tag !== `v${version}`) throw new Error(`Tag ${tag} must be v${version}`)
  return version
}
function main() {
  const [mode, value, extra] = process.argv.slice(2)
  if (mode === 'set') {
    if (!validVersion(value) || extra) throw new Error('Usage: pnpm version:set <major.minor.patch[-prerelease]>')
    // Prepare every change first, so malformed files cannot leave partially updated versions.
    const changes = [
      ['package.json', JSON.stringify({ ...readJSON('package.json'), version: value }, null, 2) + '\n'],
      ['apps/desktop/src-tauri/tauri.conf.json', JSON.stringify({ ...readJSON('apps/desktop/src-tauri/tauri.conf.json'), version: value }, null, 2) + '\n'],
      ...[['apps/desktop/src-tauri/Cargo.toml', false], ['apps/desktop/src-tauri/Cargo.lock', true]].map(([path, lock]) => [path, updateCargo(readFileSync(resolve(root, path), 'utf8'), value, lock)]),
    ]
    for (const [path, contents] of changes) writeFileSync(resolve(root, path), contents)
    console.log(`Version set to ${checkVersion()}. No commit, tag or push was performed.`)
  } else if (mode === 'check' && (!value || value === '--tag') && !process.argv[5]) {
    if (value && !extra) throw new Error('--tag requires a value')
    console.log(`Version ${checkVersion(value ? extra : undefined)} is consistent.`)
  } else throw new Error('Usage: pnpm version:check [--tag vX.Y.Z] or pnpm version:set X.Y.Z')
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()

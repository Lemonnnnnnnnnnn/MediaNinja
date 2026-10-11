import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { readFileSync, rmSync } from 'node:fs'
import { dirname, resolve, relative, isAbsolute } from 'node:path'
import { fileURLToPath } from 'node:url'

export const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
export const readJSON = path => JSON.parse(readFileSync(resolve(root, path), 'utf8'))
export const sha256 = path => createHash('sha256').update(readFileSync(path)).digest('hex')
export function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', windowsHide: true, ...options })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} failed (${result.status})`)
  return result
}
export function pnpm(args) {
  if (!process.env.npm_execpath) throw new Error('Run this command through pnpm')
  run(process.execPath, [process.env.npm_execpath, ...args])
}
export function removeGenerated(path, parent) {
  const child = relative(resolve(parent), resolve(path))
  if (!child || child.startsWith('..') || isAbsolute(child)) throw new Error(`Unsafe generated path: ${path}`)
  rmSync(path, { recursive: true, force: true })
}

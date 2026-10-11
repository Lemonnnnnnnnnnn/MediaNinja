import { createHash } from 'node:crypto'
import { availableParallelism } from 'node:os'
import { basename, dirname, resolve } from 'node:path'
import { readFileSync, writeFileSync, existsSync, mkdirSync, copyFileSync, readdirSync, openSync, closeSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { root, readJSON, sha256, run, removeGenerated } from './lib/release.mjs'

export const ffmpegDirectory = resolve(root, '.cache/ffmpeg/windows-x64')
export function lockKey(lock) { return createHash('sha256').update(JSON.stringify(lock)).digest('hex') }
export function verifyDownload(path, expected) {
  if (!/^[a-f0-9]{64}$/.test(expected) || sha256(path) !== expected) throw new Error(`SHA256 mismatch: ${path}`)
}
async function download(spec, directory) {
  const path = resolve(directory, basename(new URL(spec.url).pathname))
  if (existsSync(path)) { verifyDownload(path, spec.sha256); return path }
  if (!spec.url.startsWith('https://')) throw new Error('Dependency downloads must use HTTPS')
  console.log(`Downloading ${spec.url}`)
  const response = await fetch(spec.url, { signal: AbortSignal.timeout(300_000) })
  if (!response.ok) throw new Error(`Download failed: ${response.status} ${spec.url}`)
  const contents = Buffer.from(await response.arrayBuffer())
  if (createHash('sha256').update(contents).digest('hex') !== spec.sha256) throw new Error(`SHA256 mismatch: ${spec.url}`)
  writeFileSync(path, contents)
  return path
}
function gitBash() {
  const paths = run('where.exe', ['git'], { encoding: 'utf8', stdio: 'pipe' }).stdout.trim().split(/\r?\n/)
  const bash = paths.map(path => resolve(dirname(path), '../bin/bash.exe')).find(existsSync)
  if (!bash) throw new Error('Git for Windows (Git Bash) is required to build bundled FFmpeg')
  return bash
}
export async function prepareFFmpeg() {
  if (process.platform !== 'win32' || process.arch !== 'x64') throw new Error('Bundled FFmpeg currently supports Windows x64 only')
  const lock = readJSON('scripts/ffmpeg.lock.json')
  const key = lockKey({ ...lock, script: sha256(resolve(root, 'scripts/build-ffmpeg.sh')) })
  const downloads = resolve(root, '.cache/release-tools')
  mkdirSync(downloads, { recursive: true })
  const source = await download(lock.source, downloads)
  const compiler = await download(lock.toolchain, downloads)
  const stamp = resolve(ffmpegDirectory, 'manifest.json')
  if (existsSync(stamp)) {
    const saved = JSON.parse(readFileSync(stamp, 'utf8'))
    if (saved.key === key && Object.entries(saved.files).every(([name, hash]) => existsSync(resolve(ffmpegDirectory, name)) && sha256(resolve(ffmpegDirectory, name)) === hash)) {
      console.log(`Bundled FFmpeg ${lock.version} is ready (verified cache).`)
      return ffmpegDirectory
    }
  }
  const tools = resolve(root, `.cache/ffmpeg/toolchain-${lock.toolchain.version}`)
  if (!existsSync(resolve(tools, 'w64devkit/bin/gcc.exe'))) {
    mkdirSync(tools, { recursive: true })
    run(compiler, ['-y', `-o${tools}`])
  }
  const workspace = resolve(root, `.cache/ffmpeg/build-${key.slice(0, 16)}`)
  removeGenerated(workspace, resolve(root, '.cache/ffmpeg'))
  mkdirSync(workspace, { recursive: true })
  run('tar', ['-xf', source, '-C', workspace])
  const extracted = resolve(workspace, `ffmpeg-${lock.version}`)
  console.log(`Building FFmpeg ${lock.version} from pinned official source…`)
  const logPath = resolve(workspace, 'build.log')
  const log = openSync(logPath, 'w')
  console.log(`Compiler output: ${logPath}`)
  try {
    run(gitBash(), [resolve(root, 'scripts/build-ffmpeg.sh'), extracted, resolve(tools, 'w64devkit/bin'), ...lock.configure], {
      env: { ...process.env, MEDIANINJA_BUILD_JOBS: String(Math.min(availableParallelism(), 8)) },
      stdio: ['ignore', log, log],
    })
  } catch (error) {
    console.error(readFileSync(logPath, 'utf8').split('\n').slice(-35).join('\n'))
    throw error
  } finally { closeSync(log) }
  const binary = resolve(extracted, 'ffmpeg.exe')
  const info = run(binary, ['-version'], { encoding: 'utf8', stdio: 'pipe' }).stdout
  if (/--enable-(gpl|nonfree)/.test(info)) throw new Error('Bundled FFmpeg must not enable GPL or nonfree components')
  removeGenerated(ffmpegDirectory, resolve(root, '.cache/ffmpeg'))
  mkdirSync(ffmpegDirectory, { recursive: true })
  copyFileSync(binary, resolve(ffmpegDirectory, 'ffmpeg.exe'))
  for (const name of ['COPYING.LGPLv2.1', 'COPYING.LGPLv3', 'LICENSE.md']) copyFileSync(resolve(extracted, name), resolve(ffmpegDirectory, name))
  // Distribute matching unmodified sources, exact flags and toolchain details together.
  copyFileSync(source, resolve(ffmpegDirectory, basename(source)))
  copyFileSync(resolve(root, 'scripts/ffmpeg.lock.json'), resolve(ffmpegDirectory, 'build.json'))
  copyFileSync(resolve(root, 'scripts/build-ffmpeg.sh'), resolve(ffmpegDirectory, 'build-ffmpeg.sh'))
  writeFileSync(resolve(ffmpegDirectory, 'version.txt'), info)
  writeFileSync(resolve(ffmpegDirectory, 'NOTICE.txt'), `MediaNinja uses FFmpeg ${lock.version} as a separate executable under LGPL-2.1-or-later.\nFFmpeg: https://ffmpeg.org/\nMatching unmodified source archive, license texts, build flags and script are included in this directory.\nNo GPL, nonfree or external codec libraries are enabled. No source modifications were made.\nThis build supports local MPEG-TS to MP4 remuxing; it is not a general-purpose FFmpeg distribution.\nToolchain: w64devkit ${lock.toolchain.version} (build tool only, not redistributed).\nTo rebuild: extract the source archive and run build-ffmpeg.sh with the source directory, w64devkit bin directory, and configure flags from build.json under Git Bash.\n`)
  const files = Object.fromEntries(readdirSync(ffmpegDirectory).map(name => [name, sha256(resolve(ffmpegDirectory, name))]))
  writeFileSync(stamp, JSON.stringify({ key, files }, null, 2) + '\n')
  console.log(`Bundled FFmpeg ready: ${ffmpegDirectory}`)
  return ffmpegDirectory
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await prepareFFmpeg()

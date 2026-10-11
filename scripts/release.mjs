import { cpSync, copyFileSync, mkdirSync, existsSync, writeFileSync, readdirSync } from 'node:fs'
import { resolve, basename } from 'node:path'
import { checkVersion } from './version.mjs'
import { testFFmpeg } from './test-ffmpeg.mjs'
import { root, run, pnpm, sha256, removeGenerated } from './lib/release.mjs'

if (process.platform !== 'win32' || process.arch !== 'x64') throw new Error('Releases currently support Windows x64 only')
const version = checkVersion(process.env.GITHUB_REF_TYPE === 'tag' ? process.env.GITHUB_REF_NAME : undefined)
pnpm(['typecheck'])
pnpm(['test:release'])
run('go', ['test', './...'])
run('go', ['vet', './...'])
pnpm(['build:web'])
testFFmpeg(resolve(root, 'ffmpeg.exe'))
pnpm(['desktop:build'])
pnpm(['test:sidecar'])
const directory = resolve(root, 'release')
const staged = resolve(directory, 'staging')
removeGenerated(directory, root)
mkdirSync(staged, { recursive: true })
copyFileSync(resolve(root, 'MediaNinja.exe'), resolve(staged, 'MediaNinja.exe'))
copyFileSync(resolve(root, 'ffmpeg.exe'), resolve(staged, 'ffmpeg.exe'))
cpSync(resolve(root, 'licenses/ffmpeg'), resolve(staged, 'licenses/ffmpeg'), { recursive: true })
const versionInfo = run(resolve(staged, 'MediaNinja.exe'), ['--version'], { encoding: 'utf8', stdio: 'pipe' }).stdout
if (!versionInfo.includes(` ${version} `)) throw new Error('Go version metadata differs from release version')
writeFileSync(resolve(staged, 'README.txt'), `MediaNinja ${version} — Windows x64\n\nDouble-click MediaNinja.exe to open the local Web UI, or use it from a terminal.\nFFmpeg is included; keep ffmpeg.exe alongside MediaNinja.exe.\nVersion: ${versionInfo.trim()}\n\nFFmpeg is a separate LGPL-2.1-or-later executable. Its licenses, matching source archive and build recipe are included in licenses/ffmpeg.\nSettings and task history currently last for this session only.\n`)
const zip = resolve(directory, `MediaNinja_${version}_windows_x64.zip`)
// These fixed workspace paths are passed as structured argv; no interpolated shell code.
run('tar', ['-a', '-cf', zip, '-C', staged, '.'])
const installer = resolve(root, `apps/desktop/src-tauri/target/release/bundle/nsis/MediaNinja_${version}_x64-setup.exe`)
if (!existsSync(installer)) throw new Error(`Installer missing: ${installer}`)
copyFileSync(installer, resolve(directory, basename(installer)))
// Provide sources directly as a Release asset as well as inside both distributions.
const sourceName = readdirSync(resolve(root, 'licenses/ffmpeg')).find(name => /^ffmpeg-.*\.tar\.xz$/.test(name))
if (!sourceName) throw new Error('Matching FFmpeg source archive is missing')
copyFileSync(resolve(root, 'licenses/ffmpeg', sourceName), resolve(directory, sourceName))
copyFileSync(resolve(root, 'licenses/ffmpeg/build.json'), resolve(directory, 'ffmpeg-build.json'))
copyFileSync(resolve(root, 'scripts/build-ffmpeg.sh'), resolve(directory, 'build-ffmpeg.sh'))
const assets = readdirSync(directory).filter(name => name !== 'staging').sort()
writeFileSync(resolve(directory, 'SHA256SUMS.txt'), assets.map(name => `${sha256(resolve(directory, name))}  ${name}`).join('\n') + '\n')
writeFileSync(resolve(directory, 'release-notes.md'), `## Downloads\n\n- **Desktop:** MediaNinja_${version}_x64-setup.exe\n- **Portable Web + CLI:** MediaNinja_${version}_windows_x64.zip\n\nBoth packages include FFmpeg; no separate FFmpeg installation is needed. Windows x64 only. The desktop installer requires WebView2 and currently is unsigned.\n\nThis software uses the separate [FFmpeg](https://ffmpeg.org/) executable under LGPL-2.1-or-later. Matching source code, license texts and build instructions are included in both packages. The matching FFmpeg source archive and build recipe are also attached to this release.\n\nSettings and task history currently persist only for the running session.\n`)
removeGenerated(staged, directory)
console.log(`Release assets ready in ${directory}. No tag, push or publication was performed.`)

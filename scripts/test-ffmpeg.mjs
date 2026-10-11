import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, writeFileSync, existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { ffmpegDirectory } from './ffmpeg.mjs'
import { run, removeGenerated } from './lib/release.mjs'

export function testFFmpeg(binary = resolve(ffmpegDirectory, 'ffmpeg.exe')) {
  if (!existsSync(binary)) throw new Error('Run pnpm prepare:ffmpeg first')
  const directory = mkdtempSync(resolve(tmpdir(), 'medianinja-ffmpeg-'))
  try {
    const info = run(binary, ['-version'], { encoding: 'utf8', stdio: 'pipe' }).stdout
    assert.match(info, /^ffmpeg version /)
    assert.doesNotMatch(info, /--enable-(gpl|nonfree)/)
    // Generate a real PCM WAV locally, then encode AAC into TS and remux it.
    // This verifies the shipped binary, not a developer's system FFmpeg.
    const samples = 48000
    const wave = Buffer.alloc(44 + samples * 2)
    wave.write('RIFF'); wave.writeUInt32LE(wave.length - 8, 4); wave.write('WAVEfmt ', 8)
    wave.writeUInt32LE(16, 16); wave.writeUInt16LE(1, 20); wave.writeUInt16LE(1, 22)
    wave.writeUInt32LE(48000, 24); wave.writeUInt32LE(96000, 28); wave.writeUInt16LE(2, 32); wave.writeUInt16LE(16, 34)
    wave.write('data', 36); wave.writeUInt32LE(samples * 2, 40)
    for (let index = 0; index < samples; index++) wave.writeInt16LE(Math.round(Math.sin(index * 2 * Math.PI * 440 / 48000) * 8000), 44 + index * 2)
    const input = resolve(directory, 'input.wav'), ts = resolve(directory, 'input.ts'), mp4 = resolve(directory, 'output.mp4')
    writeFileSync(input, wave)
    run(binary, ['-hide_banner', '-loglevel', 'error', '-i', input, '-c:a', 'aac', '-f', 'mpegts', ts])
    run(binary, ['-hide_banner', '-loglevel', 'error', '-i', ts, '-c', 'copy', '-bsf:a', 'aac_adtstoasc', mp4])
    const output = readFileSync(mp4)
    assert.ok(output.length > 1000)
    assert.equal(output.toString('ascii', 4, 8), 'ftyp')
    run('go', ['test', './core/request/downloader', '-run', '^TestBundledFFmpegRemux$', '-count=1'], { env: { ...process.env, MEDIANINJA_FFMPEG: binary } })
    console.log('Bundled FFmpeg: version, license configuration, AAC remux and Go M3U8 → H.264 MP4 pipeline passed.')
  } finally { removeGenerated(directory, tmpdir()) }
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) testFFmpeg()

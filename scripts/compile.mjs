// Compile CADUCEUS to a standalone Bun executable.
//
// Usage:
//   bun scripts/compile.mjs [--target linux-amd64|linux-arm64|darwin-amd64|darwin-arm64]
//
// Set CADUCEUS_COMPILE_OUTDIR to override the default `dist-bin` destination.

import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { spawn } from 'node:child_process'

const targetAliases = {
  'linux-amd64': 'linux-x64',
  'linux-x64': 'linux-x64',
  'linux-arm64': 'linux-arm64',
  'darwin-amd64': 'darwin-x64',
  'darwin-x64': 'darwin-x64',
  'darwin-arm64': 'darwin-arm64',
}

const targetIndex = process.argv.indexOf('--target')
const requestedTarget = targetIndex >= 0
  ? process.argv[targetIndex + 1]
  : defaultTarget()
const compileTarget = targetAliases[requestedTarget]

if (!compileTarget) {
  console.error(`Unsupported compile target: ${requestedTarget || '(missing)'}`)
  console.error(`Allowed targets: ${Object.keys(targetAliases).join(', ')}`)
  process.exit(64)
}

const outdir = process.env.CADUCEUS_COMPILE_OUTDIR || 'dist-bin'
mkdirSync(outdir, { recursive: true })
const outfile = join(outdir, `caduceus-${requestedTarget}`)

process.env.NODE_ENV = 'production'

const proc = spawn('bun', [
  'build',
  '--compile',
  '--target',
  `bun-${compileTarget}`,
  'scripts/entry.mjs',
  '--outfile',
  outfile,
], {
  stdio: 'inherit',
  cwd: process.cwd(),
  env: process.env,
})

await new Promise(/** @param {(value?: void) => void} resolve */ (resolve, reject) => {
  proc.once('error', reject)
  proc.once('exit', (code) => {
    if (code === 0) resolve()
    else reject(new Error(`bun build --compile exited ${code}`))
  })
})

console.log(`Compiled CADUCEUS for ${requestedTarget}`)
console.log(`Binary: ${outfile}`)

function defaultTarget() {
  const platform = process.platform === 'darwin' ? 'darwin' : 'linux'
  const architecture = process.arch === 'arm64' ? 'arm64' : 'amd64'
  return `${platform}-${architecture}`
}

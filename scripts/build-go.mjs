#!/usr/bin/env bun

import { mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'

const root = resolve(import.meta.dir, '..')
const args = process.argv.slice(2)
const outputIndex = args.indexOf('--output')
const defaultName = process.platform === 'win32' ? 'caduceus.exe' : 'caduceus'
const output = resolve(
  root,
  outputIndex >= 0 ? (args[outputIndex + 1] || `dist-bin/${defaultName}`) : `dist-bin/${defaultName}`,
)
const version = clean(process.env.BUILD_VERSION, 'dev')
const commit = clean(process.env.BUILD_COMMIT, 'unknown')
const builtAt = buildTimestamp()
const packagePath = 'github.com/d31ma/caduceus/internal/platform/buildinfo'
const linkerFlags = [
  '-s',
  '-w',
  `-X=${packagePath}.version=${version}`,
  `-X=${packagePath}.commit=${commit}`,
  `-X=${packagePath}.builtAt=${builtAt}`,
].join(' ')

mkdirSync(dirname(output), { recursive: true })
const result = Bun.spawnSync([
  'go',
  'build',
  '-buildvcs=false',
  '-trimpath',
  '-ldflags',
  linkerFlags,
  '-o',
  output,
  './cmd/caduceus',
], {
  cwd: root,
  stdout: 'inherit',
  stderr: 'inherit',
  env: { ...process.env, GOFLAGS: '-buildvcs=false' },
})
if (result.exitCode !== 0) process.exit(result.exitCode || 1)
console.log(output)

/** @param {string | undefined} value @param {string} fallback */
function clean(value, fallback) {
  const normalized = String(value || '').trim()
  if (!normalized) return fallback
  if (/[\u0000-\u001f\u007f\s]/.test(normalized)) {
    console.error('Build metadata must not contain whitespace or control characters.')
    process.exit(2)
  }
  return normalized
}

function buildTimestamp() {
  const epoch = process.env.SOURCE_DATE_EPOCH
  if (!epoch) return clean(process.env.BUILD_TIME, 'unknown')
  if (!/^(?:0|[1-9][0-9]*)$/.test(epoch)) {
    console.error('SOURCE_DATE_EPOCH must be a non-negative integer.')
    process.exit(2)
  }
  return new Date(Number(epoch) * 1000).toISOString()
}

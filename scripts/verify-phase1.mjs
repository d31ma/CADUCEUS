#!/usr/bin/env bun

import { createHash } from 'node:crypto'
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'

const root = resolve(import.meta.dir, '..')
const failures = []

const runtime = readJson('runtime.lock.json')
const config = readJson('contracts/config/v1.schema.json')
const routes = readJson('contracts/http/routes.v1.json')
readJson('contracts/http/system.v1.schema.json')
readJson('contracts/runtime-lock.schema.json')

for (const path of [
  'go.mod',
  'cmd/caduceus/main.go',
  'internal/platform/cli/run.go',
  'internal/platform/config/config.go',
  'internal/platform/httpserver/server.go',
  'internal/platform/preflight/checker.go',
  'internal/system/service.go',
  'test/blackbox/runtime_test.go',
  'docs/PHASE1_RUNTIME.md',
  'docs/PHASE1_EVIDENCE.md',
]) {
  requireFile(path)
}

const projectPlan = readFileSync(resolve(root, 'docs/PROJECT_PLAN.md'), 'utf8')
const evidence = readFileSync(resolve(root, 'docs/PHASE1_EVIDENCE.md'), 'utf8')
if (!projectPlan.includes('### Phase 1: Go Runtime Foundation\n\nStatus: **Complete')) {
  fail('project plan does not mark Phase 1 complete')
}
if (!evidence.includes('Status: complete')) {
  fail('Phase 1 evidence is not marked complete')
}

if (runtime.artifacts?.go?.version !== '1.26.5') {
  fail('runtime.lock.json must pin Go 1.26.5')
}
const goVersion = Bun.spawnSync(['go', 'env', 'GOVERSION'], { cwd: root })
if (goVersion.exitCode !== 0 || goVersion.stdout.toString().trim() !== 'go1.26.5') {
  fail(`active Go toolchain is not go1.26.5: ${goVersion.stderr.toString().trim()}`)
}

for (const key of [
  'HOST',
  'PORT',
  'METRICS_HOST',
  'METRICS_PORT',
  'STATIC_DIR',
  'RUNTIME_LOCK',
  'SHUTDOWN_TIMEOUT',
  'FYLO_BINARY',
  'FYLO_ROOT',
  'FYLO_SCHEMA',
  'SESAME_BINARY',
  'SESAME_DEPLOYMENT',
  'SESAME_ISSUER',
  'INBOUND_WEBHOOK_SECRET',
]) {
  if (!config.properties?.[key]) fail(`configuration contract is missing ${key}`)
}

if (routes.phase1Aliases?.['/system'] !== '/api/v1/system') {
  fail('route contract does not freeze the /system compatibility alias')
}
for (const path of ['/healthz', '/readyz', '/metrics']) {
  if (!(routes.operations?.routes ?? []).some(route => route.path === path)) {
    fail(`operations contract is missing ${path}`)
  }
}

verifyFrontend()

if (failures.length > 0) {
  console.error(`Phase 1 static verification failed (${failures.length}):`)
  for (const message of failures) console.error(`  - ${message}`)
  process.exit(1)
}

console.log('Phase 1 contracts and synchronized frontend are internally consistent.')

function verifyFrontend() {
  const staticRoot = resolve(root, 'build', 'static')
  const manifest = readJson('build/static/.caduceus-bundle.json')
  if (manifest.schemaVersion !== 1 || manifest.source !== 'tachyon') {
    fail('frontend manifest is unsupported')
    return
  }
  const expected = new Map((manifest.files ?? []).map(file => [file.path, file]))
  const actual = []
  walk(staticRoot, actual)
  actual.sort()
  if (actual.length !== expected.size) {
    fail(`frontend manifest lists ${expected.size} files but static directory contains ${actual.length}`)
  }
  const aggregate = createHash('sha256')
  for (const path of actual) {
    const file = expected.get(path)
    if (!file) {
      fail(`frontend file is absent from manifest: ${path}`)
      continue
    }
    const absolute = resolve(staticRoot, path)
    const bytes = statSync(absolute).size
    const sha256 = createHash('sha256').update(readFileSync(absolute)).digest('hex')
    if (bytes !== file.bytes || sha256 !== file.sha256) {
      fail(`frontend file differs from manifest: ${path}`)
    }
    aggregate.update(path)
    aggregate.update('\0')
    aggregate.update(String(bytes))
    aggregate.update('\0')
    aggregate.update(sha256)
    aggregate.update('\n')
  }
  if (aggregate.digest('hex') !== manifest.digest) {
    fail('frontend aggregate digest differs from manifest')
  }
}

function walk(directory, output) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const absolute = join(directory, entry.name)
    if (entry.isDirectory()) {
      walk(absolute, output)
    } else if (entry.isFile() && entry.name !== '.caduceus-bundle.json') {
      output.push(relative(resolve(root, 'build', 'static'), absolute).split('\\').join('/'))
    }
  }
}

function requireFile(path) {
  const absolute = resolve(root, path)
  if (!existsSync(absolute)) fail(`missing required Phase 1 artifact: ${path}`)
  return absolute
}

function readJson(path) {
  const absolute = requireFile(path)
  if (!existsSync(absolute)) return {}
  try {
    return JSON.parse(readFileSync(absolute, 'utf8'))
  } catch (error) {
    fail(`${path} is not valid JSON: ${error instanceof Error ? error.message : String(error)}`)
    return {}
  }
}

function fail(message) {
  failures.push(message)
}

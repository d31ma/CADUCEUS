#!/usr/bin/env bun

import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(import.meta.dir, '..')
const failures = []
const notes = []

function fail(message) {
  failures.push(message)
}

function requireFile(path) {
  const absolute = resolve(root, path)
  if (!existsSync(absolute)) fail(`missing required Phase 0 artifact: ${path}`)
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

function sha256(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

function listFiles(paths, patterns) {
  const result = Bun.spawnSync([
    'rg',
    '--files',
    ...patterns.flatMap(pattern => ['-g', pattern]),
    ...paths,
  ], { cwd: root })
  if (result.exitCode !== 0 && result.exitCode !== 1) {
    fail(`could not inventory ${paths.join(', ')}: ${result.stderr.toString().trim()}`)
    return []
  }
  return result.stdout.toString().trim().split('\n').filter(Boolean)
}

const requiredDocs = [
  'docs/PROJECT_PLAN.md',
  'docs/BASELINE.md',
  'docs/API_V1.md',
  'docs/CONFIGURATION.md',
  'docs/COMPATIBILITY.md',
  'docs/MIGRATION_ROLLBACK.md',
  'docs/THREAT_MODEL.md',
  'docs/adr/0002-typescript-frontend-go-backend.md',
]
requiredDocs.forEach(requireFile)

const runtime = readJson('runtime.lock.json')
readJson('contracts/runtime-lock.schema.json')
readJson('contracts/config/v1.schema.json')
readJson('contracts/errors/v1.json')
const routes = readJson('contracts/http/routes.v1.json')
const parity = readJson('contracts/parity/v1.json')

if (runtime.schemaVersion !== 1) fail('runtime.lock.json schemaVersion must be 1')
for (const name of ['tachyon', 'fylo', 'chex', 'ttid', 'bun']) {
  if (!runtime.artifacts?.[name]) fail(`runtime.lock.json does not lock ${name}`)
}

const lockPath = requireFile(runtime.dependencyLock?.path ?? 'bun.lock')
if (
  existsSync(lockPath)
  && runtime.dependencyLock?.sha256
  && sha256(lockPath) !== runtime.dependencyLock.sha256
) {
  fail('bun.lock digest differs from runtime.lock.json')
}

for (const command of ['bun', 'ty', 'fylo', 'chex', 'ttid']) {
  if (!Bun.which(command)) fail(`required command is not installed: ${command}`)
}

const ty = Bun.spawnSync(['ty', '--version'], { cwd: root })
const tyVersion = ty.stdout.toString().trim()
if (ty.exitCode !== 0) fail(`ty --version failed: ${ty.stderr.toString().trim()}`)
if (tyVersion !== runtime.artifacts?.tachyon?.version) {
  fail(`Tachyon version ${tyVersion || '<empty>'} does not match runtime lock`)
}

const developmentPreview = runtime.artifacts?.fylo?.developmentPreview
if (!developmentPreview?.binary) {
  notes.push('local FYLO preview absent; released-only checks applied')
} else {
  const previewPath = resolve(root, developmentPreview.binary)
  if (!existsSync(previewPath)) {
    notes.push('local FYLO preview absent; released-only checks applied')
  } else if (sha256(previewPath) !== developmentPreview.sha256) {
    fail('local FYLO preview digest differs from runtime.lock.json')
  } else {
    notes.push('local FYLO preview digest verified')
  }
}

const phases = new Set((parity.migrationSlices ?? []).map(slice => slice.phase))
for (let phase = 1; phase <= 7; phase++) {
  if (!phases.has(phase)) fail(`parity contract has no external tests for migration phase ${phase}`)
}
for (const slice of parity.migrationSlices ?? []) {
  if (!Array.isArray(slice.externalTests) || slice.externalTests.length === 0) {
    fail(`parity phase ${slice.phase} has no external test files`)
  }
  for (const path of slice.externalTests ?? []) requireFile(path)
  if (!Array.isArray(slice.commands) || slice.commands.length === 0) {
    fail(`parity phase ${slice.phase} has no executable command`)
  }
}

const targetRoutes = new Map(
  (routes.routes ?? []).map(route => [route.path, new Set(route.methods)]),
)
const retiredRoutes = new Set(routes.removedLegacyRoutes ?? [])
let currentRouteCount = 0
// The Go server's routes by controller (tests/support/routes.json). A socket
// route (STREAM) opens with GET, which the inventory records.
const serverRoutes = readJson('tests/support/routes.json')
for (const [controller, route] of Object.entries(serverRoutes)) {
  if (controller.startsWith('Test')) continue
  const path = route.path
  const methods = route.verbs.map(verb => verb === 'STREAM' ? 'GET' : verb)
  if (methods.length === 0) {
    fail(`could not discover an HTTP method for ${controller}`)
    continue
  }
  currentRouteCount++
  if (retiredRoutes.has(path)) continue
  const expected = targetRoutes.get(path)
  if (!expected) {
    fail(`current route is missing from the v1 or retired inventory: ${path}`)
    continue
  }
  for (const method of methods) {
    if (!expected.has(method)) fail(`${method} ${path} is missing from the v1 route inventory`)
  }
}
notes.push(`${currentRouteCount} non-test Go controllers covered by the compatibility inventory`)

const dockerfile = readFileSync(requireFile('Dockerfile'), 'utf8')
const compose = readFileSync(requireFile('docker-compose.yml'), 'utf8')
for (const token of ['EXPOSE 8080 25 587 465 80', 'ENV NODE_ENV=production', 'USER 65532:65532']) {
  if (!dockerfile.includes(token)) fail(`Dockerfile is missing baseline invariant: ${token}`)
}
for (const mapping of ['8080:8080', '25:2525', '587:2587']) {
  if (!compose.includes(mapping)) fail(`docker-compose.yml is missing documented port mapping ${mapping}`)
}

const forbiddenDocs = []
for (const path of listFiles(['docs', 'contracts'], ['*.md', '*.json'])) {
  const text = readFileSync(resolve(root, path), 'utf8')
  if (/\bsms\b|sent\.dm/i.test(text)) forbiddenDocs.push(path)
}
if (forbiddenDocs.length > 0) {
  fail(`retired carrier-verification terminology remains in: ${forbiddenDocs.join(', ')}`)
}

if (failures.length > 0) {
  console.error(`Phase 0 verification failed (${failures.length}):`)
  for (const message of failures) console.error(`  - ${message}`)
  process.exit(1)
}

console.log('Phase 0 contracts and baseline are internally consistent.')
for (const note of notes) console.log(`  - ${note}`)

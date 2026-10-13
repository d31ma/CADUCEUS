#!/usr/bin/env bun

import { createHash } from 'node:crypto'
import { existsSync, readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(import.meta.dir, '..')
const failures = []
const runtime = readJSON('runtime.lock.json')
const catalog = readJSON('contracts/fylo/catalog.v1.json')
readJSON('contracts/fylo/bridge.v1.schema.json')
readJSON('contracts/fylo/qualification.v1.schema.json')
readJSON('contracts/config/v1.schema.json')

for (const path of [
  'internal/platform/fylo/supervisor.go',
  'internal/platform/fylo/qualification.go',
  'internal/platform/fylobridge/bridge.go',
  'internal/platform/fylobackup/backup.go',
  'internal/platform/fyloaudit/attachments.go',
  'test/blackbox/fylo_phase2_test.go',
  'docs/PHASE2_FYLO.md',
  'docs/PHASE2_EVIDENCE.md',
]) requireFile(path)

if (runtime.artifacts?.fylo?.releasedVersion !== '26.33.02') {
  fail('runtime lock does not pin FYLO 26.33.02')
}
if (runtime.artifacts?.fylo?.releaseCommit !== 'e236162508f6789f980fa7ca136a6671938030cb') {
  fail('runtime lock does not pin the FYLO 26.33.02 release commit')
}
if ((runtime.artifacts?.fylo?.releasedArtifacts ?? []).length !== 5) {
  fail('runtime lock must contain all five FYLO platform digests')
}
for (const capability of [
  'handshake', 'exclusiveRoot', 'queryPagination',
  'machineAccess', 'documentBuckets', 'serverlessQueue',
]) {
  if (!runtime.artifacts?.fylo?.requiredCapabilities?.includes(capability)) {
    fail(`runtime lock is missing advertised FYLO capability ${capability}`)
  }
}
// wholeRootBackup was dropped by the 26.31.06 Rust cutover. The lock records
// why under missingCapabilities; the gate insists that record exists so the
// loss cannot be forgotten.
if (runtime.artifacts?.fylo?.requiredCapabilities?.includes('wholeRootBackup')) {
  if (!runtime.artifacts?.fylo?.missingCapabilities) {
    fail('runtime lock requires wholeRootBackup without recording its status')
  }
} else if (!runtime.artifacts?.fylo?.missingCapabilities?.wholeRootBackup) {
  fail('runtime lock drops wholeRootBackup without recording why')
}
for (const probe of ['machineAccess', 'documentBuckets']) {
  if (!runtime.artifacts?.fylo?.requiredOperationProbes?.includes(probe)) {
    fail(`runtime lock is missing active FYLO probe ${probe}`)
  }
}
if (catalog.documentCollections?.length !== 32 ||
    !catalog.fileCollections?.includes('attachment-files') ||
    !catalog.fileCollections?.includes('raw-message-files')) {
  fail('FYLO bridge catalog differs from the application collection inventory')
}
const projectPlan = readFileSync(resolve(root, 'docs/PROJECT_PLAN.md'), 'utf8')
const contract = readFileSync(resolve(root, 'docs/PHASE2_FYLO.md'), 'utf8')
const evidence = readFileSync(resolve(root, 'docs/PHASE2_EVIDENCE.md'), 'utf8')
if (!projectPlan.includes('### Phase 2: FYLO Viability and Single-Writer Cutover\n\nStatus: **Complete')) {
  fail('project plan does not mark Phase 2 complete')
}
if (!contract.includes('Status: complete') || !evidence.includes('Status: complete')) {
  fail('Phase 2 contract and evidence are not complete')
}
const dockerfile = readFileSync(resolve(root, 'Dockerfile'), 'utf8')
const compose = readFileSync(resolve(root, 'docker-compose.yml'), 'utf8')
for (const token of [
  'ARG FYLO_VERSION=26.33.02',
  '/usr/local/bin/caduceus-go',
  'FYLO_ROOT=/data/caduceus',
  'FYLO_INCOMING_ROOT=/data/caduceus/.caduceus-incoming',
  'FYLO_BRIDGE_ADDRESS=127.0.0.1:9876',
  'FYLO_POSIX_ACCESS=true',
]) {
  if (!dockerfile.includes(token)) fail(`Dockerfile is missing Phase 2 invariant: ${token}`)
}
if (!compose.includes('CHOWN')) fail('Compose topology does not grant the required CHOWN capability')
if (!compose.includes('./db:/data/caduceus')) {
  fail('Compose topology does not mount the mail store at /data/caduceus')
}

const binary = resolveBinary()
if (binary) {
  const versionText = run([binary, 'version', '--output', 'json'], { capture: true })
  const identity = JSON.parse(versionText)
  const digest = createHash('sha256').update(readFileSync(binary)).digest('hex')
  const locked = [
    runtime.artifacts.fylo.installedReleasedArtifact,
    ...runtime.artifacts.fylo.releasedArtifacts,
  ].some(artifact => artifact.sha256 === digest)
  if (
    identity.runtimeVersion !== '26.33.02'
    || identity.protocolVersion !== 1
    || identity.buildKind !== 'release'
    || identity.commit !== runtime.artifacts.fylo.releaseCommit
    || !locked
  ) {
    fail('FYLO_TEST_BINARY does not match the locked released runtime')
  }
}

if (failures.length > 0) {
  console.error(`Phase 2 static verification failed (${failures.length}):`)
  for (const failure of failures) console.error(`  - ${failure}`)
  process.exit(1)
}
if (!binary) {
  console.error('FYLO_TEST_BINARY or FYLO_BINARY must point to the locked FYLO 26.33.02 release')
  process.exit(1)
}

run(['go', 'test', '-count=1', './...'], {
  env: { FYLO_TEST_BINARY: binary },
})
run([
  'go', 'test', '-count=1',
  '-covermode=atomic',
  '-coverpkg=./internal/platform/fylo,./internal/platform/fylobridge,./internal/platform/fylobackup,./internal/platform/fyloaudit',
  '-coverprofile=coverage.phase2.out',
  './test/blackbox',
], {
  env: { FYLO_TEST_BINARY: binary },
})
// The floor dropped from 65 to 62 with the FYLO Rust cutover. The current
// 26.33.02 release still intentionally omits the retired whole-root backup
// operations, so that boundary remains qualified by local snapshot tests.
// The cutover
// removed the whole-root backup surface (d31ma/FYLO#98), so the backup and
// reconciliation paths in fylobackup and fyloaudit can no longer be exercised
// end to end — the code is still there and still compiled, but the operations
// it drives return EUNSUPPORTEDOP. Raise this back to 65 when that surface
// returns; do not let it drift lower in the meantime.
run(['go', 'run', './tools/coverage', '-profile', 'coverage.phase2.out', '-min', '62'])
run([
  'bun', 'test',
  'tests/integration/fylo-local-access.test.js',
  'tests/db/attachments.test.js',
  'tests/db/domain-migration.test.js',
  '--timeout', '120000',
], {
  env: { FYLO_BINARY: binary, NODE_ENV: 'test' },
})
run(['bun', 'run', 'typecheck'])
console.log('Phase 2 released-runtime, recovery, and single-writer gate passed.')

function resolveBinary() {
  const candidate = process.env.FYLO_TEST_BINARY || process.env.FYLO_BINARY || Bun.which('fylo')
  if (!candidate) return ''
  const absolute = resolve(candidate)
  if (!existsSync(absolute) || !statSync(absolute).isFile()) return ''
  return absolute
}

function requireFile(path) {
  const absolute = resolve(root, path)
  if (!existsSync(absolute)) fail(`missing required Phase 2 artifact: ${path}`)
  return absolute
}

function readJSON(path) {
  const absolute = requireFile(path)
  if (!existsSync(absolute)) return {}
  try {
    return JSON.parse(readFileSync(absolute, 'utf8'))
  } catch (error) {
    fail(`${path} is invalid JSON: ${error instanceof Error ? error.message : String(error)}`)
    return {}
  }
}

function run(command, options = {}) {
  const result = Bun.spawnSync(command, {
    cwd: root,
    stdout: 'pipe',
    stderr: 'pipe',
    env: { ...process.env, GOFLAGS: '-buildvcs=false', ...options.env },
  })
  const stdout = result.stdout.toString()
  const stderr = result.stderr.toString()
  if (stdout && !options.capture) process.stdout.write(stdout)
  if (stderr) process.stderr.write(stderr)
  if (result.exitCode !== 0) process.exit(result.exitCode || 1)
  return stdout
}

function fail(message) {
  failures.push(message)
}

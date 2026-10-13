#!/usr/bin/env bun

/**
 * Phase 3 gate.
 *
 * Two questions. Is the identity boundary configured the way the contract says
 * — one pinned engine, one composed operation surface, no retired credential
 * concept left anywhere? And does it behave that way against the released
 * engine? The first is checked here; the second is `go test`, which this runs.
 */

import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { resolve, join } from 'node:path'

const root = resolve(import.meta.dir, '..')
const failures = []
const runtime = readJSON('runtime.lock.json')

for (const path of [
  'internal/auth/engine.go',
  'internal/auth/flows.go',
  'internal/auth/authorize.go',
  'internal/auth/credentials.go',
  'internal/auth/provisioning.go',
  'internal/auth/bootstrap.go',
  'internal/auth/policy.go',
  'internal/auth/tenancy.go',
  'internal/auth/jwt.go',
  'internal/auth/ops.go',
  'internal/platform/cli/auth.go',
  'server/platform/auth/bridge.js',
  'server/middleware/require-auth.js',
  'server/middleware/guards.js',
  'test/blackbox/auth_phase3_test.go',
  'docs/PHASE3_AUTH.md',
  'docs/PHASE3_EVIDENCE.md',
  'docs/adr/0003-sesame-authentication-boundary.md',
  'tests/support/identity.mjs',
]) requireFile(path)

const evidence = readFileSync(resolve(root, 'docs/PHASE3_EVIDENCE.md'), 'utf8')
if (!evidence.includes('Status: complete')) fail('Phase 3 evidence is not complete')
const plan = readFileSync(resolve(root, 'docs/PROJECT_PLAN.md'), 'utf8')
if (!plan.includes('### Phase 3: Authentication and Tenant Boundary\n\nStatus: **Complete')) {
  fail('the project plan does not mark Phase 3 complete')
}

// ── The pinned engine ────────────────────────────────────────────────────

const sesame = runtime.artifacts?.sesame ?? {}
if (sesame.releasedVersion !== '26.33.02') {
  fail('runtime lock does not pin SESAME 26.33.02')
}
if (sesame.releaseCommit !== 'e8f36b6cf946a6979b2e2aa6df604e4ef6d8f1cb') {
  fail('runtime lock does not pin the SESAME 26.33.02 release commit')
}
if (sesame.goClientModule !== 'github.com/d31ma/sesame') {
  fail('runtime lock does not name the SESAME Go client module')
}
if (sesame.goClientVersion !== 'v0.0.0-20260811145757-e8f36b6cf946') {
  fail('runtime lock does not pin the SESAME 26.33.02 Go client pseudo-version')
}
if (sesame.compatibleFYLOVersion !== '26.33.02') {
  fail('runtime lock does not record the FYLO release required by SESAME 26.33.02')
}
if ((sesame.releasedArtifacts ?? []).length !== 5) {
  fail('runtime lock must contain all five SESAME platform digests')
}
for (const operation of [
  'authenticator.passkey_register_begin', 'authenticator.passkey_register_finish',
  'authenticator.passkey_list', 'authenticator.passkey_remove',
  'authenticator.recovery_codes_issue', 'authenticator.set_password',
  'authenticator.totp_activate', 'authenticator.totp_enroll', 'authn.begin',
  'authn.complete', 'authn.passkey_options', 'authn.verify_passkey',
  'authn.verify_password', 'authn.verify_recovery_code', 'authn.verify_totp',
  'authorize.decide', 'grant.create', 'group.create', 'group.member_add',
  'group.member_remove', 'oidc.authorize', 'oidc.interaction_complete',
  'oidc.introspect', 'oidc.logout', 'oidc.token', 'oidc_client.get',
  'oidc_client.register', 'principal.create', 'principal.get',
  'principal.suspend', 'role.create', 'session.revoke', 'session.verify',
  'tenant.bootstrap', 'tenant.get', 'token.jwks',
]) {
  if (!sesame.requiredOperations?.includes(operation)) {
    fail(`runtime lock is missing required SESAME operation ${operation}`)
  }
}
if (sesame.requiredOperations?.length !== 36) {
  fail('runtime lock SESAME operation set differs from the 36-operation CADUCEUS boundary')
}
if (sesame.ownsSeparateFYLORoot !== true) {
  fail('runtime lock does not record that SESAME owns its own FYLO root')
}

const goMod = readFileSync(resolve(root, 'go.mod'), 'utf8')
if (!goMod.includes(`github.com/d31ma/sesame ${sesame.goClientVersion}`)) {
  fail('go.mod does not depend on the pinned SESAME client version')
}

// ── Nothing retired survives ─────────────────────────────────────────────

const retired = [
  'signJwt', 'verifyJwt', 'getJwtSecret', 'JWT_SECRET',
  'AUTH_PASSWORD_PEPPER', 'CADUCEUS_AUTH_STEP_UP_MAX_AGE_SECONDS',
  'features/auth/', 'WebauthnService', 'TotpService', 'AuthPolicyService',
  'AuthSecurityRepository', 'MfaRepository',
]
for (const directory of ['server', 'client', 'internal', 'cmd', 'scripts']) {
  for (const file of walk(resolve(root, directory))) {
    if (file.endsWith('verify-phase3.mjs')) continue // this file names them to find them
    const contents = readFileSync(file, 'utf8')
    for (const token of retired) {
      if (contents.includes(token)) {
        fail(`retired authentication concept ${token} survives in ${file.slice(root.length + 1)}`)
      }
    }
  }
}
if (existsSync(resolve(root, 'server/features/auth'))) {
  fail('the retired server/features/auth tree still exists')
}

// Retired route paths, not just retired identifiers. A caller that still asks
// for one of these compiles, typechecks, and fails only when someone clicks it,
// which is how the settings view kept calling `/mfa/provision` after the route
// behind it was deleted.
const retiredRoutes = [
  '/mfa/provision', '/mfa/devices', '/auth/webauthn/', '/auth/totp/enroll',
  '/auth/password/setup', '/auth/password/verify', '/auth/recovery/codes',
  '/auth/recovery/admin', '/auth/recovery/verify', '/auth/mfa/confirm',
  '/test/seed/device',
]
for (const directory of ['server', 'client', 'scripts', 'tests']) {
  for (const file of walk(resolve(root, directory))) {
    if (file.endsWith('verify-phase3.mjs')) continue
    const contents = readFileSync(file, 'utf8')
    for (const route of retiredRoutes) {
      if (contents.includes(`'${route}`) || contents.includes(`"${route}`) ||
          contents.includes(`\`${route}`)) {
        fail(`retired route ${route} is still called from ${file.slice(root.length + 1)}`)
      }
    }
  }
}
if (existsSync(resolve(root, 'server/routes/test/seed/device'))) {
  fail('the forged passkey seed route still exists')
}

// ── The boundary is the only path ────────────────────────────────────────

const ops = readFileSync(resolve(root, 'internal/auth/ops.go'), 'utf8')
for (const operation of [
  'auth.login.begin', 'auth.login.password', 'auth.login.passkey',
  'auth.login.totp', 'auth.login.recovery', 'auth.login.complete',
  'auth.token.refresh', 'auth.token.authenticate', 'auth.request.authorize',
  'auth.logout', 'auth.session.revoke', 'auth.jwks',
  'auth.provision.domain', 'auth.provision.mailbox',
  'auth.provision.role', 'auth.provision.role.revoke',
]) {
  if (!ops.includes(`"${operation}"`)) fail(`the boundary does not publish ${operation}`)
}

// Multi-domain administration is a group membership, and it only works because
// every mailbox is a principal in one tenant.
const tenancy = readFileSync(resolve(root, 'internal/auth/tenancy.go'), 'utf8')
if (!/const TenantName = "caduceus"/.test(tenancy)) {
  fail('the deployment tenant is not a single constant')
}
const provisioning = readFileSync(resolve(root, 'internal/auth/provisioning.go'), 'utf8')
for (const token of ['GroupMemberAdd', 'GroupMemberRemove', 'GrantCreateForGroup']) {
  if (!provisioning.includes(token)) {
    fail(`domain authority does not use ${token}; it must be a group membership`)
  }
}

// The test suite must not be able to mint a token of its own.
const harness = readFileSync(resolve(root, 'tests/support/identity.mjs'), 'utf8')
if (!harness.includes('auth.test.bootstrap') || !harness.includes('completeLogin')) {
  fail('the test harness does not obtain tokens from a real login')
}

const bridge = readFileSync(resolve(root, 'internal/platform/fylobridge/bridge.go'), 'utf8')
if (!bridge.includes('authOperationPrefix') || !bridge.includes('bridge.authOps[operation]')) {
  fail('the bridge does not allowlist auth operations separately from FYLO operations')
}

const guards = readFileSync(resolve(root, 'server/middleware/guards.js'), 'utf8')
if (guards.includes("level: 'sensitive'") || guards.includes("role: 'admin'")) {
  fail('route guards still carry the retired role and assurance vocabulary')
}

// Every guarded route must reach the boundary, never a local claim check.
for (const file of walk(resolve(root, 'server/routes'))) {
  const contents = readFileSync(file, 'utf8')
  if (/auth\.(role|acr|amr)\b/.test(contents)) {
    fail(`route reads a retired claim directly: ${file.slice(root.length + 1)}`)
  }
}

// ── Configuration ────────────────────────────────────────────────────────

const config = readFileSync(resolve(root, 'internal/platform/config/config.go'), 'utf8')
for (const key of [
  'SESAME_BINARY', 'SESAME_DEPLOYMENT', 'SESAME_ISSUER',
  'SESAME_SESSION_LIFETIME', 'SESAME_REQUEST_TIMEOUT',
]) {
  if (!config.includes(key)) fail(`configuration does not define ${key}`)
}

const dockerfile = readFileSync(resolve(root, 'Dockerfile'), 'utf8')
for (const token of ['ARG SESAME_VERSION=26.33.02', 'SESAME_DEPLOYMENT=/data/sesame']) {
  if (!dockerfile.includes(token)) fail(`Dockerfile is missing ${token}`)
}
const entry = readFileSync(resolve(root, 'internal/runtime/entry/entry.go'), 'utf8')
if (!entry.includes('"auth", "init"')) {
  fail('the container entrypoint does not initialize the identity deployment')
}

report()

// ── Behaviour against the released engine ────────────────────────────────

const engine = locate(process.env.SESAME_TEST_BINARY) || locate(Bun.which('sesame'))
const storage = locate(process.env.FYLO_TEST_BINARY) || locate(Bun.which('fylo'))
if (!engine || !storage) {
  console.error('SESAME_TEST_BINARY and FYLO_TEST_BINARY must point at the locked releases')
  process.exit(1)
}

run(['gofmt', '-l', 'internal', 'cmd', 'test'], { expectEmpty: true })
run(['go', 'vet', './...'])
run(['go', 'test', './internal/...'])
run(['go', 'test', '-run', 'TestPhase3', '-v', './test/blackbox/'], {
  env: { SESAME_TEST_BINARY: engine, FYLO_TEST_BINARY: storage },
})

// Exercise the same authorization boundary at the persistence and route layers.
// The legacy HTTP harness still boots through Tachyon and belongs to the
// separately documented frontend/server migration gate.
run(['bun', 'run', 'test:db'], {
  env: { SESAME_TEST_BINARY: engine, FYLO_TEST_BINARY: storage },
})
run(['bun', 'run', 'test:routes'], {
  env: { SESAME_TEST_BINARY: engine, FYLO_TEST_BINARY: storage },
})

console.log('Phase 3 identity qualification passed.')

// ── helpers ──────────────────────────────────────────────────────────────

function walk(directory) {
  if (!existsSync(directory)) return []
  const found = []
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name.startsWith('.')) continue
    const path = join(directory, entry.name)
    if (entry.isDirectory()) found.push(...walk(path))
    else if (/\.(js|mjs|ts|go)$/.test(entry.name)) found.push(path)
  }
  return found
}

function locate(candidate) {
  if (!candidate) return ''
  const absolute = resolve(candidate)
  if (!existsSync(absolute) || !statSync(absolute).isFile()) return ''
  return absolute
}

function requireFile(path) {
  const absolute = resolve(root, path)
  if (!existsSync(absolute)) fail(`missing required Phase 3 artifact: ${path}`)
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
  if (stdout) process.stdout.write(stdout)
  if (stderr) process.stderr.write(stderr)
  if (options.expectEmpty && stdout.trim()) {
    console.error(`${command.join(' ')} reported unformatted files`)
    process.exit(1)
  }
  if (result.exitCode !== 0) process.exit(result.exitCode || 1)
  return stdout
}

function fail(message) {
  failures.push(message)
}

function report() {
  if (failures.length === 0) return
  console.error('Phase 3 verification failed:')
  for (const failure of failures) console.error(`  - ${failure}`)
  process.exit(1)
}

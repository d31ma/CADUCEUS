/**
 * Identity harness.
 *
 * The test suite cannot mint its own tokens any more, and that is the point: a
 * harness that can forge a credential proves nothing about the boundary that
 * verifies them. So it starts the real thing — the Go owner, the released
 * SESAME engine, and the released FYLO underneath both — provisions a mailbox
 * through it, and signs in with the credentials it hands back.
 *
 * The bridge owns the mail FYLO root exclusively, so a suite that uses this
 * harness reaches persistence through the same bridge the product does. That
 * is not incidental: two writers on one root is what the deployment forbids.
 */

import { mkdtempSync, rmSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { spawn, spawnSync } from 'node:child_process'
import { createConnection } from 'node:net'
import { randomBytes } from 'node:crypto'

const CADUCEUS_ROOT = join(import.meta.dir, '..', '..')
const SEED_PASSWORD = 'harness-mailbox-password-0123456789'

/** Locate one required binary, or return '' so the caller can skip clearly. */
function binary(envName, command) {
  const configured = process.env[envName]
  if (configured && existsSync(configured)) return configured
  return Bun.which(command) ?? ''
}

/**
 * Report what is missing, so a skipped suite says why rather than failing
 * somewhere deep in a login.
 *
 * @returns {string}
 */
export function identityRequirements() {
  const missing = []
  if (!binary('SESAME_TEST_BINARY', 'sesame')) missing.push('SESAME_TEST_BINARY (or `sesame` on PATH)')
  if (!binary('FYLO_TEST_BINARY', 'fylo')) missing.push('FYLO_TEST_BINARY (or `fylo` on PATH)')
  if (!existsSync(join(CADUCEUS_ROOT, 'dist-bin', 'caduceus'))) {
    missing.push('dist-bin/caduceus (run `bun run go:build`)')
  }
  return missing.length ? `identity harness needs: ${missing.join(', ')}` : ''
}

/**
 * Start one Go owner over fresh temporary roots.
 *
 * @returns {Promise<{
 *   env: Record<string, string>,
 *   fyloRoot: string,
 *   diagnostics: () => string,
 *   mint: (address: string, role?: string) => Promise<any>,
 *   stop: () => void,
 * }>}
 */
export async function startIdentity() {
  const requirement = identityRequirements()
  if (requirement) throw new Error(requirement)

  const sesameBinary = binary('SESAME_TEST_BINARY', 'sesame')
  const fyloBinary = binary('FYLO_TEST_BINARY', 'fylo')
  const caduceus = join(CADUCEUS_ROOT, 'dist-bin', 'caduceus')

  const root = mkdtempSync(join(tmpdir(), 'caduceus-identity-'))
  const fyloRoot = join(root, 'fylo')
  const deployment = join(root, 'sesame')
  const port = await availablePort()

  const env = {
    ...process.env,
    NODE_ENV: 'test',
    CADUCEUS_ENABLE_TEST_ROUTES: 'true',
    FYLO_ROOT: fyloRoot,
    FYLO_BINARY: fyloBinary,
    FYLO_SCHEMA: join(CADUCEUS_ROOT, 'db', 'schemas'),
    FYLO_INCOMING_ROOT: join(fyloRoot, '.caduceus-incoming'),
    FYLO_BRIDGE_ADDRESS: `127.0.0.1:${port}`,
    FYLO_BRIDGE_SECRET: randomBytes(32).toString('hex'),
    EXTERNAL_ACCOUNT_CREDENTIAL_KEYS: JSON.stringify({ 1: randomBytes(32).toString('base64url') }),
    EXTERNAL_ACCOUNT_ACTIVE_KEY: '1',
    EXTERNAL_ACCOUNT_CONNECTION_TIMEOUT: '2s',
    EXTERNAL_ACCOUNT_PRIVATE_CIDRS: '127.0.0.0/8',
    FYLO_POSIX_ACCESS: 'false',
    RUNTIME_LOCK: join(CADUCEUS_ROOT, 'runtime.lock.json'),
    STATIC_DIR: join(CADUCEUS_ROOT, 'build', 'static'),
    SESAME_BINARY: sesameBinary,
    SESAME_DEPLOYMENT: deployment,
    SESAME_ISSUER: 'https://localhost:8080',
    SESAME_REDIRECT_URI: 'https://localhost:8080/auth/callback',
    INBOUND_WEBHOOK_SECRET: 'harness-inbound-secret',
    SMTP_ADAPTER: 'console',
    WEB_PUSH_DISABLED: 'true',
  }
  Bun.spawnSync(['mkdir', '-p', fyloRoot])

  const initialized = spawnSync(caduceus, ['auth', 'init'], { env, encoding: 'utf8' })
  if (initialized.status !== 0) {
    rmSync(root, { recursive: true, force: true })
    throw new Error(`caduceus auth init failed: ${initialized.stderr}`)
  }

  const owner = spawn(caduceus, ['fylo-bridge'], { env, stdio: ['ignore', 'ignore', 'pipe'] })
  let diagnostics = ''
  owner.stderr?.on('data', chunk => { diagnostics += String(chunk) })

  const stop = () => {
    owner.kill()
    rmSync(root, { recursive: true, force: true })
  }
  try {
    await waitForBridge(owner, '127.0.0.1', port)
  } catch (error) {
    stop()
    throw new Error(`${String(error)}\n${diagnostics.slice(-2000)}`)
  }

  return {
    env,
    fyloRoot,
    diagnostics: () => diagnostics.slice(-4000),
    stop,
    mint: (address, role) => mintToken(env, address, role),
  }
}

/**
 * Provision one mailbox and sign in as it, exactly as a person would: password
 * first, then the recovery code the bootstrap handed back.
 *
 * @param {Record<string, string>} env
 * @param {string} address
 * @param {string} [role] - 'admin' or 'viewer'; anything else grants no domain authority.
 */
export async function mintToken(env, address, role) {
  const previous = {
    address: process.env.FYLO_BRIDGE_ADDRESS,
    secret: process.env.FYLO_BRIDGE_SECRET,
  }
  process.env.FYLO_BRIDGE_ADDRESS = env.FYLO_BRIDGE_ADDRESS
  process.env.FYLO_BRIDGE_SECRET = env.FYLO_BRIDGE_SECRET
  try {
    const { authCall, Auth } = await import('../../server/platform/auth/Auth.js')
    const domainRole = role === 'admin' ? 'domain-admin'
      : role === 'viewer' ? 'domain-viewer'
        : ''
    const seeded = await authCall('auth.test.bootstrap', {
      address, password: SEED_PASSWORD, ...(domainRole ? { role: domainRole } : {}),
    })
    const begun = await Auth.loginBegin(address)
    await Auth.verifyPassword(begun.transactionId, SEED_PASSWORD)
    await Auth.verifyRecoveryCode(begun.transactionId, seeded.recoveryCodes[0])
    const tokens = await Auth.completeLogin(begun.transactionId)
    return {
      ...tokens,
      email: tokens.address,
      password: SEED_PASSWORD,
      token: tokens.accessToken,
      role: role ?? 'viewer',
      domains: [tokens.domain],
      recoveryCodes: seeded.recoveryCodes.slice(1),
    }
  } finally {
    if (previous.address === undefined) delete process.env.FYLO_BRIDGE_ADDRESS
    else process.env.FYLO_BRIDGE_ADDRESS = previous.address
    if (previous.secret === undefined) delete process.env.FYLO_BRIDGE_SECRET
    else process.env.FYLO_BRIDGE_SECRET = previous.secret
  }
}

/**
 * Give one mailbox authority over a domain that is not its own.
 *
 * @param {Record<string, string>} env
 * @param {string} address
 * @param {string} domain
 * @param {'domain-admin' | 'domain-viewer'} role
 */
export async function grantDomain(env, address, domain, role) {
  process.env.FYLO_BRIDGE_ADDRESS = env.FYLO_BRIDGE_ADDRESS
  process.env.FYLO_BRIDGE_SECRET = env.FYLO_BRIDGE_SECRET
  const { authCall } = await import('../../server/platform/auth/Auth.js')
  await authCall('auth.provision.domain', { domain })
  return authCall('auth.provision.role', { address, domain, role })
}

async function waitForBridge(child, host, port, timeoutMs = 180_000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`the Go owner exited with code ${child.exitCode}`)
    }
    const connected = await new Promise(resolve => {
      const socket = createConnection({ host, port })
      socket.setTimeout(500)
      socket.once('connect', () => { socket.destroy(); resolve(true) })
      socket.once('timeout', () => { socket.destroy(); resolve(false) })
      socket.once('error', () => resolve(false))
    })
    if (connected) return
    await Bun.sleep(100)
  }
  throw new Error('the Go owner did not open its bridge in time')
}

async function availablePort() {
  const { createServer } = await import('node:net')
  return await new Promise((resolve, reject) => {
    const reservation = createServer()
    reservation.unref()
    reservation.once('error', reject)
    reservation.listen(0, '127.0.0.1', () => {
      const address = reservation.address()
      if (!address || typeof address === 'string') {
        reservation.close()
        reject(new Error('unable to reserve a loopback port'))
        return
      }
      reservation.close(error => error ? reject(error) : resolve(address.port))
    })
  })
}

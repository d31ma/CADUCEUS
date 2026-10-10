// CADUCEUS entry point for the self-hosted server.
// Compiled to binary via: bun build --compile scripts/entry.mjs --outfile caduceus
//
// Commands:
//   ./caduceus serve                       Standalone mail server
//   ./caduceus admin:create --email=...    Create admin user
//   ./caduceus domain:migrate --from=...   Migrate domain
//   ./caduceus help                        Show help

import { OutboundQueueWorker } from '../background/OutboundQueueWorker.ts'
import { FollowUpReminderWorker } from '../background/FollowUpReminderWorker.ts'
import { SnoozeWorker } from '../background/SnoozeWorker.ts'
import { ProviderNotificationWorker } from '../background/ProviderNotificationWorker.ts'
import { ServiceSupervisor } from '../background/ServiceSupervisor.ts'
import { randomBytes } from 'node:crypto'

const command = process.argv[2] || 'serve'
const args = process.argv.slice(3)

if (command === 'serve') {
  // ── Startup configuration summary ──────────────────────────────────────
  const smtp = process.env.SMTP_ADAPTER || (process.env.NODE_ENV === 'production' ? 'direct' : 'console')
  const pushDisabled = process.env.WEB_PUSH_DISABLED === 'true'
  const pushKeys = !!(process.env.VAPID_PUBLIC_KEY && process.env.VAPID_PRIVATE_KEY)
  const hasIdentity = !!process.env.SESAME_DEPLOYMENT && !!process.env.SESAME_BINARY
  const hasInbound = !!process.env.INBOUND_WEBHOOK_SECRET

  console.log('')
  console.log('╔══════════════════════════════════════╗')
  console.log('║         CADUCEUS mail server           ║')
  console.log('╚══════════════════════════════════════╝')
  console.log('')
  console.log('Secrets:')
  console.log(`  SESAME identity         ${hasIdentity ? '✓' : '✗ MISSING (required)'}`)
  console.log(`  INBOUND_WEBHOOK_SECRET  ${hasInbound ? '✓' : '✗ MISSING (required)'}`)
  console.log('')
  const mailServer = process.env.MAIL_SERVER_ENABLED === 'true'
  const inboundPort = process.env.SMTP_INBOUND_PORT || '25'
  console.log('Delivery:')
  console.log(`  SMTP → ${smtp}${smtp === 'console' ? ' (logs only, no delivery)' : ''}`)
  console.log(`  Inbound SMTP → ${mailServer ? `standalone MX on :${inboundPort}` : 'disabled (set MAIL_SERVER_ENABLED=true)'}`)
  console.log(`  Submission   → ${process.env.MAIL_SUBMISSION_ENABLED === 'true' ? `on :${process.env.SMTP_SUBMISSION_PORT || '587'}${process.env.SMTP_SUBMISSION_TLS_PORT ? '/' + process.env.SMTP_SUBMISSION_TLS_PORT : ''}` : 'disabled (set MAIL_SUBMISSION_ENABLED=true)'}`)
  console.log(`  Outbound queue → ${process.env.OUTBOUND_QUEUE_WORKER === 'off' ? 'worker off' : 'retry/bounce worker on'}`)
  console.log(`  Follow-up reminders → ${process.env.REMINDER_WORKER === 'off' ? 'worker off' : 'due worker on'}`)
  console.log(`  Snooze return → ${process.env.SNOOZE_WORKER === 'off' ? 'worker off' : 'due worker on'}`)
  console.log(`  Provider alerts → ${process.env.PROVIDER_NOTIFICATION_WORKER === 'off' ? 'worker off' : 'durable push worker on'}`)
  console.log(`  TLS certs    → ${process.env.MAIL_ACME_ENABLED === 'true' ? `automatic (Let's Encrypt${process.env.MAIL_ACME_STAGING === 'true' ? ' staging' : ''})` : 'manual (MAIL_TLS_CERT/KEY) or none'}`)
  if (smtp === 'console') {
    console.log('  Console delivery is intended only for development and tests.')
  }
  console.log('')
  console.log('Push notifications:')
  console.log(`  ${pushDisabled ? 'Disabled (WEB_PUSH_DISABLED=true)' : pushKeys ? 'Enabled (VAPID keys configured)' : 'Disabled (no VAPID keys set)'}`)
  console.log('')
  if (!hasIdentity || !hasInbound) {
    console.error('ERROR: SESAME_BINARY, SESAME_DEPLOYMENT, and INBOUND_WEBHOOK_SECRET are required.')
    console.error('Set them as environment variables before starting CADUCEUS.')
    process.exit(1)
  }

  const { spawn } = await import('node:child_process')
  const bridgeChild = await startPersistenceBridge(spawn)

  await runBootstrapIfConfigured()
  await writeHandlerSettings()

  // Start the standalone Tachyon runtime as a child process.
  const child = spawn('ty', ['serve', ...args], {
    stdio: 'inherit',
    env: process.env,
  })

  /** @type {ServiceSupervisor | undefined} */
  let supervisor
  /** @type {OutboundQueueWorker | undefined} */
  let outboundWorker
  /** @type {FollowUpReminderWorker | undefined} */
  let reminderWorker
  /** @type {SnoozeWorker | undefined} */
  let snoozeWorker
  /** @type {ProviderNotificationWorker | undefined} */
  let providerNotificationWorker
  /** @type {ReturnType<typeof setInterval> | undefined} */
  let statusTimer
  /** @type {number | undefined} */
  let requestedExitCode
  /** @type {Promise<void> | undefined} */
  let shutdownPromise

  const childExit = new Promise((resolve) => {
    child.once('error', (error) => {
      console.error('[runtime] Tachyon failed to start:', error instanceof Error ? error.message : error)
      requestedExitCode = 1
      resolve(1)
    })
    child.once('exit', (code, signal) => {
      if (code !== null) resolve(code)
      else resolve(requestedExitCode ?? (signal ? 1 : 0))
    })
  })
  bridgeChild.once('exit', (code, signal) => {
    if (code === 0 && requestedExitCode !== undefined) return
    console.error(`[runtime] Go FYLO owner exited (${signal || code || 'unknown'})`)
    void shutdown('SIGTERM', 1)
  })

  /**
   * Stop background services before terminating Tachyon. The returned promise
   * makes repeated signals and fatal-service callbacks share one shutdown.
   *
   * @param {NodeJS.Signals} [signal]
   * @param {number} [exitCode]
   */
  function shutdown(signal = 'SIGTERM', exitCode = 0) {
    requestedExitCode = Math.max(requestedExitCode ?? 0, exitCode)
    if (shutdownPromise) return shutdownPromise
    shutdownPromise = (async () => {
      if (statusTimer) {
        clearInterval(statusTimer)
        statusTimer = undefined
      }
      await supervisor?.stop()
      if (!child.killed && child.exitCode === null) child.kill(signal)
    })()
    return shutdownPromise
  }

  for (const signal of ['SIGINT', 'SIGTERM']) {
    process.once(signal, () => {
      void shutdown(/** @type {NodeJS.Signals} */ (signal))
    })
  }

  // Background helpers run in this (parent) process and reach the API over
  // localhost, so the API server stays the single writer of the store.
  const apiUrl = `http://127.0.0.1:${process.env.PORT || 8080}`
  void (async () => {
    try {
      await waitForApi(apiUrl)

      supervisor = new ServiceSupervisor({
        onRequiredFailure: async (name, error) => {
          console.error(`[mail] required service "${name}" failed:`, error.message)
          await shutdown('SIGTERM', 1)
        },
      })

      if (process.env.MAIL_ACME_ENABLED === 'true') {
        supervisor.register({
          name: 'acme-renewal',
          required: false,
          start: async () => {
            const { AcmeService } = await import('../background/AcmeService.js')
            const dir = process.env.MAIL_TLS_DIR || '/data/tls'
            process.env.MAIL_TLS_CERT_FILE ||= `${dir}/${process.env.MAIL_HOSTNAME}.cert.pem`
            process.env.MAIL_TLS_KEY_FILE ||= `${dir}/${process.env.MAIL_HOSTNAME}.key.pem`
            const acme = new AcmeService({
              hostname: process.env.MAIL_HOSTNAME || '',
              email: process.env.MAIL_ACME_EMAIL || '',
              certPath: process.env.MAIL_TLS_CERT_FILE,
              keyPath: process.env.MAIL_TLS_KEY_FILE,
              accountPath: process.env.MAIL_ACME_ACCOUNT_FILE || `${dir}/acme-account.pem`,
              httpPort: Number(process.env.MAIL_ACME_HTTP_PORT || 80),
              staging: process.env.MAIL_ACME_STAGING === 'true',
            })
            await acme.startRenewal()
            return () => acme.stop()
          },
        })
      }

      if (mailServer) {
        supervisor.register({
          name: 'inbound-smtp',
          start: async () => {
            const { InboundSmtpServer } = await import('../background/InboundSmtpServer.js')
            const server = await new InboundSmtpServer({ apiUrl }).start()
            server.once('error', (error) => supervisor?.reportFailure('inbound-smtp', error))
            return () => closeSmtpServer(server)
          },
        })
      }

      if (process.env.MAIL_SUBMISSION_ENABLED === 'true') {
        supervisor.register({
          name: 'smtp-submission',
          start: async () => {
            const { SubmissionServer } = await import('../background/SubmissionServer.js')
            /** @type {import('smtp-server').SMTPServer[]} */
            const servers = []
            try {
              servers.push(await new SubmissionServer({
                port: Number(process.env.SMTP_SUBMISSION_PORT || 587),
                secure: false,
                apiUrl,
              }).start())
              if (process.env.SMTP_SUBMISSION_TLS_PORT) {
                servers.push(await new SubmissionServer({
                  port: Number(process.env.SMTP_SUBMISSION_TLS_PORT),
                  secure: true,
                  apiUrl,
                }).start())
              }
            } catch (error) {
              await Promise.all(servers.map(closeSmtpServer))
              throw error
            }
            for (const server of servers) {
              server.once('error', (error) => supervisor?.reportFailure('smtp-submission', error))
            }
            return async () => {
              await Promise.all([...servers].reverse().map(closeSmtpServer))
            }
          },
        })
      }

      if (process.env.OUTBOUND_QUEUE_WORKER !== 'off') {
        supervisor.register({
          name: 'outbound-queue',
          start: async () => {
            outboundWorker = new OutboundQueueWorker({
              apiUrl,
              secret: process.env.INBOUND_WEBHOOK_SECRET || '',
              intervalMs: positiveNumber(process.env.OUTBOUND_QUEUE_INTERVAL_MS, 30_000),
              timeoutMs: positiveNumber(process.env.OUTBOUND_QUEUE_TIMEOUT_MS, 60_000),
            })
            const stop = outboundWorker.start()
            void outboundWorker.runNow()
            return stop
          },
        })
      }

      if (process.env.SNOOZE_WORKER !== 'off') {
        supervisor.register({
          name: 'snooze-due',
          start: async () => {
            snoozeWorker = new SnoozeWorker({
              apiUrl,
              secret: process.env.INBOUND_WEBHOOK_SECRET || '',
              intervalMs: positiveNumber(process.env.SNOOZE_INTERVAL_MS, 30_000),
              timeoutMs: positiveNumber(process.env.SNOOZE_TIMEOUT_MS, 60_000),
            })
            const stop = snoozeWorker.start()
            void snoozeWorker.runNow()
            return stop
          },
        })
      }

      if (process.env.REMINDER_WORKER !== 'off') {
        supervisor.register({
          name: 'follow-up-reminders',
          start: async () => {
            reminderWorker = new FollowUpReminderWorker({
              apiUrl,
              secret: process.env.INBOUND_WEBHOOK_SECRET || '',
              intervalMs: positiveNumber(process.env.REMINDER_INTERVAL_MS, 30_000),
              timeoutMs: positiveNumber(process.env.REMINDER_TIMEOUT_MS, 60_000),
            })
            const stop = reminderWorker.start()
            void reminderWorker.runNow()
            return stop
          },
        })
      }

      if (process.env.PROVIDER_NOTIFICATION_WORKER !== 'off') {
        supervisor.register({
          name: 'provider-new-mail',
          start: async () => {
            providerNotificationWorker = new ProviderNotificationWorker({
              apiUrl,
              secret: process.env.INBOUND_WEBHOOK_SECRET || '',
              intervalMs: positiveNumber(process.env.PROVIDER_NOTIFICATION_INTERVAL_MS, 15_000),
              timeoutMs: positiveNumber(process.env.PROVIDER_NOTIFICATION_TIMEOUT_MS, 60_000),
            })
            const stop = providerNotificationWorker.start()
            void providerNotificationWorker.runNow()
            return stop
          },
        })
      }

      await supervisor.start()
      logMailStatus('ready', supervisor, outboundWorker, snoozeWorker, reminderWorker, providerNotificationWorker)
      const statusIntervalMs = nonNegativeNumber(process.env.MAIL_STATUS_INTERVAL_MS, 300_000)
      if (statusIntervalMs > 0) {
        statusTimer = setInterval(
          () => logMailStatus('periodic', supervisor, outboundWorker, snoozeWorker, reminderWorker, providerNotificationWorker),
          statusIntervalMs,
        )
        statusTimer.unref?.()
      }
    } catch (err) {
      console.error('[mail] background mail services failed to start:', err)
      await shutdown('SIGTERM', 1)
    }
  })()

  const childCode = await childExit
  await shutdown('SIGTERM', requestedExitCode ?? Number(childCode ?? 1))
  await stopChild(bridgeChild)
  logMailStatus('stopped', supervisor, outboundWorker, snoozeWorker, reminderWorker, providerNotificationWorker)
  process.exit(requestedExitCode ?? Number(childCode ?? 1))
} else if (command === 'admin:create') {
  process.exit(await runOperatorWithBridge('scripts/create-admin.mjs', args))
} else if (command === 'domain:migrate') {
  process.exit(await runOperatorWithBridge('scripts/migrate-domain.mjs', args))
} else if (command === 'help' || command === '--help' || command === '-h') {
  console.log([
    'Caduceus container commands:',
    '  serve           Start the Caduceus API and frontend server',
    '  admin:create    Create the first admin for a domain',
    '  domain:migrate  Promote users from one domain suffix to another',
    '',
    'Examples:',
    '  docker run ghcr.io/d31ma/caduceus:latest',
    '  docker run -v caduceus-data:/data ghcr.io/d31ma/caduceus:latest admin:create --email=admin@example.com --domain=example.com',
    '  docker run -v caduceus-data:/data ghcr.io/d31ma/caduceus:latest domain:migrate --from=old.example --to=new.example --apply',
  ].join('\n'))
  process.exit(0)
} else {
  console.error(`Unsupported Caduceus container command: ${command}`)
  console.error('Allowed commands: serve, admin:create, domain:migrate')
  process.exit(64)
}

/**
 * Tachyon gives its handler processes no application variables; they read
 * `yon.*` settings from server/application.json and the project `.env`. Hand
 * them the settings this process holds, with paths made absolute because a
 * handler runs in its own working directory.
 */
async function writeHandlerSettings() {
  const { readFile, writeFile } = await import('node:fs/promises')
  const { resolve } = await import('node:path')
  const { ServerEnvironment } = await import('../tests/support/legacy-server/shared/runtime/ServerEnvironment.js')
  const target = resolve('.env')
  const existing = await readFile(target, 'utf8').catch(() => null)
  if (existing !== null && !existing.startsWith(ServerEnvironment.GENERATED_MARKER)) {
    throw new Error(`${target} was not written by CADUCEUS; move it aside so the entry point can write the handler settings`)
  }
  /** @type {Record<string, string | undefined>} */
  const environment = { ...process.env }
  for (const name of ServerEnvironment.HANDLER_SETTINGS) {
    const value = environment[name]
    if (value && /(_ROOT|_FILE|_BINARY|_EXEC_PATH)$/.test(name)) environment[name] = resolve(value)
  }
  const { text, skipped } = ServerEnvironment.dotenv(environment)
  for (const name of skipped) console.warn(`[runtime] ${name} spans several lines; set ${name}_FILE for the API handlers instead`)
  await writeFile(target, text, { mode: 0o600 })
}

async function startPersistenceBridge(spawn) {
  const binary = process.env.CADUCEUS_GO_BINARY || '/usr/local/bin/caduceus-go'
  process.env.FYLO_BRIDGE_ADDRESS ||= '127.0.0.1:9876'
  process.env.FYLO_BRIDGE_SECRET ||= randomBytes(32).toString('hex')
  process.env.FYLO_INCOMING_ROOT ||= `${process.env.FYLO_ROOT || '/data/caduceus'}/.caduceus-incoming`
  requireExternalAccountKeyring()
  await ensureDataRoots()
  await ensureIdentityDeployment(spawn, binary)
  const child = spawn(binary, ['fylo-bridge'], {
    stdio: 'inherit',
    env: process.env,
  })
  await waitForBridge(child, process.env.FYLO_BRIDGE_ADDRESS)
  return child
}

function requireExternalAccountKeyring() {
  if (process.env.EXTERNAL_ACCOUNT_CREDENTIAL_KEYS && process.env.EXTERNAL_ACCOUNT_ACTIVE_KEY) return
  throw new Error(
    'EXTERNAL_ACCOUNT_CREDENTIAL_KEYS and EXTERNAL_ACCOUNT_ACTIVE_KEY are required; see docs/CONFIGURATION.md',
  )
}

/**
 * Create the data directories on a volume that has none.
 *
 * The image pre-creates them, but a bind mount over /data hides everything the
 * image put there, and that is a normal way to run this. Creating a missing
 * directory is safe; the Go owner still refuses one it cannot write to.
 */
async function ensureDataRoots() {
  const { mkdir } = await import('node:fs/promises')
  for (const directory of [
    process.env.FYLO_ROOT,
    process.env.FYLO_INCOMING_ROOT,
    process.env.SESAME_DEPLOYMENT,
  ]) {
    if (directory) await mkdir(directory, { recursive: true, mode: 0o700 })
  }
}

/**
 * Create the identity deployment on a volume that has none.
 *
 * `caduceus auth init` is idempotent and refuses to touch an existing
 * deployment, so a restart never mints a new signing key over the one every
 * issued token was verified against. A volume that loses its keys is a
 * deliberate re-initialization, not a silent one: the new deployment ends
 * every session, and the operator sees it happen here.
 */
async function ensureIdentityDeployment(spawn, binary) {
  if (!process.env.SESAME_DEPLOYMENT) return
  const child = spawn(binary, ['auth', 'init'], { stdio: 'inherit', env: process.env })
  const code = await childResult(child)
  if (code !== 0) {
    throw new Error(`Identity deployment initialization failed with code ${code}`)
  }
}

async function runOperatorWithBridge(script, args) {
  const { spawn } = await import('node:child_process')
  const bridge = await startPersistenceBridge(spawn)
  const child = spawn(process.execPath, [script, ...args], {
    stdio: 'inherit',
    env: process.env,
  })
  const code = await childResult(child)
  await stopChild(bridge)
  return code
}

async function waitForBridge(child, address, timeoutMs = 30_000) {
  const { createConnection } = await import('node:net')
  const endpoint = parseTCPAddress(address)
  if (!endpoint) throw new Error(`Invalid Go FYLO owner address: ${address}`)
  const { host, port } = endpoint
  let spawnError
  child.once('error', error => {
    spawnError = error
  })
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (spawnError) {
      throw new Error(`Go FYLO owner failed to start: ${String(spawnError)}`)
    }
    if (child.exitCode !== null) {
      throw new Error(`Go FYLO owner exited with code ${child.exitCode}`)
    }
    const connected = await new Promise(resolve => {
      const socket = createConnection({ host, port })
      socket.setTimeout(500)
      socket.once('connect', () => {
        socket.destroy()
        resolve(true)
      })
      socket.once('timeout', () => {
        socket.destroy()
        resolve(false)
      })
      socket.once('error', () => resolve(false))
    })
    if (connected) return
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  child.kill('SIGTERM')
  throw new Error(`Go FYLO owner did not bind ${address} within ${timeoutMs}ms`)
}

function parseTCPAddress(address) {
  const value = String(address)
  const match = value.match(/^\[([^\]]+)\]:(\d+)$/) ?? value.match(/^([^:]+):(\d+)$/)
  if (!match) return undefined
  const port = Number(match[2])
  if (!Number.isSafeInteger(port) || port < 1 || port > 65535) return undefined
  return { host: match[1], port }
}

function childResult(child) {
  return new Promise(resolve => {
    child.once('error', () => resolve(1))
    child.once('exit', (code, signal) => resolve(code ?? (signal ? 1 : 0)))
  })
}

async function stopChild(child) {
  if (child.exitCode !== null) return
  child.kill('SIGTERM')
  await Promise.race([
    childResult(child),
    new Promise(resolve => setTimeout(resolve, 10_000)),
  ])
  if (child.exitCode === null) child.kill('SIGKILL')
}

/**
 * Poll the API server until it answers, so the inbound SMTP bridge only starts
 * once the webhook it delivers to is reachable.
 *
 * @param {string} apiUrl
 * @param {number} [timeoutMs]
 */
async function waitForApi(apiUrl, timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${apiUrl}/health`, {
        signal: AbortSignal.timeout(1000),
      })
      if (response.ok) return
    } catch {
      // The server may still be binding or loading routes.
    }
    await new Promise(r => setTimeout(r, 300))
  }
  throw new Error(`API server not reachable at ${apiUrl} after ${timeoutMs}ms`)
}

async function runBootstrapIfConfigured() {
  const hasMultiBootstrap = !!process.env.CADUCEUS_BOOTSTRAP_DOMAINS
  const hasSingleBootstrap = !!(
    process.env.CADUCEUS_ADMIN_EMAIL &&
    process.env.CADUCEUS_ADMIN_DOMAIN
  )
  if (!hasMultiBootstrap && !hasSingleBootstrap) return

  const originalArgv = process.argv
  try {
    process.argv = ['bun', 'scripts/create-admin.mjs']
    await import('./create-admin.mjs')
  } finally {
    process.argv = originalArgv
  }
}

/**
 * @param {import('smtp-server').SMTPServer} server
 * @returns {Promise<void>}
 */
function closeSmtpServer(server) {
  return new Promise((resolve) => {
    try {
      server.close(resolve)
    } catch {
      resolve()
    }
  })
}

/**
 * Emit a stable, secret-free status record that operators can collect from
 * container logs and compare before deciding whether to extract a Go daemon.
 *
 * @param {string} event
 * @param {ServiceSupervisor | undefined} supervisor
 * @param {OutboundQueueWorker | undefined} outboundWorker
 * @param {SnoozeWorker | undefined} snoozeWorker
 * @param {FollowUpReminderWorker | undefined} reminderWorker
 * @param {ProviderNotificationWorker | undefined} providerNotificationWorker
 */
function logMailStatus(event, supervisor, outboundWorker, snoozeWorker, reminderWorker, providerNotificationWorker) {
  console.log(`[mail:status] ${JSON.stringify({
    event,
    at: new Date().toISOString(),
    runtime: supervisor?.snapshot() ?? { state: 'not-started', components: {} },
    ...(outboundWorker ? { outboundQueue: outboundWorker.snapshot() } : {}),
    ...(snoozeWorker ? { snooze: snoozeWorker.snapshot() } : {}),
    ...(reminderWorker ? { followUpReminders: reminderWorker.snapshot() } : {}),
    ...(providerNotificationWorker ? { providerNewMail: providerNotificationWorker.snapshot() } : {}),
  })}`)
}

/** @param {string | undefined} value @param {number} fallback */
function positiveNumber(value, fallback) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

/** @param {string | undefined} value @param {number} fallback */
function nonNegativeNumber(value, fallback) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback
}

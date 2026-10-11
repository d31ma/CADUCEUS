import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { createConnection, createServer } from 'node:net'
import { join } from 'node:path'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { call } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'
import { TestServer } from '../support/TestServer.mjs'

// End to end: the Go binary's inbound MX (`caduceus inbound-smtp`) receives a
// message over SMTP and delivers it to the server's signed POST
// /inbound/webhook, which stores it in the recipient's inbox.

const SECRET = 'inbound-smtp-test-secret-0123456789abcdef'
const BINARY = process.env.CADUCEUS_TEST_BINARY || join(import.meta.dir, '..', '..', 'dist-bin', 'caduceus')

let app
let mx
let port

function freePort() {
  return new Promise((resolve, reject) => {
    const reservation = createServer()
    reservation.once('error', reject)
    reservation.listen(0, '127.0.0.1', () => {
      const chosen = reservation.address().port
      reservation.close(() => resolve(chosen))
    })
  })
}

async function startMx() {
  port = await freePort()
  const child = spawn(BINARY, ['inbound-smtp'], {
    env: {
      PATH: process.env.PATH,
      HOME: process.env.HOME,
      CADUCEUS_API_URL: TestServer.current.url,
      INBOUND_WEBHOOK_SECRET: SECRET,
      SMTP_INBOUND_HOST: '127.0.0.1',
      SMTP_INBOUND_PORT: String(port),
      MAIL_HOSTNAME: 'mx.inbound-smtp.test',
      MAIL_AUTH_VERIFY: 'off',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  let output = ''
  child.stdout.on('data', chunk => { output += chunk })
  child.stderr.on('data', chunk => { output += chunk })
  const deadline = Date.now() + 30_000
  while (!output.includes('inbound SMTP server listening')) {
    if (child.exitCode !== null || Date.now() > deadline) throw new Error(`caduceus inbound-smtp did not start:\n${output}`)
    await Bun.sleep(50)
  }
  return child
}

/**
 * One SMTP transaction; answers the reply to RCPT when it refuses, else the
 * reply to the end of DATA.
 * @param {string} recipient
 * @param {string} message
 */
async function send(recipient, message) {
  const socket = createConnection({ host: '127.0.0.1', port })
  socket.setEncoding('utf8')
  socket.setTimeout(60_000, () => socket.destroy())
  const replies = []
  let wake = () => {}
  let buffer = ''
  socket.on('data', chunk => {
    buffer += chunk
    let end
    while ((end = buffer.indexOf('\r\n')) >= 0) {
      const line = buffer.slice(0, end)
      buffer = buffer.slice(end + 2)
      if (!/^\d{3}-/.test(line)) { replies.push(line); wake() }
    }
  })
  socket.on('close', () => { replies.push('closed'); wake() })
  socket.on('error', () => {})
  const reply = async () => {
    while (!replies.length) await new Promise(resolve => { wake = resolve })
    return replies.shift()
  }
  const command = async text => { socket.write(`${text}\r\n`); return reply() }
  try {
    await reply()
    await command('EHLO client.sender.test')
    await command('MAIL FROM:<boss@sender.test>')
    const accepted = await command(`RCPT TO:<${recipient}>`)
    if (!accepted.startsWith('250')) return accepted
    await command('DATA')
    return await command(`${message.replace(/\r\n\./g, '\r\n..')}\r\n.`)
  } finally {
    socket.end('QUIT\r\n')
  }
}

function message(subject, body) {
  return [
    'From: Boss <boss@sender.test>',
    'To: Alice <alice@example.com>',
    `Subject: ${subject}`,
    'MIME-Version: 1.0',
    'Content-Type: text/plain; charset=utf-8',
    '',
    body,
  ].join('\r\n')
}

beforeAll(async () => {
  if (!existsSync(BINARY)) throw new Error(`${BINARY} is missing; run bun run go:build or set CADUCEUS_TEST_BINARY`)
  app = await setupRoutes({ INBOUND_WEBHOOK_SECRET: SECRET, SPAM_FILTER_DISABLED: 'true' })
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
  await app.domains.put({
    domain: 'example.com', inboundEnabled: true,
    routes: [{ id: 'all', match: '*@example.com', action: { type: 'store' }, enabled: true }],
  })
  mx = await startMx()
}, 300_000)

afterAll(async () => {
  mx?.kill()
  await teardownRoutes()
})

describe('caduceus inbound-smtp into POST /inbound/webhook', () => {
  it('stores a message received over SMTP in the recipient inbox', async () => {
    const reply = await send('alice@example.com', message('Inbound over SMTP', 'Hello over port 25.'))
    expect(reply).toStartWith('250')

    const inbox = await call(Routes.InboxController, 'GET', req({ token: token() }))
    const stored = inbox.find(email => email.subject === 'Inbound over SMTP')
    expect(stored).toBeDefined()
    const detail = await call(Routes.InboxIdController, 'GET', req({ token: token(), paths: { id: stored.id } }))
    expect(detail).toMatchObject({ folder: 'inbox', headerFrom: 'boss@sender.test' })
    expect(JSON.stringify(detail)).toContain('Hello over port 25.')
  }, 120_000)

  it('refuses a recipient on a domain the server does not host', async () => {
    const reply = await send('nobody@unhosted.test', message('Not hosted', 'Nobody here.'))
    expect(reply).toStartWith('550')
  }, 120_000)

  it('refuses permanently a message over the server request limit', async () => {
    const reply = await send('alice@example.com', message('Too large', 'x'.repeat(76).concat('\r\n').repeat(20_000)))
    expect(reply).toStartWith('552')
  }, 120_000)
})

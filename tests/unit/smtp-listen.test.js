/**
 * Bind both SMTP servers on an ephemeral port. This is the only way to cover
 * `start()` — it wires the `smtp-server` callbacks and calls listen — but the
 * listeners are closed immediately and never accept a connection.
 */
import { describe, it, expect, afterEach } from 'bun:test'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { InboundSmtpServer } from '@/features/delivery/services/smtp-inbound-server.js'
import { SubmissionServer } from '@/features/delivery/services/smtp-submission-server.js'
import { listenForConnections } from '@/platform/runtime/ListenableServer.ts'
import tlsOptions from 'smtp-server/lib/tls-options.js'

/** @type {Array<{ close: (cb?: () => void) => void }>} */
const opened = []
const track = server => { opened.push(server); return server }

afterEach(async () => {
  await Promise.all(opened.splice(0).map(server => new Promise(resolve => server.close(resolve))))
})

// A throwaway self-signed cert so the TLS paths run.
const selfSigned = () => {
  const { key, cert } = tlsOptions({})
  return { key, cert }
}

describe('InboundSmtpServer.start', () => {
  it('listens without TLS', async () => {
    const server = track(await new InboundSmtpServer({ secret: 'shh', port: 0, host: '127.0.0.1' }).start())
    expect(server.server.address().port).toBeGreaterThan(0)
  })

  it('listens with inline STARTTLS material', async () => {
    const { key, cert } = selfSigned()
    const server = track(await new InboundSmtpServer({
      secret: 'shh', port: 0, host: '127.0.0.1', tls: { key, cert },
    }).start())
    expect(server.server.address().port).toBeGreaterThan(0)
  })

  it('rejects a port it cannot bind', async () => {
    const first = track(await new InboundSmtpServer({ secret: 'shh', port: 0, host: '127.0.0.1' }).start())
    const port = first.server.address().port
    await expect(new InboundSmtpServer({ secret: 'shh', port, host: '127.0.0.1' }).start()).rejects.toThrow()
  })
})

describe('SubmissionServer.start', () => {
  it('listens with insecure auth allowed', async () => {
    const server = track(await new SubmissionServer({ allowInsecureAuth: true, port: 0, host: '127.0.0.1' }).start())
    expect(server.server.address().port).toBeGreaterThan(0)
  })

  it('listens with TLS material and an SNI callback', async () => {
    const root = mkdtempSync(join(tmpdir(), 'caduceus-sni-'))
    const { key, cert } = selfSigned()
    const keyFile = join(root, 'key.pem')
    const certFile = join(root, 'cert.pem')
    writeFileSync(keyFile, key)
    writeFileSync(certFile, cert)
    process.env.MAIL_TLS_KEY_FILE = keyFile
    process.env.MAIL_TLS_CERT_FILE = certFile
    try {
      const server = track(await new SubmissionServer({ port: 0, host: '127.0.0.1', tls: { key, cert } }).start())
      expect(server.server.address().port).toBeGreaterThan(0)
    } finally {
      delete process.env.MAIL_TLS_KEY_FILE
      delete process.env.MAIL_TLS_CERT_FILE
      rmSync(root, { recursive: true, force: true })
    }
  })
})

describe('listenForConnections', () => {
  it('rejects when listen throws synchronously', async () => {
    const server = {
      once() {}, off() {},
      listen() { throw new Error('cannot bind') },
    }
    await expect(listenForConnections(server, 0, '127.0.0.1')).rejects.toThrow('cannot bind')
  })
})

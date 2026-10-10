import { describe, it, expect, beforeEach, afterEach } from 'bun:test'
import { Readable } from 'node:stream'
import { InboundSmtpServer } from '@/features/delivery/services/smtp-inbound-server.js'
import { SubmissionServer } from '@/features/delivery/services/smtp-submission-server.js'

const RAW = ['From: bob@other.com', 'To: alice@example.com', 'Subject: Hi', '', 'body'].join('\r\n')

const session = (over = {}) => ({
  remoteAddress: '203.0.113.5',
  clientHostname: 'mail.other.com',
  envelope: {
    mailFrom: { address: 'bob@other.com' },
    rcptTo: [{ address: 'alice@example.com' }],
    ...over.envelope,
  },
  ...over,
})

/** Capture the SMTP error a handler passes to its callback. */
const capture = () => {
  const calls = []
  const callback = err => calls.push(err)
  return { calls, callback, code: () => calls[0]?.responseCode, message: () => calls[0]?.message }
}

let originalFetch
beforeEach(() => { originalFetch = globalThis.fetch })
afterEach(() => { globalThis.fetch = originalFetch })

describe('InboundSmtpServer construction', () => {
  it('refuses to start without a webhook secret', () => {
    expect(() => new InboundSmtpServer({ secret: '' })).toThrow(/INBOUND_WEBHOOK_SECRET/)
  })

  it('applies defaults and strips a trailing slash from the api url', () => {
    const server = new InboundSmtpServer({ secret: 's', apiUrl: 'http://api.local/' })
    expect(server.apiUrl).toBe('http://api.local')
    expect(server.port).toBe(25)
    expect(server.host).toBe('0.0.0.0')
    expect(server.verifyAuth).toBe(true)
  })

  it('honours MAIL_AUTH_VERIFY=off', () => {
    process.env.MAIL_AUTH_VERIFY = 'off'
    try {
      expect(new InboundSmtpServer({ secret: 's' }).verifyAuth).toBe(false)
    } finally {
      delete process.env.MAIL_AUTH_VERIFY
    }
  })
})

describe('InboundSmtpServer.loadTls', () => {
  afterEach(() => {
    delete process.env.MAIL_TLS_KEY
    delete process.env.MAIL_TLS_CERT
    delete process.env.MAIL_TLS_KEY_FILE
  })

  it('returns null without both halves', () => {
    expect(InboundSmtpServer.loadTls()).toBeNull()
    process.env.MAIL_TLS_KEY = 'KEY'
    expect(InboundSmtpServer.loadTls()).toBeNull()
  })

  it('returns inline PEM when both are set', () => {
    process.env.MAIL_TLS_KEY = 'KEY'
    process.env.MAIL_TLS_CERT = 'CERT'
    expect(InboundSmtpServer.loadTls()).toEqual({ key: 'KEY', cert: 'CERT' })
  })

  it('reads an unreadable file as empty rather than throwing', () => {
    expect(InboundSmtpServer.safeRead('/nonexistent/path.pem')).toBe('')
  })
})

describe('InboundSmtpServer.readStream', () => {
  it('concatenates chunks', async () => {
    const buf = await InboundSmtpServer.readStream(Readable.from([Buffer.from('ab'), Buffer.from('cd')]), 100)
    expect(buf.toString()).toBe('abcd')
  })

  it('drops chunks past the size cap', async () => {
    const buf = await InboundSmtpServer.readStream(Readable.from([Buffer.from('abc'), Buffer.from('defgh')]), 3)
    expect(buf.toString()).toBe('abc')
  })

  it('rejects on a stream error', async () => {
    const stream = new Readable({ read() { this.destroy(new Error('boom')) } })
    await expect(InboundSmtpServer.readStream(stream, 100)).rejects.toThrow('boom')
  })
})

describe('InboundSmtpServer.reject', () => {
  it('passes an error carrying the SMTP response code', () => {
    const { callback, code, message } = capture()
    InboundSmtpServer.reject(callback, 451, 'try later')
    expect(code()).toBe(451)
    expect(message()).toBe('try later')
  })
})

describe('InboundSmtpServer.processMessage', () => {
  const server = (over = {}) => Object.assign(new InboundSmtpServer({ secret: 'shh', verifyAuth: false }), over)
  /** Stand-in for mailparser — the real one is not what these branches exercise. */
  const parse = async raw => (String(raw).includes('Subject:')
    ? { subject: 'Hi', from: { value: [{ address: 'bob@other.com' }] } }
    : {})
  const run = (s, sess = session(), raw = RAW, exceeded = false) =>
    s.processMessage(Buffer.from(raw), sess, exceeded, parse)

  it('rejects an oversized message', async () => {
    expect(await run(server(), session(), RAW, true))
      .toEqual({ code: 552, message: 'Message exceeds maximum permitted size' })
  })

  it('accepts a hosted recipient', async () => {
    const delivered = []
    const s = server({ deliverViaWebhook: async p => { delivered.push(p); return { ok: true, accepted: true } } })
    expect(await run(s)).toBeNull()
    expect(delivered[0].recipient).toBe('alice@example.com')
    expect(delivered[0].subject).toBe('Hi')
    expect(delivered[0].sender).toBe('bob@other.com')
  })

  it('falls back to the header sender without an envelope sender', async () => {
    const delivered = []
    const s = server({ deliverViaWebhook: async p => { delivered.push(p); return { ok: true, accepted: true } } })
    await run(s, session({ envelope: { mailFrom: null, rcptTo: [{ address: 'alice@example.com' }] } }))
    expect(delivered[0].sender).toBe('bob@other.com')
  })

  it('defaults the subject for an unparseable message', async () => {
    const delivered = []
    const s = server({ deliverViaWebhook: async p => { delivered.push(p); return { ok: true, accepted: true } } })
    await run(s, session(), '')
    expect(delivered[0].subject).toBe('(no subject)')
  })

  it('survives a parser that throws', async () => {
    const delivered = []
    const s = server({ deliverViaWebhook: async p => { delivered.push(p); return { ok: true, accepted: true } } })
    await s.processMessage(Buffer.from(RAW), session(), false, async () => { throw new Error('bad mime') })
    expect(delivered[0].subject).toBe('(no subject)')
  })

  it('returns a temporary failure when the webhook is unreachable', async () => {
    const s = server({ deliverViaWebhook: async () => ({ ok: false, accepted: false, reason: 'webhook 500' }) })
    expect(await run(s)).toEqual({ code: 451, message: 'Temporary local delivery failure, try again later' })
  })

  it('refuses a recipient on an unhosted domain', async () => {
    const s = server({ deliverViaWebhook: async () => ({ ok: true, accepted: false, reason: 'domain not configured' }) })
    expect(await run(s)).toEqual({ code: 550, message: 'No such user here' })
  })

  it('accepts when a message is rejected for a reason other than routing', async () => {
    const s = server({ deliverViaWebhook: async () => ({ ok: true, accepted: false, reason: 'suppressed sender' }) })
    expect(await run(s)).toBeNull()
  })

  it('prepends the authentication-results header when verification is on', async () => {
    const delivered = []
    const s = server({
      verifyAuth: true,
      authenticateMessage: async () => ({ headerText: 'Authentication-Results: caduceus; spf=fail\r\n', spamVerdict: 'fail', spamScore: 6, summary: '' }),
      deliverViaWebhook: async p => { delivered.push(p); return { ok: true, accepted: true } },
    })
    await run(s)
    expect(delivered[0].body.startsWith('Authentication-Results:')).toBe(true)
    expect(delivered[0].spamVerdict).toBe('fail')
    expect(delivered[0].spamScore).toBe(6)
  })

  it('delivers one copy per recipient', async () => {
    const delivered = []
    const s = server({ deliverViaWebhook: async p => { delivered.push(p); return { ok: true, accepted: true } } })
    const two = session({ envelope: { mailFrom: { address: 'bob@other.com' }, rcptTo: [{ address: 'a@example.com' }, { address: 'b@example.com' }] } })
    await run(s, two)
    expect(delivered.map(d => d.recipient)).toEqual(['a@example.com', 'b@example.com'])
  })
})

describe('InboundSmtpServer.deliverViaWebhook', () => {
  const server = () => new InboundSmtpServer({ secret: 'shh', apiUrl: 'http://api.local' })

  it('signs the payload and reads the pipeline decision', async () => {
    let seen
    globalThis.fetch = async (url, init) => {
      seen = { url, init }
      return new Response(JSON.stringify({ accepted: true }), { headers: { 'Content-Type': 'application/json' } })
    }
    const res = await server().deliverViaWebhook({ recipient: 'a@example.com', sender: 'b@x.com', subject: 's', body: 'b' })
    expect(res).toEqual({ ok: true, accepted: true, reason: undefined })
    expect(seen.url).toBe('http://api.local/inbound/webhook')
    expect(seen.init.headers['X-Caduceus-Signature']).toMatch(/^[0-9a-f]{64}$/)
  })

  it('reports a non-2xx webhook as a failure', async () => {
    globalThis.fetch = async () => new Response('nope', { status: 500 })
    const res = await server().deliverViaWebhook({ recipient: 'a@example.com', sender: 'b@x.com', subject: 's', body: 'b' })
    expect(res).toEqual({ ok: false, accepted: false, reason: 'webhook 500' })
  })

  it('treats an unparseable body as accepted', async () => {
    globalThis.fetch = async () => new Response('not json')
    const res = await server().deliverViaWebhook({ recipient: 'a@example.com', sender: 'b@x.com', subject: 's', body: 'b' })
    expect(res.accepted).toBe(true)
  })
})

describe('InboundSmtpServer.authenticateMessage', () => {
  const server = () => new InboundSmtpServer({ secret: 'shh' })
  const raw = Buffer.from(RAW)

  it('passes the session envelope to the verifier', async () => {
    let opts
    await server().authenticateMessage(raw, session(), async (_raw, o) => { opts = o; return {} })
    expect(opts.ip).toBe('203.0.113.5')
    expect(opts.helo).toBe('mail.other.com')
    expect(opts.sender).toBe('bob@other.com')
  })

  it('scores a DMARC failure as an authenticated spoof', async () => {
    const res = await server().authenticateMessage(raw, session(), async () => ({
      headers: 'Authentication-Results: x\r\n',
      spf: { status: { result: 'fail' } },
      dmarc: { status: { result: 'fail' } },
      dkim: { results: [] },
    }))
    expect(res.spamVerdict).toBe('fail')
    expect(res.spamScore).toBe(6)
    expect(res.summary).toBe('spf=fail dkim=fail/none dmarc=fail')
  })

  it('adds suspicion for a hard SPF failure on an unsigned message', async () => {
    const res = await server().authenticateMessage(raw, session(), async () => ({
      spf: { status: { result: 'fail' } },
      dmarc: { status: { result: 'none' } },
      dkim: { results: [] },
    }))
    expect(res.spamVerdict).toBeUndefined()
    expect(res.spamScore).toBe(3)
  })

  it('leaves a signed message unscored despite an SPF failure', async () => {
    const res = await server().authenticateMessage(raw, session(), async () => ({
      spf: { status: { result: 'fail' } },
      dkim: { results: [{ status: { result: 'pass' } }] },
    }))
    expect(res.spamScore).toBeUndefined()
    expect(res.summary).toContain('dkim=pass')
  })

  it('reports n/a for missing results', async () => {
    const res = await server().authenticateMessage(raw, session(), async () => ({}))
    expect(res.summary).toBe('spf=n/a dkim=fail/none dmarc=n/a')
    expect(res.headerText).toBe('')
  })

  it('delivers unverified when verification throws', async () => {
    const res = await server().authenticateMessage(raw, session(), async () => { throw new Error('dns timeout') })
    expect(res).toEqual({ headerText: '', summary: 'auth=error' })
  })
})

describe('SubmissionServer construction', () => {
  it('requires TLS unless insecure auth is explicitly allowed', () => {
    expect(() => new SubmissionServer({})).toThrow(/requires TLS/)
    expect(new SubmissionServer({ allowInsecureAuth: true }).port).toBe(587)
    expect(new SubmissionServer({ allowInsecureAuth: true, secure: true }).port).toBe(465)
  })

  it('returns null TLS without both halves', () => {
    expect(SubmissionServer.loadTls()).toBeNull()
    expect(SubmissionServer.safeRead('/nonexistent/path.pem')).toBe('')
  })
})

describe('SubmissionServer.onAuth', () => {
  const server = () => new SubmissionServer({ allowInsecureAuth: true, apiUrl: 'http://api.local' })
  const done = () => {
    let resolve
    const promise = new Promise(r => { resolve = r })
    return { promise, callback: (err, res) => resolve({ err, res }) }
  }

  it('refuses every submission login until application passwords ship', async () => {
    // SMTP AUTH carries one secret and a session needs multi-factor assurance,
    // so there is no password that can succeed here. Refusing is the correct
    // answer, and it must not depend on the network.
    globalThis.fetch = async () => { throw new Error('the network must not be reached') }
    for (const credentials of [
      { username: 'Alice@Example.com', password: 'correct horse battery staple' },
      { username: 'a@b.com', password: '' },
      {},
    ]) {
      const { promise, callback } = done()
      server().onAuth(credentials, callback)
      const { err, res } = await promise
      expect(res).toBeUndefined()
      expect(err.message).toMatch(/application password/)
    }
  })
})

describe('SubmissionServer.onData', () => {
  const server = () => new SubmissionServer({ allowInsecureAuth: true, apiUrl: 'http://api.local' })
  const user = JSON.stringify({ email: 'alice@example.com', token: 'tok' })

  /** Run onData and resolve once its callback fires. */
  const run = (srv, raw, sess, parser) => new Promise(resolve => {
    srv.onData(Readable.from([Buffer.from(raw)]), sess, err => resolve(err), parser)
  })

  const parsed = (over = {}) => async () => ({
    subject: 'Hello',
    text: 'body',
    to: { value: [{ address: 'Bob@Other.com' }] },
    ...over,
  })

  it('rejects an oversized submission', async () => {
    const srv = server()
    srv.maxSize = 2
    const stream = Readable.from([Buffer.from('too long')])
    const err = await new Promise(resolve => {
      const s = Object.assign(stream, { sizeExceeded: true })
      srv.onData(s, { user, envelope: { mailFrom: null, rcptTo: [] } }, resolve, parsed())
    })
    expect(err.responseCode).toBe(552)
  })

  it('refuses an envelope sender that is not the authenticated user', async () => {
    const sess = { user, envelope: { mailFrom: { address: 'mallory@evil.com' }, rcptTo: [] } }
    const err = await run(server(), RAW, sess, parsed())
    expect(err.responseCode).toBe(550)
    expect(err.message).toMatch(/does not match the authenticated user/)
  })

  it('submits header and envelope recipients, separating bcc', async () => {
    let body
    globalThis.fetch = async (url, init) => { body = JSON.parse(init.body); return new Response('{}') }
    const sess = {
      user,
      envelope: {
        mailFrom: { address: 'alice@example.com' },
        rcptTo: [{ address: 'bob@other.com' }, { address: 'carol@other.com' }, { address: 'dan@other.com' }],
      },
    }
    const err = await run(server(), RAW, sess, parsed({ cc: { value: [{ address: 'carol@other.com' }] } }))
    expect(err).toBeUndefined()
    expect(body.to).toEqual(['bob@other.com'])
    expect(body.cc).toEqual(['carol@other.com'])
    expect(body.bcc).toEqual(['dan@other.com'])
  })

  it('defaults the subject and carries html', async () => {
    let body
    globalThis.fetch = async (url, init) => { body = JSON.parse(init.body); return new Response('{}') }
    const sess = { user, envelope: { mailFrom: null, rcptTo: [{ address: 'bob@other.com' }] } }
    await run(server(), RAW, sess, parsed({ subject: '', text: '', html: '<p>hi</p>' }))
    expect(body.subject).toBe('(no subject)')
    expect(body.html).toBe('<p>hi</p>')
    expect(body.text).toBeUndefined()
  })

  it('maps a rate limit to a temporary SMTP failure', async () => {
    globalThis.fetch = async () => new Response('{}', { status: 429 })
    const sess = { user, envelope: { mailFrom: null, rcptTo: [] } }
    expect((await run(server(), RAW, sess, parsed())).responseCode).toBe(452)
  })

  it('maps a suppressed recipient to a permanent rejection', async () => {
    globalThis.fetch = async () => new Response('{}', { status: 422 })
    const sess = { user, envelope: { mailFrom: null, rcptTo: [] } }
    expect((await run(server(), RAW, sess, parsed())).responseCode).toBe(550)
  })

  it('maps any other rejection to 554', async () => {
    globalThis.fetch = async () => new Response('{}', { status: 400 })
    const sess = { user, envelope: { mailFrom: null, rcptTo: [] } }
    expect((await run(server(), RAW, sess, parsed())).responseCode).toBe(554)
  })

  it('reports a temporary failure when parsing throws', async () => {
    const sess = { user, envelope: { mailFrom: null, rcptTo: [] } }
    const failing = async () => { throw new Error('bad mime') }
    expect((await run(server(), RAW, sess, failing)).responseCode).toBe(451)
  })
})

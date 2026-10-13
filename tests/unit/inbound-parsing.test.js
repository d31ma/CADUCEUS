import { describe, it, expect } from 'bun:test'
import InboundService from '@/services/InboundService.js'
import { InboundRules } from '@/shared/delivery/InboundRules.js'
import { Providers } from '../../scripts/support/Providers.mjs'

/** Parsing needs no collaborators — build a bare instance. */
const parser = () => /** @type {InboundService} */ (Providers.instance(InboundService))

const rfc = (headers, body) => [...headers, '', body].join('\r\n')

describe('parseMessage — non-MIME input', () => {
  it('returns empty fields for an empty body', async () => {
    expect(await parser().parseMessage('')).toEqual({ text: '', html: '', attachments: [], headers: {} })
  })

  it('keeps plain text verbatim', async () => {
    expect((await parser().parseMessage('just a note')).text).toBe('just a note')
  })

  it('detects bare HTML and derives the text', async () => {
    const res = await parser().parseMessage('<p>Hello <b>world</b></p>')
    expect(res.html).toBe('<p>Hello <b>world</b></p>')
    expect(res.text).toBe('Hello world')
  })

  it('recognises void HTML tags', async () => {
    expect((await parser().parseMessage('one<br>two')).html).toBe('one<br>two')
  })

  it('does not treat a colon-prefixed first line as headers', async () => {
    const raw = 'Note: lunch\n\nTomorrow at noon'
    expect((await parser().parseMessage(raw)).headers).toEqual({})
  })

  it('strips script and style blocks and decodes entities', async () => {
    const raw = '<div><style>p{}</style><script>x()</script>a &amp; b &lt;c&gt; &quot;d&quot; &#39;e&#39; &nbsp;f</div>'
    expect((await parser().parseMessage(raw)).text).toBe('a & b <c> "d" \'e\' f')
  })
})

describe('parseMessage — simple RFC messages', () => {
  it('exposes headers lower-cased', async () => {
    const res = await parser().parseMessage(rfc(['Subject: Hi', 'X-Spam-Flag: YES'], 'body'))
    expect(res.headers).toEqual({ subject: 'Hi', 'x-spam-flag': 'YES' })
    expect(res.text).toBe('body')
  })

  it('unfolds continued header lines', async () => {
    const raw = 'Subject: part one\r\n  part two\r\nTo: a@b.com\r\n\r\nbody'
    expect((await parser().parseMessage(raw)).headers.subject).toBe('part one part two')
  })

  it('treats a text/html body as HTML', async () => {
    const res = await parser().parseMessage(rfc(['Content-Type: text/html'], '<p>Hi there</p>'))
    expect(res.html).toBe('<p>Hi there</p>')
    expect(res.text).toBe('Hi there')
  })

  it('treats a block with a malformed line as plain text, not headers', async () => {
    const res = await parser().parseMessage(rfc(['Subject: Hi', 'nonsense'], 'body'))
    expect(res.headers).toEqual({})
  })
})

describe('parseMessage — full MIME via mailparser', () => {
  const multipart = (parts, boundary = 'BOUND') => [
    'From: bob@other.com',
    'Subject: With parts',
    `Content-Type: multipart/mixed; boundary="${boundary}"`,
    '',
    ...parts.map(p => `--${boundary}\r\n${p}`),
    `--${boundary}--`,
  ].join('\r\n')

  it('returns text, html, attachments, and flattened headers', async () => {
    const res = await parser().parseMessage(multipart([
      'Content-Type: text/plain\r\n\r\nplain part',
      'Content-Type: text/html\r\n\r\n<p>html part</p>',
      [
        'Content-Type: text/plain; name="note.txt"',
        'Content-Disposition: attachment; filename="note.txt"',
        'Content-Transfer-Encoding: base64',
        '',
        Buffer.from('hello attachment').toString('base64'),
      ].join('\r\n'),
    ]))
    expect(res.text).toContain('plain part')
    expect(res.html).toContain('html part')
    expect(res.attachments[0].filename).toBe('note.txt')
    expect(Buffer.from(res.attachments[0].content).toString()).toBe('hello attachment')
    expect(res.headers.subject).toBe('With parts')
    expect(typeof res.headers.from).toBe('string')
  })

  it('flattens structured From and Reply-To address headers into mailboxes', async () => {
    const raw = [
      'From: Billing Team <billing@vendor.example>',
      'Reply-To: Accounts <accounts@vendor.example>',
      'Content-Type: text/plain; charset=utf-8',
      'Content-Transfer-Encoding: quoted-printable',
      '',
      'invoice',
    ].join('\r\n')
    const res = await parser().parseMessage(raw)
    expect(res.headers.from).toBe('billing@vendor.example')
    expect(res.headers['reply-to']).toBe('accounts@vendor.example')
  })

  it('encodes a quoted-printable body', async () => {
    const raw = [
      'Subject: Encoded',
      'Content-Type: text/plain; charset=utf-8',
      'Content-Transfer-Encoding: quoted-printable',
      '',
      'caf=C3=A9',
    ].join('\r\n')
    expect((await parser().parseMessage(raw)).text.trim()).toBe('café')
  })

  it('derives text from an html-only multipart', async () => {
    const res = await parser().parseMessage(multipart(['Content-Type: text/html\r\n\r\n<p>only <b>html</b></p>']))
    expect(res.text.trim()).toBe('only html')
  })

  it('parses binary EML input without first coercing the source bytes to text', async () => {
    const raw = Buffer.concat([
      Buffer.from('From: sender@other.com\r\nSubject: Binary import\r\nContent-Type: application/octet-stream; name="bytes.bin"\r\nContent-Disposition: attachment; filename="bytes.bin"\r\nContent-Transfer-Encoding: base64\r\n\r\n'),
      Buffer.from(Buffer.from([0xff, 0x00, 0x80]).toString('base64')),
    ])
    const result = await parser().parseMessage(raw)
    expect(result.headers.subject).toBe('Binary import')
    expect(Buffer.from(result.attachments[0].content)).toEqual(Buffer.from([0xff, 0x00, 0x80]))
  })
})

describe('parseMultipartMessage — the no-mailparser fallback', () => {
  const multipart = (parts, boundary = 'BOUND') => [
    'From: bob@other.com',
    'Subject: With parts',
    `Content-Type: multipart/mixed; boundary="${boundary}"`,
    '',
    ...parts.map(p => `--${boundary}\r\n${p}`),
    `--${boundary}--`,
  ].join('\r\n')

  it('returns null without a header/body separator', () => {
    expect(InboundRules.parseMultipartMessage('no separator here')).toBeNull()
  })

  it('returns null when the content-type declares no boundary', () => {
    expect(InboundRules.parseMultipartMessage('Content-Type: multipart/mixed\r\n\r\nbody')).toBeNull()
  })

  it('accepts an unquoted boundary', () => {
    const raw = [
      'Content-Type: multipart/mixed; boundary=SIMPLE',
      '',
      '--SIMPLE',
      'Content-Type: text/plain',
      '',
      'hello',
      '--SIMPLE--',
    ].join('\r\n')
    expect(InboundRules.parseMultipartMessage(raw).text).toBe('hello')
  })

  it('collects the text and html alternatives and the top headers', () => {
    const res = InboundRules.parseMultipartMessage(multipart([
      'Content-Type: text/plain\r\n\r\nplain part',
      'Content-Type: text/html\r\n\r\n<p>html part</p>',
    ]))
    expect(res.text).toContain('plain part')
    expect(res.html).toContain('<p>html part</p>')
    expect(res.headers.from).toBe('bob@other.com')
  })

  it('extracts a base64 attachment', () => {
    const [attachment] = InboundRules.parseMultipartMessage(multipart([
      'Content-Type: text/plain\r\n\r\nsee attached',
      [
        'Content-Type: text/plain; name="note.txt"',
        'Content-Disposition: attachment; filename="note.txt"',
        'Content-Transfer-Encoding: base64',
        '',
        Buffer.from('hello attachment').toString('base64'),
      ].join('\r\n'),
    ])).attachments
    expect(attachment.filename).toBe('note.txt')
    expect(Buffer.from(attachment.content).toString()).toBe('hello attachment')
    expect(attachment.disposition).toBe('attachment')
  })

  it('reads an unencoded inline attachment with a content id', () => {
    const [attachment] = InboundRules.parseMultipartMessage(multipart([
      [
        'Content-Type: image/png',
        'Content-Disposition: inline; filename=logo.png',
        'Content-ID: <logo@example.com>',
        '',
        'RAWBYTES',
      ].join('\r\n'),
    ])).attachments
    expect(attachment.disposition).toBe('inline')
    expect(attachment.contentId).toBe('logo@example.com')
    expect(attachment.contentType).toBe('image/png')
    expect(Buffer.from(attachment.content).toString()).toBe('RAWBYTES')
  })

  it('falls back to the content-type name when no filename is given', () => {
    const res = InboundRules.parseMultipartMessage(multipart([
      'Content-Type: application/pdf; name="report.pdf"\r\nContent-Disposition: attachment\r\n\r\nPDF',
    ]))
    expect(res.attachments[0].filename).toBe('report.pdf')
  })

  it('names an unnamed attachment part', () => {
    const res = InboundRules.parseMultipartMessage(multipart([
      'Content-Type: application/pdf\r\nContent-Disposition: attachment\r\n\r\nPDF',
    ]))
    expect(res.attachments[0].filename).toBe('attachment')
  })

  it('skips parts with no header/body separator', () => {
    const res = InboundRules.parseMultipartMessage(multipart(['Content-Type: text/plain\r\n\r\nkept', 'malformed-part']))
    expect(res.text).toContain('kept')
    expect(res.attachments).toEqual([])
  })

  it('derives text from html when there is no plain part', () => {
    expect(InboundRules.parseMultipartMessage(multipart(['Content-Type: text/html\r\n\r\n<p>only html</p>'])).text).toBe('only html')
  })

  it('ignores parts with an unhandled content type', () => {
    const res = InboundRules.parseMultipartMessage(multipart(['Content-Type: application/json\r\n\r\n{}']))
    expect(res.attachments).toEqual([])
    expect(res.text).toBe(multipart(['Content-Type: application/json\r\n\r\n{}']))
  })
})

describe('applyInboxRules', () => {
  /** Collect what the rule engine did instead of touching Fylo or SMTP. */
  const spyService = (rules) => {
    const patched = []
    const forwarded = []
    const service = Providers.instance(InboundService)
    service.rules = { listEnabledForMailbox: async () => rules }
    service.emails = {
      findById: async id => [`doc-${id}`, { id }],
      update: async (docId, update) => { patched.push({ docId, update }) },
    }
    service.smtp = { send: async (message) => { forwarded.push(message) } }
    return { service, patched, forwarded }
  }

  const meta = { sender: 'bob@other.com', recipient: 'alice@example.com', subject: 'Weekly report', body: 'numbers inside' }

  it('applies a rule with no conditions', async () => {
    const { service, patched } = spyService([{ actions: [{ type: 'folder', folder: 'reports' }] }])
    await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
    expect(patched[0]).toEqual({ docId: 'doc-m1', update: { folder: 'reports' } })
  })

  it('matches each condition field', async () => {
    for (const [field, value] of [['from', 'bob@other.com'], ['to', 'alice@example.com'], ['subject', 'weekly'], ['body', 'numbers']]) {
      const { service, patched } = spyService([{
        conditionMatch: 'all',
        conditions: [{ field, op: field === 'from' || field === 'to' ? 'equals' : 'contains', value }],
        actions: [{ type: 'folder', folder: 'hit' }],
      }])
      await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
      expect(patched).toHaveLength(1)
    }
  })

  it('ignores an unknown condition field', async () => {
    const { service, patched } = spyService([{
      conditions: [{ field: 'priority', op: 'equals', value: 'high' }],
      actions: [{ type: 'folder', folder: 'x' }],
    }])
    await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
    expect(patched).toEqual([])
  })

  it('supports every string operator', async () => {
    const cases = [
      ['contains', 'weekly', true],
      ['equals', 'weekly report', true],
      ['startsWith', 'weekly', true],
      ['endsWith', 'report', true],
      ['notContains', 'monthly', true],
      ['notEquals', 'monthly report', true],
      ['matches', 'weekly', false],
      ['equals', 'nope', false],
    ]
    for (const [op, value, expected] of cases) {
      const { service, patched } = spyService([{
        conditions: [{ field: 'subject', op, value }],
        actions: [{ type: 'folder', folder: 'x' }],
      }])
      await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
      expect(patched.length > 0).toBe(expected)
    }
  })

  it('requires every condition under conditionMatch all', async () => {
    const { service, patched } = spyService([{
      conditionMatch: 'all',
      conditions: [{ field: 'subject', op: 'contains', value: 'weekly' }, { field: 'from', op: 'equals', value: 'nobody@x.com' }],
      actions: [{ type: 'folder', folder: 'x' }],
    }])
    await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
    expect(patched).toEqual([])
  })

  it('requires only one condition under conditionMatch any', async () => {
    const { service, patched } = spyService([{
      conditionMatch: 'any',
      conditions: [{ field: 'subject', op: 'contains', value: 'weekly' }, { field: 'from', op: 'equals', value: 'nobody@x.com' }],
      actions: [{ type: 'folder', folder: 'x' }],
    }])
    await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
    expect(patched).toHaveLength(1)
  })

  it('forwards a copy', async () => {
    const { service, forwarded } = spyService([{ actions: [{ type: 'forward', to: 'archive@example.com' }] }])
    await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
    expect(forwarded[0].to).toEqual(['archive@example.com'])
    expect(forwarded[0].subject).toBe('Fwd: Weekly report')
  })

  it('survives a failing forward', async () => {
    const { service } = spyService([{ actions: [{ type: 'forward', to: 'a@b.com' }] }])
    service.smtp = { send: async () => { throw new Error('smtp down') } }
      await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
  })

  it('survives a failing rule lookup', async () => {
    const service = Providers.instance(InboundService)
    service.rules = { listEnabledForMailbox: async () => { throw new Error('fylo down') } }
    await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
  })

  it('applies state actions in one update and stops later rules', async () => {
    const { service, patched } = spyService([
      { stopProcessing: true, actions: [{ type: 'markRead', value: true }, { type: 'star', value: true }] },
      { actions: [{ type: 'folder', folder: 'should-not-run' }] },
    ])
    await service.applyInboxRules('m1', 'alice@example.com', 'example.com', meta)
    expect(patched).toEqual([{ docId: 'doc-m1', update: { read: true, starred: true } }])
  })
})

describe('matchRoute', () => {
  const service = () => /** @type {InboundService} */ (Providers.instance(InboundService))

  it('skips disabled routes', () => {
    expect(service().matchRoute([{ id: 'r', match: '*', enabled: false }], 'a@b.com')).toBeNull()
  })

  it('matches a catch-all, a domain glob, and an exact address', () => {
    const catchAll = { id: '1', match: '*', enabled: true }
    const domain = { id: '2', match: '*@b.com', enabled: true }
    const exact = { id: '3', match: 'a@b.com', enabled: true }
    expect(service().matchRoute([catchAll], 'a@b.com')).toBe(catchAll)
    expect(service().matchRoute([domain], 'a@b.com')).toBe(domain)
    expect(service().matchRoute([exact], 'a@b.com')).toBe(exact)
  })

  it('returns null when nothing matches', () => {
    expect(service().matchRoute([{ id: '1', match: 'z@z.com', enabled: true }], 'a@b.com')).toBeNull()
  })
})

describe('applyRouteAction', () => {
  const service = (over = {}) => Object.assign(Providers.instance(InboundService), over)
  const meta = { messageId: 'm1', sender: 'bob@other.com', recipient: 'alice@example.com', subject: 'Hi' }

  it('posts to a webhook', async () => {
    const calls = []
    const original = globalThis.fetch
    globalThis.fetch = async (url, init) => { calls.push({ url, init }); return new Response('ok') }
    try {
      await service().applyRouteAction({ action: { type: 'webhook', url: 'https://hook.example/x' } }, meta)
      expect(calls[0].url).toBe('https://hook.example/x')
    } finally {
      globalThis.fetch = original
    }
  })

  it('swallows a failing webhook', async () => {
    const original = globalThis.fetch
    globalThis.fetch = async () => { throw new Error('network down') }
    try {
      await service().applyRouteAction({ action: { type: 'webhook', url: 'https://hook.example/x' } }, meta)
    } finally {
      globalThis.fetch = original
    }
  })

  it('forwards to a different address', async () => {
    const sent = []
    await service({ smtp: { send: async m => { sent.push(m) } } })
      .applyRouteAction({ action: { type: 'forward', to: 'archive@example.com' } }, meta)
    expect(sent[0].to).toEqual(['archive@example.com'])
  })

  it('does not forward back to the recipient', async () => {
    const sent = []
    await service({ smtp: { send: async m => { sent.push(m) } } })
      .applyRouteAction({ action: { type: 'forward', to: meta.recipient } }, meta)
    expect(sent).toEqual([])
  })

  it('is a no-op for store and drop', async () => {
    await service().applyRouteAction({ action: { type: 'store' } }, meta)
    await service().applyRouteAction({ action: { type: 'drop' } }, meta)
  })
})

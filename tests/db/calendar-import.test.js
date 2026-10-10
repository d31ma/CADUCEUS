import { describe, it, expect, beforeEach, afterEach } from 'bun:test'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { DatabaseRepository } from '@/platform/persistence/fylo.js'
import { CalendarEventsRepository } from '@/features/calendar/repositories/calendar-events.js'
import { CalendarImportService } from '@/features/calendar/services/calendar-import.js'
import { UsersRepository } from '@/features/users/repositories/users.js'
import { InboundService } from '@/features/delivery/services/inbound.js'
import { EmailsRepository } from '@/features/mailbox/repositories/emails.js'
import { AttachmentsRepository } from '@/features/mailbox/repositories/attachments.js'
import { RulesRepository } from '@/features/mailbox/repositories/rules.js'
import { SuppressedRepository } from '@/features/delivery/repositories/suppressed.js'

let fylo, root, events, service

const ics = (body, method = 'REQUEST') => [
  'BEGIN:VCALENDAR',
  'VERSION:2.0',
  `METHOD:${method}`,
  'BEGIN:VEVENT',
  body,
  'END:VEVENT',
  'END:VCALENDAR',
].join('\r\n')

const VEVENT = [
  'UID:evt-1@example.com',
  'SUMMARY:Standup',
  'DTSTART:20260301T090000Z',
  'DTEND:20260301T093000Z',
  'ORGANIZER:mailto:boss@other.com',
  'ATTENDEE;CN=Alice;PARTSTAT=NEEDS-ACTION:mailto:alice@example.com',
].join('\r\n')

const ctx = (over = {}) => ({
  recipient: 'alice@example.com',
  sourceEmailId: 'email-1',
  textBody: '',
  attachments: [],
  ...over,
})

beforeEach(async () => {
  root = mkdtempSync(join(tmpdir(), 'caduceus-calimport-'))
  fylo = await DatabaseRepository.createDb(root)
  events = new CalendarEventsRepository(fylo)
  service = new CalendarImportService(events)
})
afterEach(async () => {
  await fylo?.close?.()
  rmSync(root, { recursive: true, force: true })
})

describe('CalendarImportService.importFromInboundEmail', () => {
  it('ignores an email with no calendar payload', async () => {
    expect(await service.importFromInboundEmail(ctx({ textBody: 'just a note' })))
      .toEqual({ imported: 0, cancelled: 0, replyApplied: 0 })
  })

  it('imports an invite from the plain-text body', async () => {
    const res = await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT) }))
    expect(res.imported).toBe(1)
    const [, stored] = await events.findByIcsUid('alice@example.com', 'evt-1@example.com')
    expect(stored.title).toBe('Standup')
    expect(stored.source).toBe('invite')
    expect(stored.sourceEmailId).toBe('email-1')
    expect(stored.organizer).toBe('boss@other.com')
  })

  it('imports an invite from a text/calendar attachment', async () => {
    const res = await service.importFromInboundEmail(ctx({
      attachments: [{ filename: 'x', contentType: 'text/calendar; method=REQUEST', content: new TextEncoder().encode(ics(VEVENT)) }],
    }))
    expect(res.imported).toBe(1)
  })

  it('imports an invite from an .ics attachment with a generic type', async () => {
    const res = await service.importFromInboundEmail(ctx({
      attachments: [{ filename: 'invite.ICS', contentType: 'application/octet-stream', content: new TextEncoder().encode(ics(VEVENT)) }],
    }))
    expect(res.imported).toBe(1)
  })

  it('imports a legacy .vcs attachment and ignores a duplicate provider representation', async () => {
    const content = new TextEncoder().encode(ics(VEVENT))
    const res = await service.importFromInboundEmail(ctx({
      attachments: [
        { filename: 'invite.vcs', contentType: 'application/octet-stream', content },
        { filename: 'calendar.dat', contentType: 'text/calendar', content },
      ],
    }))
    expect(res.imported).toBe(1)
  })

  it('skips attachments that are neither calendar-typed nor .ics', async () => {
    const res = await service.importFromInboundEmail(ctx({
      attachments: [{ filename: 'notes.txt', contentType: 'text/plain', content: new TextEncoder().encode(ics(VEVENT)) }],
    }))
    expect(res.imported).toBe(0)
  })

  it('skips an attachment whose content is missing', async () => {
    const res = await service.importFromInboundEmail(ctx({
      attachments: [{ filename: 'empty.ics', contentType: 'text/calendar' }],
    }))
    expect(res.imported).toBe(0)
  })

  it('skips a calendar with no VEVENT', async () => {
    const res = await service.importFromInboundEmail(ctx({ textBody: 'BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nEND:VCALENDAR' }))
    expect(res.imported).toBe(0)
  })

  it('updates an existing event in place rather than duplicating it', async () => {
    await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT) }))
    const updated = VEVENT.replace('SUMMARY:Standup', 'SUMMARY:Standup (moved)')
    await service.importFromInboundEmail(ctx({ textBody: ics(updated) }))

    const all = await events.list('alice@example.com')
    expect(all.filter(e => e.icsUid === 'evt-1@example.com')).toHaveLength(1)
    expect(all.find(e => e.icsUid === 'evt-1@example.com').title).toBe('Standup (moved)')
  })

  it('treats a calendar with no METHOD as a request', async () => {
    const noMethod = ['BEGIN:VCALENDAR', 'BEGIN:VEVENT', VEVENT, 'END:VEVENT', 'END:VCALENDAR'].join('\r\n')
    expect((await service.importFromInboundEmail(ctx({ textBody: noMethod }))).imported).toBe(1)
  })

  it('cancels a previously imported event', async () => {
    await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT) }))
    const res = await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT, 'CANCEL') }))
    expect(res.cancelled).toBe(1)
    const [, cancelled] = await events.findByIcsUid('alice@example.com', 'evt-1@example.com')
    expect(cancelled.status).toBe('cancelled')
    expect(cancelled.sourceEmailIds).toEqual(['email-1'])
  })

  it('ignores a cancellation for an unknown event', async () => {
    expect((await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT, 'CANCEL') }))).cancelled).toBe(0)
  })

  it('imports and cancels one recurring exception without removing the series', async () => {
    const master = [
      VEVENT,
      'RRULE:FREQ=WEEKLY;COUNT=3',
      'EXDATE:20260315T090000Z',
    ].join('\r\n')
    await service.importFromInboundEmail(ctx({ textBody: ics(master) }))
    const exception = [
      'UID:evt-1@example.com', 'SUMMARY:Moved standup',
      'RECURRENCE-ID:20260308T090000Z',
      'DTSTART:20260309T110000Z', 'DTEND:20260309T120000Z',
    ].join('\r\n')
    expect((await service.importFromInboundEmail(ctx({ sourceEmailId: 'email-update', textBody: ics(exception) }))).imported).toBe(1)

    const [, storedMaster] = await events.findByIcsUid('alice@example.com', 'evt-1@example.com')
    const [, storedException] = await events.findException(storedMaster.id, '2026-03-08T09:00:00.000Z')
    expect(storedException).toMatchObject({
      title: 'Moved standup',
      seriesId: storedMaster.id,
      recurrenceId: '2026-03-08T09:00:00.000Z',
      sourceEmailId: 'email-update',
    })

    expect((await service.importFromInboundEmail(ctx({ sourceEmailId: 'email-cancel', textBody: ics(exception, 'CANCEL') }))).cancelled).toBe(1)
    const [, cancelled] = await events.findException(storedMaster.id, '2026-03-08T09:00:00.000Z')
    expect(cancelled.status).toBe('cancelled')
    expect((await events.findByIcsUid('alice@example.com', 'evt-1@example.com'))[1].status).toBe('confirmed')
  })

  it('applies an attendee reply to a known event', async () => {
    await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT) }))
    const reply = [
      'UID:evt-1@example.com', 'SUMMARY:Standup',
      'DTSTART:20260301T090000Z', 'DTEND:20260301T093000Z',
      'ATTENDEE;PARTSTAT=ACCEPTED:mailto:ALICE@example.com',
    ].join('\r\n')
    const res = await service.importFromInboundEmail(ctx({ sourceEmailId: 'email-reply', textBody: ics(reply, 'REPLY') }))
    expect(res.replyApplied).toBe(1)

    const [, stored] = await events.findByIcsUid('alice@example.com', 'evt-1@example.com')
    expect(stored.attendees.find(a => a.email.toLowerCase() === 'alice@example.com').status).toBe('accepted')
    expect(stored.sourceEmailId).toBe('email-reply')
    expect(stored.sourceEmailIds).toEqual(['email-1', 'email-reply'])
  })

  it('adds a replier who was not on the invite', async () => {
    await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT) }))
    const reply = [
      'UID:evt-1@example.com', 'SUMMARY:Standup',
      'DTSTART:20260301T090000Z', 'DTEND:20260301T093000Z',
      'ATTENDEE;CN=Carol;PARTSTAT=DECLINED:mailto:carol@other.com',
    ].join('\r\n')
    await service.importFromInboundEmail(ctx({ textBody: ics(reply, 'REPLY') }))

    const [, stored] = await events.findByIcsUid('alice@example.com', 'evt-1@example.com')
    expect(stored.attendees.find(a => a.email === 'carol@other.com').status).toBe('declined')
  })

  it('ignores a reply for an unknown event or with no attendee', async () => {
    expect((await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT, 'REPLY') }))).replyApplied).toBe(0)

    await service.importFromInboundEmail(ctx({ textBody: ics(VEVENT) }))
    const noAttendee = ['UID:evt-1@example.com', 'SUMMARY:Standup', 'DTSTART:20260301T090000Z', 'DTEND:20260301T093000Z'].join('\r\n')
    expect((await service.importFromInboundEmail(ctx({ textBody: ics(noAttendee, 'REPLY') }))).replyApplied).toBe(0)
  })

  it('keeps going when one VEVENT fails to store', async () => {
    const broken = new CalendarImportService({
      findByIcsUid: async () => { throw new Error('fylo down') },
    })
    expect(await broken.importFromInboundEmail(ctx({ textBody: ics(VEVENT) })))
      .toEqual({ imported: 0, cancelled: 0, replyApplied: 0 })
  })
})

describe('UsersRepository.upsertByEmail', () => {
  /** @type {UsersRepository} */
  let users
  beforeEach(() => { users = new UsersRepository(fylo) })

  const alice = { email: 'Alice@Example.com', aliases: [], domains: ['example.com'], role: 'admin' }

  it('inserts a new user with a normalised email', async () => {
    await users.upsertByEmail(alice)
    const [, stored] = await users.findByEmail('alice@example.com')
    expect(stored.role).toBe('admin')
  })

  it('updates an existing user in place', async () => {
    await users.upsertByEmail(alice)
    await users.upsertByEmail({ ...alice, role: 'viewer' })
    const all = await users.list()
    expect(all.filter(u => u.email === 'alice@example.com')).toHaveLength(1)
    expect(all[0].role).toBe('viewer')
  })

  it('keeps existing aliases and preferences when the update omits them', async () => {
    await users.upsertByEmail({
      ...alice,
      aliases: ['a2@example.com'],
      calendarVisibility: 'busy',
      mailboxNames: { 'example.com': 'Work' },
      unreadBadgeScope: 'domain',
      unreadBadgeDomain: 'example.com',
    })
    await users.upsertByEmail({ ...alice, aliases: [] })
    const [, stored] = await users.findByEmail('alice@example.com')
    expect(stored.aliases).toEqual(['a2@example.com'])
    expect(stored.calendarVisibility).toBe('busy')
    expect(stored.mailboxNames).toEqual({ 'example.com': 'Work' })
    expect(stored.unreadBadgeScope).toBe('domain')
    expect(stored.unreadBadgeDomain).toBe('example.com')
  })

  it('replaces aliases and preferences when the update supplies them', async () => {
    await users.upsertByEmail({
      ...alice,
      aliases: ['a2@example.com'],
      mailboxNames: { 'example.com': 'Work' },
      unreadBadgeScope: 'domain',
      unreadBadgeDomain: 'example.com',
    })
    await users.upsertByEmail({
      ...alice,
      aliases: ['a3@example.com'],
      mailboxNames: { 'example.com': 'Personal' },
      unreadBadgeScope: 'all',
      unreadBadgeDomain: 'example.com',
    })
    const [, stored] = await users.findByEmail('alice@example.com')
    expect(stored.aliases).toEqual(['a3@example.com'])
    expect(stored.mailboxNames).toEqual({ 'example.com': 'Personal' })
    expect(stored.unreadBadgeScope).toBe('all')
    expect(stored.unreadBadgeDomain).toBe('example.com')
  })
})

describe('InboundService dispatch', () => {
  const build = (over = {}) => {
    const attachments = new AttachmentsRepository(fylo)
    const service = new InboundService(
      new EmailsRepository(fylo, attachments),
      new RulesRepository(fylo),
      new SuppressedRepository(fylo),
      attachments,
      { send: async () => ({ messageId: 'x' }) },
      fylo,
    )
    return Object.assign(service, over)
  }

  const payload = (over = {}) => ({
    recipient: 'alice@example.com',
    sender: 'bob@other.com',
    subject: 'Hi',
    body: 'body text',
    ...over,
  })

  const rule = (actions, over = {}) => ({
    id: `r-${Math.random().toString(36).slice(2)}`,
    domain: 'example.com',
    name: 'route',
    enabled: true,
    conditionMatch: 'all',
    conditions: [],
    actions,
    ...over,
  })

  it('rejects an unparseable recipient', async () => {
    expect(await build().processInbound(payload({ recipient: 'not-an-email' })))
      .toEqual({ ok: false, error: 'Invalid recipient' })
  })

  it('rejects a recipient with no matching route', async () => {
    expect(await build().processInbound(payload())).toEqual({ ok: false, error: 'No matching route' })
  })

  it('rejects a suppressed sender', async () => {
    const service = build()
    await service.rules.put(rule([{ type: 'store' }]))
    await service.suppressed.suppress('bob@other.com', 'bounce')
    expect(await service.processInbound(payload())).toEqual({ ok: false, error: 'Sender is suppressed' })
  })

  it('stores a message and its attachments', async () => {
    const service = build()
    await service.rules.put(rule([{ type: 'store' }]))
    const res = await service.processInbound(payload({
      htmlBody: '<p>hi</p>',
      attachments: [{ filename: 'a.txt', contentType: 'text/plain', content: Buffer.from('hi') }],
    }))
    expect(res.ok).toBe(true)
    const [, stored] = await service.emails.findById(res.emailId)
    expect(stored.htmlBody).toBe('<p>hi</p>')
    expect(await service.attachments.listSummaries(res.emailId)).toHaveLength(1)
  })

  it('uses the message id as the stored id when present', async () => {
    const service = build()
    await service.rules.put(rule([{ type: 'store' }]))
    const res = await service.processInbound(payload({ messageId: 'mid-123' }))
    expect(res.emailId).toBe('mid-123')
  })

  it('forwards and stores a copy for the forward target', async () => {
    const sent = []
    const service = build({ smtp: { send: async m => { sent.push(m); return { messageId: 'x' } } } })
    await service.rules.put(rule([{ type: 'forward', to: 'archive@example.com' }]))
    const res = await service.processInbound(payload())
    expect(res.ok).toBe(true)
    expect(sent[0].to).toEqual(['archive@example.com'])
    const [, stored] = await service.emails.findById(res.emailId)
    expect(stored.recipient).toBe('archive@example.com')
  })

  it('posts a signed webhook', async () => {
    const original = globalThis.fetch
    const calls = []
    globalThis.fetch = async (url, init) => { calls.push({ url, init }); return new Response('ok') }
    try {
      const service = build()
      await service.rules.put(rule([{ type: 'webhook', url: 'https://hooks.example.com/x', secret: 'shh' }]))
      expect(await service.processInbound(payload())).toEqual({ ok: true })
      expect(calls[0].init.headers['X-Caduceus-Signature']).toMatch(/^[0-9a-f]{64}$/)
    } finally {
      globalThis.fetch = original
    }
  })

  it('posts an unsigned webhook when no secret is configured', async () => {
    const original = globalThis.fetch
    const calls = []
    globalThis.fetch = async (url, init) => { calls.push({ url, init }); return new Response('ok') }
    try {
      const service = build()
      await service.rules.put(rule([{ type: 'webhook', url: 'https://hooks.example.com/x' }]))
      await service.processInbound(payload())
      expect(calls[0].init.headers['X-Caduceus-Signature']).toBeUndefined()
    } finally {
      globalThis.fetch = original
    }
  })

  it('acknowledges a drop without storing', async () => {
    const service = build()
    await service.rules.put(rule([{ type: 'drop' }]))
    expect(await service.processInbound(payload())).toEqual({ ok: true })
    expect(await service.emails.list(['example.com'])).toEqual([])
  })

  it('stores when the rule carries no recognised action', async () => {
    const service = build()
    await service.rules.put(rule([{ type: 'label' }]))
    expect((await service.processInbound(payload())).ok).toBe(true)
  })

  it('honours rule conditions on the recipient', async () => {
    const service = build()
    await service.rules.put(rule([{ type: 'store' }], {
      conditions: [{ field: 'to', op: 'equals', value: 'someone-else@example.com' }],
    }))
    expect(await service.processInbound(payload())).toEqual({ ok: false, error: 'No matching route' })
  })

  it('ignores conditions on fields other than the recipient', async () => {
    const service = build()
    await service.rules.put(rule([{ type: 'store' }], {
      conditionMatch: 'any',
      conditions: [{ field: 'subject', op: 'contains', value: 'Hi' }],
    }))
    expect(await service.processInbound(payload())).toEqual({ ok: false, error: 'No matching route' })
  })
})

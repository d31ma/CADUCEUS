import { describe, it, expect, beforeAll, afterAll } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import * as MboxUtils from '../support/mbox.mjs'
import { call } from '../support/controller-call.mjs'
import { TestContainer } from '../support/TestContainer.mjs'
import { Routes } from '../support/Routes.mjs'

const Detail = Routes.InboxIdController
const Spam = Routes.InboxIdSpamController
const Attachment = Routes.InboxIdAttachmentsAttachmentIdController
const Source = Routes.InboxIdSourceController
const RemoteImages = Routes.InboxIdImagesController
const ExportSources = Routes.InboxExportRfc822Controller
const ImportEml = Routes.InboxImportEmlController
const Search = Routes.SearchController
const Threads = Routes.ThreadsController

const writeMboxFromFrameStream = (...args) => MboxUtils.writeMboxFromFrameStream(...args)

let app
/** SSE route: its window/poll constants are read at module load, so import it
 *  only after the test values are in the environment. */
let Stream
const auth = () => token()
const foreign = () => token({ email: 'carol@other.com' })

beforeAll(async () => {
  app = await setupRoutes()
  process.env.STREAM_MAX_MS = '5'
  process.env.STREAM_POLL_MS = '1'
  Stream = (await import('@/controllers/InboxStreamController.js')).InboxStreamController
}, 300_000)
afterAll(teardownRoutes)

describe('GET /inbox/:id', () => {
  it('404s without an id', async () => {
    expect((await call(Detail, 'GET', req({ token: auth() }))).notFound).toBe(true)
  })

  it('404s for an unknown id', async () => {
    const res = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'nope' } }))
    expect(res.notFound).toBe(true)
  })

  it('403s when the email is outside the caller domains', async () => {
    await app.emails.put(email({ id: 'g1' }))
    const res = await call(Detail, 'GET', req({ token: foreign(), paths: { id: 'g1' } }))
    expect(res.forbidden).toBe(true)
  })

  it('returns the email with attachments and no invite', async () => {
    await app.emails.put(email({ id: 'g2', subject: 'Report' }))
    const res = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'g2' } }))
    expect(res.subject).toBe('Report')
    expect(res.attachments).toEqual([])
    expect(res.calendarInvite).toBeNull()
  })

  it('does not reveal another mailbox copy from the same hosted domain', async () => {
    await app.emails.put(email({
      id: 'same-domain-private-detail',
      ownerEmail: 'bob@example.com',
      recipient: 'bob@example.com',
      subject: 'Bob tenant secret',
    }))
    const res = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'same-domain-private-detail' } }))
    expect(res.notFound).toBe(true)
  })

  it('attaches the linked calendar invite when one exists', async () => {
    await app.emails.put(email({ id: 'g3' }))
    await app.calendarEvents.put({
      id: 'evt-1',
      userEmail: 'alice@example.com',
      title: 'Standup',
      start: '2026-01-01T10:00:00.000Z',
      end: '2026-01-01T10:15:00.000Z',
      sourceEmailId: 'g3',
      attendees: [{ email: 'ALICE@example.com', name: 'Alice', status: 'tentative' }],
      organizer: 'bob@other.com',
      status: 'confirmed',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    })
    const res = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'g3' } }))
    expect(res.calendarInvite.title).toBe('Standup')
    expect(res.calendarInvite.myStatus).toBe('tentative')
  })

  it('defaults myStatus when the caller is not an attendee', async () => {
    await app.emails.put(email({ id: 'g4' }))
    await app.calendarEvents.put({
      id: 'evt-2',
      userEmail: 'alice@example.com',
      title: 'Review',
      start: '2026-01-02T10:00:00.000Z',
      end: '2026-01-02T11:00:00.000Z',
      sourceEmailId: 'g4',
      attendees: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    })
    const res = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'g4' } }))
    expect(res.calendarInvite.myStatus).toBe('needsAction')
    expect(res.calendarInvite.location).toBe('')
  })

  it('selects and exposes the best calendar attachment while suppressing provider duplicates', async () => {
    await app.emails.put(email({ id: 'g-calendar-attachment', sender: 'Boss <boss@other.com>' }))
    const calendar = [
      'BEGIN:VCALENDAR', 'VERSION:2.0', 'METHOD:REQUEST', 'BEGIN:VEVENT',
      'UID:attachment-event@example.com', 'SUMMARY:Planning',
      'DTSTART:20260103T100000Z', 'DTEND:20260103T110000Z',
      'ORGANIZER:mailto:boss@other.com',
      'ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:alice@example.com',
      'END:VEVENT', 'END:VCALENDAR',
    ].join('\r\n')
    const bytes = Buffer.from(calendar)
    const saved = await app.attachments.saveForEmail('g-calendar-attachment', 'example.com', [
      { filename: 'calendar.dat', contentType: 'text/calendar', content: bytes },
      { filename: 'event.ics', contentType: 'application/octet-stream', content: bytes },
      { filename: 'same-size.pdf', contentType: 'application/pdf', content: Buffer.alloc(bytes.length) },
    ])
    await app.calendarEvents.put({
      id: 'evt-calendar-attachment', userEmail: 'alice@example.com', title: 'Planning',
      start: '2026-01-03T10:00:00.000Z', end: '2026-01-03T11:00:00.000Z',
      sourceEmailId: 'g-calendar-attachment', icsUid: 'attachment-event@example.com',
      attendees: [{ email: 'alice@example.com', status: 'needsAction' }], source: 'invite',
      createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
    })

    const res = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'g-calendar-attachment' } }))
    expect(res.calendarInvite.attachment).toEqual({
      id: saved[1].id, filename: 'event.ics', contentType: 'application/octet-stream', size: bytes.length,
    })
    expect(res.calendarInvite.messageMethod).toBe('REQUEST')
    expect(res.calendarInvite.canRespond).toBe(true)
    expect(res.attachments.map(attachment => attachment.filename)).toEqual(['same-size.pdf'])
  })

  it('presents the sender response from a METHOD:REPLY message without RSVP controls', async () => {
    await app.emails.put(email({ id: 'g-calendar-reply', sender: 'Carol <carol@other.com>' }))
    const reply = [
      'BEGIN:VCALENDAR', 'VERSION:2.0', 'METHOD:REPLY', 'BEGIN:VEVENT',
      'UID:reply-event@example.com', 'SUMMARY:Planning',
      'DTSTART:20260103T100000Z', 'DTEND:20260103T110000Z',
      'ATTENDEE;CN=Carol;PARTSTAT=DECLINED:mailto:carol@other.com',
      'END:VEVENT', 'END:VCALENDAR',
    ].join('\r\n')
    await app.attachments.saveForEmail('g-calendar-reply', 'example.com', [{ filename: 'reply.ics', contentType: 'text/calendar', content: Buffer.from(reply) }])
    await app.calendarEvents.put({
      id: 'evt-calendar-reply', userEmail: 'alice@example.com', title: 'Planning',
      start: '2026-01-03T10:00:00.000Z', end: '2026-01-03T11:00:00.000Z',
      sourceEmailId: 'g-calendar-reply', sourceEmailIds: ['g-calendar-invite', 'g-calendar-reply'],
      icsUid: 'reply-event@example.com', attendees: [{ email: 'carol@other.com', name: 'Carol', status: 'declined' }],
      source: 'invite', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
    })

    const res = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'g-calendar-reply' } }))
    expect(res.calendarInvite.messageMethod).toBe('REPLY')
    expect(res.calendarInvite.canRespond).toBe(false)
    expect(res.calendarInvite.senderResponse).toEqual({ email: 'carol@other.com', name: 'Carol', status: 'declined' })
    expect(res.attachments).toEqual([])
  })

  it('exposes only exact GitHub notification targets from GitHub senders', async () => {
    await app.emails.put(email({
      id: 'g5',
      sender: 'notifications@github.com',
      htmlBody: '<a href="https://github.com/acme/widget/issues/42">View it on GitHub</a>',
    }))
    await app.emails.put(email({
      id: 'g6',
      sender: 'notifications@github.com.evil.test',
      htmlBody: '<a href="https://github.com/acme/widget/issues/42">View it on GitHub</a>',
    }))
    expect((await call(Detail, 'GET', req({ token: auth(), paths: { id: 'g5' } }))).githubUrl)
      .toBe('https://github.com/acme/widget/issues/42')
    expect((await call(Detail, 'GET', req({ token: auth(), paths: { id: 'g6' } }))).githubUrl).toBeUndefined()
  })
})

describe('Detail.parsePatch', () => {
  it('rejects a non-object payload', () => {
    expect(Detail.parsePatch(null)).toBeInstanceOf(Error)
    expect(Detail.parsePatch('x')).toBeInstanceOf(Error)
  })

  it('rejects an empty patch', () => {
    expect(Detail.parsePatch({})).toBeInstanceOf(Error)
  })

  it('normalises folder to lowercase', () => {
    expect(Detail.parsePatch({ folder: ' Archive ' })).toEqual({ folder: 'archive' })
  })

  it('rejects a blank or non-string folder', () => {
    expect(Detail.parsePatch({ folder: '  ' })).toBeInstanceOf(Error)
    expect(Detail.parsePatch({ folder: 3 })).toBeInstanceOf(Error)
  })

  it('rejects an over-long custom folder', () => {
    expect(Detail.parsePatch({ folder: 'x'.repeat(61) })).toBeInstanceOf(Error)
  })

  it('accepts a custom folder within the length cap', () => {
    expect(Detail.parsePatch({ folder: 'Projects' })).toEqual({ folder: 'projects' })
  })

  it('accepts boolean read and starred', () => {
    expect(Detail.parsePatch({ read: true, starred: false })).toEqual({ read: true, starred: false })
  })

  it('rejects non-boolean read and starred', () => {
    expect(Detail.parsePatch({ read: 'yes' })).toBeInstanceOf(Error)
    expect(Detail.parsePatch({ starred: 1 })).toBeInstanceOf(Error)
  })
})

describe('PUT /inbox/:id', () => {
  it('404s without an id', async () => {
    expect((await call(Detail, 'PUT', req({ token: auth(), body: { read: true } }))).notFound).toBe(true)
  })

  it('400s on an invalid patch', async () => {
    const res = await call(Detail, 'PUT', req({ token: auth(), paths: { id: 'x' }, body: {} }))
    expect(res.error).toMatch(/No supported fields/)
  })

  it('404s for an unknown email', async () => {
    const res = await call(Detail, 'PUT', req({ token: auth(), paths: { id: 'gone' }, body: { read: true } }))
    expect(res.notFound).toBe(true)
  })

  it('403s across domains', async () => {
    await app.emails.put(email({ id: 'p1' }))
    const res = await call(Detail, 'PUT', req({ token: foreign(), paths: { id: 'p1' }, body: { read: true } }))
    expect(res.forbidden).toBe(true)
  })

  it('applies the patch and echoes the updated email', async () => {
    await app.emails.put(email({
      id: 'p2',
      sender: 'notifications@github.com',
      htmlBody: '<a href="https://github.com/acme/widget/issues/42">View it on GitHub</a>',
    }))
    const res = await call(Detail, 'PUT', req({ token: auth(), paths: { id: 'p2' }, body: { read: true, starred: true } }))
    expect(res.read).toBe(true)
    expect(res.starred).toBe(true)
    expect(res.githubUrl).toBe('https://github.com/acme/widget/issues/42')
    const [, stored] = await app.emails.findById('p2')
    expect(stored.read).toBe(true)
    expect(stored.githubUrl).toBeUndefined()
  })
})

describe('DELETE /inbox/:id', () => {
  it('404s without an id', async () => {
    expect((await call(Detail, 'DELETE', req({ token: auth() }))).notFound).toBe(true)
  })

  it('404s for an unknown email', async () => {
    expect((await call(Detail, 'DELETE', req({ token: auth(), paths: { id: 'gone' } }))).notFound).toBe(true)
  })

  it('403s across domains', async () => {
    await app.emails.put(email({ id: 'd1' }))
    const res = await call(Detail, 'DELETE', req({ token: foreign(), paths: { id: 'd1' } }))
    expect(res.forbidden).toBe(true)
  })

  it('deletes the email', async () => {
    await app.emails.put(email({ id: 'd2' }))
    const res = await call(Detail, 'DELETE', req({ token: auth(), paths: { id: 'd2' } }))
    expect(res.deleted).toBe('d2')
    const [docId] = await app.emails.findById('d2')
    expect(docId).toBeNull()
  })
})

describe('PUT /inbox/:id/spam', () => {
  it('404s without an id', async () => {
    expect((await call(Spam, 'PUT', req({ token: auth(), body: { spam: true } }))).notFound).toBe(true)
  })

  it('400s when spam is not a boolean', async () => {
    const res = await call(Spam, 'PUT', req({ token: auth(), paths: { id: 's1' }, body: { spam: 'yes' } }))
    expect(res.error).toMatch(/boolean/)
  })

  it('404s for an unknown email', async () => {
    const res = await call(Spam, 'PUT', req({ token: auth(), paths: { id: 'gone' }, body: { spam: true } }))
    expect(res.notFound).toBe(true)
  })

  it('403s across domains', async () => {
    await app.emails.put(email({ id: 's2' }))
    const res = await call(Spam, 'PUT', req({ token: foreign(), paths: { id: 's2' }, body: { spam: true } }))
    expect(res.forbidden).toBe(true)
  })

  it('moves a reported email to spam', async () => {
    await app.emails.put(email({ id: 's3' }))
    const res = await call(Spam, 'PUT', req({ token: auth(), paths: { id: 's3' }, body: { spam: true } }))
    expect(res.folder).toBe('spam')
    expect(res.spamReasons).toEqual(['user-reported'])
  })

  it('restores a not-spam email and clears spam metadata', async () => {
    await app.emails.put(email({ id: 's4', folder: 'spam', spamScore: 9, spamReasons: ['bayes'] }))
    const res = await call(Spam, 'PUT', req({ token: auth(), paths: { id: 's4' }, body: { spam: false } }))
    expect(res.folder).toBe('inbox')
    expect(res.spamScore).toBeUndefined()
    expect(res.spamReasons).toBeUndefined()
  })
})

describe('GET /inbox/:id/source', () => {
  it('requires an existing email and retained source', async () => {
    expect((await call(Source, 'GET', req({ token: auth(), paths: { id: 'missing' } }))).notFound).toBe(true)
    await app.emails.put(email({ id: 'source-none' }))
    expect((await call(Source, 'GET', req({ token: auth(), paths: { id: 'source-none' } }))).notFound).toBe(true)
  })

  it('does not expose source across mailbox domains', async () => {
    await app.emails.put(email({ id: 'source-private' }))
    await app.rawMessages.saveForEmail('source-private', 'example.com', 'Subject: Private\r\n\r\nSecret')
    const result = await call(Source, 'GET', req({ token: foreign(), paths: { id: 'source-private' } }))
    expect(result.forbidden).toBe(true)
  })

  it('returns exact source bytes with EML metadata', async () => {
    const source = 'From: bob@other.com\r\nSubject: Original\r\n\r\nExact body\r\n'
    await app.emails.put(email({ id: 'source-ok', subject: 'Original / message' }))
    await app.rawMessages.saveForEmail('source-ok', 'example.com', source)
    const detail = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'source-ok' } }))
    expect(detail.hasRawSource).toBe(true)

    const result = await call(Source, 'GET', req({ token: auth(), paths: { id: 'source-ok' } }))
    expect(result.contentType).toBe('message/rfc822')
    expect(result.filename).toBe('Original - message.eml')
    expect(Buffer.from(result.contentBase64, 'base64').toString()).toBe(source)
    expect(result.sha256).toMatch(/^[a-f0-9]{64}$/)
  })
})

describe('POST /inbox/:id/images', () => {
  const originalFetchImage = RemoteImages._fetchImage

  afterAll(() => { RemoteImages._fetchImage = originalFetchImage })

  it('requires a bounded URL list and an existing exact-owner message', async () => {
    expect((await call(RemoteImages, 'POST', req({ token: auth(), paths: { id: 'missing' }, body: { urls: [] } }))).error).toMatch(/between 1 and 6/)
    expect((await call(RemoteImages, 'POST', req({ token: auth(), paths: { id: 'missing' }, body: { urls: ['https://images.example/a.png'] } }))).notFound).toBe(true)

    await app.emails.put(email({
      id: 'remote-image-private',
      ownerEmail: 'bob@example.com',
      recipient: 'bob@example.com',
      htmlBody: '<img src="https://images.example/a.png">',
    }))
    expect((await call(RemoteImages, 'POST', req({
      token: auth(), paths: { id: 'remote-image-private' }, body: { urls: ['https://images.example/a.png'] },
    }))).notFound).toBe(true)
  })

  it('proxies only exact image URLs retained in the owned message and reports partial failures', async () => {
    const good = 'https://images.example/a.png'
    const failed = 'https://images.example/fail.png'
    await app.emails.put(email({
      id: 'remote-image-owned',
      htmlBody: `<img src="${good}"><div style="background:url('${failed}')"></div>`,
    }))
    RemoteImages._fetchImage = async url => {
      if (url === failed) throw new Error('upstream unavailable')
      return { url, contentType: 'image/png', contentBase64: 'iVBORw0KGgo=', size: 8 }
    }

    const rejected = await call(RemoteImages, 'POST', req({
      token: auth(), paths: { id: 'remote-image-owned' }, body: { urls: ['https://unrelated.example/a.png'] },
    }))
    expect(rejected.error).toMatch(/from this message/)

    const response = await call(RemoteImages, 'POST', req({
      token: auth(), paths: { id: 'remote-image-owned' }, body: { urls: [good, failed, good] },
    }))
    expect(response).toEqual({
      images: [{ url: good, contentType: 'image/png', contentBase64: 'iVBORw0KGgo=', size: 8 }],
      failed: 1,
    })
  })
})

describe('GET /inbox/export/rfc822', () => {
  it('streams one authorized folder as bounded source frames consumable as mboxrd', async () => {
    const first = [
      'Return-Path: <one@sender.test>',
      'From: One <one@sender.test>',
      'Date: Fri, 14 Aug 2026 12:00:00 +0000',
      'Subject: First',
      '',
      'From body delimiter',
    ].join('\r\n')
    const second = 'From: Two <two@sender.test>\r\nSubject: Second\r\n\r\nSecond body\r\n'
    await app.emails.put(email({ id: 'mbox-first', folder: 'archive', receivedAt: '2026-08-14T12:00:00Z' }))
    await app.emails.put(email({ id: 'mbox-second', folder: 'archive', receivedAt: '2026-08-14T12:01:00Z' }))
    await app.emails.put(email({ id: 'mbox-other-folder', folder: 'inbox' }))
    await app.rawMessages.saveForEmail('mbox-first', 'example.com', first)
    await app.rawMessages.saveForEmail('mbox-second', 'example.com', second)

    const response = await call(ExportSources, 'GET', req({
      token: auth(),
      query: { folder: 'archive', domain: 'example.com' },
    }))
    expect(response).toBeInstanceOf(Response)
    expect(response.headers.get('content-type')).toContain('application/vnd.caduceus.rfc822-frames')
    const chunks = []
    const result = await writeMboxFromFrameStream(response.body, chunk => chunks.push(chunk))
    const mbox = Buffer.concat(chunks.map(chunk => Buffer.from(chunk))).toString('latin1')
    expect(result.messages).toBe(2)
    expect(mbox.match(/^From /gm)).toHaveLength(2)
    expect(mbox).toContain('\n>From body delimiter\n\n')
    expect(mbox).not.toContain('mbox-other-folder')
  })

  it('rejects malformed folders and cross-domain export attempts', async () => {
    const invalid = await call(ExportSources, 'GET', req({
      token: auth(), query: { folder: '../inbox', domain: 'example.com' },
    }))
    expect(invalid.error).toMatch(/folder is invalid/)

    const denied = await call(ExportSources, 'GET', req({
      token: foreign(), query: { folder: 'inbox', domain: 'example.com' },
    }))
    expect(denied.forbidden).toBe(true)
  })
})

describe('POST /inbox/import/eml', () => {
  it('retains exact source and imports parsed metadata and attachments atomically', async () => {
    const source = [
      'From: Importer <importer@sender.test>',
      'To: Alice <alice@example.com>',
      'Date: Fri, 14 Aug 2026 12:34:56 +0000',
      'Message-ID: <import-one@sender.test>',
      'Subject: Imported message',
      'MIME-Version: 1.0',
      'Content-Type: multipart/mixed; boundary="import-boundary"',
      '',
      '--import-boundary',
      'Content-Type: text/plain; charset=utf-8',
      '',
      'Imported body',
      '--import-boundary',
      'Content-Type: text/plain; name="note.txt"',
      'Content-Disposition: attachment; filename="note.txt"',
      'Content-Transfer-Encoding: base64',
      '',
      'aGVsbG8=',
      '--import-boundary--',
      '',
    ].join('\r\n')

    const result = await call(ImportEml, 'POST', req({
      token: auth(),
      body: {
        mailbox: 'alice@example.com',
        folder: 'archive',
        contentBase64: Buffer.from(source).toString('base64'),
      },
    }))
    expect(result.imported).toBe(true)
    expect(result.subject).toBe('Imported message')
    expect(result.sender).toBe('importer@sender.test')
    expect(result.attachmentCount).toBe(1)

    const [, stored] = await app.emails.findById(result.id)
    expect(stored.folder).toBe('archive')
    expect(stored.rfcMessageId).toBe('<import-one@sender.test>')
    expect(stored.receivedAt).toBe('2026-08-14T12:34:56.000Z')
    const raw = await app.rawMessages.findForEmail(result.id)
    expect(Buffer.from(await app.rawMessages.readContent(raw)).toString()).toBe(source)
    const [attachment] = await app.attachments.list(result.id)
    expect(attachment.filename).toBe('note.txt')
    expect(Buffer.from(await app.attachments.readContent(attachment)).toString()).toBe('hello')
  })

  it('rejects malformed files, drafts, and mailboxes outside caller authority', async () => {
    const malformed = await call(ImportEml, 'POST', req({
      token: auth(),
      body: { mailbox: 'alice@example.com', folder: 'inbox', contentBase64: Buffer.from('not mail').toString('base64') },
    }))
    expect(malformed.error).toMatch(/not a recognizable/)

    const draft = await call(ImportEml, 'POST', req({
      token: auth(),
      body: {
        mailbox: 'alice@example.com', folder: 'drafts',
        contentBase64: Buffer.from('Subject: Draft\r\n\r\nbody').toString('base64'),
      },
    }))
    expect(draft.error).toMatch(/cannot be imported as drafts/)

    const denied = await call(ImportEml, 'POST', req({
      token: foreign(),
      body: {
        mailbox: 'alice@example.com', folder: 'inbox',
        contentBase64: Buffer.from('Subject: Private\r\n\r\nbody').toString('base64'),
      },
    }))
    expect(denied.forbidden).toBe(true)
  })
})

describe('GET /inbox/:id/attachments/:attachmentId', () => {
  it('404s without ids', async () => {
    expect((await call(Attachment, 'GET', req({ token: auth() }))).notFound).toBe(true)
  })

  it('404s for an unknown email', async () => {
    const res = await call(Attachment, 'GET', req({ token: auth(), paths: { id: 'gone', attachment_id: 'a' } }))
    expect(res.notFound).toBe(true)
  })

  it('403s across domains', async () => {
    await app.emails.put(email({ id: 'a1' }))
    const res = await call(Attachment, 'GET', req({ token: foreign(), paths: { id: 'a1', attachment_id: 'x' } }))
    expect(res.forbidden).toBe(true)
  })

  it('404s when the attachment belongs to another email', async () => {
    await app.emails.put(email({ id: 'a2' }))
    const res = await call(Attachment, 'GET', req({ token: auth(), paths: { id: 'a2', attachment_id: 'missing' } }))
    expect(res.notFound).toBe(true)
  })

  it('returns the attachment bytes base64-encoded', async () => {
    await app.emails.put(email({ id: 'a3' }))
    const [saved] = await app.attachments.saveForEmail('a3', 'example.com', [{
      filename: 'note.txt',
      contentType: 'text/plain',
      content: Buffer.from('hello'),
    }])
    const res = await call(Attachment, 'GET', req({ token: auth(), paths: { id: 'a3', attachment_id: saved.id } }))
    expect(res.filename).toBe('note.txt')
    expect(Buffer.from(res.contentBase64, 'base64').toString()).toBe('hello')
  })
})

describe('GET /search', () => {
  it('requires auth', async () => {
    expect((await call(Search, 'GET', req())).unauthorized).toBe(true)
  })

  it('returns everything for an empty query', async () => {
    const res = await call(Search, 'GET', req({ token: auth(), query: {} }))
    expect(Array.isArray(res)).toBe(true)
  })

  it('filters by sender', async () => {
    await app.emails.put(email({ id: 'q1', sender: 'carol@other.com', subject: 'Invoice' }))
    const res = await call(Search, 'GET', req({ token: auth(), query: { q: 'from:carol@other.com' } }))
    expect(res.map(e => e.id)).toContain('q1')
  })

  it('excludes trash and spam from ordinary searches unless in: is explicit', async () => {
    const suffix = Date.now().toString(36)
    await app.emails.put(email({ id: `search-trash-${suffix}`, subject: `Discarded ${suffix}`, folder: 'trash' }))
    const ordinary = await call(Search, 'GET', req({ token: auth(), query: { q: `Discarded ${suffix}` } }))
    expect(ordinary.map(item => item.id)).not.toContain(`search-trash-${suffix}`)
    const explicit = await call(Search, 'GET', req({ token: auth(), query: { q: `Discarded ${suffix} in:trash` } }))
    expect(explicit.map(item => item.id)).toContain(`search-trash-${suffix}`)
  })

  it('filters by unread and starred flags', async () => {
    await app.emails.put(email({ id: 'q2', starred: true, read: false, subject: 'Pinned' }))
    const res = await call(Search, 'GET', req({ token: auth(), query: { q: 'is:unread is:starred Pinned' } }))
    expect(res.map(e => e.id)).toContain('q2')
  })

  it('honours has:attachment', async () => {
    const res = await call(Search, 'GET', req({ token: auth(), query: { q: 'has:attachment' } }))
    expect(res.every(e => e.id !== 'q2')).toBe(true)
  })

  it('clips results with before: and after:', async () => {
    await app.emails.put(email({ id: 'q3', receivedAt: '2020-01-01T00:00:00.000Z', subject: 'Old' }))
    const before = await call(Search, 'GET', req({ token: auth(), query: { q: 'Old before:2021-01-01' } }))
    expect(before.map(e => e.id)).toContain('q3')
    const after = await call(Search, 'GET', req({ token: auth(), query: { q: 'Old after:2021-01-01' } }))
    expect(after.map(e => e.id)).not.toContain('q3')
  })

  it('ignores unparseable before:/after: dates', async () => {
    const res = await call(Search, 'GET', req({ token: auth(), query: { q: 'before:notadate after:alsonot' } }))
    expect(Array.isArray(res)).toBe(true)
  })

  it('supports quoted phrases, OR, NOT, parentheses, folder scope, and negative flags', async () => {
    const suffix = Date.now().toString(36)
    await app.emails.put(email({
      id: `search-grammar-${suffix}`, sender: 'Grace Hopper <grace@example.net>',
      subject: `Compiler Planning ${suffix}`, body: 'Bring the launch checklist.',
      folder: 'archive', read: true, starred: false,
    }))
    const query = `(from:grace OR from:ada) "launch checklist" in:archive is:read is:unstarred NOT in:trash`
    const res = await call(Search, 'GET', req({ token: auth(), query: { q: query } }))
    expect(res.map(item => item.id)).toContain(`search-grammar-${suffix}`)
  })

  it('scopes a search to one authorized hosted account', async () => {
    await app.emails.put(email({
      id: 'search-account-example', ownerEmail: 'alice@example.com', domain: 'example.com',
      recipient: 'alice@example.com', subject: 'Account scoped needle',
    }))
    await app.emails.put(email({
      id: 'search-account-other', ownerEmail: 'alice@example.com', domain: 'other.com',
      recipient: 'alice@example.com', subject: 'Account scoped needle',
    }))
    const scoped = await call(Search, 'GET', req({
      token: token({ email: 'alice@example.com', domains: ['example.com', 'other.com'] }),
      query: { q: '"Account scoped needle"', domain: 'example.com' },
    }))
    expect(scoped.map(item => item.id)).toContain('search-account-example')
    expect(scoped.map(item => item.id)).not.toContain('search-account-other')
    const forbidden = await call(Search, 'GET', req({ token: auth(), query: { q: 'needle', domain: 'other.com' } }))
    expect(forbidden.forbidden).toBe(true)
  })

  it('bounds query and result sizes and falls back to literal full-text search on syntax errors', async () => {
    const tooLong = await call(Search, 'GET', req({ token: auth(), query: { q: 'x'.repeat(2001) } }))
    expect(tooLong.error).toMatch(/2,000/)
    const badLimit = await call(Search, 'GET', req({ token: auth(), query: { limit: '0' } }))
    expect(badLimit.error).toMatch(/limit/i)
    const malformed = await call(Search, 'GET', req({ token: auth(), query: { q: 'subject:"unterminated' } }))
    expect(Array.isArray(malformed)).toBe(true)
  })
})

describe('GET /threads', () => {
  it('requires auth', async () => {
    expect((await call(Threads, 'GET', req())).unauthorized).toBe(true)
  })

  it('lists threads for the default folder', async () => {
    const res = await call(Threads, 'GET', req({ token: auth(), query: {} }))
    expect(Array.isArray(res)).toBe(true)
  })

  it('searches conversations with the full structured grammar and returns the complete thread', async () => {
    const suffix = Date.now().toString(36)
    const threadId = `thread-search-${suffix}`.padEnd(64, 'f')
    await app.emails.put(email({
      id: `thread-search-hit-${suffix}`, threadId, folder: 'archive',
      sender: 'Grace Hopper <grace@example.net>', subject: `Compiler Plan ${suffix}`,
      body: 'Bring the launch checklist.', read: true,
      receivedAt: '2026-01-01T10:00:00.000Z',
    }))
    await app.emails.put(email({
      id: `thread-search-reply-${suffix}`, threadId, folder: 'sent',
      sender: 'alice@example.com', subject: `Re: Compiler Plan ${suffix}`,
      body: 'Confirmed.', read: true,
      receivedAt: '2026-01-01T11:00:00.000Z',
    }))

    const res = await call(Threads, 'GET', req({
      token: auth(),
      query: { q: `(from:grace OR from:ada) "launch checklist" in:archive NOT is:unread` },
    }))
    const found = res.find(item => item.id === threadId)
    expect(found.emails.map(item => item.id)).toEqual([
      `thread-search-hit-${suffix}`,
      `thread-search-reply-${suffix}`,
    ])
  })

  it('enforces account scope and search input bounds for threaded results', async () => {
    const scoped = await call(Threads, 'GET', req({
      token: auth(), query: { q: 'Compiler', domain: 'example.com', limit: '1' },
    }))
    expect(scoped.length).toBeLessThanOrEqual(1)
    expect(scoped.flatMap(item => item.emails).every(item => item.domain === 'example.com')).toBe(true)
    expect((await call(Threads, 'GET', req({
      token: auth(), query: { q: 'Compiler', domain: 'other.com' },
    }))).forbidden).toBe(true)
    expect((await call(Threads, 'GET', req({
      token: auth(), query: { q: 'x'.repeat(2001) },
    }))).error).toMatch(/2,000/)
    expect((await call(Threads, 'GET', req({ token: auth(), query: { limit: '0' } }))).error).toMatch(/limit/i)
  })

  it('classifies direct, group, and list conversations for the authenticated mailbox', async () => {
    const suffix = Date.now().toString(36)
    await app.emails.put(email({
      id: `personal-only-${suffix}`,
      subject: `Personal only ${suffix}`,
      threadId: 'c'.repeat(64),
      to: ['alice@example.com'],
    }))
    await app.emails.put(email({
      id: `personal-group-${suffix}`,
      subject: `Personal group ${suffix}`,
      threadId: 'd'.repeat(64),
      to: ['alice@example.com', 'bob@example.net'],
    }))
    await app.emails.put(email({
      id: `personal-list-${suffix}`,
      subject: `Personal list ${suffix}`,
      threadId: 'e'.repeat(64),
      recipient: 'alice@example.com',
      to: ['staff@example.net'],
    }))

    const res = await call(Threads, 'GET', req({ token: auth(), query: { folder: 'inbox' } }))
    const only = res.find(thread => thread.id === 'c'.repeat(64))
    const group = res.find(thread => thread.id === 'd'.repeat(64))
    const list = res.find(thread => thread.id === 'e'.repeat(64))

    expect(only).toMatchObject({
      personalLevel: 2,
      participants: ['bob@other.com', 'alice@example.com'],
    })
    expect(group).toMatchObject({
      personalLevel: 1,
      participants: ['bob@other.com', 'alice@example.com', 'bob@example.net'],
    })
    expect(list).toMatchObject({
      personalLevel: 0,
      participants: ['bob@other.com', 'staff@example.net'],
    })
  })

  it('finds the selected conversation across folders with attachment summaries', async () => {
    const subject = `Print conversation ${Date.now()}`
    await app.emails.put(email({
      id: 'print-inbox',
      subject,
      folder: 'inbox',
      receivedAt: '2026-01-01T10:00:00.000Z',
    }))
    await app.emails.put(email({
      id: 'print-sent',
      subject: `Re: ${subject}`,
      folder: 'sent',
      sender: 'alice@example.com',
      receivedAt: '2026-01-01T11:00:00.000Z',
    }))
    await app.emails.put(email({
      id: 'print-archive',
      subject: `Fwd: ${subject}`,
      folder: 'archive',
      receivedAt: '2026-01-01T12:00:00.000Z',
    }))
    await app.attachments.saveForEmail('print-sent', 'example.com', [{
      filename: 'agenda.pdf',
      contentType: 'application/pdf',
      content: new Uint8Array([1, 2, 3]),
    }])
    await app.emails.put(email({
      id: 'print-foreign',
      domain: 'other.com',
      recipient: 'carol@other.com',
      subject,
      folder: 'sent',
      receivedAt: '2026-01-01T13:00:00.000Z',
    }))

    const res = await call(Threads, 'GET', req({ token: auth(), query: { emailId: 'print-inbox' } }))

    expect(res).toHaveLength(1)
    expect(res[0].emails.map(message => message.id)).toEqual([
      'print-inbox',
      'print-sent',
      'print-archive',
    ])
    expect(res[0].emails[1].attachments).toEqual([
      expect.objectContaining({ filename: 'agenda.pdf', size: 3 }),
    ])
    expect(res[0].emails.some(message => message.id === 'print-foreign')).toBe(false)
  })

  it('uses RFC reference edges across folders without merging unrelated equal subjects', async () => {
    const subject = `RFC thread ${Date.now()}`
    const threadId = 'a'.repeat(64)
    await app.emails.put(email({
      id: 'rfc-root', subject, folder: 'archive', rfcMessageId: '<root@sender.test>', threadId,
      receivedAt: '2026-02-01T10:00:00.000Z',
    }))
    await app.emails.put(email({
      id: 'rfc-reply', subject: `Re: ${subject}`, folder: 'inbox', rfcMessageId: '<reply@sender.test>',
      inReplyTo: '<root@sender.test>', references: ['<root@sender.test>'], threadId,
      receivedAt: '2026-02-01T11:00:00.000Z',
    }))
    await app.emails.put(email({
      id: 'rfc-unrelated', subject, folder: 'inbox', rfcMessageId: '<unrelated@sender.test>',
      threadId: 'b'.repeat(64), receivedAt: '2026-02-01T12:00:00.000Z',
    }))

    const selected = await call(Threads, 'GET', req({ token: auth(), query: { emailId: 'rfc-reply' } }))
    expect(selected).toHaveLength(1)
    expect(selected[0].id).toBe(threadId)
    expect(selected[0].emails.map(message => message.id)).toEqual(['rfc-root', 'rfc-reply'])

    const inbox = await call(Threads, 'GET', req({ token: auth(), query: { folder: 'inbox' } }))
    const matching = inbox.filter(thread => thread.subject === subject)
    expect(matching).toHaveLength(2)
  })

  it('does not reveal a conversation selected outside the caller domains', async () => {
    const subject = `Foreign print ${Date.now()}`
    await app.emails.put(email({
      id: 'print-only-foreign',
      domain: 'other.com',
      recipient: 'carol@other.com',
      subject,
    }))

    expect(await call(Threads, 'GET', req({ token: auth(), query: { emailId: 'print-only-foreign' } })))
      .toEqual([])
  })
})

/** @param {Record<string, string>} query */
function streamOf(query) {
  const url = new URL('http://caduceus.test/inbox/stream')
  for (const [key, value] of Object.entries(query)) url.searchParams.set(key, value)
  return TestContainer.current.inject(new Stream()).streamInbox(new Request(url))
}

describe('GET /inbox/stream', () => {
  it('emits an unauthorized event without a token', async () => {
    const frames = []
    for await (const frame of streamOf({})) frames.push(frame)
    expect(frames.map(frame => frame.event)).toEqual(['unauthorized'])
  })

  it('accepts a token via the query string and replays fresh mail', async () => {
    const since = new Date(Date.now() - 60_000).toISOString()
    await app.emails.put(email({ id: 'st1', receivedAt: new Date().toISOString() }))
    const frames = []
    for await (const frame of streamOf({ token: auth(), since })) frames.push(frame)
    const events = frames.map(frame => frame.event)
    expect(events).toContain('connected')
    expect(events).toContain('reconnect')
  })

  it('builds a stream message with and without a cursor', () => {
    expect(Stream._sse('ping', { a: 1 }, '7')).toEqual({ event: 'ping', data: { a: 1 }, cursor: '7' })
    expect(Stream._sse('new-email', { a: 1 }, null)).toEqual({ event: 'new-email', data: { a: 1 }, cursor: null })
  })

  it('summarises an email without body or attachments', () => {
    const s = Stream._summarize({ ...email({ id: 'sum' }), spamScore: 4 })
    expect(s.body).toBeUndefined()
    expect(s.spamScore).toBe(4)
    expect(Stream._summarize(email({ id: 'sum2' })).spamScore).toBeUndefined()
  })

  it('keeps independent hosted and provider replay cursors', () => {
    const cursor = Stream._parseCursor('2026-08-22T12:00:00.000Z')
    cursor.hosted = { at: '2026-08-22T12:00:01.000Z', id: 'hosted-1' }
    const encoded = Stream._cursorID(cursor)
    expect(Stream._parseCursor(encoded)).toEqual({
      hosted: { at: '2026-08-22T12:00:01.000Z', id: 'hosted-1' },
      external: { at: '2026-08-22T12:00:00.000Z', id: '' },
    })
    expect(Stream._afterCursor({ receivedAt: '2026-08-22T12:00:00.000Z', id: 'provider-1' }, Stream._parseCursor(encoded).external)).toBe(true)
  })

  it('streams a credential-free connected-account summary', () => {
    expect(Stream._summarize({
      id: 'extmsg.route', external: true, providerAccountId: 'ext_AAAAAAAAAAAAAAAAAAAA', providerStableId: 'INBOX:1:2',
      mailboxAddress: 'alice@remote.example', mailboxLabel: 'Remote', mailboxColor: '#336699', sender: 'sender@example.net',
      recipient: 'alice@remote.example', subject: 'Provider mail', folder: 'inbox', systemFolder: 'inbox', read: false,
      starred: false, receivedAt: '2026-08-22T12:00:00.000Z', rawRFC822Base64: 'must-not-leak',
    })).toMatchObject({ external: true, providerStableId: 'INBOX:1:2', mailboxLabel: 'Remote' })
  })

  it('returns fresh inbox mail oldest-first', async () => {
    await app.emails.put(email({
      id: 'same-domain-private-stream',
      ownerEmail: 'bob@example.com',
      recipient: 'bob@example.com',
      subject: 'Bob stream secret',
    }))
    const fresh = await Stream._freshSince(
      app,
      ['example.com'],
      '2000-01-01T00:00:00.000Z',
      'alice@example.com',
    )
    const times = fresh.map(e => e.receivedAt)
    expect(times).toEqual([...times].sort())
    expect(fresh.map(e => e.id)).not.toContain('same-domain-private-stream')
  })

  it('replays the oldest bounded slice when more than one page was missed', async () => {
    const base = Date.parse('2030-01-01T00:00:00.000Z')
    for (let index = 0; index < 55; index++) {
      await app.emails.put(email({
        id: `stream-gap-${index}`,
        receivedAt: new Date(base + index * 1_000).toISOString(),
      }))
    }
    const fresh = await Stream._freshSince(app, ['example.com'], '2029-12-31T23:59:59.000Z', 'alice@example.com')
    expect(fresh).toHaveLength(50)
    expect(fresh[0].id).toBe('stream-gap-0')
    expect(fresh[49].id).toBe('stream-gap-49')
  })
})

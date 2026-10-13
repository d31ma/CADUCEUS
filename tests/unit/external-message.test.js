import { describe, expect, it } from 'bun:test'
import { Buffer } from 'node:buffer'
import { ExternalMessage } from '../support/legacy-server/shared/mailbox/ExternalMessage.js'

describe('external provider message presentation', () => {
  const accountId = 'ext_AAAAAAAAAAAAAAAAAAAA'
  const stableId = 'INBOX:77:9'

  it('round-trips only bounded route-safe identities', () => {
    const id = ExternalMessage.externalMessageId(accountId, stableId)
    expect(id).toMatch(/^extmsg\./)
    expect(ExternalMessage.parseExternalMessageId(id)).toEqual({ accountId, stableId })
    expect(ExternalMessage.parseExternalMessageId(`${id}x`)).toBeNull()
    expect(ExternalMessage.externalMessageId(accountId, 'bad\r\nid')).toBe('')
  })

  it('normalizes flags, folder role, mailbox metadata, and attachments for unified rows', () => {
    expect(ExternalMessage.presentExternalSummary({
      accountId, stableId, mailboxAddress: 'work@remote.example', accountLabel: 'Work', accountColor: '#336699',
      folderRole: 'inbox', sender: 'Sender <sender@example.net>', to: ['work@remote.example'], subject: 'Provider',
      snippet: 'body', flags: ['\\Seen', '\\Flagged'], labels: ['Projects', 'Projects', ' Finance '], receivedAt: '2026-08-21T12:00:00Z', attachmentNames: ['report.pdf'],
    })).toMatchObject({ external: true, providerAccountId: accountId, folder: 'inbox', labels: ['Projects', 'Finance'], read: true, starred: true, hasAttachments: true, domain: 'remote.example' })
  })

  it('parses authorized raw source and binds attachment reads to its index', async () => {
    const raw = Buffer.from('From: Sender <sender@example.net>\r\nTo: work@remote.example\r\nSubject: Provider\r\nMessage-ID: <one@example.net>\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain\r\n\r\nHello\r\n--x\r\nContent-Type: text/plain; name=note.txt\r\nContent-Disposition: attachment; filename=note.txt\r\n\r\nAttachment\r\n--x--\r\n')
    const indexed = { accountId, stableId, mailboxAddress: 'work@remote.example', folderRole: 'inbox', flags: [], receivedAt: '2026-08-21T12:00:00Z' }
    const detail = await ExternalMessage.presentExternalDetail(indexed, raw.toString('base64'))
    expect(detail).toMatchObject({ external: true, subject: 'Provider', body: 'Hello', hasRawSource: true, hasAttachments: true })
    expect(detail.attachments).toContainEqual(expect.objectContaining({ id: 'extatt.0', filename: 'note.txt' }))
    expect(await ExternalMessage.readExternalAttachment(raw.toString('base64'), 'extatt.0')).toMatchObject({ filename: 'note.txt', contentBase64: Buffer.from('Attachment').toString('base64') })
    expect(await ExternalMessage.readExternalAttachment(raw.toString('base64'), 'extatt.100')).toBeNull()
  })
})

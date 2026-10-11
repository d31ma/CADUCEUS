import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { createHmac } from 'node:crypto'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { call } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'
import { TestServer } from '../support/TestServer.mjs'

// POST /inbound/webhook runs the whole inbound pipeline: MIME parsing, the
// domain route, the owner's system folders, unsubscribe metadata, calendar
// invitations and found-in-mail contacts.

const SECRET = 'inbound-route-test-secret-0123456789abcdef'
const Detail = Routes.InboxIdController
const Attachment = Routes.InboxIdAttachmentsAttachmentIdController

let app
const image = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0xff, 0xfe, 0x80, 0xc3, 0x28, 0x00, 0x7f])

async function deliver(payload) {
  const body = JSON.stringify(payload)
  const response = await fetch(new URL('/inbound/webhook', TestServer.current.url), {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'x-caduceus-signature': createHmac('sha256', SECRET).update(body).digest('hex') },
    body,
  })
  return { status: response.status, body: await response.json() }
}

function message() {
  const invite = [
    'BEGIN:VCALENDAR', 'VERSION:2.0', 'METHOD:REQUEST', 'BEGIN:VEVENT',
    'UID:inbound-planning@sender.test', 'SUMMARY:Inbound planning',
    'DTSTART:20260904T150000Z', 'DTEND:20260904T160000Z',
    'ORGANIZER:mailto:Boss@Sender.test',
    'ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:alice@example.com',
    'END:VEVENT', 'END:VCALENDAR',
  ].join('\r\n')
  return [
    'From: Boss <Boss@Sender.test>',
    'To: Alice <alice@example.com>',
    'Subject: Planning with logo',
    'Message-ID: <inbound-one@sender.test>',
    'List-Unsubscribe: <mailto:leave@sender.test?subject=stop&body=now>, <https://sender.test/unsubscribe?u=1>',
    'List-Unsubscribe-Post: List-Unsubscribe=One-Click',
    'MIME-Version: 1.0',
    'Content-Type: multipart/mixed; boundary="outer"',
    '',
    '--outer',
    'Content-Type: multipart/related; boundary="inner"',
    '',
    '--inner',
    'Content-Type: text/html; charset=utf-8',
    '',
    '<p>Agenda attached</p><img src="cid:logo@sender.test">',
    '--inner',
    'Content-Type: image/png; name="logo.png"',
    'Content-Disposition: inline; filename="logo.png"',
    'Content-ID: <logo@sender.test>',
    'Content-Transfer-Encoding: base64',
    '',
    image.toString('base64'),
    '--inner--',
    '--outer',
    'Content-Type: text/calendar; method=REQUEST; charset=utf-8',
    'Content-Disposition: attachment; filename="invite.ics"',
    'Content-Transfer-Encoding: base64',
    '',
    Buffer.from(invite).toString('base64'),
    '--outer--',
    '',
  ].join('\r\n')
}

beforeAll(async () => {
  app = await setupRoutes({ INBOUND_WEBHOOK_SECRET: SECRET, SPAM_FILTER_DISABLED: 'true' })
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
  await app.domains.put({
    domain: 'example.com', inboundEnabled: true,
    routes: [{ id: 'all', match: '*@example.com', action: { type: 'store' }, enabled: true }],
  })
}, 300_000)
afterAll(teardownRoutes)

describe('POST /inbound/webhook', () => {
  it('delivers a MIME message with an inline image, an invitation and unsubscribe links', async () => {
    const delivered = await deliver({
      recipient: 'alice@example.com', sender: 'boss@sender.test', subject: 'Planning with logo',
      messageId: 'inbound-one', body: message(),
    })
    expect(delivered.status).toBe(200)
    expect(delivered.body).toMatchObject({ accepted: true, emailId: 'inbound-one' })

    const detail = await call(Detail, 'GET', req({ token: token(), paths: { id: 'inbound-one' } }))
    expect(detail).toMatchObject({
      folder: 'inbox', systemFolder: 'inbox', headerFrom: 'boss@sender.test',
      unsubscribeUrl: 'https://sender.test/unsubscribe?u=1',
      unsubscribeMailto: 'mailto:leave@sender.test?subject=stop&body=now',
      unsubscribeOneClick: true,
    })
    expect(detail.htmlBody).toContain('cid:logo@sender.test')
    const logo = detail.attachments.find(attachment => attachment.filename === 'logo.png')
    expect(logo).toMatchObject({ contentType: 'image/png', size: image.length })
    const fetched = await call(Attachment, 'GET', req({ token: token(), paths: { id: 'inbound-one', attachment_id: logo.id } }))
    expect(Buffer.from(fetched.contentBase64, 'base64').equals(image)).toBe(true)

    const events = await app.calendarEvents.list('alice@example.com')
    expect(events.find(event => event.icsUid === 'inbound-planning@sender.test')).toMatchObject({
      title: 'Inbound planning', sourceEmailId: 'inbound-one',
    })
    const contact = await app.contactRecords.findByEmail('alice@example.com', 'boss@sender.test')
    expect(contact).toMatchObject({ source: 'mail', interactionCount: 1 })
  })
})

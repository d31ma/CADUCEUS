import { expect, test } from '@playwright/test'
import {
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test.setTimeout(60_000)

async function authenticate(page) {
  const domain = `invite-card-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  return identity
}

function calendar({ uid, attendee, method = 'REQUEST', status = 'NEEDS-ACTION' }) {
  return [
    'BEGIN:VCALENDAR', 'VERSION:2.0', `METHOD:${method}`, 'BEGIN:VEVENT',
    `UID:${uid}`, 'SUMMARY:Quarterly planning',
    'DTSTART:20300301T140000Z', 'DTEND:20300301T150000Z',
    'LOCATION:Board room', 'ORGANIZER:mailto:boss@example.net',
    `ATTENDEE;CN=Carol;PARTSTAT=${status}:mailto:${attendee}`,
    'END:VEVENT', 'END:VCALENDAR',
  ].join('\r\n')
}

function multipartCalendarMessage(payload) {
  const encoded = Buffer.from(payload).toString('base64')
  return [
    'MIME-Version: 1.0',
    'Content-Type: multipart/mixed; boundary="caduceus-calendar-boundary"', '',
    '--caduceus-calendar-boundary', 'Content-Type: text/plain; charset=utf-8', '', 'Calendar update attached.',
    '--caduceus-calendar-boundary', 'Content-Type: text/calendar; name="calendar.dat"',
    'Content-Transfer-Encoding: base64', 'Content-Disposition: attachment; filename="calendar.dat"', '', encoded,
    '--caduceus-calendar-boundary', 'Content-Type: application/octet-stream; name="event.ics"',
    'Content-Transfer-Encoding: base64', 'Content-Disposition: attachment; filename="event.ics"', '', encoded,
    '--caduceus-calendar-boundary--', '',
  ].join('\r\n')
}

test('handles invite download, duplicate suppression, RSVP, and attendee replies', async ({ page }) => {
  const errors = captureUiErrors(page)
  const identity = await authenticate(page)
  const uid = `planning-${Date.now()}@example.net`
  const invitationSubject = `Planning invitation ${Date.now()}`
  const requestPayload = calendar({ uid, attendee: identity.email })
  const invitation = await deliverEmail({
    recipient: identity.email,
    sender: 'boss@example.net',
    subject: invitationSubject,
    body: multipartCalendarMessage(requestPayload),
    messageId: `invite-${Date.now()}@example.net`,
  })

  expect(invitation.accepted).toBe(true)
  await page.goto(`/email/${encodeURIComponent(invitation.emailId)}`)
  await waitForUiReady(page)

  const card = page.locator('.invite-card')
  await expect(card).toBeVisible()
  await expect(card).toContainText('Quarterly planning')
  await expect(card).toContainText('Board room')
  await expect(card).toContainText('Organizer: boss@example.net')
  await expect(card.getByRole('button', { name: 'Accept', exact: true })).toBeVisible()
  await expect(page.locator('.attachment-chip-name').filter({ hasText: /event\.ics|calendar\.dat/ })).toHaveCount(0)

  const downloadPromise = page.waitForEvent('download')
  await card.getByRole('button', { name: 'Download invitation event.ics' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toBe('event.ics')
  expect(await download.createReadStream().then(async stream => {
    const chunks = []
    for await (const chunk of stream) chunks.push(chunk)
    return Buffer.concat(chunks).toString('utf8')
  })).toBe(requestPayload)

  await card.getByRole('button', { name: 'Tentative', exact: true }).click()
  await expect(card.locator('.invite-status')).toContainText('Maybe')

  const replySubject = `Planning response ${Date.now()}`
  const response = await deliverEmail({
    recipient: identity.email,
    sender: 'Carol <carol@example.net>',
    subject: replySubject,
    body: multipartCalendarMessage(calendar({ uid, attendee: 'carol@example.net', method: 'REPLY', status: 'DECLINED' })),
    messageId: `reply-${Date.now()}@example.net`,
  })
  expect(response.accepted).toBe(true)
  await page.goto(`/email/${encodeURIComponent(response.emailId)}`)
  await waitForUiReady(page)
  await expect(card).toContainText('Carol declined this invitation.')
  await expect(card.getByRole('button', { name: 'Accept', exact: true })).toHaveCount(0)
  await expect(card.locator('.invite-status')).toContainText('Not going')
  expect(errors).toEqual([])
})

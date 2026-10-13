import { test, expect } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { deliverEmail, getToken, loginAs, seedDomain, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function authenticatedMailbox(page) {
  const domain = `mbox-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  // Exercise the portable fallback in headless Chromium. The native File
  // System Access path is covered by the same streaming writer and differs
  // only in its destination sink.
  await page.addInitScript(() => {
    try { Object.defineProperty(window, 'showSaveFilePicker', { value: undefined, configurable: true }) } catch {}
  })
  return { domain, email }
}

test('exports the current folder as a byte-safe mboxrd download', async ({ page }) => {
  const { domain, email } = await authenticatedMailbox(page)
  const first = [
    'Return-Path: <bounce@sender.test>',
    'From: Sender <sender@sender.test>',
    `To: ${email}`,
    'Date: Fri, 14 Aug 2026 12:00:00 +0000',
    'Subject: First export message',
    '',
    'From body delimiter',
  ].join('\r\n')
  const second = [
    'From: Other <other@sender.test>',
    `To: ${email}`,
    'Date: Fri, 14 Aug 2026 12:01:00 +0000',
    'Subject: Second export message',
    '',
    '>From already quoted',
  ].join('\r\n')
  await deliverEmail({ recipient: email, sender: 'sender@sender.test', subject: 'First export message', body: first })
  await deliverEmail({ recipient: email, sender: 'other@sender.test', subject: 'Second export message', body: second })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.getByText('First export message')).toBeVisible()
  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export folder as mbox' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toBe(`${domain}-inbox.mbox`)
  const path = await download.path()
  const content = await readFile(path, 'utf8')
  expect(content.match(/^From /gm)).toHaveLength(2)
  expect(content).toContain('\n>From body delimiter\n\n')
  expect(content).toContain('\n>>From already quoted\n\n')
  await expect(page.getByText(/Exported 2 messages/)).toBeVisible()
})

test('exports a folder as an archive of exact individual EML sources', async ({ page }) => {
  const { domain, email } = await authenticatedMailbox(page)
  const deliveryDate = new Date().toISOString().slice(0, 10)
  const first = [
    'From: Sender <sender@sender.test>',
    `To: ${email}`,
    'Date: Fri, 14 Aug 2026 12:00:00 +0000',
    'Subject: First / archive message',
    '',
    'First exact body',
  ].join('\r\n')
  const second = [
    'From: Other <other@sender.test>',
    `To: ${email}`,
    'Date: Fri, 14 Aug 2026 12:01:00 +0000',
    'Subject: Second archive message',
    '',
    'Second exact body',
  ].join('\n')
  await deliverEmail({ recipient: email, sender: 'sender@sender.test', subject: 'First / archive message', body: first })
  await deliverEmail({ recipient: email, sender: 'other@sender.test', subject: 'Second archive message', body: second })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.getByText('First / archive message')).toBeVisible()
  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export folder as EML archive' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toBe(`${domain}-inbox-eml.zip`)
  const path = await download.path()
  const archive = await readFile(path)
  const entries = localZipEntries(archive)
  expect([...entries.keys()]).toEqual([
    `messages/000001-${deliveryDate}-First - archive message.eml`,
    `messages/000002-${deliveryDate}-Second archive message.eml`,
  ])
  expect(entries.get([...entries.keys()][0])).toEqual(Buffer.from(first))
  expect(entries.get([...entries.keys()][1])).toEqual(Buffer.from(second))
  await expect(page.getByText(/Exported 2 messages as individual EML files/)).toBeVisible()
})

test('cancels a stalled export and installs no partial download', async ({ page }) => {
  await authenticatedMailbox(page)
  await page.goto('/inbox')
  await waitForUiReady(page)
  // The export request never gets an answer, so the export stalls until it
  // is canceled.
  await page.route('**/inbox/export/rfc822**', () => {})

  let downloaded = false
  page.on('download', () => { downloaded = true })
  await page.getByRole('button', { name: 'Export folder as mbox' }).click()
  await expect(page.locator('[data-mbox-export-progress]')).toBeVisible()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(page.locator('[data-mbox-export-progress]')).toHaveCount(0)
  await expect(page.getByText('Mailbox export canceled.')).toBeVisible()
  expect(downloaded).toBe(false)
})

function localZipEntries(archive) {
  const entries = new Map()
  let offset = 0
  while (offset + 4 <= archive.byteLength && archive.readUInt32LE(offset) === 0x04034b50) {
    const nameLength = archive.readUInt16LE(offset + 26)
    const extraLength = archive.readUInt16LE(offset + 28)
    const nameStart = offset + 30
    const extraStart = nameStart + nameLength
    expect(archive.readUInt16LE(extraStart)).toBe(0x0001)
    const size = Number(archive.readBigUInt64LE(extraStart + 4))
    const dataStart = extraStart + extraLength
    const name = archive.subarray(nameStart, extraStart).toString('utf8')
    entries.set(name, archive.subarray(dataStart, dataStart + size))
    offset = dataStart + size
  }
  return entries
}

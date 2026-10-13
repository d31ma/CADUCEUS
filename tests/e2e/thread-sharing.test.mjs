import { test, expect } from '@playwright/test'
import {
  API,
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

function sharedMime() {
  return [
    'MIME-Version: 1.0',
    'Content-Type: multipart/mixed; boundary="shared-boundary"',
    '',
    '--shared-boundary',
    'Content-Type: text/html; charset=utf-8',
    '',
    '<p id="shared-proof">Private <strong>launch plan</strong></p><script>window.__sharedScriptRan=true</script><img src="https://tracker.invalid/share-pixel">',
    '--shared-boundary',
    'Content-Type: text/plain; charset=utf-8; name="launch.txt"',
    'Content-Transfer-Encoding: base64',
    'Content-Disposition: attachment; filename="launch.txt"',
    '',
    Buffer.from('shared attachment proof\n').toString('base64'),
    '--shared-boundary--',
    '',
  ].join('\r\n')
}

test('creates, reads, downloads, and immediately revokes a private conversation capability', async ({ page }) => {
  const domain = `share-${Date.now()}.test`
  const mailbox = `alice@${domain}`
  await seedUser(mailbox, { domains: [domain], role: 'admin' })
  const alice = await getToken(mailbox)
  await seedDomain(alice.token, domain)
  const subject = `Private launch ${Date.now()}`
  await deliverEmail({
    recipient: mailbox,
    sender: 'partner@example.net',
    subject,
    body: sharedMime(),
    messageId: `shared-${Date.now()}@example.net`,
  })

  const remoteRequests = []
  page.on('request', request => {
    if (request.url().startsWith('https://tracker.invalid/')) remoteRequests.push(request.url())
  })
  await page.setViewportSize({ width: 1280, height: 900 })
  await useTestApi(page)
  await loginAs(page, alice.token, alice.email, alice.role, alice.domains)
  const errors = captureUiErrors(page)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText(subject, { exact: true }).first().click()

  await page.getByRole('button', { name: 'Share conversation' }).click()
  const dialog = page.getByRole('dialog', { name: 'Share conversation' })
  await expect(dialog).toBeVisible()
  await dialog.getByLabel('Sharing link expiry').selectOption('1')
  const createdResponse = page.waitForResponse(response =>
    response.url().endsWith('/thread-shares') && response.request().method() === 'POST')
  await dialog.getByRole('button', { name: 'Create new link' }).click()
  const created = await (await createdResponse).json()
  expect(created.token).toMatch(/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]{43}$/)
  const linkInput = dialog.getByLabel('New conversation sharing link')
  await expect(linkInput).toHaveValue(new RegExp(`/share/${created.token.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`))
  await expect(dialog.getByRole('button', { name: 'Copy link' })).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Open link' })).toBeVisible()
  await expect(dialog.getByText(/Expires/).first()).toBeVisible()

  // The product list deliberately cannot reproduce a raw capability after creation.
  const allManaged = await fetch(`${API}/thread-shares`, {
    headers: { Authorization: `Bearer ${alice.token}` },
  }).then(response => response.json())
  expect(JSON.stringify(allManaged)).not.toContain(created.token)

  const shareUrl = await linkInput.inputValue()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(shareUrl)
  await expect(page.getByRole('heading', { level: 1, name: subject })).toBeVisible()
  await expect(page.getByText('Privacy protection is active.')).toBeVisible()
  const frame = page.frameLocator('.shared-thread-html')
  await expect(frame.locator('#shared-proof')).toContainText('Private launch plan')
  expect(await page.evaluate(() => window.__sharedScriptRan)).toBeUndefined()
  expect(remoteRequests).toEqual([])

  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Download launch.txt' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toBe('launch.txt')
  expect(await download.createReadStream().then(async stream => {
    const chunks = []
    for await (const chunk of stream) chunks.push(chunk)
    return Buffer.concat(chunks).toString('utf8')
  })).toBe('shared attachment proof\n')

  const revoked = await fetch(`${API}/thread-shares/${encodeURIComponent(created.id)}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${alice.token}` },
  })
  expect(revoked.ok).toBe(true)
  const revokedPublic = await fetch(`${API}/thread-shares/public/${encodeURIComponent(created.token)}`)
  expect(revokedPublic.status).toBe(404)
  expect(errors).toEqual([])
})

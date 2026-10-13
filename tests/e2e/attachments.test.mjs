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

const PNG_1X1 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

async function authenticate(page, label) {
  const domain = `${label}-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  return identity
}

function relatedMessage() {
  return [
    'MIME-Version: 1.0',
    'Content-Type: multipart/related; boundary="caduceus-attachment-boundary"',
    '',
    '--caduceus-attachment-boundary',
    'Content-Type: text/html; charset=utf-8',
    'Content-Transfer-Encoding: 8bit',
    '',
    '<p>Inline company mark</p><img id="cid-logo" src="cid:logo@example.test" alt="Company mark">',
    '--caduceus-attachment-boundary',
    'Content-Type: image/png; name="logo.png"',
    'Content-Transfer-Encoding: base64',
    'Content-ID: <logo@example.test>',
    'Content-Disposition: inline; filename="logo.png"',
    '',
    PNG_1X1,
    '--caduceus-attachment-boundary',
    'Content-Type: text/plain; charset=utf-8; name="notes.txt"',
    'Content-Transfer-Encoding: base64',
    'Content-Disposition: attachment; filename="notes.txt"',
    '',
    Buffer.from('attachment preview proof\n').toString('base64'),
    '--caduceus-attachment-boundary',
    'Content-Type: image/svg+xml; name="active.svg"',
    'Content-Transfer-Encoding: base64',
    'Content-Disposition: attachment; filename="active.svg"',
    '',
    Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>').toString('base64'),
    '--caduceus-attachment-boundary--',
    '',
  ].join('\r\n')
}

test('renders authenticated CID images and inert attachment previews and downloads', async ({ page }) => {
  const errors = captureUiErrors(page)
  const identity = await authenticate(page, 'attachment-reader')
  const subject = `Attachment reader ${Date.now()}`
  const delivered = await deliverEmail({
    recipient: identity.email,
    sender: 'design@example.net',
    subject,
    body: relatedMessage(),
    messageId: `attachment-reader-${Date.now()}@example.net`,
  })

  await page.goto(`/email/${delivered.emailId}`)
  await waitForUiReady(page)
  await expect(page.getByText(subject, { exact: true })).toBeVisible()

  const frameImage = page.frameLocator('.email-html-frame').locator('#cid-logo')
  await expect(frameImage).toBeVisible()
  await expect(frameImage).toHaveAttribute('src', /^data:image\/png;base64,/)
  await expect(page.locator('.attachment-thumbnail')).toHaveCount(1)
  await expect(page.getByRole('button', { name: 'Preview active.svg' })).toHaveCount(0)

  await page.getByRole('button', { name: 'Preview notes.txt' }).click()
  const dialog = page.getByRole('dialog', { name: 'notes.txt' })
  await expect(dialog).toBeVisible()
  await expect(dialog.locator('.attachment-preview-text')).toContainText('attachment preview proof')
  await dialog.getByRole('button', { name: 'Close attachment preview' }).click()
  await expect(dialog).toHaveCount(0)

  const disabled = await fetch(`${API}/users/me/preferences`, {
    method: 'PUT',
    headers: { Authorization: `Bearer ${identity.token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ mailPreferences: { displayAttachmentPreviews: false } }),
  })
  expect(disabled.status).toBe(200)
  await page.reload()
  await waitForUiReady(page)
  await expect(page.frameLocator('.email-html-frame').locator('#cid-logo')).toBeVisible()
  await expect(page.locator('.attachment-thumbnail')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Preview notes.txt' })).toHaveCount(0)

  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Download notes.txt' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toBe('notes.txt')
  expect(await download.createReadStream().then(async stream => {
    const chunks = []
    for await (const chunk of stream) chunks.push(chunk)
    return Buffer.concat(chunks).toString('utf8')
  })).toBe('attachment preview proof\n')
  expect(errors).toEqual([])
})

test('accepts the same bounded attachment set from drag/drop and paste', async ({ page }) => {
  const errors = captureUiErrors(page)
  await authenticate(page, 'attachment-compose')
  await page.goto('/compose')
  await waitForUiReady(page)
  await expect(page.locator('.compose-panel')).toHaveClass(/ready/)

  await page.locator('.compose-panel').evaluate(panel => {
    const transfer = new DataTransfer()
    transfer.items.add(new File(['drag proof'], 'drag-note.txt', { type: 'text/plain' }))
    panel.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: transfer }))
  })
  await expect(page.getByText('Drop files to attach', { exact: true })).toBeVisible()

  await page.locator('.compose-panel').evaluate(panel => {
    const transfer = new DataTransfer()
    transfer.items.add(new File(['drag proof'], 'drag-note.txt', { type: 'text/plain' }))
    panel.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: transfer }))
  })
  await expect(page.locator('.attachment-pill').filter({ hasText: 'drag-note.txt' })).toBeVisible()

  await page.locator('[data-compose-editor]').evaluate(editor => {
    const transfer = new DataTransfer()
    transfer.items.add(new File(['paste proof'], 'pasted-note.txt', { type: 'text/plain' }))
    editor.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: transfer }))
  })
  await expect(page.locator('.attachment-pill').filter({ hasText: 'pasted-note.txt' })).toBeVisible()
  await page.getByRole('button', { name: 'Remove drag-note.txt' }).click()
  await expect(page.locator('.attachment-pill').filter({ hasText: 'drag-note.txt' })).toHaveCount(0)
  expect(errors).toEqual([])
})

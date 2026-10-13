import { test, expect } from '@playwright/test'
import { API, captureUiErrors, deliverEmail, getToken, loginAs, seedDomain, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

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

test('formats semantic rich text with keyboard and accessible toolbar controls', async ({ page }) => {
  const errors = captureUiErrors(page)
  await authenticate(page, 'rich-format')
  await page.goto('/compose')
  await waitForUiReady(page)
  const editor = page.locator('[data-compose-editor]')
  await editor.fill('Format this')
  await editor.evaluate(node => {
    const range = document.createRange()
    range.selectNodeContents(node)
    const selection = getSelection()
    selection.removeAllRanges()
    selection.addRange(range)
  })
  await page.getByRole('button', { name: 'Bold' }).click()
  expect(errors).toEqual([])
  await expect(editor.locator('strong')).toHaveText('Format this')

  await editor.press('Control+u')
  await expect(editor.locator('u')).toHaveCount(1)
  const toolbar = page.getByRole('toolbar', { name: 'Message formatting' })
  await toolbar.getByLabel('Block style').focus()
  await page.keyboard.press('ArrowRight')
  await expect(toolbar.getByLabel('Font family')).toBeFocused()

  await editor.fill('**markdown bold**')
  await expect(editor.getByText('markdown bold', { exact: true })).toBeVisible()
  await editor.fill('Visit https://example.com')
  await editor.press(' ')
  await expect(editor.locator('a')).toHaveAttribute('href', 'https://example.com/')
  await editor.press('Control+Shift+b')
  await expect(page.locator('[data-compose-bcc] input')).toBeFocused()
  await page.locator('[data-compose-bcc] input').fill('private@example.net')
  await page.keyboard.press('Control+Shift+t')
  await expect(page.locator('[data-compose-to] input')).toBeFocused()
  expect(errors).toEqual([])
})

test('normalizes rich colors and Windows line endings at the paste boundary', async ({ page }) => {
  const errors = captureUiErrors(page)
  await authenticate(page, 'rich-paste')
  await page.goto('/compose')
  await waitForUiReady(page)
  const editor = page.locator('[data-compose-editor]')

  await editor.evaluate(node => {
    node.focus()
    const range = document.createRange()
    range.selectNodeContents(node)
    range.collapse(false)
    const selection = getSelection()
    selection.removeAllRanges()
    selection.addRange(range)
    const transfer = new DataTransfer()
    transfer.setData('text/html', '<span style="color:rgb(240,239,236);font-family:Example-Sans, sans-serif">Theme-safe</span><span style="color:rgb(200,20,20)">Intentional red</span><img src="https://tracker.invalid/pixel" onerror="alert(1)"><script>alert(1)</script>')
    transfer.setData('text/plain', 'Theme-safe Intentional red')
    node.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: transfer }))
  })

  await expect(editor.getByText('Theme-safe', { exact: true })).toBeVisible()
  const richHTML = await editor.innerHTML()
  expect(richHTML).toContain('Example-Sans')
  expect(richHTML).not.toContain('240, 239, 236')
  expect(richHTML).toMatch(/color:\s*rgb\(200,\s*20,\s*20\)/)
  expect(richHTML).not.toContain('<img')
  expect(richHTML).not.toContain('<script')

  await editor.evaluate(node => {
    const range = document.createRange()
    range.selectNodeContents(node)
    range.collapse(false)
    const selection = getSelection()
    selection.removeAllRanges()
    selection.addRange(range)
    const transfer = new DataTransfer()
    transfer.setData('text/plain', 'First\r\n\r\nSecond')
    node.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: transfer }))
  })
  await expect(editor).toContainText('First')
  const plainHTML = await editor.innerHTML()
  expect(plainHTML).not.toContain('\r')
  expect(plainHTML).toContain('First<br><br>Second')
  expect(errors).toEqual([])
})

test('authors, resizes, autosaves, and sends inline CID images', async ({ page }) => {
  const errors = captureUiErrors(page)
  const failedRequests = []
  page.on('requestfailed', request => failedRequests.push(request.url()))
  await authenticate(page, 'rich-inline')
  await page.goto('/compose')
  await waitForUiReady(page)
  const editor = page.locator('[data-compose-editor]')
  await editor.fill('<p>Inline image follows</p>')
  await page.locator('[data-inline-image-input]').setInputFiles({
    name: 'inline.png', mimeType: 'image/png', buffer: Buffer.from(PNG_1X1, 'base64'),
  })
  await expect.poll(() => errors).toEqual([])
  await expect(page.locator('.attachment-pill.inline')).toContainText('Inline')
  await expect.poll(() => page.evaluate(async () => await new Promise(resolve => {
    const request = indexedDB.open('caduceus-offline-write-vault')
    request.onerror = () => resolve(0)
    request.onsuccess = () => {
      const read = request.result.transaction('snapshots', 'readonly').objectStore('snapshots').getAll()
      read.onerror = () => resolve(0)
      read.onsuccess = () => resolve(read.result.length)
    }
  }))).toBeGreaterThan(0)
  const recoveryStorage = await page.evaluate(async () => ({
    plaintext: localStorage.getItem('caduceus-compose-recovery'),
    serializedVault: await new Promise(resolve => {
      const request = indexedDB.open('caduceus-offline-write-vault')
      request.onerror = () => resolve('')
      request.onsuccess = () => {
        const read = request.result.transaction('snapshots', 'readonly').objectStore('snapshots').getAll()
        read.onerror = () => resolve('')
        read.onsuccess = () => resolve(JSON.stringify(read.result))
      }
    }),
  }))
  expect(recoveryStorage.plaintext).toBeNull()
  expect(recoveryStorage.serializedVault).not.toContain('cid:caduceus.')

  const image = editor.locator('img[data-caduceus-inline-image]')
  await expect(image).toBeVisible()
  await expect(image).toHaveAttribute('src', /^data:image\/png;base64,/)
  await expect(page.locator('.attachment-pill.inline')).toContainText('Inline')
  await image.click()
  await page.getByRole('group', { name: 'Selected inline image' }).getByRole('button', { name: 'Small' }).click()
  await expect(image).toHaveAttribute('width', '240')

  await page.locator('[data-compose-to] input').fill('recipient@example.net')
  await page.locator('[data-compose-bcc] input').fill('audit@example.net')
  await page.locator('[data-compose-subject] input').fill('Inline image delivery')
  let payload
  await page.route(`${API}/send`, async route => {
    payload = route.request().postDataJSON()
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ sent: true, undoId: 'rich-inline' }) })
  })
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect.poll(() => payload).toBeTruthy()
  expect(payload.html).toMatch(/src="cid:caduceus\.[^"]+@inline"/)
  expect(payload.html).not.toContain('data:image/png;base64')
  expect(payload.attachments).toHaveLength(1)
  expect(payload.attachments[0]).toMatchObject({
    filename: 'inline.png', contentType: 'image/png', disposition: 'inline',
  })
  expect(payload.attachments[0].contentId).toMatch(/^caduceus\..+@inline$/)
  expect(payload.bcc).toEqual(['audit@example.net'])
  expect(failedRequests).toEqual([])
  expect(errors).toEqual([])
})

test('encrypts a failed draft autosave and reconciles it after reconnect', async ({ page }) => {
  const identity = await authenticate(page, 'offline-draft')
  let attempts = 0
  await page.route(`${API}/drafts`, async route => {
    if (route.request().method() !== 'POST') return route.continue()
    attempts += 1
    if (attempts === 1) return route.abort('failed')
    return route.continue()
  })
  await page.goto('/compose')
  await waitForUiReady(page)
  const subject = `Encrypted offline draft ${Date.now()}`
  const body = 'This plaintext must never be persisted outside the encrypted vault.'
  await page.locator('[data-compose-to] input').fill('recipient@example.net')
  await page.locator('[data-compose-subject] input').fill(subject)
  await page.locator('[data-compose-editor]').fill(body)
  await expect(page.locator('.compose-draft-status')).toContainText('encrypted autosave queued')

  const persisted = await page.evaluate(async () => await new Promise(resolve => {
    const request = indexedDB.open('caduceus-offline-write-vault')
    request.onerror = () => resolve({ count: 0, serialized: '' })
    request.onsuccess = () => {
      const read = request.result.transaction('writes', 'readonly').objectStore('writes').getAll()
      read.onerror = () => resolve({ count: 0, serialized: '' })
      read.onsuccess = () => resolve({ count: read.result.length, serialized: JSON.stringify(read.result) })
    }
  }))
  expect(persisted.count).toBe(1)
  expect(persisted.serialized).not.toContain(subject)
  expect(persisted.serialized).not.toContain(body)
  expect(await page.evaluate(() => localStorage.getItem('caduceus-compose-recovery'))).toBeNull()

  const synchronized = page.waitForResponse(response => response.url().endsWith('/drafts')
    && response.request().method() === 'POST' && response.ok())
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  await synchronized
  await expect(page.locator('.compose-draft-status')).toContainText('Saved')
  expect(attempts).toBe(2)

  const drafts = await fetch(`${API}/inbox?folder=drafts`, {
    headers: { Authorization: `Bearer ${identity.token}` },
  }).then(response => response.json())
  expect(drafts.filter(draft => draft.subject === subject)).toHaveLength(1)
  await expect.poll(() => page.evaluate(async () => await new Promise(resolve => {
    const request = indexedDB.open('caduceus-offline-write-vault')
    request.onerror = () => resolve(-1)
    request.onsuccess = () => {
      const count = request.result.transaction('writes', 'readonly').objectStore('writes').count()
      count.onerror = () => resolve(-1)
      count.onsuccess = () => resolve(count.result)
    }
  }))).toBe(0)
})

test('collapses, expands, and removes safely escaped reply quotes', async ({ page }) => {
  const errors = captureUiErrors(page)
  const identity = await authenticate(page, 'rich-quote')
  const subject = `Quote source ${Date.now()}`
  const delivered = await deliverEmail({
    recipient: identity.email,
    sender: 'source@example.net',
    subject,
    body: 'quoted & source proof',
    messageId: `rich-quote-${Date.now()}@example.net`,
  })
  await page.goto(`/email/${delivered.emailId}`)
  await waitForUiReady(page)
  await expect(page.getByText(subject, { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Reply', exact: true }).first().click()
  await waitForUiReady(page)

  const editor = page.locator('[data-compose-editor]')
  const quote = editor.locator('[data-caduceus-quote]')
  await expect(quote).toBeHidden()
  await page.getByRole('button', { name: 'Show quoted text' }).click()
  await expect(quote).toBeVisible()
  await expect(quote).toContainText('quoted & source proof')
  await expect(quote.locator('b')).toHaveCount(0)
  await page.getByRole('button', { name: 'Remove quoted text' }).click()
  await expect(quote).toHaveCount(0)
  expect(errors).toEqual([])
})

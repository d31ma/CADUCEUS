/**
 * E2E app interaction tests — runs against the real local API server.
 *
 * Each test seeds an isolated user and obtains a high-assurance test JWT.
 *
 * Run: bun run test:e2e
 */
import { test, expect } from '@playwright/test'
import { API, captureUiErrors, seedUser, seedDomain, deliverEmail, useTestApi, loginAs, getToken, waitForUiReady } from './helpers.mjs'

// ── Fixture: authenticated page ───────────────────────────────────────────────

async function withAuth(page, opts = {}) {
  const domain = opts.domain ?? `e2e-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = opts.email ?? `test-${Date.now()}@${domain}`
  const userOpts = { domains: [domain], ...(opts.user ?? {}) }
  await seedUser(email, userOpts)
  const data = await getToken(email)
  await useTestApi(page)
  await loginAs(page, data.token, data.email, data.role, data.domains)
  return { email, token: data.token, domains: data.domains }
}

// Navigates to '/' and waits for the app screen to be visible.
async function gotoApp(page) {
  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.getByRole('link', { name: 'CADUCEUS' })).toBeVisible()
}

const composeField = (page, placeholder) => {
  const hosts = {
    'recipient@example.com': '[data-compose-to]',
    optional: '[data-compose-cc]',
    Subject: '[data-compose-subject]',
  }
  const selector = hosts[placeholder]
  return selector ? page.locator(`${selector} input`) : page.locator(`input[placeholder="${placeholder}"]`)
}

// ── Navigation ────────────────────────────────────────────────────────────────

test('shows app header after login', async ({ page }) => {
  const { email } = await withAuth(page)
  await gotoApp(page)
  await expect(page.locator('.app-user-trigger')).toHaveAttribute('title', email)
})

test('shows login screen when not authenticated', async ({ page }) => {
  await useTestApi(page)
  await page.goto('/')
  await expect(page.getByText('Sign in')).toBeVisible()
})

test('Sign out returns to login screen', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await page.locator('.app-user-trigger').click()
  await page.getByRole('menuitem', { name: 'Sign out' }).click()
  await expect(page.getByText('Sign in')).toBeVisible()
})

// ── Inbox ─────────────────────────────────────────────────────────────────────

test('shows empty inbox message when no emails', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await expect(page.getByText(/No emails/)).toBeVisible()
})

test('401 response logs out and shows login screen', async ({ page }) => {
  await useTestApi(page)
  await page.addInitScript(() => {
    if (sessionStorage.getItem('caduceus_e2e_session_seeded')) return
    sessionStorage.setItem('caduceus_token', 'invalid-token')
    sessionStorage.setItem('caduceus_email', 'user@example.com')
    sessionStorage.setItem('caduceus_role', 'admin')
    sessionStorage.setItem('caduceus_domains', '["example.com"]')
    sessionStorage.setItem('caduceus_e2e_session_seeded', 'true')
  })
  await page.goto('/')
  await expect(page.getByText('Sign in')).toBeVisible()
})

test('searches inbox messages', async ({ page }) => {
  const { email, token, domains } = await withAuth(page)
  await seedDomain(token, domains[0])
  const key = `playwright-${Date.now()}`
  await deliverEmail({
    recipient: email,
    sender: 'billing@vendor.com',
    subject: `${key} Invoice`,
    body: 'blue searchable body',
    messageId: `${key}-invoice`,
  })
  await deliverEmail({
    recipient: email,
    sender: 'alerts@vendor.com',
    subject: `${key} Alert`,
    body: 'green hidden body',
    messageId: `${key}-alert`,
  })

  await gotoApp(page)
  await expect(page.getByText(`${key} Invoice`)).toBeVisible()
  await page.locator('[data-thread-search-input]').fill('blue searchable')
  await page.getByRole('button', { name: 'Search', exact: true }).click()
  await expect(page.getByText(`${key} Invoice`)).toBeVisible()
  await expect(page.getByText(`${key} Alert`)).not.toBeVisible()
})

test('uses the off-canvas sidebar and scrollable rows at phone widths', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await withAuth(page)
  await gotoApp(page)

  await expect(page.locator('.app-sidebar')).toBeHidden()
  await expect(page.locator('.mobile-bottom-nav,.mobile-nav')).toHaveCount(0)
  await page.getByRole('button', { name: 'Open navigation menu' }).click()
  await expect(page.locator('.app-sidebar')).toBeVisible()
  await page.locator('.sidebar-scrim').click({ position: { x: 380, y: 20 } })
  await expect(page.locator('.app-sidebar')).toBeHidden()

  await expect(page.locator('.folder-tabs')).toBeVisible()
  await expect(page.locator('.folder-tabs .overflow-more-btn')).toHaveCount(0)
  expect(await page.locator('.folder-tabs').evaluate(el => getComputedStyle(el).overflowX)).toMatch(/auto|scroll/)
})

test('loads app pages directly from Tachyon routes', async ({ page }) => {
  await withAuth(page)

  await page.goto('/compose')
  await waitForUiReady(page)
  await expect(composeField(page, 'recipient@example.com')).toBeVisible()

  await page.goto('/settings')
  await waitForUiReady(page)
  await expect(page.locator('.settings-panel .settings-section-title').filter({ hasText: 'Notifications' })).toBeVisible()

  await page.goto('/contacts')
  await waitForUiReady(page)
  await expect(page.getByRole('heading', { name: 'Contacts' })).toBeVisible()

  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.getByText(/No emails/)).toBeVisible()
})

test('loads an email detail page directly', async ({ page }) => {
  const { email, token, domains } = await withAuth(page)
  await seedDomain(token, domains[0])
  const key = `direct-${Date.now()}`
  const delivered = await deliverEmail({
    recipient: email,
    sender: 'news@example.com',
    subject: `${key} Direct route`,
    body: 'opened from a Tachyon page route',
    messageId: `${key}-message`,
  })

  await page.goto(`/email/${encodeURIComponent(delivered.emailId)}`)
  await waitForUiReady(page)
  await expect(page.getByText(`${key} Direct route`)).toBeVisible()
  await expect(page.getByText('opened from a Tachyon page route')).toBeVisible()
})

test('switches safe render modes and downloads the exact original EML', async ({ page }) => {
  const { email, token, domains } = await withAuth(page)
  await seedDomain(token, domains[0])
  const key = `source-${Date.now()}`
  const subject = `${key} Source modes`
  const raw = [
    'From: Sender <sender@other.test>',
    `To: ${email}`,
    `Subject: ${subject}`,
    'MIME-Version: 1.0',
    'Content-Type: multipart/alternative; boundary="caduceus-boundary"',
    '',
    '--caduceus-boundary',
    'Content-Type: text/plain; charset=utf-8',
    '',
    'Plain source body',
    '--caduceus-boundary',
    'Content-Type: text/html; charset=utf-8',
    '',
    '<p>Protected <strong>HTML source body</strong></p>',
    '--caduceus-boundary--',
    '',
  ].join('\r\n')
  const delivered = await deliverEmail({
    recipient: email,
    sender: 'sender@other.test',
    subject,
    body: raw,
    messageId: `${key}-message`,
  })

  await page.goto(`/email/${encodeURIComponent(delivered.emailId)}`)
  await waitForUiReady(page)
  await expect(page.frameLocator('.email-html-frame').getByText('HTML source body')).toBeVisible()

  await page.getByRole('button', { name: 'Plain text', exact: true }).click()
  await expect(page.locator('.email-body')).toContainText('Plain source body')
  await expect(page.locator('.email-html-frame')).toHaveCount(0)

  const sourceResponse = page.waitForResponse(response =>
    response.url().endsWith(`/inbox/${encodeURIComponent(delivered.emailId)}/source`) && response.status() === 200)
  await page.getByRole('button', { name: 'Original', exact: true }).click()
  await sourceResponse
  await expect(page.getByLabel('Original RFC 822 message source')).toHaveText(raw)

  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Download original EML' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/\.eml$/)
  const downloadPath = await download.path()
  expect(downloadPath).toBeTruthy()
  expect(await Bun.file(downloadPath).text()).toBe(raw)
})

test('threads RFC replies across folders without merging an equal-subject message', async ({ page }) => {
  const { email, token, domains } = await withAuth(page)
  await seedDomain(token, domains[0])
  const key = `thread-${Date.now()}`
  const subject = `${key} Deployment review`
  const source = ({ messageId, inReplyTo = '', references = '', text }) => [
    'From: Reviewer <reviewer@other.test>',
    `To: ${email}`,
    `Subject: ${inReplyTo ? `Re: ${subject}` : subject}`,
    `Message-ID: <${messageId}>`,
    ...(inReplyTo ? [`In-Reply-To: <${inReplyTo}>`] : []),
    ...(references ? [`References: ${references}`] : []),
    'Content-Type: text/plain; charset=utf-8',
    '',
    text,
  ].join('\r\n')

  await deliverEmail({
    recipient: email, sender: 'reviewer@other.test', subject,
    body: source({ messageId: `${key}-root@other.test`, text: 'Root message' }),
    messageId: `${key}-root`,
  })
  await deliverEmail({
    recipient: email, sender: 'reviewer@other.test', subject: `Re: ${subject}`,
    body: source({
      messageId: `${key}-reply@other.test`,
      inReplyTo: `${key}-root@other.test`,
      references: `<${key}-root@other.test>`,
      text: 'Threaded reply',
    }),
    messageId: `${key}-reply`,
  })
  await deliverEmail({
    recipient: email, sender: 'reviewer@other.test', subject,
    body: source({ messageId: `${key}-unrelated@other.test`, text: 'Same subject, separate conversation' }),
    messageId: `${key}-unrelated`,
  })

  await gotoApp(page)
  const threadsResponse = page.waitForResponse(response =>
    response.url().includes('/threads?') && response.status() === 200)
  await page.locator('.filter-strip w-chip').filter({ hasText: 'Threaded' }).click()
  await threadsResponse
  await expect(page.locator('.thread-summary')).toHaveCount(2)
  expect((await page.locator('.thread-count').allTextContents()).sort()).toEqual(['1', '2'])

  const conversation = page.locator('.thread-summary').filter({ has: page.locator('.thread-count', { hasText: '2' }) })
  await conversation.click()
  await expect(page.locator('.thread-email-row')).toHaveCount(2)
})

test('blocks remote email content until consent and never loads tracking pixels', async ({ page }) => {
  const { email, token, domains } = await withAuth(page)
  await seedDomain(token, domains[0])
  const key = `privacy-${Date.now()}`
  const sender = 'newsletter@publisher.test'
  const requested = []
  const proxied = []
  const transparentPng = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', 'base64')
  await page.route('https://mail-assets.example.test/**', route => {
    requested.push(new URL(route.request().url()).pathname)
    return route.fulfill({ status: 200, contentType: 'image/png', body: transparentPng })
  })
  await page.route(`${API}/inbox/*/images`, route => {
    const urls = route.request().postDataJSON()?.urls || []
    proxied.push(...urls.map(url => new URL(url).pathname))
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        images: urls.map(url => ({
          url,
          contentType: 'image/png',
          contentBase64: transparentPng.toString('base64'),
          size: transparentPng.byteLength,
        })),
        failed: 0,
      }),
    })
  })

  const html = suffix => `<p>Protected newsletter ${suffix}</p>
    <script>window.top.__emailScriptExecuted = true</script>
    <link rel="stylesheet" href="https://mail-assets.example.test/${suffix}-leak.css">
    <style>@import url('https://mail-assets.example.test/${suffix}-import.css');</style>
    <img src="https://mail-assets.example.test/${suffix}-hero.png" width="640" height="320" alt="Hero">
    <img src="https://mail-assets.example.test/${suffix}-pixel.png" width="1" height="1" alt="">
    <div style="background-image:url('https://mail-assets.example.test/${suffix}-background.png')">Background</div>`
  const first = await deliverEmail({
    recipient: email,
    sender,
    subject: `${key} first`,
    body: 'Protected newsletter first',
    htmlBody: html('first'),
    messageId: `${key}-first`,
  })

  await page.goto(`/email/${encodeURIComponent(first.emailId)}`)
  await waitForUiReady(page)
  await expect(page.getByText(/2 remote images are blocked and 1 tracking pixel was removed/)).toBeVisible()
  await page.waitForTimeout(250)
  expect(requested).toEqual([])
  expect(await page.evaluate(() => /** @type {any} */ (window).__emailScriptExecuted)).toBeUndefined()

  await page.getByRole('button', { name: 'Show images safely' }).click()
  await expect.poll(() => proxied.slice().sort()).toEqual([
    '/first-background.png',
    '/first-hero.png',
  ])
  expect(proxied).not.toContain('/first-pixel.png')
  expect(proxied).not.toContain('/first-leak.css')
  expect(proxied).not.toContain('/first-import.css')
  expect(requested).toEqual([])

  const second = await deliverEmail({
    recipient: email,
    sender,
    subject: `${key} second`,
    body: 'Protected newsletter second',
    htmlBody: html('second'),
    messageId: `${key}-second`,
  })
  await page.goto(`/email/${encodeURIComponent(second.emailId)}`)
  await waitForUiReady(page)
  await page.waitForTimeout(250)
  expect(proxied).not.toContain('/second-hero.png')
  await page.getByRole('button', { name: `Always show from ${sender}` }).click()
  await expect.poll(() => proxied).toContain('/second-hero.png')
  expect(proxied).not.toContain('/second-pixel.png')
  expect(requested).toEqual([])

  const third = await deliverEmail({
    recipient: email,
    sender,
    subject: `${key} third`,
    body: 'Protected newsletter third',
    htmlBody: html('third'),
    messageId: `${key}-third`,
  })
  await page.goto(`/email/${encodeURIComponent(third.emailId)}`)
  await waitForUiReady(page)
  await expect.poll(() => proxied).toContain('/third-hero.png')
  expect(proxied).not.toContain('/third-pixel.png')
  expect(requested).toEqual([])
})

test('marks disguised and homograph links and requires an explicit destination confirmation', async ({ page }) => {
  const errors = captureUiErrors(page)
  const { email, token, domains } = await withAuth(page)
  await seedDomain(token, domains[0])
  const key = `homograph-${Date.now()}`
  const delivered = await deliverEmail({
    recipient: email,
    sender: 'security@sender.test',
    subject: `${key} Review links`,
    body: 'Review links',
    htmlBody: `
      <p><a href="https://evil.example.test/login">https://accounts.example.test/login</a></p>
      <p><a href="https://раypal.example.test/account">Open billing account</a></p>
      <p><a href="https://safe.example.test/help">safe.example.test/help</a></p>
    `,
    messageId: `${key}-message`,
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/email/${encodeURIComponent(delivered.emailId)}`)
  await waitForUiReady(page)
  const frame = page.frameLocator('.email-html-frame')
  await expect(frame.locator('a.caduceus-suspicious-link')).toHaveCount(2)
  await expect(frame.getByText('safe.example.test/help')).not.toHaveClass(/caduceus-suspicious-link/)

  await frame.getByText('https://accounts.example.test/login').click()
  const warning = page.getByRole('alertdialog', { name: 'Check this link before opening it' })
  await expect(warning).toBeVisible()
  await expect(warning).toContainText('accounts.example.test')
  await expect(warning).toContainText('evil.example.test')
  const bounds = await warning.boundingBox()
  expect(bounds?.x).toBeGreaterThanOrEqual(0)
  expect((bounds?.x || 0) + (bounds?.width || 0)).toBeLessThanOrEqual(390)
  await page.getByRole('button', { name: 'Cancel' }).click()
  await expect(warning).toBeHidden()
  expect(errors).toEqual([])
})

test('prints the complete cross-folder conversation in a privacy-safe browser view', async ({ page, context }) => {
  const { email, token, domains } = await withAuth(page)
  await seedDomain(token, domains[0])
  const key = `print-${Date.now()}`
  const subject = `${key} Quarterly review`
  const delivered = await deliverEmail({
    recipient: email,
    sender: 'reviewer@other.test',
    subject,
    body: 'Inbound review notes',
    messageId: `${key}-inbound`,
  })
  const originalResponse = await fetch(`${API}/inbox/${encodeURIComponent(delivered.emailId)}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  const originalBody = await originalResponse.text()
  expect(originalResponse.ok, originalBody).toBe(true)
  const original = JSON.parse(originalBody)
  const sent = await fetch(`${API}/send`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      to: ['reviewer@other.test'],
      subject: `Re: ${subject}`,
      text: 'Outbound response notes',
      inReplyTo: original.rfcMessageId,
      references: [original.rfcMessageId],
    }),
  })
  expect(sent.ok, await sent.text()).toBe(true)

  await context.addInitScript(() => {
    window.print = () => { document.documentElement.dataset.printInvoked = 'true' }
  })
  await page.goto(`/email/${encodeURIComponent(delivered.emailId)}`)
  await waitForUiReady(page)

  const popupPromise = page.waitForEvent('popup')
  await page.getByRole('button', { name: 'Print conversation' }).click()
  const printPage = await popupPromise
  await printPage.waitForLoadState()

  await expect(printPage.locator('h1')).toHaveText(subject)
  await expect(printPage.locator('.print-message')).toHaveCount(2)
  await expect(printPage.getByText('Inbound review notes')).toBeVisible()
  await expect(printPage.getByText('Outbound response notes')).toBeVisible()
  await expect(printPage.getByText(/Privacy-safe print view/)).toBeVisible()
  await expect(printPage.locator('script,img')).toHaveCount(0)
  await expect.poll(() => printPage.evaluate(() => document.documentElement.dataset.printInvoked))
    .toBe('true')
})

test('protected app routes show login when unauthenticated', async ({ page }) => {
  await useTestApi(page)
  await page.goto('/settings')
  await expect(page.getByText('Sign in')).toBeVisible()
})

// ── Compose ───────────────────────────────────────────────────────────────────

test('shows error when To or Subject is empty', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await page.getByRole('button', { name: 'Compose' }).click()
  await expect(composeField(page, 'recipient@example.com')).toBeVisible()
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(page.getByText('To and Subject are required.')).toBeVisible()
})

test('opens compose via C keyboard shortcut', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await page.keyboard.press('c')
  await expect(composeField(page, 'recipient@example.com')).toBeVisible()
})

test('Discard returns to inbox', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await page.getByRole('button', { name: 'Compose' }).click()
  await expect(composeField(page, 'recipient@example.com')).toBeVisible()
  await page.getByRole('button', { name: 'Discard' }).click()
  await expect(page.getByText(/No emails/)).toBeVisible()
})

test('send succeeds and returns to inbox', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await page.getByRole('button', { name: 'Compose' }).click()
  await expect(composeField(page, 'recipient@example.com')).toBeVisible()
  await composeField(page, 'recipient@example.com').fill('bob@example.com')
  await composeField(page, 'Subject').fill('Test email')
  await page.locator('[data-compose-editor]').fill('End-to-end message body')
  await expect(composeField(page, 'recipient@example.com')).toHaveValue('bob@example.com')
  await expect(composeField(page, 'Subject')).toHaveValue('Test email')
  await expect(page.locator('[data-compose-editor]')).toHaveText('End-to-end message body')
  const [sendResponse] = await Promise.all([
    page.waitForResponse(response => response.url().endsWith('/send') && response.request().method() === 'POST'),
    page.getByRole('button', { name: 'Send', exact: true }).click(),
  ])
  const sendPayload = sendResponse.request().postDataJSON()
  expect(sendPayload.html || sendPayload.text, JSON.stringify(sendPayload)).toBeTruthy()
  expect(sendResponse.status()).toBe(200)
  await expect(page).toHaveURL(/\/inbox\/?$/)
  await expect(page.getByText(/No emails/)).toBeVisible()
})

test('autosaves, recovers, and conflict-protects a durable draft', async ({ page }) => {
  const { token } = await withAuth(page)
  await page.goto('/compose')
  await waitForUiReady(page)

  const createdResponsePromise = page.waitForResponse(response =>
    response.url().endsWith('/drafts') && response.request().method() === 'POST' && response.status() === 200)
  await composeField(page, 'recipient@example.com').fill('recipient@other.test')
  await composeField(page, 'Subject').fill('Durable browser draft')
  await page.locator('[data-compose-editor]').fill('Crash-safe body')
  const createdResponse = await createdResponsePromise
  const created = await createdResponse.json()
  await expect(page.locator('.compose-draft-status')).toHaveText('Saved')
  expect(created.draftVersion).toBe(1)

  await page.reload()
  await waitForUiReady(page)
  await expect(composeField(page, 'recipient@example.com')).toHaveValue('recipient@other.test')
  await expect(composeField(page, 'Subject')).toHaveValue('Durable browser draft')
  await expect(page.locator('[data-compose-editor]')).toContainText('Crash-safe body')

  const remoteUpdate = await fetch(`${API}/drafts/${encodeURIComponent(created.id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({
      clientId: created.draftClientId,
      version: 1,
      to: ['recipient@other.test'], cc: [], bcc: [], replyTo: [],
      subject: 'Remote tab version', text: 'Remote body', html: '<p>Remote body</p>',
      references: [], attachments: [],
    }),
  })
  expect(remoteUpdate.status).toBe(200)

  const conflictResponse = page.waitForResponse(response =>
    response.url().includes(`/drafts/${encodeURIComponent(created.id)}`) &&
    response.request().method() === 'PUT' && response.status() === 409)
  await composeField(page, 'Subject').fill('Local tab version')
  await conflictResponse
  await expect(page.getByText('This draft changed in another session.')).toBeVisible()

  const copyResponse = page.waitForResponse(response =>
    response.url().endsWith('/drafts') && response.request().method() === 'POST' && response.status() === 200)
  await page.getByRole('button', { name: 'Save local edits as a copy' }).click()
  const copied = await (await copyResponse).json()
  expect(copied.id).not.toBe(created.id)
  await expect(page.locator('.compose-draft-status')).toHaveText('Saved')

  const discardedResponse = page.waitForResponse(response =>
    response.url().includes(`/drafts/${encodeURIComponent(copied.id)}`) && response.request().method() === 'DELETE')
  await page.getByRole('button', { name: 'Discard' }).click()
  await discardedResponse
  await expect(page).toHaveURL(/\/inbox\/?$/)
})

// ── Settings ──────────────────────────────────────────────────────────────────

test('navigates to settings', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await page.locator('.app-user-trigger').click()
  await page.getByRole('menuitem', { name: 'Settings' }).click()
  await expect(page.locator('.settings-panel .settings-section-title').filter({ hasText: 'Routing Rules' })).toBeVisible()
})

test('requires fresh step-up verification before adding a passkey', async ({ page }) => {
  await withAuth(page)
  await gotoApp(page)
  await page.locator('.app-user-trigger').click()
  await page.getByRole('menuitem', { name: 'Settings' }).click()

  // Click add passkey button
  await page.getByRole('button', { name: 'Add passkey' }).click()

  // Credential enrollment must not begin until the user proves a fresh
  // password and recovery factor for this sensitive operation.
  await expect(page.locator('[data-settings-stepup]')).toBeVisible()
  await expect(page.locator('[data-settings-stepup-password] input')).toBeVisible()
  await expect(page.locator('[data-settings-stepup-code] input')).toBeVisible()
  await expect(page.locator('input[placeholder="e.g. iPhone 15"]')).toHaveCount(0)
})

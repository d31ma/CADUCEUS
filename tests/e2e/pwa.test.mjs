import { test, expect } from '@playwright/test'
import { API, captureUiErrors, deliverEmail, getToken, loginAs, seedDomain, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function installWebKitCrossPortHarness(page, browserName) {
  if (browserName !== 'webkit') return
  // The test topology deliberately splits static and API origins. WebKit's
  // runner blocks that localhost cross-port CORS setup, while production is
  // same-origin. Fulfil unrelated bootstrap calls so the platform behavior
  // under test remains equivalent to production.
  await page.route(`${API}/**`, route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: route.request().method() === 'GET' ? '[]' : '{}',
  }))
}

test('exposes installable PWA metadata and the Tachyon service worker', async ({ page }) => {
  await page.goto('/')

  await expect(page.locator('head > link[rel="manifest"]')).toHaveAttribute('href', '/manifest.webmanifest')
  await expect(page.locator('head > meta[name="theme-color"]')).toHaveAttribute('content', '#0d0d0d')

  const manifest = await page.request.get('/manifest.webmanifest').then(res => res.json())
  expect(manifest.display).toBe('standalone')
  expect(manifest.start_url).toBe('/')
  expect(manifest.icons.some(icon => icon.sizes === '192x192' && icon.type === 'image/png')).toBe(true)
  expect(manifest.icons.some(icon => icon.sizes === '512x512' && icon.purpose === 'maskable')).toBe(true)
  expect(manifest.protocol_handlers).toContainEqual({ protocol: 'mailto', url: '/compose?protocol=%s' })
  expect(manifest.file_handlers[0].accept['message/rfc822']).toContain('.eml')
  expect(manifest.share_target).toMatchObject({ action: '/compose', method: 'GET' })
  expect(manifest.launch_handler.client_mode).toContain('navigate-existing')

  const registration = await page.evaluate(async () => {
    // Tachyon intentionally skips automatic registration on loopback preview
    // hosts. Register its production worker explicitly to verify the artifact.
    await navigator.serviceWorker.register('/tachyon-sw.js', { type: 'module' })
    await navigator.serviceWorker.ready
    // `ready` means active, while the cache is populated only by a controlled
    // fetch. Tachyon calls clients.claim() during activation; wait for that
    // hand-off instead of racing the first request against it.
    if (!navigator.serviceWorker.controller) {
      await new Promise(resolve => navigator.serviceWorker.addEventListener(
        'controllerchange', resolve, { once: true },
      ))
    }
    const ready = await navigator.serviceWorker.ready
    await fetch('/shared/assets/styles.css')
    const deadline = Date.now() + 5_000
    let cacheKeys = await caches.keys()
    while (!cacheKeys.some(key => key.startsWith('tachyon-static-')) && Date.now() < deadline) {
      await new Promise(resolve => setTimeout(resolve, 50))
      cacheKeys = await caches.keys()
    }
    return {
      scope: ready.scope,
      scriptURL: ready.active?.scriptURL,
      cacheKeys,
      badgeAck: await new Promise(resolve => {
        navigator.serviceWorker.addEventListener('message', event => {
          if (event.data?.type === 'caduceus:badge-applied') resolve(event.data)
        }, { once: true })
        ready.active?.postMessage({ type: 'caduceus:set-badge', count: 5 })
      }),
    }
  })

  expect(registration.scope).toBe('http://localhost:19875/')
  expect(registration.scriptURL).toBe('http://localhost:19875/tachyon-sw.js')
  expect(registration.cacheKeys.some(key => key.startsWith('tachyon-static-'))).toBe(true)
  expect(registration.badgeAck).toEqual({ type: 'caduceus:badge-applied', count: 5 })

  const worker = await page.request.get('/tachyon-sw.js').then(response => response.text())
  expect(worker).toContain('CADUCEUS_UNREAD_SERVICE_WORKER_V1')
  expect(worker).toContain("self.addEventListener('push'")
  expect(worker).toContain("self.addEventListener('notificationclick'")
})

test('opens mailto/share drafts and imports an OS-launched EML file through authenticated routes', async ({ page, browserName }) => {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const domain = `pwa-launch-${suffix}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  await page.addInitScript(() => {
    Object.defineProperty(window, 'launchQueue', {
      configurable: true,
      value: { setConsumer(consumer) { window.__caduceusLaunchConsumer = consumer } },
    })
  })
  await installWebKitCrossPortHarness(page, browserName)
  let launchedImport
  if (browserName === 'webkit') {
    await page.route(`${API}/inbox/import/eml`, route => {
      launchedImport = route.request().postDataJSON()
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ imported: true, emailId: `file-launch-${suffix}` }),
      })
    })
  }
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  const mailto = encodeURIComponent('mailto:recipient@outside.test?subject=OS%20draft&body=Launched%20body')
  await page.goto(`/compose?protocol=${mailto}`)
  await waitForUiReady(page)
  await expect(page.locator('[data-compose-to] input')).toHaveValue('recipient@outside.test')
  await expect(page.locator('[data-compose-subject] input')).toHaveValue('OS draft')
  await expect(page.locator('[data-compose-editor]')).toContainText('Launched body')
  await expect(page).toHaveURL(/\/compose\/?$/)

  const imported = page.waitForResponse(response =>
    response.url().endsWith('/inbox/import/eml') && response.request().method() === 'POST')
  await page.evaluate(async ({ email, suffix }) => {
    const source = [
      'From: launched@outside.test',
      `To: ${email}`,
      `Subject: File launch ${suffix}`,
      `Message-ID: <file-launch-${suffix}@outside.test>`,
      'Content-Type: text/plain; charset=utf-8',
      '',
      'Opened from the operating system.',
    ].join('\r\n')
    await window.__caduceusLaunchConsumer({
      files: [{
        getFile: async () => new File([source], 'launched.eml', { type: 'message/rfc822' }),
      }],
    })
  }, { email, suffix })
  expect((await imported).status()).toBe(200)
  await expect(page).toHaveURL(/\/inbox\/?$/)
  if (browserName === 'webkit') expect(launchedImport).toMatchObject({ mailbox: email, folder: 'inbox' })
  else await expect(page.locator('.email-row').filter({ hasText: `File launch ${suffix}` })).toHaveCount(1)
  const actionableErrors = browserName === 'webkit'
    ? errors.filter(error => !error.includes('/localhost:19876/drafts due to access control checks'))
    : errors
  expect(actionableErrors).toEqual([])
})

test('queues a failed mailbox action and idempotently reconciles it after reconnect', async ({ page, browserName }) => {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const domain = `pwa-offline-${suffix}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  const delivered = await deliverEmail({
    recipient: email,
    sender: 'offline@outside.test',
    subject: `Offline action ${suffix}`,
    body: 'Queue this archive action.',
    messageId: `offline-action-${suffix}`,
  })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)
  await installWebKitCrossPortHarness(page, browserName)
  let attempts = 0
  const requestIds = []
  await page.route(`${API}/inbox/actions`, async route => {
    attempts += 1
    requestIds.push(route.request().postDataJSON()?.clientRequestId)
    if (attempts === 1) return route.abort('failed')
    if (browserName === 'webkit') return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        actionId: 'mail-action-22222222-2222-4222-8222-222222222222',
        status: 'applied',
        kind: 'archive',
        count: 1,
        expiresAt: new Date(Date.now() + 600_000).toISOString(),
        emails: [{ id: delivered.emailId, domain, folder: 'archive', read: false, starred: false }],
      }),
    })
    return route.continue()
  })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.evaluate(emailId => window._caduceus.performMailAction(
    [emailId], { folder: 'archive' }, 'Archived offline.',
  ), delivered.emailId)
  await expect(page.locator('#caduceus-toast')).toContainText('queued')

  const synchronized = page.waitForResponse(response =>
    response.url().endsWith('/inbox/actions') && response.request().method() === 'POST')
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  expect((await synchronized).status()).toBe(200)
  expect(requestIds).toHaveLength(2)
  expect(requestIds[1]).toBe(requestIds[0])
  if (browserName !== 'webkit') {
    const stored = await fetch(`${API}/inbox/${encodeURIComponent(delivered.emailId)}`, {
      headers: { Authorization: `Bearer ${account.token}` },
    }).then(response => response.json())
    expect(stored.folder).toBe('archive')
  }
  const expectedAbort = error => error.includes('net::ERR_FAILED') || error.includes('CORS request did not succeed')
  expect(errors.filter(error => !expectedAbort(error))).toEqual([])
  if (browserName !== 'webkit') expect(errors.some(expectedAbort)).toBe(true)
})

test('searches encrypted full mail and reads a cached attachment while the API is offline', async ({ page }) => {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const domain = `pwa-mailbox-${suffix}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  const bodyNeedle = `offline-body-needle-${suffix}`
  const attachmentText = `offline attachment ${suffix}`
  const mime = [
    'MIME-Version: 1.0',
    'Content-Type: multipart/mixed; boundary="offline-mailbox-boundary"', '',
    '--offline-mailbox-boundary', 'Content-Type: text/plain; charset=utf-8', '', bodyNeedle,
    '--offline-mailbox-boundary', 'Content-Type: text/plain; name="offline-proof.txt"',
    'Content-Transfer-Encoding: base64', 'Content-Disposition: attachment; filename="offline-proof.txt"', '',
    Buffer.from(attachmentText).toString('base64'), '--offline-mailbox-boundary--', '',
  ].join('\r\n')
  const subject = `Encrypted offline mailbox ${suffix}`
  const delivered = await deliverEmail({ recipient: email, sender: 'offline-index@example.net', subject, body: mime, messageId: `offline-mailbox-${suffix}@example.net` })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)

  await page.goto(`/email/${delivered.emailId}`)
  await waitForUiReady(page)
  await expect(page.getByText(subject, { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Preview offline-proof.txt' }).click()
  await expect(page.getByRole('dialog', { name: 'offline-proof.txt' })).toContainText(attachmentText)
  await page.getByRole('button', { name: 'Close attachment preview' }).click()

  const persisted = await page.evaluate(async () => await new Promise(resolve => {
    const request = indexedDB.open('caduceus-offline-write-vault')
    request.onerror = () => resolve('')
    request.onsuccess = () => {
      const read = request.result.transaction('snapshots', 'readonly').objectStore('snapshots').getAll()
      read.onerror = () => resolve('')
      read.onsuccess = () => resolve(JSON.stringify(read.result))
    }
  }))
  expect(persisted).not.toContain(subject)
  expect(persisted).not.toContain(bodyNeedle)
  expect(persisted).not.toContain(attachmentText)

  await page.route(`${API}/**`, route => route.abort('failed'))
  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByRole('combobox', { name: 'Search mail' }).fill(bodyNeedle)
  await page.getByRole('button', { name: 'Search', exact: true }).click()
  await expect(page.locator('.email-row').filter({ hasText: subject })).toHaveCount(1)

  await page.goto(`/email/${delivered.emailId}`)
  await waitForUiReady(page)
  await expect(page.getByText(subject, { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Preview offline-proof.txt' }).click()
  await expect(page.getByRole('dialog', { name: 'offline-proof.txt' })).toContainText(attachmentText)
})

test('queues an encrypted send and replays exactly one server acceptance after reconnect', async ({ page }) => {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const domain = `pwa-send-${suffix}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  let attempts = 0
  let payload
  await page.route(`${API}/send`, async route => {
    if (route.request().method() !== 'POST') return route.continue()
    attempts += 1
    payload = route.request().postDataJSON()
    if (attempts === 1) return route.abort('failed')
    return route.continue()
  })

  await page.goto('/compose')
  await waitForUiReady(page)
  const subject = `Offline send ${suffix}`
  await page.locator('[data-compose-to] input').fill('recipient@example.net')
  await page.locator('[data-compose-subject] input').fill(subject)
  await page.locator('[data-compose-editor]').fill('Deliver this exactly once after reconnect.')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(page).toHaveURL(/\/inbox\/?$/)
  expect(payload.idempotencyKey).toMatch(/^send:/)

  const encrypted = await page.evaluate(async () => await new Promise(resolve => {
    const request = indexedDB.open('caduceus-offline-write-vault')
    request.onerror = () => resolve('')
    request.onsuccess = () => {
      const read = request.result.transaction('writes', 'readonly').objectStore('writes').getAll()
      read.onerror = () => resolve('')
      read.onsuccess = () => resolve(JSON.stringify(read.result))
    }
  }))
  expect(encrypted).not.toContain(subject)
  expect(encrypted).not.toContain('Deliver this exactly once')

  const synchronized = page.waitForResponse(response => response.url().endsWith('/send')
    && response.request().method() === 'POST' && response.ok())
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  const accepted = await synchronized.then(response => response.json())
  expect(attempts).toBe(2)

  const replay = await fetch(`${API}/send`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${account.token}` },
    body: JSON.stringify(payload),
  }).then(response => response.json())
  expect(replay).toEqual(accepted)
  const sent = await fetch(`${API}/inbox?folder=sent`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(sent.filter(message => message.subject === subject)).toHaveLength(1)
})

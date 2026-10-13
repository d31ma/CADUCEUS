import { expect, test } from '@playwright/test'
import {
  API,
  captureUiErrors,
  getToken,
  loginAs,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

const providers = [
  { id: 'gmail', name: 'Gmail', provider: 'gmail', authentication: 'oauth', guidance: 'none', preset: null },
  {
    id: 'fastmail', name: 'Fastmail', provider: 'imap', authentication: 'password', guidance: 'app_password',
    preset: {
      imap: { host: 'imap.fastmail.com', port: 993, security: 'tls', allowInsecureTLS: false },
      smtp: { host: 'smtp.fastmail.com', port: 465, security: 'tls', allowInsecureTLS: false },
    },
  },
  { id: 'imap', name: 'IMAP / SMTP', provider: 'imap', authentication: 'password', guidance: 'none', preset: null },
]

test('discovers, connects, controls, reconnects, and removes an account on mobile', async ({ page }) => {
  const email = `external-ui-${Date.now()}@example.com`
  await seedUser(email, { domains: ['example.com'], role: 'admin' })
  const owner = await getToken(email)
  await useTestApi(page)
  await loginAs(page, owner.token, owner.email, owner.role, owner.domains)
  const errors = captureUiErrors(page)
  const requests = []
  let accounts = []

  const fulfill = (route, body, status = 200) => route.fulfill({
    status, contentType: 'application/json', body: JSON.stringify(body),
    headers: { 'access-control-allow-origin': '*' },
  })
  await page.route(`${API}/users/me/preferences`, async route => {
    const body = route.request().postDataJSON()
    if (route.request().method() !== 'PUT' || !Array.isArray(body?.newMailNotificationExternalAccounts)) {
      return route.continue()
    }
    requests.push({ kind: 'notification-preferences', body })
    await fulfill(route, {
      newMailNotificationDomains: ['example.com'],
      newMailNotificationExternalAccounts: body.newMailNotificationExternalAccounts,
      newMailNotificationPrivacy: 'full',
      newMailNotificationActions: true,
    })
  })
  await page.route(`${API}/api/v1/provider-catalog`, route => fulfill(route, { providers }))
  await page.route(`${API}/api/v1/provider-discovery`, async route => {
    requests.push({ kind: 'discover', body: route.request().postDataJSON() })
    await fulfill(route, {
      choiceId: 'fastmail', source: 'preset', connection: providers[1].preset,
    })
  })
  await page.route(`${API}/api/v1/external-accounts/validate`, async route => {
    requests.push({ kind: 'validate', body: route.request().postDataJSON() })
    await fulfill(route, { capabilities: { mailRead: true, mailWrite: true, send: true, contacts: false, calendar: true, labels: false, sharedMailbox: false } })
  })
  await page.route(new RegExp(`${API}/api/v1/external-accounts/ext_[^/]+/reconnect$`), async route => {
    const body = route.request().postDataJSON()
    requests.push({ kind: 'reconnect', body })
    accounts = accounts.map(account => ({ ...account, state: 'ready', revision: body.revision + 1 }))
    await fulfill(route, accounts[0])
  })
  await page.route(new RegExp(`${API}/api/v1/external-accounts/ext_[^/]+$`), async route => {
    const method = route.request().method()
    const body = route.request().postDataJSON()
    requests.push({ kind: method.toLowerCase(), body })
    if (method === 'PATCH') {
      accounts = accounts.map(account => ({ ...account, state: body.action === 'pause' ? 'paused' : 'ready', revision: body.revision + 1 }))
      return fulfill(route, accounts[0])
    }
    if (method === 'DELETE') return fulfill(route, { deleting: true })
    return fulfill(route, accounts[0] || {}, accounts.length ? 200 : 404)
  })
  await page.route(`${API}/api/v1/external-accounts`, async route => {
    if (route.request().method() === 'GET') return fulfill(route, { accounts })
    const body = route.request().postDataJSON()
    requests.push({ kind: 'create', body })
    accounts = [{
      id: 'ext_abcdefghijklmnopqrst', mailboxAddress: body.mailboxAddress, provider: 'imap',
      label: body.label, color: body.color, order: body.order, state: 'ready', revision: 1,
      authIdentity: null, sharedMailboxAddress: null,
      connection: { imap: body.imap, smtp: body.smtp, caldav: body.caldav, containerFolder: body.containerFolder },
      capabilities: { mailRead: true, mailWrite: true, send: true, contacts: false, calendar: true, labels: false, sharedMailbox: false },
      progress: null, failure: null, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
    }]
    return fulfill(route, accounts[0])
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/settings')
  await waitForUiReady(page)

  const panel = page.locator('[data-external-account-settings]')
  await expect(panel.getByText('No outside mailboxes connected', { exact: true })).toBeVisible()
  await panel.getByRole('button', { name: 'Start guided setup', exact: true }).click()
  await expect(panel.getByRole('list', { name: 'Account setup progress' })).toContainText('Choose provider')
  await expect(panel.getByRole('list', { name: 'Account setup progress' })).toContainText('Authorize')
  await expect(panel.getByRole('list', { name: 'Account setup progress' })).toContainText('Validate and sync')
  await panel.getByLabel('Email address').fill('owner@fastmail.com')
  await panel.getByRole('button', { name: 'Find settings', exact: true }).click()
  await expect(panel.getByText('This provider requires an app-specific password.', { exact: true })).toBeVisible()
  await expect(panel.getByLabel('IMAP server')).toHaveValue('imap.fastmail.com')
  await panel.getByLabel('Account label').fill('Client work')
  await panel.getByLabel('Password', { exact: true }).fill('one-time-app-password')
  await panel.getByLabel('CalDAV URL (optional)').fill('https://calendar.fastmail.example/dav/')
  await panel.getByLabel('CalDAV username (optional)').fill('calendar-owner')
  await panel.getByRole('button', { name: 'Validate and add', exact: true }).click()

  await expect(panel.getByText('Client work', { exact: true })).toBeVisible()
  await expect(panel.getByText('owner@fastmail.com', { exact: true })).toBeVisible()
  await expect(panel.getByRole('region', { name: 'Connect an outside mailbox' })).toHaveCount(0)
  expect(await page.evaluate(() => localStorage.getItem(`caduceus_account_onboarding_complete:${sessionStorage.getItem('caduceus_email')}`))).toBe('1')
  expect(requests.find(request => request.kind === 'discover')?.body).toEqual({ address: 'owner@fastmail.com' })
  expect(requests.find(request => request.kind === 'validate')?.body.credentials.password).toBe('one-time-app-password')
  expect(requests.find(request => request.kind === 'create')?.body.credentials.password).toBe('one-time-app-password')
  expect(requests.find(request => request.kind === 'validate')?.body.caldav).toEqual({
    url: 'https://calendar.fastmail.example/dav/', username: 'calendar-owner', allowInsecureTLS: false,
  })
  expect(requests.find(request => request.kind === 'create')?.body.caldav).toEqual({
    url: 'https://calendar.fastmail.example/dav/', username: 'calendar-owner', allowInsecureTLS: false,
  })
  await expect(panel.locator('input[type="password"]')).toHaveCount(0)

  const connectedAlert = page.getByRole('checkbox', { name: 'Client work · owner@fastmail.com', exact: true })
  await expect(connectedAlert).not.toBeChecked()
  await connectedAlert.check()
  await expect.poll(() => requests.find(request => request.kind === 'notification-preferences')?.body)
    .toEqual({ newMailNotificationExternalAccounts: ['ext_abcdefghijklmnopqrst'] })

  await panel.getByRole('button', { name: 'Pause', exact: true }).click()
  await expect(panel.getByText('paused', { exact: true })).toBeVisible()
  expect(requests.find(request => request.kind === 'patch')?.body).toEqual({ action: 'pause', revision: 1 })

  await panel.getByRole('button', { name: 'Reconnect', exact: true }).click()
  await panel.getByLabel('New password').fill('replacement-password')
  await panel.getByRole('button', { name: 'Validate and reconnect', exact: true }).click()
  await expect(panel.getByText('ready', { exact: true })).toBeVisible()
  expect(requests.find(request => request.kind === 'reconnect')?.body).toMatchObject({
    revision: 2, credentials: { username: 'owner@fastmail.com', password: 'replacement-password' },
  })

  await panel.getByRole('button', { name: 'Remove', exact: true }).click()
  await expect(panel.getByRole('alertdialog', { name: 'Confirm account removal' })).toBeVisible()
  await panel.getByRole('button', { name: 'Confirm removal', exact: true }).click()
  await expect(panel.getByText('deleting', { exact: true })).toBeVisible()

  const overflow = await page.evaluate(() => ({ width: innerWidth, document: document.documentElement.scrollWidth }))
  expect(overflow.document - overflow.width).toBeLessThanOrEqual(1)
  expect(errors).toEqual([])
})

test('completes and reauthorizes a Microsoft 365 shared mailbox without exposing OAuth parameters', async ({ page }) => {
  const email = `microsoft-ui-${Date.now()}@example.com`
  await seedUser(email, { domains: ['example.com'], role: 'admin' })
  const owner = await getToken(email)
  await useTestApi(page)
  await loginAs(page, owner.token, owner.email, owner.role, owner.domains)
  const errors = captureUiErrors(page)
  const requests = []
  const now = new Date().toISOString()
  let account = null
  const microsoft = {
    id: 'office365', name: 'Microsoft 365', provider: 'microsoft', authentication: 'oauth',
    guidance: 'none', configured: true, preset: null,
  }
  const fulfill = (route, body, status = 200) => route.fulfill({
    status, contentType: 'application/json', body: JSON.stringify(body),
    headers: { 'access-control-allow-origin': '*' },
  })
  await page.route(`${API}/api/v1/provider-catalog`, route => fulfill(route, { providers: [microsoft] }))
  await page.route(`${API}/api/v1/external-accounts`, route => fulfill(route, { accounts: account ? [account] : [] }))
  await page.route(`${API}/api/v1/external-accounts/oauth/microsoft/complete`, async route => {
    requests.push({ kind: 'complete', body: route.request().postDataJSON() })
    account = {
      id: 'ext_mmmmmmmmmmmmmmmmmmmm', mailboxAddress: 'support@example.com', provider: 'microsoft',
      authIdentity: 'delegate@example.com', sharedMailboxAddress: 'support@example.com', label: 'Support', color: '#0078D4', order: 0,
      state: 'paused', revision: 4, connection: {
        imap: { host: 'outlook.office365.com', port: 993, security: 'tls' },
        smtp: { host: 'smtp.office365.com', port: 587, security: 'starttls' },
      },
      capabilities: { mailRead: true, mailWrite: true, send: true, sharedMailbox: true },
      progress: null, failure: null, createdAt: now, updatedAt: now,
    }
    return fulfill(route, account)
  })
  await page.route(`${API}/api/v1/external-accounts/oauth/microsoft/begin`, async route => {
    requests.push({ kind: 'begin', body: route.request().postDataJSON() })
    return fulfill(route, { authorizationUrl: 'https://invalid.example.test/authorize', state: 'x'.repeat(43), expiresAt: now })
  })

  await page.goto('/settings?code=authorization-code&state=browser-state&session_state=provider-session')
  await waitForUiReady(page)
  await expect(page).toHaveURL(/\/settings$/)
  const panel = page.locator('[data-external-account-settings]')
  await expect(panel.getByText('Shared mailbox connected successfully.', { exact: true })).toBeVisible()
  await expect(panel.getByText('support@example.com', { exact: true })).toBeVisible()
  await expect(panel.getByText('Signed in as delegate@example.com', { exact: true })).toBeVisible()
  await expect(panel.getByText('Full Access verified', { exact: true })).toBeVisible()
  await expect(panel.getByText('SMTP Send As ready', { exact: true })).toBeVisible()
  expect(requests.find(request => request.kind === 'complete')?.body).toEqual({ code: 'authorization-code', state: 'browser-state' })

  await panel.getByRole('button', { name: 'Reconnect', exact: true }).click()
  await panel.getByRole('button', { name: 'Continue with Microsoft', exact: true }).click()
  await expect(panel.getByText('The server returned an invalid provider authorization address.', { exact: true })).toBeVisible()
  expect(requests.find(request => request.kind === 'begin')?.body).toEqual({
    provider: 'microsoft', accountId: 'ext_mmmmmmmmmmmmmmmmmmmm', revision: 4,
  })
  expect(errors).toEqual([])
})

test('onboards Gmail through the Google callback and keeps provider tokens off the page', async ({ page }) => {
  const email = `google-ui-${Date.now()}@example.com`
  await seedUser(email, { domains: ['example.com'], role: 'admin' })
  const owner = await getToken(email)
  await useTestApi(page)
  await loginAs(page, owner.token, owner.email, owner.role, owner.domains)
  const errors = captureUiErrors(page)
  const requests = []
  const now = new Date().toISOString()
  let account = null
  const fulfill = (route, body, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body), headers: { 'access-control-allow-origin': '*' } })
  await page.route(`${API}/api/v1/provider-catalog`, route => fulfill(route, { providers: [{
    id: 'gmail', name: 'Gmail', provider: 'gmail', authentication: 'oauth', guidance: 'none', configured: true, preset: null,
  }] }))
  await page.route(`${API}/api/v1/external-accounts`, route => fulfill(route, { accounts: account ? [account] : [] }))
  await page.route(`${API}/api/v1/external-accounts/oauth/microsoft/complete`, route => fulfill(route, {
    error: 'The Microsoft authorization transaction is not valid for this callback.',
  }, 400))
  await page.route(`${API}/api/v1/external-accounts/oauth/google/complete`, async route => {
    requests.push({ kind: 'complete', body: route.request().postDataJSON() })
    account = {
      id: 'ext_gggggggggggggggggggg', mailboxAddress: 'owner@gmail.com', provider: 'gmail',
      authIdentity: 'owner@gmail.com', sharedMailboxAddress: null, label: 'Gmail', color: '#EA4335', order: 0,
      state: 'ready', revision: 1, connection: {
        imap: { host: 'imap.gmail.com', port: 993, security: 'tls' }, smtp: { host: 'smtp.gmail.com', port: 465, security: 'tls' },
      }, capabilities: { mailRead: true, mailWrite: true, send: true, labels: true, sharedMailbox: false },
      progress: null, failure: null, createdAt: now, updatedAt: now,
    }
    return fulfill(route, account)
  })
  await page.route(`${API}/api/v1/external-accounts/oauth/google/begin`, async route => {
    requests.push({ kind: 'begin', body: route.request().postDataJSON() })
    return fulfill(route, { authorizationUrl: 'https://invalid.example.test/authorize', state: 'x'.repeat(43), expiresAt: now })
  })
  await page.addInitScript(() => {
    if (window.top === window) localStorage.setItem('caduceus_oauth_provider:google-state', 'gmail')
  })
  await page.goto('/settings?code=google-code&state=google-state')
  await waitForUiReady(page)

  const panel = page.locator('[data-external-account-settings]')
  await expect(page).toHaveURL(/\/settings$/)
  await expect(panel.getByText('Google account connected successfully.', { exact: true })).toBeVisible()
  await expect(panel.getByText('owner@gmail.com', { exact: true })).toBeVisible()
  await expect(panel.getByText('SMTP sender ready', { exact: true })).toBeVisible()
  await expect(page.locator('body')).not.toContainText('google-code')
  await expect(page.locator('body')).not.toContainText('google-state')
  expect(requests.find(request => request.kind === 'complete')?.body).toEqual({ code: 'google-code', state: 'google-state' })
  await panel.getByRole('button', { name: 'Add account', exact: true }).click()
  await panel.getByLabel('Email address').fill('second@gmail.com')
  await panel.getByRole('button', { name: /Gmail OAuth/ }).click()
  await expect(panel.getByText('Gmail authorization', { exact: true })).toBeVisible()
  await expect(panel.getByLabel('Google sign-in address')).toHaveValue('second@gmail.com')
  await panel.getByRole('button', { name: 'Continue with Google', exact: true }).click()
  await expect(panel.getByText('The server returned an invalid provider authorization address.', { exact: true })).toBeVisible()
  expect(requests.find(request => request.kind === 'begin')?.body).toMatchObject({
    provider: 'gmail', mailboxAddress: 'second@gmail.com', authIdentity: 'second@gmail.com', sharedMailboxAddress: '',
  })
  expect(errors).toEqual([])
})

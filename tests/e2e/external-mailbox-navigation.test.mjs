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

const ACCOUNT_ID = 'ext_abcdefghijklmnopqrst'
const SECOND_ACCOUNT_ID = 'ext_zyxwvutsrqponmlkjihg'

const account = {
  id: ACCOUNT_ID,
  mailboxAddress: 'owner@example.com',
  provider: 'imap',
  label: 'Remote work',
  color: '#3367d6',
  order: 0,
  state: 'ready',
  revision: 1,
  connection: {},
  capabilities: { mailRead: true, mailWrite: true, send: true },
  progress: null,
  failure: null,
  updatedAt: new Date().toISOString(),
}

const secondAccount = {
  ...account,
  id: SECOND_ACCOUNT_ID,
  mailboxAddress: 'personal@outside.test',
  label: 'Remote personal',
  color: '#7b4ab5',
  order: 1,
}

const hosted = {
  id: 'hosted-message',
  domain: 'example.com',
  sender: 'hosted@sender.test',
  recipient: 'admin@example.com',
  subject: 'Hosted inbox message',
  snippet: 'Stored by CADUCEUS',
  folder: 'inbox',
  systemFolder: 'inbox',
  read: false,
  starred: false,
  receivedAt: '2026-08-21T12:00:00.000Z',
}

const remoteInbox = {
  id: 'extmsg.remote-inbox',
  external: true,
  providerAccountId: ACCOUNT_ID,
  providerStableId: 'INBOX:41',
  mailboxAddress: account.mailboxAddress,
  mailboxLabel: account.label,
  mailboxColor: account.color,
  domain: 'example.com',
  sender: 'remote@sender.test',
  recipient: account.mailboxAddress,
  subject: 'Remote inbox message',
  snippet: 'Indexed by the provider bridge',
  folder: 'inbox',
  systemFolder: 'inbox',
  read: false,
  starred: false,
  receivedAt: '2026-08-21T13:00:00.000Z',
}

const remoteSent = {
  ...remoteInbox,
  id: 'extmsg.remote-sent',
  providerStableId: 'Sent:42',
  sender: account.mailboxAddress,
  recipient: 'recipient@outside.test',
  subject: 'Remote sent message',
  snippet: 'Sent through the connected account',
  folder: 'sent',
  systemFolder: 'sent',
  receivedAt: '2026-08-21T14:00:00.000Z',
}

const remoteInboxTwo = {
  ...remoteInbox,
  id: 'extmsg.remote-inbox-two',
  providerStableId: 'INBOX:43',
  subject: 'Second remote inbox message',
  receivedAt: '2026-08-21T12:30:00.000Z',
}

test('keeps hosted and connected mailboxes distinct across navigation and selection', async ({ page }) => {
  const email = `external-nav-${Date.now()}@example.com`
  await seedUser(email, { domains: ['example.com'], role: 'admin' })
  const owner = await getToken(email)
  await useTestApi(page)
  await loginAs(page, owner.token, owner.email, owner.role, owner.domains)
  await page.addInitScript(() => {
    try { Object.defineProperty(window, 'showSaveFilePicker', { value: undefined, configurable: true }) } catch {}
  })
  const errors = captureUiErrors(page)
  const inboxRequests = []
  const providerPatches = []
  const accountPatches = []
  const syncRequests = []
  const folderMutations = []
  const exportRequests = []
  const mailActions = new Map()
  let mailActionSequence = 0
  let accountStates = [{ ...account }, { ...secondAccount }]
  let providerMessages = [remoteInbox, remoteInboxTwo, remoteSent]

  const fulfill = (route, body) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
    headers: { 'access-control-allow-origin': '*' },
  })

  await page.route(new RegExp(`${API}/api/v1/external-accounts/${ACCOUNT_ID}/folders$`), route => fulfill(route, {
    folders: [
      { stableId: 'INBOX', name: 'INBOX', role: 'inbox' },
      { stableId: 'Sent', name: 'Sent', role: 'sent' },
      { stableId: 'Archive', name: 'Archive', role: 'archive' },
      { stableId: 'Projects', name: 'Projects', role: '', delimiter: '/', attributes: [] },
      { stableId: 'Projects/Launch', name: 'Projects/Launch', role: '', delimiter: '/', attributes: [] },
      { stableId: 'Projects/Launch/Design', name: 'Projects/Launch/Design', role: '', delimiter: '/', attributes: [] },
    ],
  }))
  await page.route(new RegExp(`${API}/api/v1/external-accounts/${SECOND_ACCOUNT_ID}/folders$`), route => fulfill(route, { folders: [] }))
  await page.route(new RegExp(`${API}/api/v1/external-accounts/(?:${ACCOUNT_ID}|${SECOND_ACCOUNT_ID})/retry$`), route => {
    syncRequests.push({ id: new URL(route.request().url()).pathname.split('/').at(-2), body: route.request().postDataJSON() })
    return fulfill(route, { scheduled: true })
  })
  await page.route(new RegExp(`${API}/api/v1/external-accounts/(?:${ACCOUNT_ID}|${SECOND_ACCOUNT_ID})/mutations$`), route => {
    const body = route.request().postDataJSON()
    folderMutations.push(body)
    return fulfill(route, { id: `mut_${String(folderMutations.length).padStart(20, 'a')}`, ...body, state: 'queued', attempts: 0 })
  })
  await page.route(new RegExp(`${API}/api/v1/external-accounts/(?:${ACCOUNT_ID}|${SECOND_ACCOUNT_ID})$`), route => {
    const id = new URL(route.request().url()).pathname.split('/').pop()
    const patch = route.request().postDataJSON()
    accountPatches.push({ id, ...patch })
    const current = accountStates.find(candidate => candidate.id === id)
    const updated = { ...current, ...patch, revision: current.revision + 1 }
    delete updated.action
    accountStates = accountStates.map(candidate => candidate.id === id ? updated : candidate)
    return fulfill(route, updated)
  })
  await page.route(`${API}/api/v1/external-accounts`, route => fulfill(route, { accounts: accountStates }))
  await page.route(`${API}/inbox/actions`, route => {
    const body = route.request().postDataJSON()
    const before = body.ids.map(id => providerMessages.find(message => message.id === id)).filter(Boolean)
    const after = before.map(message => ({
      ...message, ...body.patch, ...(body.patch.folder ? { systemFolder: body.patch.folder } : {}),
    }))
    for (const message of after) providerPatches.push({ id: message.id, patch: body.patch })
    providerMessages = providerMessages.map(message => after.find(candidate => candidate.id === message.id) || message)
    const actionId = `mail-action-${String(++mailActionSequence).padStart(2, '0')}`
    mailActions.set(actionId, { before, after, patch: body.patch })
    return fulfill(route, {
      actionId, status: 'applied', kind: body.ids.length > 1 ? 'bulk' : 'move', count: after.length,
      expiresAt: new Date(Date.now() + 600_000).toISOString(), emails: after,
    })
  })
  await page.route(new RegExp(`${API}/inbox/actions/(mail-action-[^/]+)/undo$`), route => {
    const actionId = new URL(route.request().url()).pathname.split('/').at(-2)
    const action = mailActions.get(actionId)
    if (action?.patch?.folder === 'archive') {
      return route.fulfill({
        status: 409, contentType: 'application/json',
        body: JSON.stringify({ error: 'The connected account changed after this action. Refresh before choosing another result.', conflict: true }),
      })
    }
    providerMessages = providerMessages.map(message => action?.before.find(candidate => candidate.id === message.id) || message)
    return fulfill(route, {
      actionId, status: 'undone', kind: 'bulk', count: action?.before.length || 0,
      expiresAt: new Date(Date.now() + 600_000).toISOString(), emails: action?.before || [],
    })
  })
  await page.route(new RegExp(`${API}/inbox/actions/(mail-action-[^/]+)/redo$`), route => {
    const actionId = new URL(route.request().url()).pathname.split('/').at(-2)
    const action = mailActions.get(actionId)
    providerMessages = providerMessages.map(message => action?.after.find(candidate => candidate.id === message.id) || message)
    return fulfill(route, {
      actionId, status: 'applied', kind: 'bulk', count: action?.after.length || 0,
      expiresAt: new Date(Date.now() + 600_000).toISOString(), emails: action?.after || [],
    })
  })
  await page.route(new RegExp(`${API}/inbox(?:\\?.*)?$`), route => {
    const url = new URL(route.request().url())
    const scope = url.searchParams.get('account')
    inboxRequests.push(url.search)
    return fulfill(route, scope === ACCOUNT_ID ? providerMessages : [hosted, ...providerMessages])
  })
  await page.route(new RegExp(`${API}/inbox/export/rfc822(?:\\?.*)?$`), route => {
    exportRequests.push(new URL(route.request().url()).search)
    const source = Buffer.from('From: remote@sender.test\r\nSubject: Remote export\r\n\r\nbody')
    const metadata = Buffer.from(JSON.stringify({ id: 'remote-export', sender: 'remote@sender.test', subject: 'Remote export', receivedAt: remoteInbox.receivedAt }))
    const header = Buffer.alloc(8)
    header.writeUInt32BE(metadata.byteLength, 0)
    header.writeUInt32BE(source.byteLength, 4)
    return route.fulfill({
      status: 200,
      body: Buffer.concat([Buffer.from('CADUCEUS-RFC822-FRAMES/1\n'), header, metadata, source]),
      headers: { 'content-type': 'application/vnd.caduceus.rfc822-frames; version=1' },
    })
  })

  await page.goto('/inbox')
  await waitForUiReady(page)

  const accountInboxLink = page.locator(`a[href="/inbox?account=${ACCOUNT_ID}"]`).first()
  const accountSentLink = page.locator(`a[href="/folder/sent?account=${ACCOUNT_ID}"]:visible`).first()
  const mailboxSelect = page.locator('.inbox-domain-select')
  await expect(accountInboxLink).toContainText('Remote work')
  await expect(mailboxSelect).toHaveAttribute('value', 'all')
  await expect(mailboxSelect.locator('[role="option"][value="domain:example.com"]')).toHaveText('example.com')
  await expect(mailboxSelect.locator(`[role="option"][value="account:${ACCOUNT_ID}"]`)).toHaveText('Remote work')
  await expect(page.getByText('Hosted inbox message', { exact: true })).toBeVisible()
  await expect(page.getByText('Remote inbox message', { exact: true })).toBeVisible()

  await expect(page.locator('a[title="Projects/Launch"]')).toHaveCount(0)
  await page.getByRole('button', { name: 'Expand Projects' }).click()
  await expect(page.locator('a[title="Projects/Launch"]')).toBeVisible()
  await page.getByRole('button', { name: 'Expand Launch' }).click()
  await expect(page.locator('a[title="Projects/Launch/Design"]')).toBeVisible()
  await page.getByRole('button', { name: 'Collapse Projects' }).click()
  await expect(page.locator('a[title="Projects/Launch"]')).toHaveCount(0)

  await page.getByLabel('Actions for Remote work').click()
  await page.getByRole('menuitem', { name: 'New folder…' }).click()
  await expect(page.getByRole('dialog', { name: 'Create connected folder' })).toBeVisible()
  await page.getByLabel('Folder name').fill('Customers')
  await page.getByRole('button', { name: 'Create folder' }).click()
  await expect.poll(() => folderMutations.at(-1)).toMatchObject({
    revision: 1, kind: 'folder_create', messageStableIds: [], parentStableId: '', folderName: 'Customers',
  })
  await expect(page.getByRole('dialog', { name: 'Create connected folder' })).toHaveCount(0)

  const projectsMenu = page.getByLabel('Actions for Projects').locator('..')
  await page.getByLabel('Actions for Projects').click()
  await expect(projectsMenu.locator('button', { hasText: 'Rename' })).toHaveCount(1)
  await projectsMenu.locator('button', { hasText: 'Rename' }).click()
  await expect(page.getByRole('dialog', { name: 'Rename connected folder' })).toBeVisible()
  await page.getByLabel('Folder name').fill('Client projects')
  await page.getByRole('button', { name: 'Rename folder' }).click()
  await expect.poll(() => folderMutations.at(-1)).toMatchObject({
    revision: 1, kind: 'folder_rename', messageStableIds: [], folderStableId: 'Projects', folderName: 'Client projects',
  })
  await page.getByLabel('Actions for Projects').click()
  await projectsMenu.locator('button', { hasText: 'Delete' }).click()
  await expect(page.getByRole('dialog', { name: 'Delete connected folder' })).toBeVisible()
  await page.getByRole('button', { name: 'Delete folder' }).click()
  await expect.poll(() => folderMutations.at(-1)).toMatchObject({
    revision: 1, kind: 'folder_delete', messageStableIds: [], folderStableId: 'Projects', folderName: '',
  })
  const inboxMenu = page.getByLabel('Actions for INBOX').locator('..')
  await page.getByLabel('Actions for INBOX').click()
  await expect(inboxMenu.locator('button', { hasText: 'Rename' })).toHaveCount(0)
  await expect(inboxMenu.locator('button', { hasText: 'Delete' })).toHaveCount(0)
  await page.keyboard.press('Escape')

  const accountColor = page.getByLabel('Choose color for Remote work')
  await accountColor.fill('#aa55cc')
  await accountColor.dispatchEvent('change')
  await expect.poll(() => accountPatches.at(-1)).toMatchObject({ id: ACCOUNT_ID, action: 'update', revision: 1, color: '#aa55cc' })
  await expect(page.getByRole('button', { name: 'Move Remote work up' })).toBeDisabled()
  await page.getByRole('button', { name: 'Move Remote work down' }).click()
  await expect.poll(() => accountPatches.slice(-2)).toEqual([
    { id: SECOND_ACCOUNT_ID, action: 'update', revision: 1, order: 0 },
    { id: ACCOUNT_ID, action: 'update', revision: 2, order: 1 },
  ])
  await expect(page.getByRole('button', { name: 'Move Remote work down' })).toBeDisabled()
  await page.getByLabel('Actions for Remote work').click()
  const syncNow = page.getByRole('menuitem', { name: 'Sync now' })
  await expect(syncNow).toBeVisible()
  await syncNow.dispatchEvent('click')
  await expect.poll(() => syncRequests.at(-1)).toEqual({ id: ACCOUNT_ID, body: { revision: 3 } })
  await page.getByLabel('Actions for Remote work').click()
  await expect(page.getByRole('menuitem', { name: 'Account settings' })).toHaveAttribute('href', '/settings#connected-accounts')

  await accountInboxLink.click()
  await expect(page).toHaveURL(new RegExp(`/inbox/?\\?account=${ACCOUNT_ID}$`))
  await expect(mailboxSelect).toHaveAttribute('value', `account:${ACCOUNT_ID}`)
  await expect(page.getByText('Remote inbox message', { exact: true })).toBeVisible()
  await expect(page.getByText('Second remote inbox message', { exact: true })).toBeVisible()
  await expect(page.getByText('Hosted inbox message', { exact: true })).toHaveCount(0)
  await expect.poll(() => inboxRequests.at(-1)).toContain(`account=${ACCOUNT_ID}`)

  const exportDownload = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export folder as mbox' }).click()
  expect((await exportDownload).suggestedFilename()).toBe(`${ACCOUNT_ID}-inbox.mbox`)
  await expect.poll(() => exportRequests.at(-1)).toContain(`account=${ACCOUNT_ID}`)

  await page.getByRole('checkbox', { name: 'Select Remote inbox message', exact: true }).click()
  await page.getByRole('checkbox', { name: 'Select Second remote inbox message', exact: true }).click()
  await page.getByRole('button', { name: 'Mark read', exact: true }).click()
  await expect.poll(() => providerPatches.filter(item => item.patch.read === true).length).toBe(2)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => providerMessages.filter(message => message.folder === 'inbox' && message.read).length).toBe(0)
  await page.getByRole('button', { name: 'Redo', exact: true }).click()
  await expect.poll(() => providerMessages.filter(message => message.folder === 'inbox' && message.read).length).toBe(2)

  await page.locator('[data-email-id="extmsg.remote-inbox"]').getByRole('button', { name: 'Archive', exact: true }).click()
  await expect(page.getByText('Remote inbox message', { exact: true })).toHaveCount(0)
  expect(providerPatches).toContainEqual({ id: 'extmsg.remote-inbox', patch: { folder: 'archive' } })
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toBeVisible()
  const requestsBeforeConflictRefresh = inboxRequests.length
  await page.getByRole('button', { name: 'Refresh', exact: true }).click()
  await expect.poll(() => inboxRequests.length).toBeGreaterThan(requestsBeforeConflictRefresh)

  await accountSentLink.click()
  await expect(page).toHaveURL(new RegExp(`/folder/sent/?\\?account=${ACCOUNT_ID}$`))
  await expect(page.getByText('Remote sent message', { exact: true })).toBeVisible()
  await expect(page.getByText('Remote inbox message', { exact: true })).toHaveCount(0)

  // The sidebar is the desktop mailbox control; the select becomes the
  // equivalent tablet/mobile control once the sidebar collapses.
  await page.setViewportSize({ width: 900, height: 900 })
  await waitForUiReady(page)
  await expect(mailboxSelect).toBeVisible()
  const mailboxCombobox = mailboxSelect.locator('[role="combobox"]')
  const hostedOption = mailboxSelect.locator('[role="option"][value="domain:example.com"]')
  await expect(async () => {
    if (!await hostedOption.isVisible()) await mailboxCombobox.click({ timeout: 1_000 })
    await hostedOption.click({ timeout: 1_000 })
  }).toPass({ timeout: 10_000, intervals: [100, 250, 500] })
  await expect(mailboxSelect).toHaveAttribute('value', 'domain:example.com')
  await expect(page.getByText('Remote sent message', { exact: true })).toHaveCount(0)

  const overflow = await page.evaluate(() => ({ width: innerWidth, document: document.documentElement.scrollWidth }))
  expect(overflow.document - overflow.width).toBeLessThanOrEqual(1)
  expect(errors.filter(error => !error.includes('status of 409 (Conflict)'))).toEqual([])
})

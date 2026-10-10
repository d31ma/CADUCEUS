import { test, expect } from '@playwright/test'
import { API, getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function authenticate(page, label) {
  const nonce = `${label}-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const domain = `${nonce}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  return account
}

async function createContact(token, body) {
  const response = await fetch(`${API}/contacts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
  })
  if (!response.ok) throw new Error(`Contact seed failed: ${await response.text()}`)
  return response.json()
}

async function sendMessage(token, body) {
  const response = await fetch(`${API}/send`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
  })
  if (!response.ok) throw new Error(`Message seed failed: ${await response.text()}`)
  return response.json()
}

test('manages groups and contacts and round-trips vCards in the browser', async ({ page }) => {
  await authenticate(page, 'contacts-ui')
  await page.goto('/contacts')
  await waitForUiReady(page)
  await expect(page.getByRole('heading', { name: 'Contacts' })).toBeVisible()
  await expect(page.locator('.sidebar-folder.active')).toContainText('Contacts')

  await page.getByRole('button', { name: 'Create contact group' }).click()
  const groupDialog = page.locator('.contact-dialog')
  await groupDialog.locator('[data-contact-group-name]').fill('Vendors')
  await groupDialog.getByRole('button', { name: 'Save' }).click()
  await expect(page.locator('.contacts-filters')).toContainText('Vendors')

  await page.getByRole('button', { name: 'New contact' }).click()
  await page.locator('[data-contact-name]').fill('Grace Hopper')
  await page.locator('.contact-form-section').filter({ hasText: 'Email addresses' }).locator('input[type="email"]').fill('grace@example.net')
  await page.getByLabel('Company').fill('United States Navy')
  await page.getByLabel('Title').fill('Rear Admiral')
  await page.getByPlaceholder('Notes about this contact').fill('Compiler pioneer')
  await page.locator('.contact-group-checks label').filter({ hasText: 'Vendors' }).locator('input').check()
  await page.locator('.contact-editor-header').getByRole('button', { name: 'Save' }).click()

  await expect(page.locator('.contact-detail-title')).toContainText('Grace Hopper')
  await expect(page.locator('.contact-detail')).toContainText('grace@example.net')
  await expect(page.locator('.contact-detail')).toContainText('Compiler pioneer')

  await page.locator('.contacts-search input').fill('compiler')
  await expect(page.locator('.contacts-list')).toContainText('Grace Hopper')

  const exportPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export all' }).click()
  const download = await exportPromise
  expect(download.suggestedFilename()).toBe('contact.vcf')
  const exportPath = await download.path()
  expect(await Bun.file(exportPath).text()).toContain('FN:Grace Hopper')

  await page.locator('.contacts-search input').fill('')
  await page.locator('[data-contact-import]').setInputFiles({
    name: 'people.vcf',
    mimeType: 'text/vcard',
    buffer: Buffer.from([
      'BEGIN:VCARD', 'VERSION:4.0', 'FN:Ada Lovelace', 'N:Lovelace;Ada;;;',
      'EMAIL;TYPE=work:ada@example.net', 'END:VCARD',
    ].join('\r\n')),
  })
  await expect(page.locator('.contacts-list')).toContainText('Ada Lovelace')
  await page.getByRole('button', { name: 'Imported vCards' }).click()
  await expect(page.locator('.contacts-list')).toContainText('Ada Lovelace')
  await expect(page.locator('.contacts-list')).toContainText('Imported vCard')
})

test('completes recipients from personal and found-in-mail contacts with the keyboard', async ({ page }) => {
  const account = await authenticate(page, 'contacts-compose')
  await createContact(account.token, {
    name: 'Grace Hopper', emails: [{ value: 'grace@example.net', type: 'work' }],
  })
  await sendMessage(account.token, { to: ['ada@example.net'], subject: 'Hello Ada', text: 'Hello' })
  await page.goto('/compose')
  await waitForUiReady(page)
  const to = page.locator('[data-compose-to] input')
  await to.fill('gra')
  await expect(page.getByRole('option', { name: /Grace Hopper/ })).toBeVisible()
  await to.press('Enter')
  await expect(to).toHaveValue('grace@example.net')

  const cc = page.locator('[data-compose-cc] input')
  await cc.fill('ada')
  await expect(page.getByRole('option', { name: /ada@example.net/ })).toBeVisible()
  await page.getByRole('option', { name: /ada@example.net/ }).click()
  await expect(cc).toHaveValue('ada@example.net')
})

test('uses a list-to-detail flow without horizontal overflow on a phone', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 568 })
  const account = await authenticate(page, 'contacts-phone')
  await createContact(account.token, { name: 'Alan Turing', emails: [{ value: 'alan@example.net' }] })
  await page.goto('/contacts')
  await waitForUiReady(page)

  await expect(page.locator('.contacts-list-panel')).toBeVisible()
  await page.getByRole('option', { name: /Alan Turing/ }).click()
  await expect(page.locator('.contact-detail')).toBeVisible()
  await expect(page.locator('.contacts-list-panel')).toBeHidden()
  const width = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth }))
  expect(width.document - width.viewport).toBeLessThanOrEqual(1)
  await page.getByRole('button', { name: 'Back to contacts' }).click()
  await expect(page.locator('.contacts-list-panel')).toBeVisible()
})

test('creates a contact in a selected CardDAV address book', async ({ page }) => {
  await authenticate(page, 'contacts-carddav-create')
  const remote = {
    accountId: 'ext_AAAAAAAAAAAAAAAAAAAA',
    addressBookId: 'book_abcdefghijklmnopqrstuvwx',
    name: 'Team contacts',
    accountLabel: 'Work',
    mailboxAddress: 'alice@remote.example',
  }
  let submitted = null
  await page.route(`${API}/contacts/address-books`, route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([remote]) }))
  await page.route(`${API}/contacts`, async route => {
    if (route.request().method() !== 'POST') return route.fallback()
    submitted = route.request().postDataJSON()
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
      id: 'carddav_abcdefghijklmnopqrstuvwx', source: 'carddav', accountId: remote.accountId, addressBookId: remote.addressBookId,
      name: submitted.name, emails: submitted.emails, phones: [], addresses: [], urls: [], nicknames: [], groupIds: [],
      readOnly: false, syncState: 'queued', mutationId: 'cmut_abcdefghijklmnopqrstuvwx',
    }) })
  })
  await page.route(new RegExp(`${API}/contacts/carddav_[^/]+/mutations/cmut_[^?]+\\?accountId=`), route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ state: 'applied' }) }))

  await page.goto('/contacts')
  await waitForUiReady(page)
  await page.getByRole('button', { name: 'New contact' }).click()
  await expect(page.getByLabel('Address book')).toBeVisible()
  await page.getByLabel('Address book').selectOption(`${remote.accountId}|${remote.addressBookId}`)
  await page.locator('[data-contact-name]').fill('Remote Person')
  await page.locator('.contact-form-section').filter({ hasText: 'Email addresses' }).locator('input[type="email"]').fill('remote@example.net')
  await page.locator('.contact-editor-header').getByRole('button', { name: 'Save' }).click()
  await expect.poll(() => submitted).not.toBeNull()
  expect(submitted).toMatchObject({ source: 'carddav', accountId: remote.accountId, addressBookId: remote.addressBookId, name: 'Remote Person', groupIds: [] })
  expect(submitted.idempotencyKey).toMatch(/^contact-create-/)
})

test('keeps the CardDAV server version on conflict and offers explicit review', async ({ page }) => {
  await authenticate(page, 'contacts-carddav-conflict')
  const contact = {
    id: 'carddav_abcdefghijklmnopqrstuvwx', source: 'carddav', accountId: 'ext_AAAAAAAAAAAAAAAAAAAA',
    addressBookId: 'book_abcdefghijklmnopqrstuvwx', href: 'https://dav.example/books/a.vcf', etag: '"8"', uid: 'uid-a',
    name: 'Server Version', givenName: 'Server', familyName: 'Version', company: '', title: '',
    emails: [{ value: 'server@example.net', type: 'work' }], phones: [], addresses: [], urls: [], nicknames: [], birthday: '', notes: '',
    groupIds: [], createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(), readOnly: false,
  }
  await page.route(`${API}/contacts/address-books`, route => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }))
  await page.route(new RegExp(`${API}/contacts(?:\\?.*)?$`), route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([contact]) }))
  await page.route(`${API}/contacts/${contact.id}`, async route => {
    if (route.request().method() !== 'PUT') return route.fallback()
    const changed = route.request().postDataJSON()
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...contact, ...changed, syncState: 'queued', mutationId: 'cmut_abcdefghijklmnopqrstuvwx' }) })
  })
  await page.route(new RegExp(`${API}/contacts/${contact.id}/mutations/cmut_[^?]+\\?accountId=`), route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ state: 'conflict' }) }))

  await page.goto('/contacts')
  await waitForUiReady(page)
  await page.getByRole('option', { name: /Server Version/ }).click()
  await page.getByRole('button', { name: 'Edit' }).click()
  await page.locator('[data-contact-name]').fill('Local Version')
  await page.locator('.contact-editor-header').getByRole('button', { name: 'Save' }).click()
  const alert = page.locator('.contact-operation-alert')
  await expect(alert).toContainText('server version was kept')
  await expect(alert.getByRole('button', { name: 'Review and edit' })).toBeVisible()
  await expect(alert.getByRole('button', { name: 'Keep server version' })).toBeVisible()
  await alert.getByRole('button', { name: 'Review and edit' }).click()
  await expect(page.locator('[data-contact-name]')).toHaveValue('Server Version')
})

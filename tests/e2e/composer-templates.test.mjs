import { test, expect } from '@playwright/test'
import { API, getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function authenticate(page, label) {
  const domain = `${label}-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  return identity
}

async function createTemplate(token, input) {
  const response = await fetch(`${API}/templates`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(input),
  })
  if (!response.ok) throw new Error(`Template seed failed: ${await response.text()}`)
  return response.json()
}

test('inserts, tabs through, and strips Mailspring template blanks before sending', async ({ page }) => {
  const account = await authenticate(page, 'template-compose')
  const template = await createTemplate(account.token, {
    name: 'Introduction',
    subject: 'A quick introduction',
    to: 'recipient@example.net',
    text: '<p>Hello <code class="var">Name</code>,</p><p>Welcome to <code class="var">Company</code>.</p>',
  })

  let sentPayload
  await page.route(`${API}/send`, async route => {
    sentPayload = route.request().postDataJSON()
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ sent: true, undoId: 'template-undo' }) })
  })
  await page.goto('/compose')
  await waitForUiReady(page)

  await page.locator('.compose-template-search').fill('Intro')
  await page.locator('.compose-template-select').getByRole('combobox').click()
  await expect(page.locator('.compose-template-select').getByRole('option', { name: 'Introduction' })).toBeVisible()
  await page.locator('.compose-template-select').getByRole('option', { name: 'Introduction' }).click()

  const variables = page.locator('[data-compose-editor] code[data-template-var]')
  await expect(variables).toHaveCount(2)
  await expect(page.locator('[data-template-status]')).toContainText('Template blanks: 2')
  await page.keyboard.type('Ada')
  await page.keyboard.press('Tab')
  await page.keyboard.type('Analytical Engines')
  await expect(page.locator('[data-template-status]')).toContainText('Template blanks: 2')

  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect.poll(() => sentPayload).toBeTruthy()
  expect(sentPayload.to).toEqual(['recipient@example.net'])
  expect(sentPayload.subject).toBe('A quick introduction')
  expect(sentPayload.html).toContain('Ada')
  expect(sentPayload.html).toContain('Analytical Engines')
  expect(sentPayload.html).not.toMatch(/data-template-var|<code[^>]*\bvar\b/i)
})

test('manages template create, edit, and delete flows in settings', async ({ page }) => {
  const account = await authenticate(page, 'template-settings')
  const template = await createTemplate(account.token, {
    name: 'Renewal', subject: 'Renewal notice', text: '<p>Hello</p>',
  })
  await page.goto('/settings')
  await waitForUiReady(page)

  const section = page.locator('[data-template-settings]')
  await expect(section).toContainText('Renewal')
  await section.locator('.template-settings-select').filter({ hasText: 'Renewal' }).click()
  await section.locator('[data-template-name] input').fill('Renewal follow-up')
  await section.locator('[data-template-content] textarea').fill('<p>Hello <code class="var">Name</code></p>')
  await section.getByRole('button', { name: 'Save template' }).click()
  await expect(section).toContainText('Renewal follow-up')

  const updated = await fetch(`${API}/templates`, { headers: { Authorization: `Bearer ${account.token}` } }).then(response => response.json())
  expect(updated.find(item => item.id === template.id)).toMatchObject({
    name: 'Renewal follow-up', text: '<p>Hello <code class="var">Name</code></p>',
  })

  page.once('dialog', dialog => dialog.accept())
  await section.locator('.settings-row').filter({ hasText: 'Renewal follow-up' }).getByRole('button', { name: 'Remove' }).click()
  await expect(section).not.toContainText('Renewal follow-up')
  const remaining = await fetch(`${API}/templates`, { headers: { Authorization: `Bearer ${account.token}` } }).then(response => response.json())
  expect(remaining.some(item => item.id === template.id)).toBe(false)
})

test('saves the editable draft as a template and warns about an empty blank', async ({ page }) => {
  const account = await authenticate(page, 'template-draft')
  await page.goto('/compose')
  await waitForUiReady(page)
  await page.locator('[data-compose-subject] input').fill('Customer welcome')
  await page.locator('[data-compose-editor]').fill('Welcome aboard')
  page.once('dialog', dialog => dialog.accept('Customer Welcome'))
  await page.getByRole('button', { name: 'Save as template' }).click()
  await expect.poll(async () => {
    const templates = await fetch(`${API}/templates`, { headers: { Authorization: `Bearer ${account.token}` } }).then(response => response.json())
    return templates.find(item => item.name === 'Customer Welcome')?.text
  }).toContain('Welcome aboard')

  const empty = await createTemplate(account.token, {
    name: 'Blank check', subject: 'Complete this', to: 'recipient@example.net',
    text: '<p>Hello <code class="var empty"><br></code></p>',
  })
  await page.getByRole('button', { name: 'Discard' }).click()
  await page.goto('/compose')
  await waitForUiReady(page)
  await page.locator('.compose-template-search').fill('Blank')
  await page.locator('.compose-template-select').getByRole('combobox').click()
  await page.locator('.compose-template-select').getByRole('option', { name: 'Blank check' }).click()
  await expect(page.locator('[data-template-status]')).toContainText('1 empty')
  page.once('dialog', dialog => dialog.dismiss())
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(page.locator('.login-error')).toContainText('Fill every highlighted template blank')
  expect(empty.id).toBeTruthy()
})

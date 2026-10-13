import { test, expect } from '@playwright/test'
import { API, getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function authenticate(page, label) {
  const domain = `${label}-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  const alias = `sales@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await fetch(`${API}/users/aliases`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${identity.token}` },
    body: JSON.stringify({ alias }),
  }).then(async response => {
    if (!response.ok) throw new Error(`Alias seed failed: ${await response.text()}`)
  })
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  return { ...identity, alias }
}

async function createSignature(token, input) {
  const response = await fetch(`${API}/signatures`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(input),
  })
  if (!response.ok) throw new Error(`Signature seed failed: ${await response.text()}`)
  return response.json()
}

test('applies per-address defaults, switches signatures, and preserves quoted mail', async ({ page }) => {
  const account = await authenticate(page, 'signature-compose')
  const primary = await createSignature(account.token, {
    name: 'Primary', text: '<p>Regards,<br>Primary team</p>', defaultFor: [account.email],
  })
  const alias = await createSignature(account.token, {
    name: 'Sales', text: '<p>Cheers,<br>Sales team</p>', defaultFor: [account.alias],
  })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.evaluate(() => sessionStorage.setItem('caduceus_compose_prefill', JSON.stringify({
    to: 'recipient@example.net',
    subject: 'Re: Signed reply',
    body: '<p>My reply</p><blockquote class="gmail_quote">Earlier mail</blockquote>',
  })))
  await page.goto('/compose')
  await waitForUiReady(page)

  const editor = page.locator('[data-compose-editor]')
  await expect(page.locator('[data-compose-from]')).toHaveValue(account.email)
  await expect(editor.locator(`signature[id="${primary.id}"]`)).toContainText('Primary team')
  await expect(editor.locator('blockquote.gmail_quote')).toContainText('Earlier mail')

  await page.locator('[data-compose-from]').selectOption(account.alias)
  await expect(editor.locator(`signature[id="${alias.id}"]`)).toContainText('Sales team')
  await expect(editor.locator('blockquote.gmail_quote')).toContainText('Earlier mail')
  await expect(page.locator('[data-compose-signature]')).toHaveValue(alias.id)

  await page.locator('[data-compose-signature]').selectOption('')
  await expect(editor.locator('signature')).toHaveCount(0)
  await expect(editor.locator('blockquote.gmail_quote')).toContainText('Earlier mail')
  await page.locator('[data-compose-signature]').selectOption(primary.id)
  await expect(page.locator('[data-compose-from]')).toHaveValue(account.alias)

  let payload
  await page.route(`${API}/send`, async route => {
    payload = route.request().postDataJSON()
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ sent: true, undoId: 'signature-undo' }) })
  })
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect.poll(() => payload).toBeTruthy()
  expect(payload.from).toBe(account.alias)
  expect(payload.html).toContain(`id="${primary.id}"`)
  expect(payload.html).toContain('Earlier mail')
})

test('edits safe rich HTML, exact defaults, previews, and deletion in settings', async ({ page }) => {
  const account = await authenticate(page, 'signature-settings')
  const signature = await createSignature(account.token, {
    name: 'Imported', text: '<p>Old signature</p>', defaultFor: [account.email],
  })
  await page.goto('/settings')
  await waitForUiReady(page)

  const section = page.locator('[data-signature-settings]')
  await section.locator('.signature-settings-select').filter({ hasText: 'Imported' }).click()
  await section.locator('[data-signature-name] input').fill('Imported rich')
  const source = section.locator('[data-signature-html] textarea')
  await source.fill('')
  await source.evaluate(textarea => {
    const clipboard = new DataTransfer()
    clipboard.setData('text/plain', 'Rich Alice')
    clipboard.setData('text/html', '<p onclick="steal()"><strong>Rich Alice</strong><script>bad()</script></p>')
    textarea.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: clipboard }))
  })
  await expect(section.locator('[data-signature-preview] strong')).toHaveText('Rich Alice')
  await section.locator(`label:has-text("${account.alias}") input[data-signature-default]`).check()
  await section.getByRole('button', { name: 'Update signature' }).click()
  await expect(section).toContainText('Imported rich')

  const updated = await fetch(`${API}/signatures`, { headers: { Authorization: `Bearer ${account.token}` } }).then(response => response.json())
  expect(updated.find(item => item.id === signature.id)).toMatchObject({
    name: 'Imported rich', defaultFor: [account.email, account.alias],
  })
  expect(updated.find(item => item.id === signature.id).text).toContain('<strong>Rich Alice</strong>')
  expect(updated.find(item => item.id === signature.id).text).not.toMatch(/onclick|script/i)

  page.once('dialog', dialog => dialog.accept())
  await section.locator('.settings-row').filter({ hasText: 'Imported rich' }).getByRole('button', { name: 'Remove' }).click()
  await expect(section).not.toContainText('Imported rich')
  const remaining = await fetch(`${API}/signatures`, { headers: { Authorization: `Bearer ${account.token}` } }).then(response => response.json())
  expect(remaining.some(item => item.id === signature.id)).toBe(false)
})

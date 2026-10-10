import { test, expect } from '@playwright/test'
import {
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('creates nested categories and applies tri-state labels and folders with undo', async ({ page }) => {
  test.setTimeout(60_000)
  const errors = captureUiErrors(page)
  const domain = `category-picker-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const account = `admin@${domain}`
  await seedUser(account, { domains: [domain], role: 'admin' })
  const identity = await getToken(account)
  await seedDomain(identity.token, domain)

  const messages = [
    { id: `category-one-${Date.now()}`, subject: 'Enterprise launch one' },
    { id: `category-two-${Date.now()}`, subject: 'Enterprise launch two' },
  ]
  for (const message of messages) {
    await deliverEmail({
      recipient: account,
      sender: 'projects@other.test',
      subject: message.subject,
      body: `Body for ${message.subject}`,
      messageId: message.id,
    })
  }

  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)

  const rows = messages.map(message => page.locator(`[data-email-id="${message.id}"]`))
  for (const row of rows) await row.locator('.bulk-checkbox').check()

  const toolbar = page.getByRole('toolbar', { name: 'Selected message actions' })
  await toolbar.getByRole('button', { name: 'Labels…' }).click()
  const dialog = page.getByRole('dialog', { name: 'Apply labels' })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('searchbox', { name: 'Search categories' }).fill('Customers/Enterprise')
  await dialog.getByRole('button', { name: 'Create label “customers/enterprise”' }).click()
  await expect(dialog).toHaveCount(0)
  for (const row of rows) await expect(row.locator('.email-label')).toHaveText('customers/enterprise')

  for (const row of rows) await row.locator('.bulk-checkbox').check()
  await page.getByRole('toolbar', { name: 'Selected message actions' }).getByRole('button', { name: 'Labels…' }).click()
  await dialog.getByRole('searchbox').fill('enterprise')
  const labelOption = dialog.getByRole('option', { name: /Enterprise/ })
  await expect(labelOption).toHaveAttribute('aria-selected', 'true')
  await labelOption.click()
  for (const row of rows) await expect(row.locator('.email-label')).toHaveCount(0)

  const toast = page.locator('#caduceus-toast')
  await toast.getByRole('button', { name: 'Undo' }).click()
  for (const row of rows) await expect(row.locator('.email-label')).toHaveText('customers/enterprise')

  await page.goto(`/email/${encodeURIComponent(messages[0].id)}`)
  await waitForUiReady(page)
  await page.locator('.email-actions-state').getByRole('button', { name: 'Labels…' }).click()
  const detailDialog = page.getByRole('dialog', { name: 'Apply labels' })
  await detailDialog.getByRole('searchbox').fill('enterprise')
  await expect(detailDialog.getByRole('option', { name: /Enterprise/ })).toHaveAttribute('aria-selected', 'true')
  await detailDialog.getByRole('button', { name: 'Close category picker' }).click()

  await page.goto('/inbox')
  await waitForUiReady(page)

  await rows[0].locator('.bulk-checkbox').check()
  await page.getByRole('toolbar', { name: 'Selected message actions' }).getByRole('button', { name: 'Move to…' }).click()
  const moveDialog = page.getByRole('dialog', { name: 'Move to folder' })
  await moveDialog.getByRole('searchbox').fill('Projects/Launch')
  await moveDialog.getByRole('button', { name: 'Create folder “projects/launch”' }).click()
  await expect(rows[0]).toHaveCount(0)

  await page.goto(`/inbox?folder=projects%2Flaunch&domain=${encodeURIComponent(domain)}`)
  await waitForUiReady(page)
  await expect(page.locator(`[data-email-id="${messages[0].id}"]`)).toBeVisible()
  await expect(page.locator(`[data-email-id="${messages[1].id}"]`)).toHaveCount(0)
  expect(errors).toEqual([])
})

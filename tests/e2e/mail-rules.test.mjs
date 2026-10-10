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

const api = (token, path, init = {}) => fetch(`${API}${path}`, {
  ...init,
  headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, ...(init.headers || {}) },
})

test('mailbox owner creates, previews, edits, orders, applies, and deletes rules', async ({ page }) => {
  const domain = `rules-${Date.now()}.test`
  const adminEmail = `admin@${domain}`
  await seedUser(adminEmail, { domains: [domain], role: 'admin' })
  const admin = await getToken(adminEmail)
  await seedDomain(admin.token, domain)

  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'viewer' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, 'viewer', account.domains)
  const errors = captureUiErrors(page)

  await page.goto('/settings')
  await waitForUiReady(page)
  const section = page.locator('[data-rule-settings]')
  await expect(section).toBeVisible()
  await expect(page.getByText(email, { exact: true }).first()).toBeVisible()
  await expect(page.locator('.settings-section-title').filter({ hasText: 'Users' })).toHaveCount(0)

  await section.getByRole('button', { name: 'Add rule' }).click()
  await section.locator('[data-rule-name] input').fill('Finance invoices')
  await section.locator('[data-rule-condition-field]').getByRole('combobox').click()
  await page.getByRole('option', { name: 'Subject', exact: true }).click()
  await section.locator('[data-rule-condition-op]').getByRole('combobox').click()
  await page.getByRole('option', { name: 'Contains', exact: true }).click()
  await section.locator('[data-rule-condition-value] input').fill('invoice')
  await section.getByRole('button', { name: 'Add', exact: true }).first().click()
  await expect(section).toContainText('subject contains "invoice"')

  await section.locator('[data-rule-action-folder] input').fill('finance')
  await section.getByRole('button', { name: 'Add', exact: true }).last().click()
  await expect(section).toContainText('folder "finance"')
  await section.locator('[data-rule-action-type]').getByRole('combobox').click()
  await page.getByRole('option', { name: 'Mark read', exact: true }).click()
  await section.getByRole('button', { name: 'Add', exact: true }).last().click()
  await expect(section).toContainText('mark read')
  await section.locator('[data-rule-stop]').check()

  await section.locator('[data-rule-preview-from] input').fill('billing@vendor.test')
  await section.locator('[data-rule-preview-subject] input').fill('August invoice')
  await expect(section.locator('[data-rule-name] input')).toHaveValue('Finance invoices')
  await expect(section.locator('[data-rule-mailbox]').getByRole('combobox')).toContainText(email)
  await section.getByRole('button', { name: 'Preview rule' }).click()
  await expect(section.getByRole('status')).toContainText('Match')
  await expect(section.getByRole('status')).toContainText('finance')
  await section.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(section).toContainText('Finance invoices')

  let rules = await api(account.token, '/rules').then(response => response.json())
  const finance = rules.find(rule => rule.name === 'Finance invoices')
  expect(finance).toMatchObject({ ownerEmail: email, mailbox: email, stopProcessing: true })
  expect(finance.actions).toEqual([{ type: 'folder', folder: 'finance' }, { type: 'markRead', value: true }])

  const secondResponse = await api(account.token, '/rules', {
    method: 'POST',
    body: JSON.stringify({ name: 'Later rule', mailbox: email, actions: [{ type: 'star', value: true }] }),
  })
  expect(secondResponse.ok).toBe(true)
  await page.reload()
  await waitForUiReady(page)
  const laterRow = page.locator('[data-rule-settings] .rule-list-row').filter({ hasText: 'Later rule' })
  await laterRow.getByRole('button', { name: /Move Later rule earlier/ }).click()
  await expect.poll(async () => {
    rules = await api(account.token, '/rules').then(response => response.json())
    return rules[0]?.name
  }).toBe('Later rule')

  const financeRow = page.locator('[data-rule-settings] .rule-list-row').filter({ hasText: 'Finance invoices' })
  await financeRow.getByRole('button', { name: 'Edit' }).click()
  await section.locator('[data-rule-name] input').fill('Vendor invoices')
  await section.getByRole('button', { name: 'Update', exact: true }).click()
  await expect(section).toContainText('Vendor invoices')

  const subject = `Invoice ${Date.now()}`
  await deliverEmail({ recipient: email, sender: 'billing@vendor.test', subject, body: 'Please process this invoice.' })
  const stored = await api(account.token, '/inbox?folder=finance').then(response => response.json())
  expect(stored.find(message => message.subject === subject)).toMatchObject({ folder: 'finance', read: true })

  page.once('dialog', dialog => dialog.accept())
  await page.locator('[data-rule-settings] .rule-list-row').filter({ hasText: 'Vendor invoices' }).getByRole('button', { name: 'Remove' }).click()
  await expect(section).not.toContainText('Vendor invoices')
  expect(errors).toEqual([])
})

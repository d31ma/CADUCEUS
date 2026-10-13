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

test('reads, searches, and acts on every message in an RFC conversation', async ({ page }) => {
  const errors = captureUiErrors(page)
  const domain = `message-list-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const account = `admin@${domain}`
  await seedUser(account, { domains: [domain], role: 'admin' })
  const identity = await getToken(account)
  await seedDomain(identity.token, domain)
  const key = `conversation-${Date.now()}`
  const subject = `${key} launch review`
  const source = ({ messageId, inReplyTo = '', references = '', body }) => [
    'From: Reviewer <reviewer@other.test>',
    `To: ${account}`,
    `Subject: ${inReplyTo ? `Re: ${subject}` : subject}`,
    `Message-ID: <${messageId}>`,
    ...(inReplyTo ? [`In-Reply-To: <${inReplyTo}>`] : []),
    ...(references ? [`References: ${references}`] : []),
    'Content-Type: text/plain; charset=utf-8',
    '',
    body,
  ].join('\r\n')

  const rootId = `${key}-root@other.test`
  const replyId = `${key}-reply@other.test`
  await deliverEmail({
    recipient: account,
    sender: 'reviewer@other.test',
    subject,
    body: source({ messageId: rootId, body: 'First needle checkpoint.' }),
    messageId: `${key}-root`,
  })
  await deliverEmail({
    recipient: account,
    sender: 'reviewer@other.test',
    subject: `Re: ${subject}`,
    body: source({
      messageId: replyId,
      inReplyTo: rootId,
      references: `<${rootId}>`,
      body: 'Second needle checkpoint.\n\nOn Fri, Aug 14, Reviewer wrote:\n> First needle checkpoint.',
    }),
    messageId: `${key}-reply`,
  })
  await deliverEmail({
    recipient: account,
    sender: 'reviewer@other.test',
    subject: `Re: ${subject}`,
    body: source({
      messageId: `${key}-final@other.test`,
      inReplyTo: replyId,
      references: `<${rootId}> <${replyId}>`,
      body: 'Final approval.',
    }),
    messageId: `${key}-final`,
  })

  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)
  const finalRow = page.locator(`[data-email-id="${key}-final"]`)
  await finalRow.hover()
  await finalRow.getByRole('button', { name: 'Toggle star' }).click()
  await expect(finalRow.locator('.email-sender')).toContainText('★')
  await expect(page.locator('.conversation-message')).toHaveCount(0)
  await finalRow.getByRole('button', { name: /^Open message/ }).click({ position: { x: 120, y: 32 } })

  const cards = page.locator('.conversation-message')
  await expect(cards).toHaveCount(3)
  await expect(cards.filter({ hasText: 'Final approval.' })).toHaveCount(1)

  const rootCard = page.locator(`[data-conversation-message-id="${key}-root"]`)
  await rootCard.locator('.conversation-message-summary').click()
  await expect(rootCard.locator('.conversation-message-body')).toContainText('First needle checkpoint.')

  const replyCard = page.locator(`[data-conversation-message-id="${key}-reply"]`)
  await replyCard.locator('.conversation-message-summary').click()
  await expect(replyCard.locator('.conversation-message-body')).toHaveText('Second needle checkpoint.')
  await replyCard.getByRole('button', { name: 'Show quoted content' }).click()
  await expect(replyCard.locator('.conversation-message-body')).toContainText('First needle checkpoint.')

  await page.getByRole('button', { name: 'Find in conversation' }).first().click()
  const find = page.getByRole('search', { name: 'Find in conversation' })
  await expect(find).toBeVisible()
  await find.getByRole('searchbox', { name: 'Find in conversation' }).fill('needle')
  await expect(find.getByRole('status')).toHaveText('1 of 3')
  await expect(page.locator('mark.current-match')).toHaveCount(1)
  await find.getByRole('button', { name: 'Next conversation match' }).click()
  await expect(find.getByRole('status')).toHaveText('2 of 3')
  await find.getByRole('button', { name: 'Previous conversation match' }).click()
  await expect(find.getByRole('status')).toHaveText('1 of 3')

  const downloadPromise = page.waitForEvent('download')
  await rootCard.getByRole('button', { name: `Download original ${subject}` }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/\.eml$/)

  await rootCard.getByRole('button', { name: 'Reply to reviewer@other.test' }).click()
  await expect(page.locator('[data-compose-to] input')).toHaveValue('reviewer@other.test')
  await expect(page.locator('[data-compose-subject] input')).toHaveValue(`Re: ${subject}`)
  expect(errors).toEqual([])
})

test('bulk-selects messages and completes archive, undo, and redo in the inbox', async ({ page }) => {
  const errors = captureUiErrors(page)
  const domain = `bulk-actions-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const account = `admin@${domain}`
  await seedUser(account, { domains: [domain], role: 'admin' })
  const identity = await getToken(account)
  await seedDomain(identity.token, domain)
  const key = `bulk-${Date.now()}`
  const messages = [
    { id: `${key}-one`, subject: `${key} first` },
    { id: `${key}-two`, subject: `${key} second` },
  ]
  for (const message of messages) {
    await deliverEmail({
      recipient: account,
      sender: 'operations@other.test',
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
  for (const row of rows) {
    await expect(row).toBeVisible()
  }

  const orderedRows = page.locator('.email-row')
  const rowOpenButtons = orderedRows.locator('.email-row-open')
  await expect(orderedRows).toHaveCount(2)
  await expect(page.locator('.email-row-open[tabindex="0"]')).toHaveCount(1)
  await rowOpenButtons.first().focus()
  await expect(rowOpenButtons.first()).toBeFocused()
  await rowOpenButtons.first().press('ArrowDown')
  await expect(rowOpenButtons.nth(1)).toBeFocused()
  await expect(rowOpenButtons.nth(1)).toHaveAttribute('tabindex', '0')
  expect(await orderedRows.first().evaluate(element => getComputedStyle(element).contentVisibility)).toBe('auto')

  await orderedRows.nth(1).click({ button: 'right' })
  const contextMenu = page.getByRole('menu', { name: 'Message actions' })
  await expect(contextMenu).toBeVisible()
  await contextMenu.getByRole('menuitem', { name: 'Star', exact: true }).click()
  await expect(orderedRows.nth(1).locator('.email-sender')).toContainText('★')

  for (const row of rows) await row.locator('.bulk-checkbox').check()

  const toolbar = page.getByRole('toolbar', { name: 'Selected message actions' })
  await expect(toolbar).toBeVisible()
  await expect(toolbar).toContainText('2 selected')
  await toolbar.getByRole('button', { name: 'Archive' }).click()
  for (const row of rows) await expect(row).toHaveCount(0)

  const toast = page.locator('#caduceus-toast')
  await expect(toast).toContainText('Archived selected messages.')
  await toast.getByRole('button', { name: 'Undo' }).click()
  for (const row of rows) await expect(row).toBeVisible()

  await expect(toast).toContainText('Action undone.')
  await toast.getByRole('button', { name: 'Redo' }).click()
  for (const row of rows) await expect(row).toHaveCount(0)
  await expect(toast).toContainText('Action redone.')
  expect(errors).toEqual([])
})

import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { createServer } from 'node:net'
import { setupRoutes, teardownRoutes, token, req, routeDiagnostics, email } from './helpers.js'
import { ExternalAccounts as ExternalAccountBridge } from '@/shared/accounts/ExternalAccounts.js'
import { MboxUtils } from '../../client/shared/utils/MboxUtils.js'
import { call } from '../support/controller-call.mjs'
import { TestContainer } from '../support/TestContainer.mjs'
import { Routes } from '../support/Routes.mjs'

const ProviderCatalog = Routes.ApiV1ProviderCatalogController
const ProviderDiscovery = Routes.ApiV1ProviderDiscoveryController
const ExternalAccounts = Routes.ApiV1ExternalAccountsController
const ExternalAccount = Routes.ApiV1ExternalAccountsIdController
const ValidateExternalAccount = Routes.ApiV1ExternalAccountsValidateController
const BeginMicrosoftOAuth = Routes.ApiV1ExternalAccountsOauthMicrosoftBeginController
const CompleteMicrosoftOAuth = Routes.ApiV1ExternalAccountsOauthMicrosoftCompleteController
const BeginGoogleOAuth = Routes.ApiV1ExternalAccountsOauthGoogleBeginController
const CompleteGoogleOAuth = Routes.ApiV1ExternalAccountsOauthGoogleCompleteController
const ReconnectExternalAccount = Routes.ApiV1ExternalAccountsIdReconnectController
const RetryExternalAccount = Routes.ApiV1ExternalAccountsIdRetryController
const ExternalAccountDiagnostics = Routes.ApiV1ExternalAccountsIdDiagnosticsController
const ExternalAccountFolders = Routes.ApiV1ExternalAccountsIdFoldersController
const ExternalAccountMessages = Routes.ApiV1ExternalAccountsIdMessagesController
const ExternalAccountMutations = Routes.ApiV1ExternalAccountsIdMutationsController
const ExternalAccountMutation = Routes.ApiV1ExternalAccountsIdMutationsMutationIdController
const Inbox = Routes.InboxController
const MessageDetail = Routes.InboxIdController
const MessageAttachment = Routes.InboxIdAttachmentsAttachmentIdController
const MessageSource = Routes.InboxIdSourceController
const MailActions = Routes.InboxActionsController
const UndoMailAction = Routes.InboxActionsIdUndoController
const RedoMailAction = Routes.InboxActionsIdRedoController
const UserPreferences = Routes.UsersMePreferencesController
const ExportSources = Routes.InboxExportRfc822Controller
const Threads = Routes.ThreadsController
const Search = Routes.SearchController
const Send = Routes.SendController
const UndoSend = Routes.SendUndoController
const ScheduledSend = Routes.SendScheduledController
const ScheduledItem = Routes.SendScheduledIdController

const writeMboxFromFrameStream = (...args) => MboxUtils.writeMboxFromFrameStream(...args)

const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })

beforeAll(setupRoutes, 300_000)
afterAll(teardownRoutes)

describe('Go-owned external account boundary', () => {
  let account
  let imapState

  it('requires SESAME authentication for provider catalog and discovery', async () => {
    expect((await call(ProviderCatalog, 'GET', req())).unauthorized).toBe(true)
    const catalog = await call(ProviderCatalog, 'GET', req({ token: alice() }))
    expect(catalog.providers).toHaveLength(10)
    expect(catalog.providers.map(provider => provider.id)).toContain('imap')
    expect(catalog.providers.find(provider => provider.id === 'office365')).toMatchObject({ authentication: 'oauth', configured: false })
    expect(catalog.providers.find(provider => provider.id === 'gmail')).toMatchObject({ authentication: 'oauth', configured: false })
    expect((await call(ExternalAccounts, 'GET', req({ token: alice() }))).accounts).toEqual([])

    expect((await call(ProviderDiscovery, 'POST', req({ token: alice(), body: {} }))).error).toMatch(/address/)
    expect((await call(BeginMicrosoftOAuth, 'POST', req({ body: {} }))).unauthorized).toBe(true)
    expect((await call(BeginMicrosoftOAuth, 'POST', req({ token: alice(), body: {
      provider: 'microsoft', mailboxAddress: 'support@example.com', authIdentity: 'delegate@example.com',
      sharedMailboxAddress: 'support@example.com', label: 'Support', color: '#336699', order: 0,
    } }))).unavailable).toBe(true)
    expect((await call(CompleteMicrosoftOAuth, 'POST', req({ token: alice(), body: { state: 'state', code: 'code' } }))).unavailable).toBe(true)
    expect((await call(BeginGoogleOAuth, 'POST', req({ token: alice(), body: {
      provider: 'gmail', mailboxAddress: 'alice@gmail.com', authIdentity: 'alice@gmail.com', label: 'Gmail', color: '#EA4335', order: 0,
    } }))).unavailable).toBe(true)
    expect((await call(CompleteGoogleOAuth, 'POST', req({ token: alice(), body: { state: 'state', code: 'code' } }))).unavailable).toBe(true)
  })

  it('validates real IMAP and SMTP exchanges before persisting a redacted account', async () => {
	imapState = { seen: true, present: true, fresh: false, freshUID: 50, folder: 'INBOX', uid: 9, inboxNext: 11, archiveNext: 41 }
    const imap = await protocolServer(connection => imapSyncFixture(connection, imapState))
    const smtp = await protocolServer(connection => {
      connection.write('220 fixture ESMTP\r\n')
      connection.on('data', chunk => {
        const command = String(chunk)
        if (command.startsWith('EHLO ')) connection.write('250-fixture\r\n250 AUTH PLAIN\r\n')
        else if (command.startsWith('AUTH PLAIN ')) connection.write('235 authenticated\r\n')
        else if (command.startsWith('MAIL FROM')) connection.write('550 validation must never send mail\r\n')
        else if (command.startsWith('QUIT')) connection.write('221 bye\r\n')
      })
    })
    try {
      const input = {
        token: bob(),
        provider: 'imap', mailboxAddress: 'alice@remote.example',
        label: 'Remote work', color: '#336699', order: 1,
        credentials: { username: 'alice@remote.example', password: 'fixture-password' },
        imap: endpoint(imap.port), smtp: endpoint(smtp.port),
      }
      const validation = await call(ValidateExternalAccount, 'POST', req({ token: alice(), body: input }))
      expect(validation.capabilities).toMatchObject({ mailRead: true, send: true })
      expect((await call(ExternalAccounts, 'GET', req({ token: alice() }))).accounts).toEqual([])
      account = await call(ExternalAccounts, 'POST', req({ token: alice(), body: input }))
      if (account.error) throw new Error(`${account.error}\n${routeDiagnostics()}`)
      const routeApp = TestContainer.current
      await routeApp.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
      expect(await call(UserPreferences, 'PUT', req({
        token: alice(), body: { newMailNotificationExternalAccounts: [account.id] },
      }))).toMatchObject({ newMailNotificationExternalAccounts: [account.id] })
      expect((await call(UserPreferences, 'PUT', req({
        token: alice(), body: { newMailNotificationExternalAccounts: ['ext_BBBBBBBBBBBBBBBBBBBB'] },
      }))).forbidden).toBe(true)
      let diagnostics
      const deadline = Date.now() + 15_000
      while (Date.now() < deadline) {
        diagnostics = await call(ExternalAccountDiagnostics, 'GET', req({ token: alice(), paths: { id: account.id } }))
		if (diagnostics.state === 'idle' && diagnostics.sync?.cursorVersion >= 3) break
        await Bun.sleep(200)
      }
		expect(diagnostics).toMatchObject({ state: 'idle', sync: { accountId: account.id } })
		expect(diagnostics.sync.cursorVersion).toBeGreaterThanOrEqual(3)
      expect((await ExternalAccountBridge.claimNotifications('route-test', 10)).notifications).toEqual([])
      account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
      const folders = await call(ExternalAccountFolders, 'GET', req({ token: alice(), paths: { id: account.id } }))
      expect(folders.folders).toContainEqual(expect.objectContaining({ stableId: 'INBOX', role: 'inbox' }))
      const messages = await call(ExternalAccountMessages, 'GET', req({ token: alice(), paths: { id: account.id }, query: { folder: 'INBOX' } }))
      expect(messages.messages).toContainEqual(expect.objectContaining({
        stableId: 'INBOX:77:9', subject: 'Synced fixture',
      }))
      expect(messages.messages[0].snippet.startsWith('provider body')).toBe(true)
      expect(Buffer.byteLength(messages.messages[0].snippet)).toBeLessThanOrEqual(4096)
      expect(messages.messages.every(message =>
        !('body' in message) && !('rawRFC822Base64' in message))).toBe(true)
      expect((await call(ExternalAccountMessages, 'GET', req({ token: bob(), paths: { id: account.id }, query: {} }))).notFound).toBe(true)

      const unified = await call(Inbox, 'GET', req({ token: alice(), query: { folder: 'inbox' } }))
      if (!Array.isArray(unified)) throw new Error(`unified inbox failed: ${JSON.stringify(unified)}\n${routeDiagnostics()}`)
      const providerMessage = unified.find(message => message.providerAccountId === account.id)
      expect(providerMessage).toMatchObject({ external: true, mailboxLabel: 'Remote work', hasAttachments: true })
      expect(providerMessage.snippet.startsWith('provider body')).toBe(true)

      const accountInbox = await call(Inbox, 'GET', req({ token: alice(), query: { folder: 'inbox', account: account.id } }))
      expect(accountInbox).toHaveLength(1)
      expect(accountInbox.every(message => message.providerAccountId === account.id)).toBe(true)
      expect((await call(Inbox, 'GET', req({ token: bob(), query: { account: account.id } }))).notFound).toBe(true)
      expect((await call(Inbox, 'GET', req({ token: alice(), query: { account: account.id, domain: 'example.com' } }))).error).toMatch(/one mailbox/i)

      const accountSearch = await call(Search, 'GET', req({ token: alice(), query: { q: 'provider body', account: account.id } }))
      expect(accountSearch.map(message => message.id)).toEqual([providerMessage.id])

      const detail = await call(MessageDetail, 'GET', req({ token: alice(), paths: { id: providerMessage.id } }))
      expect(detail).toMatchObject({ external: true, subject: 'Synced fixture', hasRawSource: true })
      expect(detail.body.startsWith('provider body')).toBe(true)
      expect(Buffer.byteLength(detail.body)).toBeGreaterThan(1 << 20)
      expect(detail.attachments).toContainEqual(expect.objectContaining({ id: 'extatt.0', filename: 'note.txt' }))
      expect((await call(MessageDetail, 'GET', req({ token: bob(), paths: { id: providerMessage.id } }))).notFound).toBe(true)

      const attachment = await call(MessageAttachment, 'GET', req({
        token: alice(), paths: { id: providerMessage.id, attachment_id: 'extatt.0' },
      }))
      expect(attachment).toMatchObject({ filename: 'note.txt', contentType: 'text/plain' })
      expect(Buffer.from(attachment.contentBase64, 'base64').toString()).toBe('attachment bytes')
      expect((await call(MessageAttachment, 'GET', req({
        token: bob(), paths: { id: providerMessage.id, attachment_id: 'extatt.0' },
      }))).notFound).toBe(true)

      const source = await call(MessageSource, 'GET', req({ token: alice(), paths: { id: providerMessage.id } }))
      expect(source).toMatchObject({ contentType: 'message/rfc822', sha256: expect.stringMatching(/^[a-f0-9]{64}$/) })
      expect(Buffer.from(source.contentBase64, 'base64').toString()).toContain('Message-ID: <provider-fixture@example.net>')
      expect((await call(MessageSource, 'GET', req({ token: bob(), paths: { id: providerMessage.id } }))).notFound).toBe(true)

      const exportResponse = await call(ExportSources, 'GET', req({ token: alice(), query: { account: account.id, folder: 'inbox' } }))
      expect(exportResponse).toBeInstanceOf(Response)
      const exported = []
      const exportResult = await writeMboxFromFrameStream(exportResponse.body, chunk => exported.push(chunk))
      expect(exportResult.messages).toBe(1)
      expect(Buffer.concat(exported.map(chunk => Buffer.from(chunk))).toString()).toContain('Subject: Synced fixture')
      expect((await call(ExportSources, 'GET', req({ token: bob(), query: { account: account.id, folder: 'inbox' } }))).error).toBeTruthy()

      const conversations = await call(Threads, 'GET', req({ token: alice(), query: { emailId: providerMessage.id } }))
      expect(conversations).toHaveLength(1)
      expect(conversations[0].emails).toContainEqual(expect.objectContaining({
        id: providerMessage.id,
        attachments: [expect.objectContaining({ id: 'extatt.0', filename: 'note.txt' })],
      }))
      expect(conversations[0].emails[0].body.startsWith('provider body')).toBe(true)
      expect(await call(Threads, 'GET', req({ token: bob(), query: { emailId: providerMessage.id } }))).toEqual([])
      const accountThreads = await call(Threads, 'GET', req({ token: alice(), query: { folder: 'inbox', account: account.id } }))
      expect(accountThreads).toHaveLength(1)
      expect(accountThreads[0].emails.every(message => message.providerAccountId === account.id)).toBe(true)

      const hostedPeer = email({ id: 'mixed-provider-hosted', ownerEmail: 'alice@example.com', read: true })
      await routeApp.emails.put(hostedPeer)
      const providerAction = await call(MailActions, 'POST', req({
        token: alice(), body: {
          ids: [hostedPeer.id, providerMessage.id], patch: { read: false },
          clientRequestId: '88888888-8888-4888-8888-888888888888',
        },
      }))
      const providerActionId = providerAction.actionId
      expect(providerAction).toMatchObject({
        actionId: expect.any(String), status: 'applied', count: 2,
        emails: expect.arrayContaining([
          expect.objectContaining({ id: hostedPeer.id, read: false }),
          expect.objectContaining({ id: providerMessage.id, external: true, read: false }),
        ]),
      })
      expect((await routeApp.emails.findById(hostedPeer.id))[1].read).toBe(false)
      let projected
      const mutationDeadline = Date.now() + 30_000
      while (Date.now() < mutationDeadline) {
        projected = await call(ExternalAccountMessages, 'GET', req({ token: alice(), paths: { id: account.id }, query: { folder: 'INBOX' } }))
        if (projected.messages?.[0] && !projected.messages[0].flags.includes('\\Seen')) break
        await Bun.sleep(200)
      }
      expect(projected.messages[0].flags).not.toContain('\\Seen')
      const providerUndo = await call(UndoMailAction, 'POST', req({
        token: alice(), paths: { id: providerActionId },
      }))
      expect(providerUndo).toMatchObject({
        status: 'undone', count: 2,
        emails: expect.arrayContaining([
          expect.objectContaining({ id: hostedPeer.id, read: true }),
          expect.objectContaining({ id: providerMessage.id, read: true }),
        ]),
      })
      expect((await routeApp.emails.findById(hostedPeer.id))[1].read).toBe(true)
      const undoDeadline = Date.now() + 30_000
      while (Date.now() < undoDeadline) {
        projected = await call(ExternalAccountMessages, 'GET', req({ token: alice(), paths: { id: account.id }, query: { folder: 'INBOX' } }))
        if (projected.messages?.[0]?.flags.includes('\\Seen')) break
        await Bun.sleep(200)
      }
      expect(projected.messages[0].flags).toContain('\\Seen')

      const providerRedo = await call(RedoMailAction, 'POST', req({
        token: alice(), paths: { id: providerActionId },
      }))
      expect(providerRedo).toMatchObject({
        status: 'applied', count: 2,
        emails: expect.arrayContaining([
          expect.objectContaining({ id: hostedPeer.id, read: false }),
          expect.objectContaining({ id: providerMessage.id, read: false }),
        ]),
      })
      expect((await routeApp.emails.findById(hostedPeer.id))[1].read).toBe(false)
      const redoDeadline = Date.now() + 30_000
      while (Date.now() < redoDeadline) {
        projected = await call(ExternalAccountMessages, 'GET', req({ token: alice(), paths: { id: account.id }, query: { folder: 'INBOX' } }))
        if (projected.messages?.[0] && !projected.messages[0].flags.includes('\\Seen')) break
        await Bun.sleep(200)
      }
      expect(projected.messages[0].flags).not.toContain('\\Seen')

		const currentProviderStableId = projected.messages[0].stableId

		let queued
      let mutationInput
      for (let attempt = 0; attempt < 5; attempt++) {
        account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
        mutationInput = { revision: account.revision, idempotencyKey: 'route-action-1', kind: 'set_read', messageStableIds: [currentProviderStableId], value: true }
        queued = await call(ExternalAccountMutations, 'POST', req({ token: alice(), paths: { id: account.id }, body: mutationInput }))
        if (!queued.conflict) break
        await Bun.sleep(100)
      }
      if (queued.conflict) throw new Error(`could not obtain a stable account revision\n${routeDiagnostics()}`)
      const duplicate = await call(ExternalAccountMutations, 'POST', req({ token: alice(), paths: { id: account.id }, body: mutationInput }))
      expect(queued).toMatchObject({ state: 'queued', kind: 'set_read', messageStableIds: [currentProviderStableId] })
      expect(duplicate.id).toBe(queued.id)
	  expect(await call(ExternalAccountMutation, 'GET', req({
		token: alice(), paths: { id: account.id, mutation_id: queued.id },
	  }))).toMatchObject({ id: queued.id, kind: 'set_read' })
	  expect((await call(ExternalAccountMutation, 'GET', req({
		token: bob(), paths: { id: account.id, mutation_id: queued.id },
	  }))).notFound).toBe(true)
		const restoreDeadline = Date.now() + 45_000
		while (Date.now() < restoreDeadline) {
			projected = await call(ExternalAccountMessages, 'GET', req({ token: alice(), paths: { id: account.id }, query: { folder: 'INBOX' } }))
			if (projected.messages?.[0]?.flags.includes('\\Seen')) break
			await Bun.sleep(200)
		}
		expect(projected.messages[0].flags).toContain('\\Seen')

		// A later incremental unread Inbox import enters the scheduler-owned
		// outbox exactly once; initial account hydration above did not.
		imapState.fresh = true
		let notificationRetry
		for (let attempt = 0; attempt < 5; attempt++) {
			account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
			notificationRetry = await call(RetryExternalAccount, 'POST', req({ token: alice(), paths: { id: account.id }, body: { revision: account.revision } }))
			if (notificationRetry.scheduled) break
			await Bun.sleep(100)
		}
		expect(notificationRetry).toEqual({ scheduled: true })
		const notificationDeadline = Date.now() + 15_000
		let providerClaim
		while (Date.now() < notificationDeadline) {
			providerClaim = await ExternalAccountBridge.claimNotifications('route-test', 10)
			if (providerClaim.notifications?.length) break
			await Bun.sleep(200)
		}
		expect(providerClaim.notifications).toContainEqual(expect.objectContaining({
			accountId: account.id, ownerAddress: 'alice@example.com', stableId: 'INBOX:77:50', subject: 'Synced fixture',
		}))
		await ExternalAccountBridge.completeNotifications(providerClaim.token, providerClaim.notifications.map(notification => notification.id), true)

		// A later complete UID window must remove the local index and raw bucket
		// projection after the provider expunges the message.
		imapState.present = false
		let retry
		for (let attempt = 0; attempt < 5; attempt++) {
			account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
			retry = await call(RetryExternalAccount, 'POST', req({ token: alice(), paths: { id: account.id }, body: { revision: account.revision } }))
			if (retry.scheduled) break
			await Bun.sleep(100)
		}
		expect(retry).toEqual({ scheduled: true })
		let afterExpunge
		const expungeDeadline = Date.now() + 15_000
		while (Date.now() < expungeDeadline) {
			afterExpunge = await call(ExternalAccountMessages, 'GET', req({ token: alice(), paths: { id: account.id }, query: { folder: 'INBOX' } }))
			if (afterExpunge.messages?.length === 0) break
			await Bun.sleep(200)
		}
		expect(afterExpunge.messages).toEqual([])
		account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
    } finally {
      await Promise.all([imap.close(), smtp.close()])
    }

    expect(account.id).toMatch(/^ext_[A-Za-z0-9_-]{20,80}$/)
    expect(['idle', 'syncing']).toContain(account.state)
    expect(account.capabilities).toMatchObject({ mailRead: true, mailWrite: true, send: true })
    expect(JSON.stringify(account)).not.toContain('fixture-password')
    expect((await call(ExternalAccounts, 'GET', req({ token: alice() }))).accounts).toHaveLength(1)
    expect((await call(ExternalAccounts, 'GET', req({ token: bob() }))).accounts).toEqual([])
    expect((await call(ExternalAccount, 'GET', req({ token: bob(), paths: { id: account.id } }))).notFound).toBe(true)
    const diagnostics = await call(ExternalAccountDiagnostics, 'GET', req({ token: alice(), paths: { id: account.id } }))
	expect(diagnostics).toMatchObject({ state: 'idle', sync: { accountId: account.id, attempts: 0 } })
	expect(diagnostics.sync.cursorVersion).toBeGreaterThanOrEqual(3)
    expect((await call(ExternalAccountDiagnostics, 'GET', req({ token: bob(), paths: { id: account.id } }))).notFound).toBe(true)
  }, 90_000)

  it('journals connected-account SMTP delivery, undo, and scheduled cancellation', async () => {
    const smtpState = { deliveries: [] }
    const smtp = await protocolServer(connection => smtpSubmissionFixture(connection, smtpState), account.connection.smtp.port)
	const imap = await protocolServer(connection => imapSyncFixture(connection, imapState), account.connection.imap.port)
    try {
	  imapState.present = true
	  imapState.fresh = false
	  imapState.folder = 'INBOX'
	  imapState.uid = 60
	  imapState.inboxNext = 61
	  account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
	  await call(RetryExternalAccount, 'POST', req({ token: alice(), paths: { id: account.id }, body: { revision: account.revision } }))
	  let archiveSource
	  const sourceDeadline = Date.now() + 15_000
	  while (Date.now() < sourceDeadline) {
		const inbox = await call(Inbox, 'GET', req({ token: alice(), query: { folder: 'inbox', account: account.id } }))
		archiveSource = inbox.find(message => message.providerAccountId === account.id)
		if (archiveSource) break
		await Bun.sleep(200)
	  }
	  expect(archiveSource).toMatchObject({ external: true, folder: 'inbox' })
	  const sendAndArchive = await call(Send, 'POST', req({ token: alice(), body: {
		from: account.mailboxAddress, to: ['recipient@example.net'], subject: 'Undo external archive', text: 'cancel me',
		delayMs: 60_000, archive: true, archiveSourceId: archiveSource.id, idempotencyKey: 'external-route-send-archive-1',
	  }}))
	  const sendArchiveUndoId = sendAndArchive.undoId
	  expect(typeof sendArchiveUndoId).toBe('string')
	  expect(sendAndArchive).toMatchObject({ sent: true, queued: true, archived: true, archivedCount: 1, undoId: expect.stringMatching(/^extundo\./) })
	  const sendArchiveUndoRequest = req({ token: alice(), body: { undoId: sendArchiveUndoId } })
	  expect(sendArchiveUndoRequest.body).toEqual({ undoId: sendArchiveUndoId })
	  expect(await call(UndoSend, 'POST', sendArchiveUndoRequest)).toMatchObject({ undone: true, archiveRestored: 1 })

      const delayed = await call(Send, 'POST', req({ token: alice(), body: {
        from: account.mailboxAddress, to: ['recipient@example.net'], subject: 'Undo external', text: 'cancel me',
        delayMs: 60_000, idempotencyKey: 'external-route-undo-1',
      }}))
      const delayedUndoId = delayed.undoId
      expect(delayed).toMatchObject({ sent: true, queued: true, messageId: expect.any(String), undoId: expect.stringMatching(/^extundo\./) })
      expect(typeof delayedUndoId).toBe('string')
      const undoRequest = req({ token: alice(), body: { undoId: delayedUndoId } })
      expect(undoRequest.body).toEqual({ undoId: delayedUndoId })
      expect(await call(UndoSend, 'POST', undoRequest)).toEqual({ undone: true })

      const accepted = await call(Send, 'POST', req({ token: alice(), body: {
        from: account.mailboxAddress, to: ['recipient@example.net'], subject: 'External delivery', text: '.provider body',
        idempotencyKey: 'external-route-send-1',
      }}))
      const acceptedQueueId = accepted.queueId
      expect(accepted).toMatchObject({ queued: true, queueId: expect.stringMatching(/^sub_/), externalAccountId: account.id })
      let submission
      const deadline = Date.now() + 15_000
      while (Date.now() < deadline) {
        submission = await ExternalAccountBridge.submission(alice(), account.id, acceptedQueueId)
        if (submission.state === 'sent') break
        await Bun.sleep(200)
      }
      expect(submission.state).toBe('sent')
      expect(smtpState.deliveries).toHaveLength(1)
      expect(smtpState.deliveries[0]).toContain('..provider body')

	  const tracked = await call(Send, 'POST', req({ token: alice(), body: {
		from: account.mailboxAddress, to: ['tracked@example.net'], subject: 'External tracked', text: 'tracked link',
		html: '<p><a href="https://example.net/follow-up">Follow up</a></p>', openTracking: true, linkTracking: true,
		idempotencyKey: 'external-route-tracked-1',
	  }}))
	  const trackedQueueId = tracked.queueId
	  expect(tracked).toMatchObject({ queued: true, queueId: expect.stringMatching(/^sub_/), messageId: expect.any(String) })
	  const trackedDeadline = Date.now() + 15_000
	  while (Date.now() < trackedDeadline) {
		if ((await ExternalAccountBridge.submission(alice(), account.id, trackedQueueId)).state === 'sent') break
		await Bun.sleep(200)
	  }
	  expect(smtpState.deliveries).toHaveLength(2)
	  expect(smtpState.deliveries[1]).toContain('/track/click')
	  expect(smtpState.deliveries[1]).toContain('/track/open')
	  const trackedGroup = await call(Send, 'POST', req({ token: alice(), body: {
		from: account.mailboxAddress, to: ['one@example.net', 'two@example.net'], subject: 'Unsafe grouped tracking',
		html: '<p>group</p>', openTracking: true, idempotencyKey: 'external-route-tracked-group-1',
	  }}))
	  const trackedGroupQueueId = trackedGroup.queueId
	  expect(trackedGroup).toMatchObject({ queued: true, queueId: expect.stringMatching(/^sub_/), messageId: expect.any(String) })
	  const trackedGroupDeadline = Date.now() + 15_000
	  while (Date.now() < trackedGroupDeadline) {
		if ((await ExternalAccountBridge.submission(alice(), account.id, trackedGroupQueueId)).state === 'sent') break
		await Bun.sleep(200)
	  }
	  expect(smtpState.deliveries).toHaveLength(4)
	  expect(smtpState.deliveries[2]).toContain('/track/open')
	  expect(smtpState.deliveries[3]).toContain('/track/open')
	  expect(smtpState.deliveries[2]).not.toBe(smtpState.deliveries[3])
	  const delayedTrackedGroup = await call(Send, 'POST', req({ token: alice(), body: {
		from: account.mailboxAddress, to: ['undo-one@example.net', 'undo-two@example.net'], subject: 'Undo grouped tracking',
		html: '<p>undo group</p>', openTracking: true, delayMs: 60_000, idempotencyKey: 'external-route-tracked-group-undo-1',
	  }}))
	  const delayedTrackedGroupUndoId = delayedTrackedGroup.undoId
	  expect(delayedTrackedGroup).toMatchObject({ sent: true, queued: true, undoId: expect.stringMatching(/^extundo\./) })
	  expect(await call(UndoSend, 'POST', req({ token: alice(), body: { undoId: delayedTrackedGroupUndoId } }))).toEqual({ undone: true })
	  expect(smtpState.deliveries).toHaveLength(4)

      account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
      const sendAt = new Date(Date.now() + 60_000).toISOString()
      const scheduled = await call(Send, 'POST', req({ token: alice(), body: {
        from: account.mailboxAddress, to: ['later@example.net'], subject: 'External later', text: 'later',
		html: '<p><a href="https://example.net/later">later</a></p>', openTracking: true, linkTracking: true, sendAt,
        idempotencyKey: 'external-route-scheduled-1',
      }}))
      const scheduledId = scheduled.id
      expect(scheduled).toMatchObject({ scheduled: true, id: expect.any(String), externalAccountId: account.id })
      expect(await call(ScheduledSend, 'GET', req({ token: alice() }))).toContainEqual(expect.objectContaining({ id: scheduledId, from: account.mailboxAddress, status: 'scheduled' }))
	  const editedSendAt = new Date(Date.now() + 120_000).toISOString()
	  expect(await call(ScheduledItem, 'PUT', req({ token: alice(), paths: { id: scheduledId }, body: {
		subject: 'External later edited', text: 'replacement body', html: '<p><a href="https://example.net/edited">edited</a></p>',
		to: ['edited@example.net'], sendAt: editedSendAt,
	  } }))).toMatchObject({ scheduled: true, id: scheduledId, sendAt: editedSendAt })
	  expect(await call(ScheduledSend, 'GET', req({ token: alice() }))).toContainEqual(expect.objectContaining({
		id: scheduledId, subject: 'External later edited', to: ['edited@example.net'], sendAt: editedSendAt, status: 'scheduled',
	  }))
      expect(await call(ScheduledSend, 'DELETE', req({ token: alice(), body: { id: scheduledId } }))).toEqual({ cancelled: true })
	  const groupedScheduled = await call(Send, 'POST', req({ token: alice(), body: {
		from: account.mailboxAddress, to: ['later-one@example.net', 'later-two@example.net'], subject: 'External grouped later',
		html: '<p>later group</p>', openTracking: true, sendAt: new Date(Date.now() + 120_000).toISOString(),
		idempotencyKey: 'external-route-scheduled-group-1',
	  }}))
	  const groupedScheduledId = groupedScheduled.id
	  expect(groupedScheduled).toMatchObject({ scheduled: true, id: expect.any(String), externalAccountId: account.id })
	  expect(await call(ScheduledSend, 'GET', req({ token: alice() }))).toContainEqual(expect.objectContaining({ id: groupedScheduledId, status: 'scheduled' }))
	  expect(await call(ScheduledSend, 'DELETE', req({ token: alice(), body: { id: groupedScheduledId } }))).toEqual({ cancelled: true })
      expect(await call(ScheduledSend, 'GET', req({ token: alice() }))).toEqual([])
      account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
    } finally {
	  await Promise.all([imap.close(), smtp.close()])
    }
  }, 90_000)

  it('uses optimistic revisions for pause, resume, and removal', async () => {
		account = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
    const presented = await call(ExternalAccount, 'PATCH', req({
      token: alice(), paths: { id: account.id }, body: {
        action: 'update', revision: account.revision, label: 'Remote personal', color: '#aa55cc', order: 0,
      },
    }))
    expect(presented).toMatchObject({ label: 'Remote personal', color: '#aa55cc', order: 0, revision: account.revision + 1 })
    expect((await call(ExternalAccount, 'PATCH', req({
      token: alice(), paths: { id: account.id }, body: { action: 'update', revision: presented.revision, color: 'red' },
    }))).error).toMatch(/invalid/i)
    account = presented
    const paused = await call(ExternalAccount, 'PATCH', req({
      token: alice(), paths: { id: account.id }, body: { action: 'pause', revision: account.revision },
    }))
    expect(paused).toMatchObject({ state: 'paused', revision: account.revision + 1 })

    const stale = await call(ExternalAccount, 'PATCH', req({
      token: alice(), paths: { id: account.id }, body: { action: 'resume', revision: account.revision },
    }))
    expect(stale.conflict).toBe(true)

    const resumed = await call(ExternalAccount, 'PATCH', req({
      token: alice(), paths: { id: account.id }, body: { action: 'resume', revision: paused.revision },
    }))
    expect(resumed).toMatchObject({ state: 'ready', revision: paused.revision + 1 })

    const imap = await protocolServer(imapFixture, account.connection.imap.port)
    const smtp = await protocolServer(smtpFixture, account.connection.smtp.port)
    let reconnected
    try {
      reconnected = await call(ReconnectExternalAccount, 'POST', req({
        token: alice(), paths: { id: account.id }, body: {
          revision: resumed.revision,
          credentials: { username: 'alice@remote.example', password: 'replacement-password' },
        },
      }))
    } finally {
      await Promise.all([imap.close(), smtp.close()])
    }
    // The persisted endpoints are reused for reconnection. Start fresh
    // fixtures on those exact ports by retaining the original account ports.
    if (reconnected.error) throw new Error(`${reconnected.error}\n${routeDiagnostics()}`)
    expect(reconnected).toMatchObject({ state: 'ready', revision: resumed.revision + 1 })

    const removed = await call(ExternalAccount, 'DELETE', req({
      token: alice(), paths: { id: account.id }, body: { revision: reconnected.revision },
    }))
    expect(removed.deleting).toBe(true)
    let afterRemoval
    const removalDeadline = Date.now() + 15_000
    while (Date.now() < removalDeadline) {
      afterRemoval = await call(ExternalAccount, 'GET', req({ token: alice(), paths: { id: account.id } }))
      if (afterRemoval.notFound) break
      expect(afterRemoval.state).toBe('deleting')
      await Bun.sleep(200)
    }
    if (!afterRemoval.notFound) throw new Error(`external account deletion did not finish: ${JSON.stringify(afterRemoval)}\n${routeDiagnostics()}`)
    expect(afterRemoval.notFound).toBe(true)
    expect((await call(ExternalAccounts, 'GET', req({ token: alice() }))).accounts).toEqual([])
    expect((await call(ExternalAccountFolders, 'GET', req({ token: alice(), paths: { id: account.id } }))).notFound).toBe(true)
    expect((await call(ExternalAccountMessages, 'GET', req({ token: alice(), paths: { id: account.id }, query: { folder: 'INBOX' } }))).notFound).toBe(true)
  }, 30_000)
})

function endpoint(port) {
  return { host: '127.0.0.1', port, security: 'none', allowInsecureTLS: false }
}

function imapFixture(connection) {
  connection.write('* OK fixture ready\r\n')
  connection.on('data', chunk => {
    const command = String(chunk)
    if (command.startsWith('a001 LOGIN ')) connection.write('a001 OK authenticated\r\n')
    else if (command.startsWith('a002 SELECT ')) connection.write('* 0 EXISTS\r\na002 OK selected\r\n')
    else if (command.startsWith('a003 LOGOUT')) connection.write('* BYE\r\na003 OK logout\r\n')
  })
}

function imapSyncFixture(connection, state) {
  // Keep the retained source above the auth bridge's 1 MiB ceiling so this
  // route suite proves account-specific source/attachment framing end to end.
  const providerBody = `provider body\r\n${'x'.repeat((1 << 20) + 4096)}`
  const raw = [
    'Message-ID: <provider-fixture@example.net>',
    'From: Provider <sender@example.net>',
    'To: alice@remote.example',
    'Subject: Synced fixture',
    'Content-Type: multipart/mixed; boundary="fixture-boundary"',
    '',
    '--fixture-boundary',
    'Content-Type: text/plain; charset=utf-8',
    '',
    providerBody,
    '--fixture-boundary',
    'Content-Type: text/plain; name="note.txt"',
    'Content-Disposition: attachment; filename="note.txt"',
    '',
    'attachment bytes',
    '--fixture-boundary--',
    '',
  ].join('\r\n')
	let selected = 'INBOX'
  connection.write('* OK fixture ready\r\n')
  connection.on('data', chunk => {
    for (const command of String(chunk).split('\r\n').filter(Boolean)) {
      const tag = command.split(' ')[0]
      const operation = command.slice(tag.length + 1).toUpperCase()
      if (operation.startsWith('LOGIN ')) connection.write(`${tag} OK authenticated\r\n`)
	  else if (operation.startsWith('LIST ')) connection.write(`* LIST (\\HasNoChildren \\Inbox) "/" "INBOX"\r\n* LIST (\\HasNoChildren \\Archive) "/" "Archive"\r\n${tag} OK listed\r\n`)
	  else if (operation.startsWith('SELECT ')) {
		selected = operation.includes('"ARCHIVE"') ? 'Archive' : 'INBOX'
		const ids = state.present && state.folder === selected ? [state.uid] : []
		if (state.present && state.fresh && selected === 'INBOX') ids.push(state.freshUID)
		const validity = selected === 'Archive' ? 88 : 77
		const next = selected === 'Archive' ? state.archiveNext : Math.max(state.inboxNext, state.fresh ? state.freshUID + 1 : 10)
		connection.write(`* ${ids.length} EXISTS\r\n* OK [UIDVALIDITY ${validity}] valid\r\n* OK [UIDNEXT ${next}] next\r\n${tag} OK selected\r\n`)
	  }
	  else if (operation.startsWith('UID SEARCH ')) {
		const ids = state.present && state.folder === selected ? [state.uid] : []
		if (state.present && state.fresh && selected === 'INBOX') ids.push(state.freshUID)
		connection.write(`* SEARCH${ids.length ? ` ${ids.join(' ')}` : ''}\r\n${tag} OK searched\r\n`)
	  }
	  else if (operation.startsWith('UID FETCH 1:')) {
		const flags = state.seen ? '(\\Seen)' : '()'
		const rows = []
		if (state.present && state.folder === selected) rows.push(`* 1 FETCH (UID ${state.uid} FLAGS ${flags})\r\n`)
		if (state.present && state.fresh && selected === 'INBOX') rows.push(`* 2 FETCH (UID ${state.freshUID} FLAGS ())\r\n`)
		connection.write(`${rows.join('')}${tag} OK refreshed\r\n`)
	  }
      else if (operation.startsWith(`UID FETCH ${state.uid} `)) connection.write(`* 1 FETCH (UID ${state.uid} FLAGS (${state.seen ? '\\Seen' : ''}) RFC822.SIZE ${Buffer.byteLength(raw)} INTERNALDATE "21-Aug-2026 12:00:00 +0000" BODY[] {${Buffer.byteLength(raw)}}\r\n${raw})\r\n${tag} OK fetched\r\n`)
      else if (operation.startsWith(`UID FETCH ${state.freshUID} `)) connection.write(`* 2 FETCH (UID ${state.freshUID} FLAGS () RFC822.SIZE ${Buffer.byteLength(raw)} INTERNALDATE "22-Aug-2026 12:00:00 +0000" BODY[] {${Buffer.byteLength(raw)}}\r\n${raw})\r\n${tag} OK fetched\r\n`)
	  else if (operation.startsWith(`UID STORE ${state.uid} -FLAGS.SILENT (\\SEEN)`)) {
		state.seen = false
		connection.write(`${tag} OK stored\r\n`)
	  }
	  else if (operation.startsWith(`UID STORE ${state.uid} +FLAGS.SILENT (\\SEEN)`)) {
		state.seen = true
		connection.write(`${tag} OK stored\r\n`)
	  }
	  else if (operation.startsWith(`UID MOVE ${state.uid} `)) {
		const sourceUID = state.uid
		const target = operation.includes('"ARCHIVE"') ? 'Archive' : 'INBOX'
		const validity = target === 'Archive' ? 88 : 77
		state.uid = target === 'Archive' ? state.archiveNext++ : state.inboxNext++
		state.folder = target
		connection.write(`${tag} OK [COPYUID ${validity} ${sourceUID} ${state.uid}] moved\r\n`)
	  }
      else if (operation.startsWith('LOGOUT')) connection.write(`* BYE\r\n${tag} OK logout\r\n`)
    }
  })
}

function smtpFixture(connection) {
  connection.write('220 fixture ESMTP\r\n')
  connection.on('data', chunk => {
    const command = String(chunk)
    if (command.startsWith('EHLO ')) connection.write('250-fixture\r\n250 AUTH PLAIN\r\n')
    else if (command.startsWith('AUTH PLAIN ')) connection.write('235 authenticated\r\n')
    else if (command.startsWith('MAIL FROM')) connection.write('550 validation must never send mail\r\n')
    else if (command.startsWith('QUIT')) connection.write('221 bye\r\n')
  })
}

function smtpSubmissionFixture(connection, state) {
  let buffer = ''
  let inData = false
  let lines = []
  connection.write('220 fixture ESMTP\r\n')
  connection.on('data', chunk => {
    buffer += String(chunk)
    while (buffer.includes('\r\n')) {
      const boundary = buffer.indexOf('\r\n')
      const line = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      if (inData) {
        if (line === '.') {
          state.deliveries.push(lines.join('\r\n'))
          lines = []
          inData = false
          connection.write('250 accepted\r\n')
        } else {
          lines.push(line)
        }
        continue
      }
      const command = line.toUpperCase()
      if (command.startsWith('EHLO ')) connection.write('250-fixture\r\n250 AUTH PLAIN\r\n')
      else if (command.startsWith('AUTH PLAIN ')) connection.write('235 authenticated\r\n')
      else if (command.startsWith('MAIL FROM:')) connection.write('250 sender ok\r\n')
      else if (command.startsWith('RCPT TO:')) connection.write('250 recipient ok\r\n')
      else if (command === 'DATA') { inData = true; connection.write('354 continue\r\n') }
      else if (command === 'RSET') connection.write('250 reset\r\n')
      else if (command === 'QUIT') connection.write('221 bye\r\n')
    }
  })
}

async function protocolServer(onConnection, port = 0) {
  const sockets = new Set()
  const server = createServer(connection => {
    sockets.add(connection)
    connection.once('close', () => sockets.delete(connection))
    onConnection(connection)
  })
  await new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(port, '127.0.0.1', resolve)
  })
  const address = server.address()
  return {
    port: typeof address === 'object' && address ? address.port : 0,
    close: () => new Promise(resolve => {
      for (const socket of sockets) socket.destroy()
      server.close(resolve)
    }),
  }
}

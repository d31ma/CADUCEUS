import { randomBytes } from 'node:crypto'
import { McpServer } from '../../deps/npm/modelcontextprotocol-server.mjs'
import { z } from '../../deps/npm/zod.mjs'
import { FyloAccessContext } from '../persistence/FyloAccessContext.js'
import { canAccessEmail } from '../../features/mailbox/repositories/EmailsRepository.js'
import { parseDraftInput } from '../mailbox/draft-input.js'
import { MailValidation } from '../delivery/MailValidation.js'
import { createMessageId } from '../mailbox/threading.js'
import { ownedSenderAddresses, resolveOwnedSender } from '../users/sender-addresses.js'
import { cancelScheduled, findOwnedScheduled, presentScheduled } from '../delivery/scheduled-sends.js'
import { normalizeCategoryPath } from '../../features/mailbox/services/CategoryMappingService.js'

const MAX_BATCH_THREADS = 100
const MAX_RESULT_ITEMS = 100
const MAX_BODY_CHARS = 100_000
const ACCESS = {
  'read-only': new Set(['read']),
  'read-write': new Set(['read', 'write']),
  'read-write-send': new Set(['read', 'write', 'send']),
}

/**
 * Build one per-request MCP server. Every callback closes over a previously
 * authenticated, hash-backed capability and still applies exact-owner,
 * domain, folder-exclusion, access-level, and audit checks on every call.
 *
 * @param {import('../../app.js').App} app
 * @param {{ownerEmail:string,tokenId:string,primaryDomain:string,accessLevel:'read-only'|'read-write'|'read-write-send',allowedDomains:string[],excludedFolders:Record<string,string[]>}} principal
 */
export function createMailboxMcpServer(app, principal) {
  const server = new McpServer({ name: 'caduceus-mail', version: '1.0.0' })
  const register = (name, category, description, inputSchema, callback) => {
    server.registerTool(name, {
      description,
      inputSchema,
      annotations: {
        readOnlyHint: category === 'read',
        destructiveHint: ['trash_threads', 'cancel_scheduled_send'].includes(name),
        idempotentHint: !['create_draft', 'reply_to_thread', 'send_draft', 'schedule_send'].includes(name),
      },
    }, /** @type {any} */ (async input => await executeTool(app, principal, name, category, input, callback)))
  }

  register('list_accounts', 'read', 'List mail accounts allowed by this MCP capability.', z.object({}), async () => {
    const [, user] = await app.users.findByEmail(principal.ownerEmail)
    return principal.allowedDomains.map(domain => ({
      id: domain,
      email: principal.ownerEmail,
      name: user?.mailboxNames?.[domain] || domain,
      provider: 'caduceus',
      usesLabels: true,
    }))
  })

  register('list_folders', 'read', 'List allowed folders and stable system roles.', z.object({
    accountId: z.string().max(253).optional(),
  }), async ({ accountId }) => {
    const domains = scopedDomains(principal, accountId)
    const accounts = await app.categoryMappings.listAccounts(principal.ownerEmail, domains) ?? []
    return accounts.map(account => ({
      accountId: account.domain,
      folders: account.categories
        .filter(category => category.kind === 'folder')
        .filter(folder => !isFolderExcluded(principal, account.domain, folder.path))
        .map(folder => ({ id: folder.path, name: folder.path, role: folder.role || null })),
      labels: account.categories
        .filter(category => category.kind === 'label')
        .map(label => ({ id: label.path, name: label.path })),
    }))
  })

  register('search_mail', 'read', 'Search exact-owner mail across allowed accounts and folders.', z.object({
    query: z.string().max(1_000).default(''),
    accountId: z.string().max(253).optional(),
    folder: z.string().max(60).optional(),
    limit: z.number().int().min(1).max(MAX_RESULT_ITEMS).default(50),
  }), async ({ query, accountId, folder, limit }) => {
    const domains = scopedDomains(principal, accountId)
    const category = normalizeRequestedFolder(folder)
    const emails = await app.emails.list(domains, {
      folder: category, query, ownerEmail: principal.ownerEmail, limit: Math.min(limit * 2, 200),
    })
    return (await presentMessages(app, principal, emails)).slice(0, limit)
  })

  register('list_threads', 'read', 'List exact-owner conversations in an allowed scope.', z.object({
    accountId: z.string().max(253).optional(),
    folder: z.string().max(60).optional(),
    unread: z.boolean().optional(),
    starred: z.boolean().optional(),
    offset: z.number().int().min(0).max(100_000).default(0),
    limit: z.number().int().min(1).max(MAX_RESULT_ITEMS).default(50),
  }), async ({ accountId, folder, unread, starred, offset, limit }) => {
    const threads = await allowedThreads(app, principal, scopedDomains(principal, accountId), folder)
    return threads.filter(thread => unread == null || thread.emails.some(email => !email.read) === unread)
      .filter(thread => starred == null || thread.emails.some(email => email.starred) === starred)
      .slice(offset, offset + limit).map(presentThreadSummary)
  })

  register('get_thread', 'read', 'Read one allowed conversation and its messages.', z.object({
    threadId: z.string().min(1).max(500),
  }), async ({ threadId }) => {
    const thread = await findAllowedThread(app, principal, threadId)
    if (!thread) throw new Error('Thread not found')
    return { ...presentThreadSummary(thread), messages: await presentMessages(app, principal, thread.emails) }
  })

  register('get_message', 'read', 'Read one exact-owner message.', z.object({
    messageId: z.string().min(1).max(500),
  }), async ({ messageId }) => {
    const message = await findAllowedMessage(app, principal, messageId)
    if (!message) throw new Error('Message not found')
    return (await presentMessages(app, principal, [message]))[0]
  })

  register('get_message_tracking', 'read', 'Return local open and click activity for an owned sent message.', z.object({
    messageId: z.string().min(1).max(500),
  }), async ({ messageId }) => {
    const message = await findAllowedMessage(app, principal, messageId)
    if (!message) throw new Error('Message not found')
    return trackingView(message)
  })

  register('list_tracked_messages', 'read', 'List owned messages with local open or link tracking.', z.object({
    accountId: z.string().max(253).optional(),
    limit: z.number().int().min(1).max(MAX_RESULT_ITEMS).default(50),
  }), async ({ accountId, limit }) => {
    const emails = await app.emails.list(scopedDomains(principal, accountId), {
      folder: 'all', ownerEmail: principal.ownerEmail,
    })
    return emails.filter(email => !isExcluded(principal, email) && (email.openTracking || email.linkTracking))
      .slice(0, limit).map(email => ({
        id: email.id, subject: email.subject, receivedAt: email.receivedAt, ...trackingView(email),
      }))
  })

  register('get_top_links', 'read', 'Aggregate clicked links from locally tracked sent messages.', z.object({
    accountId: z.string().max(253).optional(),
    limit: z.number().int().min(1).max(50).default(20),
  }), async ({ accountId, limit }) => {
    const emails = await app.emails.list(scopedDomains(principal, accountId), {
      folder: 'all', ownerEmail: principal.ownerEmail,
    })
    const links = new Map()
    for (const email of emails.filter(item => !isExcluded(principal, item))) {
      for (const click of email.clicks || []) {
        const url = String(click.url || '')
        if (!url) continue
        const current = links.get(url) || { url, title: String(click.title || ''), clicks: 0 }
        current.clicks++
        links.set(url, current)
      }
    }
    return [...links.values()].sort((a, b) => b.clicks - a.clicks).slice(0, limit)
  })

  register('list_scheduled_sends', 'read', 'List this mailbox’s pending scheduled sends.', z.object({}), async () => {
    const records = await app.scheduled.listForOwner(principal.ownerEmail)
    const result = []
    for (const entry of Object.values(records)) {
      const record = await findOwnedScheduled(app, entry.id, principal.ownerEmail)
      if (record && principal.allowedDomains.includes(String(entry.sender).split('@')[1] || '')) {
        result.push(await presentScheduled(app, record))
      }
    }
    return result.sort((a, b) => a.sendAt.localeCompare(b.sendAt)).slice(0, MAX_RESULT_ITEMS)
  })

  const draftSchema = z.object({
    from: z.string().email().optional(),
    to: z.array(z.string().email()).max(100).default([]),
    cc: z.array(z.string().email()).max(100).default([]),
    bcc: z.array(z.string().email()).max(100).default([]),
    subject: z.string().max(500).default(''),
    text: z.string().max(5_000_000).default(''),
    html: z.string().max(5_000_000).default(''),
  })
  register('create_draft', 'write', 'Create a durable draft without sending it.', draftSchema, async input => {
    return await createDraft(app, principal, input)
  })

  register('reply_to_thread', 'write', 'Create a durable reply draft for an allowed conversation.', z.object({
    threadId: z.string().min(1).max(500),
    replyAll: z.boolean().default(false),
    text: z.string().max(5_000_000).default(''),
    html: z.string().max(5_000_000).default(''),
  }), async ({ threadId, replyAll, text, html }) => {
    const thread = await findAllowedThread(app, principal, threadId)
    if (!thread) throw new Error('Thread not found')
    const latest = thread.latestEmail
    const owned = new Set(await ownedSenderAddresses(app.users, principal.ownerEmail))
    const candidates = [latest.sender, ...(replyAll ? [...(latest.to || []), ...(latest.cc || [])] : [])]
    const to = [...new Set(candidates.map(value => String(value).toLowerCase()).filter(value => value.includes('@') && !owned.has(value)))]
    if (to.length === 0) throw new Error('Thread has no reply recipient')
    const references = [...new Set([...(latest.references || []), latest.inReplyTo, latest.rfcMessageId].filter(Boolean))]
    return await createDraft(app, principal, {
      to, cc: [], bcc: [], subject: /^re:/i.test(latest.subject) ? latest.subject : `Re: ${latest.subject}`,
      text, html, inReplyTo: latest.rfcMessageId, references, sourceEmailId: latest.id,
    })
  })

  const batchSchema = z.object({ threadIds: z.array(z.string().min(1).max(500)).min(1).max(MAX_BATCH_THREADS) })
  register('archive_threads', 'write', 'Move complete allowed conversations to each account’s Archive folder.', batchSchema,
    async ({ threadIds }) => await moveThreadsToRole(app, principal, threadIds, 'archive'))
  register('trash_threads', 'write', 'Move complete allowed conversations to each account’s Trash folder.', batchSchema,
    async ({ threadIds }) => await moveThreadsToRole(app, principal, threadIds, 'trash'))
  register('move_to_folder', 'write', 'Move complete allowed conversations to one physical folder.', z.object({
    threadIds: z.array(z.string().min(1).max(500)).min(1).max(MAX_BATCH_THREADS),
    accountId: z.string().min(1).max(253),
    folder: z.string().min(1).max(60),
  }), async ({ threadIds, accountId, folder }) => {
    const domain = scopedDomains(principal, accountId)[0]
    const destination = normalizeCategoryPath(folder)
    if (!destination) throw new Error('Destination folder must be a safe folder path')
    if (isFolderExcluded(principal, domain, destination)) throw new Error('Destination folder is excluded')
    const threads = await resolveAllThreads(app, principal, threadIds)
    if (threads.some(thread => thread.emails.some(email => email.domain !== domain))) {
      throw new Error('Every selected thread must belong to the destination account')
    }
    return await patchThreads(app, threads, () => ({ folder: destination }))
  })
  register('set_labels', 'write', 'Set provider labels when the selected account supports labels.', z.object({
    threadIds: z.array(z.string().min(1).max(500)).min(1).max(MAX_BATCH_THREADS),
    labels: z.array(z.string().min(1).max(100)).max(100),
  }), async ({ threadIds, labels }) => await setThreadLabels(app, principal, threadIds, labels))
  register('set_unread', 'write', 'Mark complete allowed conversations read or unread.', z.object({
    threadIds: z.array(z.string().min(1).max(500)).min(1).max(MAX_BATCH_THREADS),
    unread: z.boolean(),
  }), async ({ threadIds, unread }) => await patchThreads(app, await resolveAllThreads(app, principal, threadIds), () => ({ read: !unread })))
  register('set_starred', 'write', 'Star or unstar complete allowed conversations.', z.object({
    threadIds: z.array(z.string().min(1).max(500)).min(1).max(MAX_BATCH_THREADS),
    starred: z.boolean(),
  }), async ({ threadIds, starred }) => await patchThreads(app, await resolveAllThreads(app, principal, threadIds), () => ({ starred })))
  register('cancel_scheduled_send', 'write', 'Cancel one exact-owner scheduled send.', z.object({
    id: z.string().min(1).max(500),
  }), async ({ id }) => {
    const record = await findOwnedScheduled(app, id, principal.ownerEmail)
    if (!record) throw new Error('Scheduled send not found')
    const result = await cancelScheduled(app, record, principal.ownerEmail)
    if (result.conflict) throw new Error('Scheduled send is currently being delivered')
    return { id, cancelled: true }
  })

  register('send_draft', 'send', 'Send one exact-owner draft immediately.', z.object({
    draftId: z.string().min(1).max(500),
  }), async ({ draftId }) => await deliverDraft(app, principal, draftId))
  register('schedule_send', 'send', 'Schedule one exact-owner draft for future delivery.', z.object({
    draftId: z.string().min(1).max(500),
    sendAt: z.string().datetime(),
  }), async ({ draftId, sendAt }) => {
    const at = Date.parse(sendAt)
    if (!Number.isFinite(at) || at <= Date.now()) throw new Error('sendAt must be in the future')
    return await deliverDraft(app, principal, draftId, at)
  })

  return server
}

async function executeTool(app, principal, name, category, input, callback) {
  const started = performance.now()
  let status = 'ok'
  let resultCount = 0
  try {
    if (!ACCESS[principal.accessLevel]?.has(category)) {
      status = 'denied'
      throw new Error(`MCP access level ${principal.accessLevel} does not permit ${category} tools`)
    }
    const result = await FyloAccessContext.run(
      { email: principal.ownerEmail, domains: principal.allowedDomains },
      () => callback(input),
    )
    const structuredContent = asObject(result)
    resultCount = countResult(result)
    return {
      content: [{ type: 'text', text: JSON.stringify(structuredContent, null, 2) }],
      structuredContent,
    }
  } catch (error) {
    if (status !== 'denied') status = 'error'
    const message = cleanError(error)
    return { isError: true, content: [{ type: 'text', text: message }] }
  } finally {
    const durationMs = Math.max(0, Math.round(performance.now() - started))
    await FyloAccessContext.runForDomain(principal.primaryDomain, () => app.mcpAuditEvents.put({
      id: `mcp-audit-${randomBytes(16).toString('base64url')}`,
      ownerEmail: principal.ownerEmail,
      domain: principal.primaryDomain,
      tokenId: principal.tokenId,
      toolName: name,
      category,
      status,
      parameterKeysJson: JSON.stringify(Object.keys(input || {}).filter(key => /^[a-z][a-zA-Z0-9]{0,63}$/.test(key)).sort()),
      resultCount,
      durationMs,
      createdAt: new Date().toISOString(),
    })).catch(error => console.error('[mcp] audit write failed:', cleanError(error)))
  }
}

function scopedDomains(principal, requested) {
  if (!requested) return [...principal.allowedDomains]
  const domain = String(requested).trim().toLowerCase()
  if (!principal.allowedDomains.includes(domain)) throw new Error('Account is not allowed by this MCP capability')
  return [domain]
}

function isFolderExcluded(principal, domain, folder) {
  return (principal.excludedFolders[domain] || []).includes(String(folder || '').toLowerCase())
}

function isExcluded(principal, email) { return isFolderExcluded(principal, email.domain, email.folder || 'inbox') }

async function allowedThreads(app, principal, domains = principal.allowedDomains, folder = 'all') {
  const category = normalizeRequestedFolder(folder)
  const threads = await app.threads.list(domains, category, principal.ownerEmail)
  return threads.filter(thread => !thread.emails.some(email => isExcluded(principal, email)))
}

function normalizeRequestedFolder(folder) {
  if (folder == null || folder === '' || String(folder).toLowerCase() === 'all') return 'all'
  const path = normalizeCategoryPath(folder)
  if (!path) throw new Error('folder must be a safe folder path')
  return path
}

async function findAllowedThread(app, principal, threadId) {
  return (await allowedThreads(app, principal)).find(thread => thread.id === threadId) || null
}

async function resolveAllThreads(app, principal, threadIds) {
  const ids = [...new Set(threadIds)]
  if (ids.length !== threadIds.length) throw new Error('threadIds must be unique')
  const all = await allowedThreads(app, principal)
  const threads = ids.map(id => all.find(thread => thread.id === id))
  if (threads.some(thread => !thread)) throw new Error('Every selected thread must be accessible')
  return threads
}

async function findAllowedMessage(app, principal, id) {
  const [, email] = await app.emails.findById(id)
  if (!email || !principal.allowedDomains.includes(email.domain) || isExcluded(principal, email)
      || !canAccessEmail(email, principal.ownerEmail)) return null
  return email
}

async function presentMessages(app, principal, messages) {
  const result = []
  for (const email of messages) {
    if (isExcluded(principal, email) || !canAccessEmail(email, principal.ownerEmail)) continue
    const attachments = await app.attachments.listSummaries(email.id)
    result.push({
      id: email.id,
      threadId: email.threadId || '',
      accountId: email.domain,
      folder: email.folder || 'inbox',
      from: email.headerFrom || email.sender,
      to: email.to?.length ? email.to : [email.recipient].filter(Boolean),
      cc: email.cc || [],
      subject: email.subject,
      text: String(email.body || '').slice(0, MAX_BODY_CHARS),
      html: String(email.htmlBody || '').slice(0, MAX_BODY_CHARS),
      read: email.read,
      starred: email.starred,
      receivedAt: email.receivedAt,
      attachments: attachments.map(item => ({
        id: item.id, filename: item.filename, contentType: item.contentType, size: item.size,
      })),
    })
  }
  return result
}

function presentThreadSummary(thread) {
  return {
    id: thread.id,
    subject: thread.subject,
    messageCount: thread.count,
    participants: thread.participants,
    unread: thread.emails.some(email => !email.read),
    starred: thread.emails.some(email => email.starred),
    latestAt: thread.latestEmail.receivedAt,
    accountIds: [...new Set(thread.emails.map(email => email.domain))],
  }
}

function trackingView(email) {
  return {
    openTracking: email.openTracking === true,
    linkTracking: email.linkTracking === true,
    openCount: Number(email.openCount || email.opens?.length || 0),
    clickCount: Number(email.clickCount || email.clicks?.length || 0),
    opens: (email.opens || []).slice(-100),
    clicks: (email.clicks || []).slice(-100),
  }
}

async function createDraft(app, principal, source) {
  const input = parseDraftInput({
    clientId: `mcp-${randomBytes(16).toString('base64url')}`,
    to: [], cc: [], bcc: [], subject: '', text: '', html: '',
    ...source,
  })
  if (input instanceof Error) throw input
  const addresses = await ownedSenderAddresses(app.users, principal.ownerEmail)
  const sender = resolveOwnedSender(input.from, addresses, principal.ownerEmail)
  if (!sender) throw new Error('From address is not owned by this mailbox')
  const senderDomain = sender.split('@')[1] || ''
  if (!principal.allowedDomains.includes(senderDomain)) throw new Error('Draft account is outside this MCP scope')
  input.from = sender
  const result = await app.drafts.create(principal.ownerEmail, input)
  return { draftId: result.draft.id, version: result.draft.draftVersion, recovered: result.recovered }
}

async function moveThreadsToRole(app, principal, threadIds, role) {
  const threads = await resolveAllThreads(app, principal, threadIds)
  return await patchThreads(app, threads, async email => ({
    folder: await app.categoryMappings.pathFor(principal.ownerEmail, email.domain, role),
  }))
}

async function patchThreads(app, threads, patcher) {
  let updatedMessages = 0
  for (const thread of threads) {
    for (const email of thread.emails) {
      const [docId, current] = await app.emails.findById(email.id)
      if (!docId || !current) throw new Error('Thread changed during the operation')
      await app.emails.update(docId, await patcher(current))
      updatedMessages++
    }
  }
  return { updatedThreads: threads.length, updatedMessages }
}

async function setThreadLabels(app, principal, threadIds, labels) {
  const normalized = labels.map(label => normalizeCategoryPath(label))
  if (normalized.some(label => !label)) throw new Error('Every label must be a safe label path')
  if (new Set(normalized).size !== normalized.length) throw new Error('labels must be unique')
  const threads = await resolveAllThreads(app, principal, threadIds)
  const domains = new Set(threads.flatMap(thread => thread.emails.map(email => email.domain)))
  if (domains.size !== 1) throw new Error('Every selected thread must belong to one account')
  return await patchThreads(app, threads, () => ({ labels: normalized }))
}

async function deliverDraft(app, principal, draftId, sendAt) {
  const draft = await app.drafts.get(draftId, principal.ownerEmail, principal.allowedDomains)
  if (!draft) throw new Error('Draft not found')
  const parsed = MailValidation.validateSendRequest({
    from: draft.sender, to: draft.to || [], cc: draft.cc || [], bcc: draft.bcc || [],
    ...(draft.replyTo?.length ? { replyTo: draft.replyTo } : {}),
    subject: draft.subject, text: draft.body, html: draft.htmlBody,
    inReplyTo: draft.inReplyTo, references: draft.references,
  })
  if (!parsed.ok) throw new Error(parsed.error)
  const parsedAttachments = MailValidation.validateSendAttachments(draft.attachments || [])
  if (!parsedAttachments.ok) throw new Error(parsedAttachments.error)
  const addresses = await ownedSenderAddresses(app.users, principal.ownerEmail)
  const sender = resolveOwnedSender(parsed.value.from, addresses, principal.ownerEmail)
  if (!sender || !principal.allowedDomains.includes(sender.split('@')[1] || '')) throw new Error('Draft sender is outside this MCP scope')
  const limit = await app.rateLimit.check(['mcp-send', principal.ownerEmail, principal.tokenId], 60, 60 * 60 * 1000)
  if (!limit.allowed) throw new Error(`Too many send attempts; retry in ${limit.retryAfterSeconds} seconds`)
  const suppressed = await app.suppressed.asSet()
  const blocked = [...parsed.value.to, ...(parsed.value.cc || []), ...(parsed.value.bcc || [])].filter(address => suppressed.has(address))
  if (blocked.length) throw new Error('One or more recipients are suppressed')
  const request = {
    ...parsed.value,
    messageId: createMessageId(sender.split('@')[1] || 'localhost'),
    ...(parsedAttachments.value.length ? { attachments: parsedAttachments.value } : {}),
  }
  const result = sendAt == null
    ? await app.sendService.sendNow(principal.ownerEmail, sender, request)
    : await app.sendService.schedule(principal.ownerEmail, sender, request, sendAt)
  await app.drafts.discard(draftId, principal.ownerEmail, principal.allowedDomains)
  return result
}

function asObject(result) {
  if (Array.isArray(result)) return { items: result }
  if (result && typeof result === 'object') return result
  return { value: result ?? null }
}

function countResult(result) {
  if (Array.isArray(result)) return result.length
  if (result && typeof result === 'object') {
    if (Array.isArray(result.items)) return result.items.length
    if (Number.isFinite(result.updatedThreads)) return Number(result.updatedThreads)
  }
  return result == null ? 0 : 1
}

function cleanError(error) {
  return String(error instanceof Error ? error.message : error || 'Tool failed').replace(/[\r\n]+/g, ' ').slice(0, 500)
}

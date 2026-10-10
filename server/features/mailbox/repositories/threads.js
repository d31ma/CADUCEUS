/** @typedef {import('../../../types').StoredEmail} StoredEmail */
/** @typedef {import('./emails.js').EmailsRepository} EmailsRepository */
/** @typedef {StoredEmail & { external?: boolean, providerAccountId?: string, mailboxAddress?: string }} ThreadMessage */
import { messageParticipants, personalLevelForThread } from '../services/personal-level.js'

/**
 * @typedef {object} Thread
 * @property {string} id
 * @property {string} subject
 * @property {number} count
 * @property {ThreadMessage} latestEmail
 * @property {ThreadMessage[]} emails
 * @property {string[]} participants
 * @property {0 | 1 | 2} personalLevel
 */

export class ThreadsRepository {
  /** @param {EmailsRepository} emails */
  constructor(emails) {
    this.emails = emails
  }

  /**
   * Group messages by RFC Message-ID relationships, newest thread first.
   * Historical rows that predate retained headers use an isolated subject
   * fallback; a modern message with the same subject is never merged by name.
   * @param {string[]} allowedDomains
   * @param {string} [folder]
   * @param {string} [ownerEmail]
   * @param {Record<string, string>} [folderByDomain]
   * @returns {Promise<Thread[]>}
   */
  async list(allowedDomains, folder, ownerEmail, folderByDomain) {
    const emails = await this.emails.list(allowedDomains, { folder: 'all', ownerEmail })
    const threads = groupMessagesIntoThreads(emails, ownerEmail)
    if (!folder || folder === 'all') return threads
    return threads.filter(thread => thread.emails.some(email => (
      email.folder || 'inbox'
    ) === (folderByDomain?.[email.domain] || folder)))
  }

  /**
   * Resolve the authorized conversation containing one selected message.
   * Unlike the ordinary folder list, printing and other conversation-level
   * actions must include messages that have moved to Sent, Archive, or a
   * custom folder. Domain filtering remains inside the email repository.
   *
   * @param {string[]} allowedDomains
   * @param {string} emailId
   * @param {string} [ownerEmail]
   * @returns {Promise<Thread | null>}
   */
  async findForEmail(allowedDomains, emailId, ownerEmail) {
    if (!emailId) return null
    const threads = await this.list(allowedDomains, 'all', ownerEmail)
    return threads.find(thread => thread.emails.some(email => email.id === emailId)) ?? null
  }
}

/**
 * Group already-authorized hosted and connected-account messages. Provider
 * account identity—not only mailbox domain—is part of every edge key so two
 * separate accounts can never become one conversation by presenting the same
 * RFC Message-ID or provider thread identifier.
 *
 * @param {ThreadMessage[]} emails
 * @param {string} [ownerEmail]
 * @returns {Thread[]}
 */
export function groupMessagesIntoThreads(emails, ownerEmail) {
  const components = connectedComponents(emails)
  const threads = []
  for (const threadEmails of components) {
    threadEmails.sort((a, b) => a.receivedAt.localeCompare(b.receivedAt))
    const latestEmail = threadEmails[threadEmails.length - 1]
    const persistedId = threadEmails.find(email => email.threadId)?.threadId
    const scope = messageScope(threadEmails[0])
    const id = persistedId ?? `legacy:${scope}:${baseSubject(threadEmails[0].subject).toLowerCase()}`
    const participants = [...new Set(threadEmails.flatMap(messageParticipants))]
    const mailbox = threadEmails[0].providerAccountId ? threadEmails[0].mailboxAddress : ownerEmail
    const personalLevel = personalLevelForThread(threadEmails, mailbox)
    threads.push({
      id,
      subject: baseSubject(threadEmails[0].subject),
      count: threadEmails.length,
      latestEmail,
      emails: threadEmails,
      participants,
      personalLevel,
    })
  }
  threads.sort((a, b) => b.latestEmail.receivedAt.localeCompare(a.latestEmail.receivedAt))
  return threads
}

/**
 * Build tenant-isolated connected components from persisted Message-ID edges.
 * Stored thread IDs also connect generated local notices and allow a reply
 * chain to survive incomplete References headers.
 *
 * @param {ThreadMessage[]} emails
 * @returns {ThreadMessage[][]}
 */
function connectedComponents(emails) {
  const parent = emails.map((_, index) => index)
  const find = index => parent[index] === index ? index : (parent[index] = find(parent[index]))
  const union = (left, right) => {
    const a = find(left); const b = find(right)
    if (a !== b) parent[b] = a
  }
  const byMessageId = new Map()
  const byThreadId = new Map()
  const legacyBySubject = new Map()

  for (let index = 0; index < emails.length; index++) {
    const email = emails[index]
    const scope = `${messageScope(email)}\0`
    if (email.rfcMessageId) connectIndex(byMessageId, `${scope}${email.rfcMessageId}`, index, union)
    if (email.threadId) connectIndex(byThreadId, `${scope}${email.threadId}`, index, union)
    const hasRfcMetadata = Boolean(email.rfcMessageId || email.inReplyTo || email.references?.length || email.threadId)
    if (!hasRfcMetadata) connectIndex(legacyBySubject, `${scope}${baseSubject(email.subject).toLowerCase()}`, index, union)
  }

  for (let index = 0; index < emails.length; index++) {
    const email = emails[index]
    const scope = `${messageScope(email)}\0`
    for (const referencedId of [email.inReplyTo, ...(email.references || [])]) {
      if (!referencedId) continue
      const linked = byMessageId.get(`${scope}${referencedId}`)
      if (linked != null) union(index, linked)
    }
  }

  /** @type {Map<number, ThreadMessage[]>} */
  const groups = new Map()
  for (let index = 0; index < emails.length; index++) {
    const root = find(index)
    const group = groups.get(root) ?? []
    group.push(emails[index])
    groups.set(root, group)
  }
  return [...groups.values()]
}

/** @param {ThreadMessage} email */
function messageScope(email) {
  const accountId = String(email.providerAccountId || '').trim()
  return accountId ? `external:${accountId}` : `hosted:${String(email.domain || '').toLowerCase()}`
}

/** @param {Map<string, number>} index @param {string} key @param {number} current @param {(a:number,b:number)=>void} union */
function connectIndex(index, key, current, union) {
  const existing = index.get(key)
  if (existing == null) index.set(key, current)
  else union(current, existing)
}

/** @param {string} subject @returns {string} */
function baseSubject(subject) {
  const stripped = subject.replace(/^(?:(?:re|fw|fwd):\s*)+/i, '').trim()
  return stripped || subject.trim()
}

import { PersonalLevel } from './PersonalLevel.js'

export class ThreadsRecords {
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
  static groupMessagesIntoThreads(emails, ownerEmail) {
    const components = ThreadsRecords.connectedComponents(emails)
    const threads = []
    for (const threadEmails of components) {
      threadEmails.sort((a, b) => a.receivedAt.localeCompare(b.receivedAt))
      const latestEmail = threadEmails[threadEmails.length - 1]
      const persistedId = threadEmails.find(email => email.threadId)?.threadId
      const scope = ThreadsRecords.messageScope(threadEmails[0])
      const id = persistedId ?? `legacy:${scope}:${ThreadsRecords.baseSubject(threadEmails[0].subject).toLowerCase()}`
      const participants = [...new Set(threadEmails.flatMap(PersonalLevel.messageParticipants))]
      const mailbox = threadEmails[0].providerAccountId ? threadEmails[0].mailboxAddress : ownerEmail
      const personalLevel = PersonalLevel.personalLevelForThread(threadEmails, mailbox)
      threads.push({
        id,
        subject: ThreadsRecords.baseSubject(threadEmails[0].subject),
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
  static connectedComponents(emails) {
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
      const scope = `${ThreadsRecords.messageScope(email)}\0`
      if (email.rfcMessageId) ThreadsRecords.connectIndex(byMessageId, `${scope}${email.rfcMessageId}`, index, union)
      if (email.threadId) ThreadsRecords.connectIndex(byThreadId, `${scope}${email.threadId}`, index, union)
      const hasRfcMetadata = Boolean(email.rfcMessageId || email.inReplyTo || email.references?.length || email.threadId)
      if (!hasRfcMetadata) ThreadsRecords.connectIndex(legacyBySubject, `${scope}${ThreadsRecords.baseSubject(email.subject).toLowerCase()}`, index, union)
    }

    for (let index = 0; index < emails.length; index++) {
      const email = emails[index]
      const scope = `${ThreadsRecords.messageScope(email)}\0`
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
  static messageScope(email) {
    const accountId = String(email.providerAccountId || '').trim()
    return accountId ? `external:${accountId}` : `hosted:${String(email.domain || '').toLowerCase()}`
  }

  /** @param {Map<string, number>} index @param {string} key @param {number} current @param {(a:number,b:number)=>void} union */
  static connectIndex(index, key, current, union) {
    const existing = index.get(key)
    if (existing == null) index.set(key, current)
    else union(current, existing)
  }

  /** @param {string} subject @returns {string} */
  static baseSubject(subject) {
    const stripped = subject.replace(/^(?:(?:re|fw|fwd):\s*)+/i, '').trim()
    return stripped || subject.trim()
  }
}

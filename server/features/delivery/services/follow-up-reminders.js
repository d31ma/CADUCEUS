/** @typedef {import('../repositories/follow-up-reminders.js').FollowUpRemindersRepository} FollowUpRemindersRepository */
/** @typedef {import('../../mailbox/repositories/emails.js').EmailsRepository} EmailsRepository */

export class FollowUpReminderService {
  /** Serialize browser-triggered and background due passes in this API process. */
  #duePass = Promise.resolve()

  /** @param {FollowUpRemindersRepository} repository @param {EmailsRepository} emails @param {import('../../mailbox/services/category-mappings.js').CategoryMappingService} categoryMappings */
  constructor(repository, emails, categoryMappings) {
    this.repository = repository
    this.emails = emails
    this.categoryMappings = categoryMappings
  }

  /**
   * @param {{ownerEmail:string,emailId:string,recipient:string,subject:string,threadId?:string,remindAt:number}} input
   */
  async schedule(input) {
    const ownerEmail = input.ownerEmail.trim().toLowerCase()
    const domain = ownerEmail.split('@')[1] ?? ''
    const existing = await this.repository.findActiveForEmail(ownerEmail, input.emailId)
    if (existing) {
      await this.repository.update(existing.docId, {
        remindAt: input.remindAt,
        recipient: input.recipient,
        subject: input.subject,
        threadKey: threadKey(input.subject),
        ...(input.threadId ? { threadId: input.threadId } : {}),
        status: 'scheduled',
      })
      return { ...existing, remindAt: input.remindAt, status: 'scheduled' }
    }
    const reminder = {
      id: `reminder-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
      ownerEmail,
      domain,
      emailId: input.emailId,
      recipient: input.recipient.toLowerCase(),
      subject: input.subject,
      threadKey: threadKey(input.subject),
      ...(input.threadId ? { threadId: input.threadId } : {}),
      remindAt: input.remindAt,
      createdAt: Date.now(),
      status: /** @type {const} */ ('scheduled'),
    }
    await this.repository.put(reminder)
    return reminder
  }

  /** Process and return reminders for one mailbox, newest due first. */
  async listForOwner(ownerEmail) {
    await this.processDue(ownerEmail)
    return (await this.repository.listForOwner(ownerEmail.toLowerCase()))
      .filter(item => item.status !== 'dismissed' && item.status !== 'completed')
      .sort((a, b) => a.remindAt - b.remindAt)
  }

  /**
   * Materialize due reminders as unread local inbox messages. The operation is
   * restart-safe: the deterministic notice id prevents duplicate resurfacing.
   */
  async processDue(ownerEmail, now = Date.now()) {
    return await this.#enqueueDue(async () => {
      const pending = await this.repository.listPending(ownerEmail.toLowerCase())
      return (await this.#processPending(pending, now)).processed
    })
  }

  /**
   * Process every mailbox without requiring an inbox read. The bounded return
   * value is emitted through the worker's operational status snapshot.
   */
  async processDueAll(now = Date.now()) {
    return await this.#enqueueDue(async () => {
      const pending = await this.repository.listPendingAll()
      const result = await this.#processPending(pending, now)
      return {
        due: result.due,
        surfaced: result.surfaced,
        reconciled: result.reconciled,
        failed: result.failed,
      }
    })
  }

  /** @param {Array<any>} pending @param {number} now */
  async #processPending(pending, now) {
    const result = { due: 0, processed: 0, surfaced: 0, reconciled: 0, failed: 0 }
    for (const reminder of pending) {
      if (reminder.remindAt > now) continue
      result.due += 1
      try {
        const noticeEmailId = `follow-up-${reminder.id}`
        const [existingNotice] = await this.emails.findById(noticeEmailId)
        if (!existingNotice) {
          const inboxFolder = await this.categoryMappings?.pathFor(reminder.ownerEmail, reminder.domain, 'inbox') || 'inbox'
          await this.emails.put({
            id: noticeEmailId,
            domain: reminder.domain,
            ownerEmail: reminder.ownerEmail,
            recipient: reminder.ownerEmail,
            sender: reminder.ownerEmail,
            subject: `Re: ${reminder.subject}`,
            body: `Follow-up reminder: nobody has replied to your message to ${reminder.recipient}.`,
            folder: inboxFolder,
            read: false,
            starred: false,
            receivedAt: new Date(now).toISOString(),
            processed: true,
            ...(reminder.threadId ? { threadId: reminder.threadId } : {}),
          })
          result.surfaced += 1
        } else {
          result.reconciled += 1
        }
        await this.repository.update(reminder.docId, {
          status: 'due', dueAt: now, noticeEmailId,
        })
        result.processed += 1
      } catch (error) {
        result.failed += 1
        console.error('[reminders] failed to surface due reminder:', error instanceof Error ? error.message : 'unknown error')
      }
    }
    return result
  }

  /** @template T @param {() => Promise<T>} task @returns {Promise<T>} */
  #enqueueDue(task) {
    const current = this.#duePass.then(task, task)
    this.#duePass = current.then(() => undefined, () => undefined)
    return current
  }

  /** Clear scheduled reminders when a matching participant replies. */
  async resolveForReply({ recipient, sender, subject, threadId }) {
    const ownerEmail = recipient.toLowerCase()
    const senderEmail = sender.toLowerCase()
    const key = threadKey(subject)
    const pending = await this.repository.listPending(ownerEmail)
    let completed = 0
    for (const reminder of pending) {
      const sameThread = reminder.threadId && threadId
        ? reminder.threadId === threadId
        : reminder.threadKey === key
      if (!sameThread || !recipientSet(reminder.recipient).has(senderEmail)) continue
      await this.repository.update(reminder.docId, { status: 'completed', completedAt: Date.now() })
      completed += 1
    }
    return completed
  }

  async dismiss(id, ownerEmail) {
    const [docId, reminder] = await this.repository.findById(id)
    if (!docId || !reminder || reminder.ownerEmail !== ownerEmail.toLowerCase()) return false
    await this.repository.update(docId, { status: 'dismissed', completedAt: Date.now() })
    return true
  }

  /** @param {string} emailId @param {string} ownerEmail */
  async dismissForEmail(emailId, ownerEmail) {
    const reminder = await this.repository.findActiveForEmail(ownerEmail.toLowerCase(), emailId)
    if (!reminder) return false
    await this.repository.update(reminder.docId, { status: 'dismissed', completedAt: Date.now() })
    return true
  }
}

/** @param {string} subject */
export function threadKey(subject) {
  return String(subject || '')
    .replace(/^(?:(?:re|fw|fwd):\s*)+/i, '')
    .trim()
    .toLowerCase()
}

/** @param {string} value */
function recipientSet(value) {
  return new Set(String(value || '').split(',').map(item => item.trim().toLowerCase()).filter(Boolean))
}

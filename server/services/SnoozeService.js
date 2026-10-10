import { EmailsRecords } from '../shared/mailbox/EmailsRecords.js'
import { SnoozeError } from '../shared/errors/SnoozeError.js'

@Service
export default class SnoozeService {
  /** @type {SnoozeHelper} */
  @Needs
  snoozeHelper

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {ThreadsHelper} */
  @Needs
  threadsHelper

  /** @type {PushService} */
  @Needs
  pushService

  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /**
   * Snooze every inbox message currently belonging to the selected RFC thread.
   * Sent and already-moved messages remain where the user put them.
   *
   * @param {{emailId:string, until:number, label?:string, clientRequestId?:string, ownerEmail:string, domains:string[], now?:number}} input
   */
  async snoozeThread(input) {
    const now = input.now ?? Date.now()
    if (input.clientRequestId) {
      const [, replay] = await this.snoozeHelper.findByClientRequestId(input.ownerEmail, input.clientRequestId)
      if (replay) return {
        snoozed: replay.status === 'active', snoozeId: replay.id, threadId: replay.threadId,
        count: replay.emailIds?.length || 1, until: new Date(replay.until).toISOString(),
      }
    }
    if (!Number.isFinite(input.until) || input.until <= now) throw new SnoozeError('until must be in the future')
    const [, selected] = await this.emailsHelper.findById(input.emailId)
    if (!selected || !input.domains.includes(selected.domain) || !EmailsRecords.canAccessEmail(selected, input.ownerEmail)) {
      throw new SnoozeError('Email not found', 404)
    }
    const thread = await this.threadsHelper.findForEmail(input.domains, input.emailId, input.ownerEmail)
    if (!thread) throw new SnoozeError('Conversation not found', 404)
    const inboxPaths = Object.fromEntries(await Promise.all(input.domains.map(async domain => [
      domain,
      await this.categoryMappingService?.pathFor(input.ownerEmail, domain, 'inbox') || 'inbox',
    ])))
    const eligible = thread.emails.filter(email => (email.folder || 'inbox') === inboxPaths[email.domain])
    if (eligible.length === 0) throw new SnoozeError('Only inbox conversations can be snoozed', 409)

    const ownerEmail = input.ownerEmail.toLowerCase()
    const [existingDocId, existing] = await this.snoozeHelper.findActiveForThread(ownerEmail, thread.id)
    const id = existing?.id || `snooze-${now.toString(36)}-${Math.random().toString(36).slice(2, 10)}`
    const emailIds = [...new Set([...(existing?.emailIds || [existing?.emailId].filter(Boolean)), ...eligible.map(email => email.id)])]
    const record = /** @type {SnoozeEntry} */ ({
      id,
      emailId: input.emailId,
      threadId: thread.id,
      ownerEmail,
      domain: selected.domain,
      emailIds,
      until: input.until,
      snoozedAt: now,
      label: String(input.label || 'Custom').slice(0, 60),
      status: 'active',
      lastError: '',
      ...(input.clientRequestId ? { clientRequestId: input.clientRequestId } : {}),
    })

    if (existingDocId) await this.snoozeHelper.update(existingDocId, record)
    else await this.snoozeHelper.put(record)
    try {
      await this.emailsHelper.updateByIds(eligible.map(email => email.id), {
        folder: 'snoozed',
        snoozeId: id,
        snoozeThreadId: thread.id,
        snoozedUntil: new Date(input.until).toISOString(),
        snoozeLabel: record.label,
      })
    } catch (error) {
      const [docId] = existingDocId ? [existingDocId] : await this.snoozeHelper.findById(id)
      if (docId) await this.snoozeHelper.update(docId, { status: 'canceled', lastError: 'message move failed' })
      throw error
    }
    return { snoozed: true, snoozeId: id, threadId: thread.id, count: eligible.length, until: new Date(input.until).toISOString() }
  }

  /** @param {string} ownerEmail @param {string[]} domains */
  async listActive(ownerEmail, domains) {
    const docs = await this.snoozeHelper.listActive(ownerEmail)
    const results = []
    for (const record of Object.values(docs)) {
      const [, selected] = await this.emailsHelper.findById(record.emailId)
      if (!selected || !domains.includes(selected.domain) || !EmailsRecords.canAccessEmail(selected, ownerEmail)) continue
      results.push({
        id: record.id,
        threadId: record.threadId || selected.threadId || selected.id,
        emailId: record.emailId,
        subject: selected.subject,
        sender: selected.sender,
        domain: selected.domain,
        count: record.emailIds?.length || 1,
        label: record.label || 'Custom',
        snoozedUntil: new Date(record.until).toISOString(),
      })
    }
    return results.sort((left, right) => left.snoozedUntil.localeCompare(right.snoozedUntil))
  }

  /** @param {string} id @param {string} ownerEmail @param {number} [now] */
  async undo(id, ownerEmail, now = Date.now()) {
    const [docId, record] = await this.snoozeHelper.findById(id)
    if (!docId || !record || record.status === 'completed' || record.status === 'canceled') {
      throw new SnoozeError('Snooze not found', 404)
    }
    if (record.ownerEmail && record.ownerEmail.toLowerCase() !== ownerEmail.toLowerCase()) {
      throw new SnoozeError('Snooze not found', 404)
    }
    const moved = await this.release(docId, record, { now, status: 'canceled', markUnread: false, notify: false })
    return { snoozed: false, snoozeId: id, count: moved }
  }

  /** @param {number} [now] */
  async processDue(now = Date.now()) {
    const due = await this.snoozeHelper.listDue(now)
    const result = { due: Object.keys(due).length, returned: 0, failed: 0, notified: 0 }
    for (const [docId, record] of Object.entries(due)) {
      try {
        const [, owner] = record.ownerEmail ? await this.usersHelper.findByEmail(record.ownerEmail) : [null, null]
        const returned = await this.release(docId, record, {
          now,
          status: 'completed',
          markUnread: true,
          notify: true,
          returnToTop: owner?.mailPreferences?.unsnoozeToTop !== false,
        })
        result.returned += returned
        if (returned > 0) result.notified += 1
      } catch (error) {
        result.failed += 1
        const message = error instanceof Error ? error.message : 'unknown snooze failure'
        await this.snoozeHelper.update(docId, { lastError: message.slice(0, 200) })
      }
    }
    return result
  }

  /**
   * Return only messages that remain in Snoozed, mirroring Mailspring's guard
   * against moving a thread the user already relocated with another client.
   * @param {string} docId @param {SnoozeEntry} record
   * @param {{now:number,status:'completed'|'canceled',markUnread:boolean,notify:boolean,returnToTop?:boolean}} options
   */
  async release(docId, record, options) {
    const ids = record.emailIds?.length ? record.emailIds : [record.emailId]
    const messages = []
    for (const id of ids) {
      const [, email] = await this.emailsHelper.findById(id)
      if (email?.folder === 'snoozed' && (!record.ownerEmail || EmailsRecords.canAccessEmail(email, record.ownerEmail))) messages.push(email)
    }
    for (const email of messages) {
      const [emailDocId] = await this.emailsHelper.findById(email.id)
      if (!emailDocId) continue
      const ownerEmail = record.ownerEmail || email.ownerEmail || email.recipient
      const inboxFolder = await this.categoryMappingService?.pathFor(ownerEmail, email.domain, 'inbox') || 'inbox'
      await this.emailsHelper.update(emailDocId, {
        folder: inboxFolder,
        ...(options.markUnread ? { read: false } : {}),
        ...(options.markUnread && options.returnToTop ? { receivedAt: new Date(options.now).toISOString() } : {}),
      })
    }
    await this.snoozeHelper.update(docId, { status: options.status, completedAt: options.now, lastError: '' })
    if (options.notify && messages.length > 0) {
      const latest = messages.slice().sort((left, right) => right.receivedAt.localeCompare(left.receivedAt))[0]
      await this.pushService.notifyUnsnoozed(latest)
    }
    return messages.length
  }
}

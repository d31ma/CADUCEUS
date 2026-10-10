import { randomUUID } from 'node:crypto'
import { Fylo } from '../../../shared/persistence/Fylo.js'
import { TrackingRecords } from '../../../shared/delivery/TrackingRecords.js'

export class TrackingRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
    /** @type {Map<string, Promise<unknown>>} */
    this.pending = new Map()
  }

  /**
   * Record one recipient-level open after the route verifies an encrypted
   * tracking capability.
   *
   * @param {string | { trackingId: string, recipient?: string }} input
   * @returns {Promise<boolean>}
   */
  async recordOpen(input) {
    const capability = typeof input === 'string' ? { trackingId: input } : input
    const trackingId = TrackingRecords.normalizeTrackingId(capability?.trackingId)
    if (!trackingId) return false
    return await this.#serialize(trackingId, async () => {
      const entry = await this.findByTrackingId(trackingId)
      if (!entry) return false
      const [docId, email] = entry
      if (email.openTracking !== true) return false
      const openedAt = new Date().toISOString()
      const opens = TrackingRecords.readArray(email.opensJson)
      opens.push({
        id: `open-${randomUUID()}`,
        recipient: TrackingRecords.normalizeEmail(capability.recipient),
        openedAt,
      })
      await this.fylo.patchDoc(Fylo.Collections.EMAILS, {
        [docId]: {
          opensJson: JSON.stringify(opens.slice(-TrackingRecords.MAX_EVENTS_PER_MESSAGE)),
          openedAt: email.openedAt || openedAt,
          openCount: Math.max(0, Number(email.openCount) || 0) + 1,
        },
      })
      return true
    })
  }

  /**
   * Record one recipient-level click only when the encrypted target exactly
   * matches the link metadata stored with the Sent copy.
   *
   * @param {{ trackingId: string, recipient?: string, url: string, linkIndex: number }} input
   * @returns {Promise<boolean>}
   */
  async recordClick(input) {
    const trackingId = TrackingRecords.normalizeTrackingId(input?.trackingId)
    if (!trackingId) return false
    return await this.#serialize(trackingId, async () => {
      const entry = await this.findByTrackingId(trackingId)
      if (!entry) return false
      const [docId, email] = entry
      if (email.linkTracking !== true) return false
      const links = TrackingRecords.readArray(email.trackedLinksJson)
      const link = links.find(item => item?.index === input.linkIndex)
      if (!link || link.url !== input.url) return false
      const clickedAt = new Date().toISOString()
      const clicks = TrackingRecords.readArray(email.clicksJson)
      clicks.push({
        id: `click-${randomUUID()}`,
        recipient: TrackingRecords.normalizeEmail(input.recipient),
        url: link.url,
        title: TrackingRecords.cleanText(link.title, 200),
        linkIndex: input.linkIndex,
        clickedAt,
      })
      await this.fylo.patchDoc(Fylo.Collections.EMAILS, {
        [docId]: {
          clicksJson: JSON.stringify(clicks.slice(-TrackingRecords.MAX_EVENTS_PER_MESSAGE)),
          clickCount: Math.max(0, Number(email.clickCount) || 0) + 1,
        },
      })
      return true
    })
  }

  /**
   * Build the exact-owner activity feed and local analytics dashboard. Only
   * Sent messages owned by the mailbox, and replies addressed to that mailbox,
   * participate even when several users share one hosted domain.
   *
   * @param {{ allowedDomains: string[], ownerEmail: string, ownedSenders?: string[], sentFolders?: Record<string, string>, draftFolders?: Record<string, string>, type?: 'all'|'open'|'click', from?: number, to?: number, since?: number, limit?: number }} options
   */
  async activity(options) {
    const now = Number.isFinite(options.to) ? Number(options.to) : Date.now()
    const from = Number.isFinite(options.from) ? Number(options.from) : 0
    const limit = Math.max(1, Math.min(TrackingRecords.MAX_ACTIVITY_EVENTS, Math.trunc(options.limit ?? 50)))
    const owner = TrackingRecords.normalizeEmail(options.ownerEmail)
    const domains = new Set((options.allowedDomains || []).map(value => String(value).toLowerCase()))
    const senders = new Set([owner, ...(options.ownedSenders || []).map(TrackingRecords.normalizeEmail)].filter(Boolean))
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.EMAILS, { $ops: [] }).collect())
    const all = Object.values(docs).filter(email => domains.has(String(email.domain || '').toLowerCase()))
    const sent = all
      .filter(email => TrackingRecords.isOwnedSent(email, owner, senders, options.sentFolders))
      .sort((a, b) => TrackingRecords.timestampOf(b.receivedAt) - TrackingRecords.timestampOf(a.receivedAt))
      .slice(0, TrackingRecords.MAX_ACTIVITY_MESSAGES)
    const received = all.filter(email => TrackingRecords.isReceivedByOwner(email, senders, options.sentFolders, options.draftFolders))
    const repliesByThread = TrackingRecords.replyThreads(received, sent, senders)
    const events = []

    for (const email of sent) {
      const fallbackRecipient = TrackingRecords.recipientList(email)[0] || ''
      const opens = TrackingRecords.readArray(email.opensJson)
      for (const item of opens) {
        const timestamp = TrackingRecords.cleanTimestamp(item?.openedAt)
        if (timestamp) events.push(TrackingRecords.activityEvent('open', email, item, timestamp, fallbackRecipient))
      }
      if (email.openedAt && opens.length === 0) {
        const timestamp = TrackingRecords.cleanTimestamp(email.openedAt)
        if (timestamp) events.push(TrackingRecords.activityEvent('open', email, {}, timestamp, fallbackRecipient))
      }
      for (const item of TrackingRecords.readArray(email.clicksJson || email.clicks)) {
        const timestamp = TrackingRecords.cleanTimestamp(item?.clickedAt || item?.timestamp)
        if (timestamp) events.push(TrackingRecords.activityEvent('click', email, item, timestamp, fallbackRecipient))
      }
    }

    const type = options.type === 'open' || options.type === 'click' ? options.type : 'all'
    const matching = events
      .filter(event => type === 'all' || event.type === type)
      .filter(event => {
        const at = TrackingRecords.timestampOf(event.timestamp)
        return at >= from && at <= now
      })
      .sort((a, b) => b.timestamp.localeCompare(a.timestamp))
    const unseenCount = Number.isFinite(options.since)
      ? matching.filter(event => TrackingRecords.timestampOf(event.timestamp) > Number(options.since)).length
      : 0
    const rangeSent = sent.filter(email => TrackingRecords.within(email.receivedAt, from, now))
    const rangeReceived = received.filter(email => TrackingRecords.within(email.receivedAt, from, now))
    const opened = rangeSent.filter(email => TrackingRecords.hasOpen(email))
    const clicked = rangeSent.filter(email => TrackingRecords.hasClick(email))
    const replied = rangeSent.filter(email => email.threadId && repliesByThread.has(email.threadId))
    const openTracked = rangeSent.filter(email => email.openTracking === true)
    const linkTracked = rangeSent.filter(email => email.linkTracking === true)
    const tracked = rangeSent.filter(email => email.openTracking === true || email.linkTracking === true)

    return {
      events: matching.slice(0, limit),
      unseenCount,
      metrics: {
        sent: rangeSent.length,
        received: rangeReceived.length,
        tracked: tracked.length,
        opened: opened.length,
        clicked: clicked.length,
        replied: replied.length,
        trackingRate: TrackingRecords.rate(tracked.length, rangeSent.length),
        openRate: TrackingRecords.rate(opened.length, openTracked.length),
        clickRate: TrackingRecords.rate(clicked.length, linkTracked.length),
        replyRate: TrackingRecords.rate(replied.length, rangeSent.length),
        bySubject: TrackingRecords.subjectMetrics(rangeSent, repliesByThread),
        daily: TrackingRecords.dailyMetrics(rangeSent, rangeReceived, events, from, now),
      },
      generatedAt: new Date(now).toISOString(),
    }
  }

  /** @param {string} trackingId @returns {Promise<[string, any] | null>} */
  async findByTrackingId(trackingId) {
    const id = TrackingRecords.normalizeTrackingId(trackingId)
    if (!id) return null
    const docs = await Fylo.collect(
      this.fylo.findDocs(Fylo.Collections.EMAILS, { $ops: [{ trackingId: { $eq: id } }] }).collect(),
    )
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], entry[1]] : null
  }

  /** @template T @param {string} key @param {() => Promise<T>} task @returns {Promise<T>} */
  async #serialize(key, task) {
    const previous = this.pending.get(key) ?? Promise.resolve()
    const next = previous.catch(() => {}).then(task)
    this.pending.set(key, next)
    try { return await next } finally {
      if (this.pending.get(key) === next) this.pending.delete(key)
    }
  }
}


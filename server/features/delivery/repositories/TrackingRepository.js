import { randomUUID } from 'node:crypto'
import { Collections, collect } from '../../../platform/persistence/Fylo.js'

const MAX_EVENTS_PER_MESSAGE = 100
const MAX_ACTIVITY_MESSAGES = 500
const MAX_ACTIVITY_EVENTS = 100
const DAY_MS = 24 * 60 * 60 * 1000

export class TrackingRepository {
  /** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo */
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
    const trackingId = normalizeTrackingId(capability?.trackingId)
    if (!trackingId) return false
    return await this.#serialize(trackingId, async () => {
      const entry = await this.findByTrackingId(trackingId)
      if (!entry) return false
      const [docId, email] = entry
      if (email.openTracking !== true) return false
      const openedAt = new Date().toISOString()
      const opens = readArray(email.opensJson)
      opens.push({
        id: `open-${randomUUID()}`,
        recipient: normalizeEmail(capability.recipient),
        openedAt,
      })
      await this.fylo.patchDoc(Collections.EMAILS, {
        [docId]: {
          opensJson: JSON.stringify(opens.slice(-MAX_EVENTS_PER_MESSAGE)),
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
    const trackingId = normalizeTrackingId(input?.trackingId)
    if (!trackingId) return false
    return await this.#serialize(trackingId, async () => {
      const entry = await this.findByTrackingId(trackingId)
      if (!entry) return false
      const [docId, email] = entry
      if (email.linkTracking !== true) return false
      const links = readArray(email.trackedLinksJson)
      const link = links.find(item => item?.index === input.linkIndex)
      if (!link || link.url !== input.url) return false
      const clickedAt = new Date().toISOString()
      const clicks = readArray(email.clicksJson)
      clicks.push({
        id: `click-${randomUUID()}`,
        recipient: normalizeEmail(input.recipient),
        url: link.url,
        title: cleanText(link.title, 200),
        linkIndex: input.linkIndex,
        clickedAt,
      })
      await this.fylo.patchDoc(Collections.EMAILS, {
        [docId]: {
          clicksJson: JSON.stringify(clicks.slice(-MAX_EVENTS_PER_MESSAGE)),
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
    const limit = Math.max(1, Math.min(MAX_ACTIVITY_EVENTS, Math.trunc(options.limit ?? 50)))
    const owner = normalizeEmail(options.ownerEmail)
    const domains = new Set((options.allowedDomains || []).map(value => String(value).toLowerCase()))
    const senders = new Set([owner, ...(options.ownedSenders || []).map(normalizeEmail)].filter(Boolean))
    const docs = await collect(this.fylo.findDocs(Collections.EMAILS, { $ops: [] }).collect())
    const all = Object.values(docs).filter(email => domains.has(String(email.domain || '').toLowerCase()))
    const sent = all
      .filter(email => isOwnedSent(email, owner, senders, options.sentFolders))
      .sort((a, b) => timestampOf(b.receivedAt) - timestampOf(a.receivedAt))
      .slice(0, MAX_ACTIVITY_MESSAGES)
    const received = all.filter(email => isReceivedByOwner(email, senders, options.sentFolders, options.draftFolders))
    const repliesByThread = replyThreads(received, sent, senders)
    const events = []

    for (const email of sent) {
      const fallbackRecipient = recipientList(email)[0] || ''
      const opens = readArray(email.opensJson)
      for (const item of opens) {
        const timestamp = cleanTimestamp(item?.openedAt)
        if (timestamp) events.push(activityEvent('open', email, item, timestamp, fallbackRecipient))
      }
      if (email.openedAt && opens.length === 0) {
        const timestamp = cleanTimestamp(email.openedAt)
        if (timestamp) events.push(activityEvent('open', email, {}, timestamp, fallbackRecipient))
      }
      for (const item of readArray(email.clicksJson || email.clicks)) {
        const timestamp = cleanTimestamp(item?.clickedAt || item?.timestamp)
        if (timestamp) events.push(activityEvent('click', email, item, timestamp, fallbackRecipient))
      }
    }

    const type = options.type === 'open' || options.type === 'click' ? options.type : 'all'
    const matching = events
      .filter(event => type === 'all' || event.type === type)
      .filter(event => {
        const at = timestampOf(event.timestamp)
        return at >= from && at <= now
      })
      .sort((a, b) => b.timestamp.localeCompare(a.timestamp))
    const unseenCount = Number.isFinite(options.since)
      ? matching.filter(event => timestampOf(event.timestamp) > Number(options.since)).length
      : 0
    const rangeSent = sent.filter(email => within(email.receivedAt, from, now))
    const rangeReceived = received.filter(email => within(email.receivedAt, from, now))
    const opened = rangeSent.filter(email => hasOpen(email))
    const clicked = rangeSent.filter(email => hasClick(email))
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
        trackingRate: rate(tracked.length, rangeSent.length),
        openRate: rate(opened.length, openTracked.length),
        clickRate: rate(clicked.length, linkTracked.length),
        replyRate: rate(replied.length, rangeSent.length),
        bySubject: subjectMetrics(rangeSent, repliesByThread),
        daily: dailyMetrics(rangeSent, rangeReceived, events, from, now),
      },
      generatedAt: new Date(now).toISOString(),
    }
  }

  /** @param {string} trackingId @returns {Promise<[string, any] | null>} */
  async findByTrackingId(trackingId) {
    const id = normalizeTrackingId(trackingId)
    if (!id) return null
    const docs = await collect(
      this.fylo.findDocs(Collections.EMAILS, { $ops: [{ trackingId: { $eq: id } }] }).collect(),
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

function activityEvent(type, email, item, timestamp, fallbackRecipient) {
  return {
    id: cleanText(item?.id, 160) || `${type}-${email.id}-${timestamp}`,
    type,
    messageId: String(email.id || ''),
    threadId: String(email.threadId || ''),
    subject: cleanText(email.subject, 500),
    recipient: normalizeEmail(item?.recipient) || fallbackRecipient,
    ...(type === 'click' ? { url: String(item?.url || ''), title: cleanText(item?.title, 200) } : {}),
    timestamp,
  }
}

function isOwnedSent(email, owner, senders, sentFolders = {}) {
  if (String(email.folder || '').toLowerCase() !== (sentFolders[email.domain] || 'sent')) return false
  const explicitOwner = normalizeEmail(email.ownerEmail)
  return explicitOwner ? explicitOwner === owner : senders.has(normalizeEmail(email.sender))
}

function isReceivedByOwner(email, senders, sentFolders = {}, draftFolders = {}) {
  const folder = String(email.folder || 'inbox').toLowerCase()
  return folder !== (sentFolders[email.domain] || 'sent') && folder !== (draftFolders[email.domain] || 'drafts') && recipientList(email).some(address => senders.has(address))
}

function recipientList(email) {
  const values = [
    ...(Array.isArray(email.to) ? email.to : []),
    ...(Array.isArray(email.cc) ? email.cc : []),
    ...String(email.recipient || '').split(','),
  ]
  return [...new Set(values.map(normalizeEmail).filter(Boolean))]
}

function replyThreads(received, sent, senders) {
  const sentRecipients = new Map()
  for (const email of sent) {
    if (!email.threadId) continue
    const values = sentRecipients.get(email.threadId) ?? new Set()
    for (const address of recipientList(email)) values.add(address)
    sentRecipients.set(email.threadId, values)
  }
  const replied = new Set()
  for (const email of received) {
    if (!email.threadId || !recipientList(email).some(address => senders.has(address))) continue
    if (sentRecipients.get(email.threadId)?.has(normalizeEmail(email.sender))) replied.add(email.threadId)
  }
  return replied
}

function subjectMetrics(sent, repliesByThread) {
  const groups = new Map()
  for (const email of sent) {
    const subject = cleanText(email.subject, 500) || '(No subject)'
    const item = groups.get(subject) ?? { subject, sent: 0, opened: 0, clicked: 0, replied: 0 }
    item.sent++
    if (hasOpen(email)) item.opened++
    if (hasClick(email)) item.clicked++
    if (email.threadId && repliesByThread.has(email.threadId)) item.replied++
    groups.set(subject, item)
  }
  return [...groups.values()].sort((a, b) => b.sent - a.sent || a.subject.localeCompare(b.subject)).slice(0, 20)
}

function dailyMetrics(sent, received, events, from, now) {
  const first = Math.max(Number.isFinite(from) && from > 0 ? from : now - 29 * DAY_MS, now - 89 * DAY_MS)
  const days = new Map()
  for (let at = startOfDay(first); at <= startOfDay(now); at += DAY_MS) {
    const date = new Date(at).toISOString().slice(0, 10)
    days.set(date, { date, sent: 0, received: 0, opens: 0, clicks: 0 })
  }
  for (const email of sent) incrementDay(days, email.receivedAt, 'sent')
  for (const email of received) incrementDay(days, email.receivedAt, 'received')
  for (const event of events) incrementDay(days, event.timestamp, event.type === 'open' ? 'opens' : 'clicks')
  return [...days.values()]
}

function incrementDay(days, value, field) {
  const key = cleanTimestamp(value)?.slice(0, 10)
  const item = key && days.get(key)
  if (item) item[field]++
}

function hasOpen(email) {
  return Boolean(email.openedAt || Number(email.openCount) > 0 || readArray(email.opensJson).length)
}

function hasClick(email) {
  return Boolean(Number(email.clickCount) > 0 || readArray(email.clicksJson || email.clicks).length)
}

function readArray(value) {
  if (Array.isArray(value)) return value.slice(0, MAX_EVENTS_PER_MESSAGE)
  if (typeof value !== 'string' || !value) return []
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed.slice(0, MAX_EVENTS_PER_MESSAGE) : []
  } catch { return [] }
}

function normalizeTrackingId(value) {
  const id = typeof value === 'string' ? value.trim() : ''
  return /^[A-Za-z0-9_-]{16,128}$/.test(id) ? id : ''
}

function normalizeEmail(value) {
  const email = typeof value === 'string' ? value.trim().toLowerCase() : ''
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) && email.length <= 320 ? email : ''
}

function cleanTimestamp(value) {
  const at = typeof value === 'string' ? Date.parse(value) : NaN
  return Number.isFinite(at) ? new Date(at).toISOString() : ''
}

function timestampOf(value) {
  const at = typeof value === 'string' ? Date.parse(value) : Number(value)
  return Number.isFinite(at) ? at : 0
}

function within(value, from, to) {
  const at = timestampOf(value)
  return at >= from && at <= to
}

function rate(numerator, denominator) {
  return denominator > 0 ? Math.round((numerator / denominator) * 1000) / 10 : 0
}

function startOfDay(value) {
  const date = new Date(value)
  return Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate())
}

function cleanText(value, max) {
  return typeof value === 'string' ? value.replace(/[\u0000-\u001f\u007f]/g, ' ').trim().slice(0, max) : ''
}

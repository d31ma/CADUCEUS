import { randomUUID } from 'node:crypto'
import { Fylo } from '../persistence/Fylo.js'

export class TrackingRecords {
  static MAX_EVENTS_PER_MESSAGE = 100

  static MAX_ACTIVITY_MESSAGES = 500

  static MAX_ACTIVITY_EVENTS = 100

  static DAY_MS = 24 * 60 * 60 * 1000

  static activityEvent(type, email, item, timestamp, fallbackRecipient) {
    return {
      id: TrackingRecords.cleanText(item?.id, 160) || `${type}-${email.id}-${timestamp}`,
      type,
      messageId: String(email.id || ''),
      threadId: String(email.threadId || ''),
      subject: TrackingRecords.cleanText(email.subject, 500),
      recipient: TrackingRecords.normalizeEmail(item?.recipient) || fallbackRecipient,
      ...(type === 'click' ? { url: String(item?.url || ''), title: TrackingRecords.cleanText(item?.title, 200) } : {}),
      timestamp,
    }
  }

  static isOwnedSent(email, owner, senders, sentFolders = {}) {
    if (String(email.folder || '').toLowerCase() !== (sentFolders[email.domain] || 'sent')) return false
    const explicitOwner = TrackingRecords.normalizeEmail(email.ownerEmail)
    return explicitOwner ? explicitOwner === owner : senders.has(TrackingRecords.normalizeEmail(email.sender))
  }

  static isReceivedByOwner(email, senders, sentFolders = {}, draftFolders = {}) {
    const folder = String(email.folder || 'inbox').toLowerCase()
    return folder !== (sentFolders[email.domain] || 'sent') && folder !== (draftFolders[email.domain] || 'drafts') && TrackingRecords.recipientList(email).some(address => senders.has(address))
  }

  static recipientList(email) {
    const values = [
      ...(Array.isArray(email.to) ? email.to : []),
      ...(Array.isArray(email.cc) ? email.cc : []),
      ...String(email.recipient || '').split(','),
    ]
    return [...new Set(values.map(TrackingRecords.normalizeEmail).filter(Boolean))]
  }

  static replyThreads(received, sent, senders) {
    const sentRecipients = new Map()
    for (const email of sent) {
      if (!email.threadId) continue
      const values = sentRecipients.get(email.threadId) ?? new Set()
      for (const address of TrackingRecords.recipientList(email)) values.add(address)
      sentRecipients.set(email.threadId, values)
    }
    const replied = new Set()
    for (const email of received) {
      if (!email.threadId || !TrackingRecords.recipientList(email).some(address => senders.has(address))) continue
      if (sentRecipients.get(email.threadId)?.has(TrackingRecords.normalizeEmail(email.sender))) replied.add(email.threadId)
    }
    return replied
  }

  static subjectMetrics(sent, repliesByThread) {
    const groups = new Map()
    for (const email of sent) {
      const subject = TrackingRecords.cleanText(email.subject, 500) || '(No subject)'
      const item = groups.get(subject) ?? { subject, sent: 0, opened: 0, clicked: 0, replied: 0 }
      item.sent++
      if (TrackingRecords.hasOpen(email)) item.opened++
      if (TrackingRecords.hasClick(email)) item.clicked++
      if (email.threadId && repliesByThread.has(email.threadId)) item.replied++
      groups.set(subject, item)
    }
    return [...groups.values()].sort((a, b) => b.sent - a.sent || a.subject.localeCompare(b.subject)).slice(0, 20)
  }

  static dailyMetrics(sent, received, events, from, now) {
    const first = Math.max(Number.isFinite(from) && from > 0 ? from : now - 29 * TrackingRecords.DAY_MS, now - 89 * TrackingRecords.DAY_MS)
    const days = new Map()
    for (let at = TrackingRecords.startOfDay(first); at <= TrackingRecords.startOfDay(now); at += TrackingRecords.DAY_MS) {
      const date = new Date(at).toISOString().slice(0, 10)
      days.set(date, { date, sent: 0, received: 0, opens: 0, clicks: 0 })
    }
    for (const email of sent) TrackingRecords.incrementDay(days, email.receivedAt, 'sent')
    for (const email of received) TrackingRecords.incrementDay(days, email.receivedAt, 'received')
    for (const event of events) TrackingRecords.incrementDay(days, event.timestamp, event.type === 'open' ? 'opens' : 'clicks')
    return [...days.values()]
  }

  static incrementDay(days, value, field) {
    const key = TrackingRecords.cleanTimestamp(value)?.slice(0, 10)
    const item = key && days.get(key)
    if (item) item[field]++
  }

  static hasOpen(email) {
    return Boolean(email.openedAt || Number(email.openCount) > 0 || TrackingRecords.readArray(email.opensJson).length)
  }

  static hasClick(email) {
    return Boolean(Number(email.clickCount) > 0 || TrackingRecords.readArray(email.clicksJson || email.clicks).length)
  }

  static readArray(value) {
    if (Array.isArray(value)) return value.slice(0, TrackingRecords.MAX_EVENTS_PER_MESSAGE)
    if (typeof value !== 'string' || !value) return []
    try {
      const parsed = JSON.parse(value)
      return Array.isArray(parsed) ? parsed.slice(0, TrackingRecords.MAX_EVENTS_PER_MESSAGE) : []
    } catch { return [] }
  }

  static normalizeTrackingId(value) {
    const id = typeof value === 'string' ? value.trim() : ''
    return /^[A-Za-z0-9_-]{16,128}$/.test(id) ? id : ''
  }

  static normalizeEmail(value) {
    const email = typeof value === 'string' ? value.trim().toLowerCase() : ''
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) && email.length <= 320 ? email : ''
  }

  static cleanTimestamp(value) {
    const at = typeof value === 'string' ? Date.parse(value) : NaN
    return Number.isFinite(at) ? new Date(at).toISOString() : ''
  }

  static timestampOf(value) {
    const at = typeof value === 'string' ? Date.parse(value) : Number(value)
    return Number.isFinite(at) ? at : 0
  }

  static within(value, from, to) {
    const at = TrackingRecords.timestampOf(value)
    return at >= from && at <= to
  }

  static rate(numerator, denominator) {
    return denominator > 0 ? Math.round((numerator / denominator) * 1000) / 10 : 0
  }

  static startOfDay(value) {
    const date = new Date(value)
    return Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate())
  }

  static cleanText(value, max) {
    return typeof value === 'string' ? value.replace(/[\u0000-\u001f\u007f]/g, ' ').trim().slice(0, max) : ''
  }
}

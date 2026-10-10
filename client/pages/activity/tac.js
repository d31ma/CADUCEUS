// @ts-check

import { formatPreferenceDate } from '../../shared/scripts/mail-preferences.js'
import { formatNumber } from '../../shared/scripts/localization.js'

const EMPTY_METRICS = Object.freeze({
  sent: 0,
  received: 0,
  tracked: 0,
  opened: 0,
  clicked: 0,
  replied: 0,
  trackingRate: 0,
  openRate: 0,
  clickRate: 0,
  replyRate: 0,
  bySubject: [],
  daily: [],
})

/** Self-hosted recipient activity and mailbox analytics. */
export default class {
  static __tachyonOnMount = ['init']

  /** @type {Array<Record<string, any>>} */
  $events = []
  /** @type {Record<string, any>} */
  $metrics = { ...EMPTY_METRICS }
  /** @type {Array<Record<string, any>>} */
  $daily = []
  /** @type {Array<Record<string, any>>} */
  @SessionState
  subjects = []
  /** @type {'all'|'open'|'click'} */
  $type = 'all'
  /** @type {'7d'|'30d'|'90d'|'all'} */
  $range = '30d'
  /** @type {boolean} */
  @SessionState
  loading = true
  /** @type {string} */
  $error = ''
  /** @type {string} */
  $generatedAt = ''

  get canUseApp() {
    return Boolean(window._caduceus?.auth?.isLoggedIn
      || (typeof sessionStorage !== 'undefined' && sessionStorage.getItem('caduceus_token')))
  }

  get canShowLogin() { return !this.canUseApp }

  get eventFilterLabel() {
    if (this.$type === 'open') return 'open events'
    if (this.$type === 'click') return 'click events'
    return 'events'
  }

  async init() { await this.load() }

  async load() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.loading = false; return }
    this.loading = true
    this.$error = ''
    try {
      const query = new URLSearchParams({ type: this.$type, range: this.$range, limit: '100' })
      const response = await apiFetch(`/activity?${query}`)
      if (!response?.ok) throw new Error('Activity could not be loaded.')
      const data = await response.json()
      this.$events = Array.isArray(data?.events) ? data.events.map(event => this.decorateEvent(event)) : []
      this.$metrics = this.decorateMetrics(data?.metrics)
      this.$daily = this.decorateDaily(this.$metrics.daily)
      this.subjects = Array.isArray(this.$metrics.bySubject)
        ? this.$metrics.bySubject.map(subject => this.decorateSubject(subject))
        : []
      this.$generatedAt = typeof data?.generatedAt === 'string' ? data.generatedAt : new Date().toISOString()
      this.markViewed(this.$generatedAt)
    } catch (error) {
      this.$events = []
      this.$metrics = { ...EMPTY_METRICS }
      this.$daily = []
      this.subjects = []
      this.$error = error instanceof Error ? error.message : 'Activity could not be loaded.'
    } finally {
      this.loading = false
    }
  }

  /** @param {Event|'all'|'open'|'click'} eventOrType @param {'all'|'open'|'click'} [requestedType] */
  async setType(eventOrType, requestedType) {
    const type = typeof eventOrType === 'string' ? eventOrType : requestedType
    if (type !== 'all' && type !== 'open' && type !== 'click') return
    this.$type = type
    await this.load()
  }

  /** @param {Event} event */
  async changeRange(event) {
    const value = /** @type {HTMLSelectElement} */ (event.target)?.value
    if (value !== '7d' && value !== '30d' && value !== '90d' && value !== 'all') return
    this.$range = value
    await this.load()
  }

  /** @param {'all'|'open'|'click'} type */
  typeButtonClass(type) {
    return this.$type === type ? 'activity-filter active' : 'activity-filter'
  }

  /** @param {Event|Record<string, any>} eventOrItem @param {Record<string, any>} [requestedItem] */
  openEvent(eventOrItem, requestedItem) {
    const item = eventOrItem && 'messageId' in eventOrItem ? eventOrItem : requestedItem
    if (item?.messageId) window._caduceus?.openEmailId(item.messageId)
  }

  /** @param {Record<string, any>} event */
  decorateEvent(event) {
    const type = event?.type === 'click' ? 'click' : 'open'
    const timestamp = typeof event?.timestamp === 'string' ? event.timestamp : ''
    return {
      ...event,
      type,
      eventIcon: type === 'open' ? '◉' : '↗',
      eventLabel: type === 'open' ? 'Opened' : 'Clicked',
      subjectLabel: cleanText(event?.subject) || '(No subject)',
      recipientLabel: cleanText(event?.recipient) || 'Recipient unavailable',
      targetLabel: type === 'click' ? safeTargetLabel(event?.title, event?.url) : '',
      timestampLabel: formatPreferenceDate(timestamp),
    }
  }

  /** @param {Record<string, any>} metrics */
  decorateMetrics(metrics) {
    const source = metrics && typeof metrics === 'object' ? metrics : EMPTY_METRICS
    return {
      ...EMPTY_METRICS,
      ...source,
      trackingRateLabel: percent(source.trackingRate),
      openRateLabel: percent(source.openRate),
      clickRateLabel: percent(source.clickRate),
      replyRateLabel: percent(source.replyRate),
    }
  }

  /** @param {Array<Record<string, any>>} daily */
  decorateDaily(daily) {
    if (!Array.isArray(daily)) return []
    const maximum = Math.max(1, ...daily.flatMap(day => [day.sent, day.received, day.opens, day.clicks].map(numberValue)))
    return daily.map(day => ({
      ...day,
      dateLabel: formatDay(day.date),
      ariaLabel: `${formatDay(day.date)}: ${numberValue(day.sent)} sent, ${numberValue(day.received)} received, ${numberValue(day.opens)} opens, ${numberValue(day.clicks)} clicks`,
      sentStyle: barStyle(day.sent, maximum),
      receivedStyle: barStyle(day.received, maximum),
      opensStyle: barStyle(day.opens, maximum),
      clicksStyle: barStyle(day.clicks, maximum),
    }))
  }

  /** @param {Record<string, any>} subject */
  decorateSubject(subject) {
    const sent = numberValue(subject?.sent)
    return {
      ...subject,
      subjectLabel: cleanText(subject?.subject) || '(No subject)',
      openRateLabel: percent(sent ? numberValue(subject?.opened) / sent * 100 : 0),
      clickRateLabel: percent(sent ? numberValue(subject?.clicked) / sent * 100 : 0),
      replyRateLabel: percent(sent ? numberValue(subject?.replied) / sent * 100 : 0),
    }
  }

  /** @param {string} timestamp */
  markViewed(timestamp) {
    const owner = window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || 'anonymous'
    try { localStorage.setItem(`caduceus_activity_viewed:${owner.toLowerCase()}`, timestamp) } catch {}
    window.dispatchEvent(new CustomEvent('caduceus:activity-viewed', { detail: { timestamp } }))
  }
}

function numberValue(value) {
  const number = Number(value)
  return Number.isFinite(number) && number > 0 ? number : 0
}

function percent(value) {
  const number = Number(value)
  return formatNumber(Number.isFinite(number) ? number : 0, {
    maximumFractionDigits: Number.isFinite(number) && number % 1 ? 1 : 0,
  }, undefined) + '%'
}

function barStyle(value, maximum) {
  const number = numberValue(value)
  const height = number ? Math.max(4, Math.round(number / maximum * 100)) : 0
  return `height:${height}%`
}

function formatDay(value) {
  const timestamp = typeof value === 'string' ? Date.parse(`${value}T00:00:00Z`) : NaN
  return Number.isFinite(timestamp)
    ? formatPreferenceDate(timestamp, { month: 'short', day: 'numeric', timeZone: 'UTC' })
    : ''
}

function cleanText(value) {
  return typeof value === 'string' ? value.replace(/[\u0000-\u001f\u007f]/g, ' ').trim() : ''
}

function safeTargetLabel(title, url) {
  const cleanTitle = cleanText(title)
  if (cleanTitle) return cleanTitle
  try {
    const target = new URL(String(url || ''))
    return `${target.hostname}${target.pathname === '/' ? '' : target.pathname}`
  } catch { return 'Tracked link' }
}

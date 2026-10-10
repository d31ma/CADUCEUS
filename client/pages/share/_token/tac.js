// @ts-check

import { buildProtectedEmailDocument } from '../../../shared/scripts/email-content-protection.js'
import {
  formatDate as formatLocalizedDate,
  formatNumber,
} from '../../../shared/scripts/localization.js'

/** Standalone, unauthenticated view for one expiring conversation capability. */
export default class {
  static __tachyonOnMount = ['init']

  /** @type {boolean} */
  @SessionState
  loading = true
  /** @type {string} */
  @SessionState
  error = ''
  /** @type {Record<string, any> | null} */
  $share = null
  /** @type {string} */
  @SessionState
  token = ''
  /** @type {string} */
  @SessionState
  downloadError = ''

  async init() {
    this.installNoReferrerPolicy()
    this.token = this.resolveToken()
    await this.loadShare()
    window.removeEventListener('visibilitychange', /** @type {EventListener} */ (window._caduceusShareVisibility))
    window._caduceusShareVisibility = () => {
      if (document.visibilityState === 'visible') void this.loadShare({ quiet: true })
    }
    window.addEventListener('visibilitychange', window._caduceusShareVisibility)
    if (window._caduceusShareInterval) clearInterval(window._caduceusShareInterval)
    window._caduceusShareInterval = setInterval(() => { void this.loadShare({ quiet: true }) }, 30_000)
  }

  installNoReferrerPolicy() {
    let meta = document.querySelector('meta[name="referrer"]')
    if (!meta) {
      meta = document.createElement('meta')
      meta.setAttribute('name', 'referrer')
      document.head.appendChild(meta)
    }
    meta.setAttribute('content', 'no-referrer')
  }

  resolveToken() {
    if (!location.pathname.startsWith('/share/')) return ''
    try { return decodeURIComponent(location.pathname.slice('/share/'.length)) } catch { return '' }
  }

  /** @param {{ quiet?: boolean }} [options] */
  async loadShare(options = {}) {
    if (!this.token) {
      this.loading = false
      this.error = 'This shared conversation is unavailable.'
      return
    }
    if (!options.quiet) this.loading = true
    try {
      const response = await fetch(this.apiUrl(`/thread-shares/public/${encodeURIComponent(this.$token)}`), {
        credentials: 'omit',
        cache: 'no-store',
        referrerPolicy: 'no-referrer',
      })
      if (!response.ok) throw new Error('unavailable')
      const share = await response.json()
      if (!Array.isArray(share?.messages) || !share.messages.length) throw new Error('unavailable')
      this.$share = {
        ...share,
        expiresAtLabel: formatLocalizedDate(share.expiresAt),
        messages: share.messages.map((message, index) => this.decorateMessage(message, index)),
      }
      this.error = ''
    } catch {
      this.$share = null
      this.error = 'This shared conversation is unavailable. It may have expired or been revoked.'
    } finally {
      this.loading = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {Record<string, any>} message @param {number} index */
  decorateMessage(message, index) {
    const protectedHtml = message.htmlBody
      ? buildProtectedEmailDocument(message.htmlBody, { allowRemote: false }).documentHtml
      : ''
    return {
      ...message,
      key: `${message.receivedAt || ''}-${index}`,
      protectedHtml,
      receivedAtLabel: formatLocalizedDate(message.receivedAt),
      toLabel: Array.isArray(message.to) ? message.to.join(', ') : '',
      ccLabel: Array.isArray(message.cc) ? message.cc.join(', ') : '',
      attachments: Array.isArray(message.attachments)
        ? message.attachments.map(attachment => ({ ...attachment, sizeLabel: formatBytes(attachment.size) }))
        : [],
    }
  }

  /** @param {Event | Record<string, any>} eventOrAttachment @param {Record<string, any>} [requestedAttachment] */
  async downloadAttachment(eventOrAttachment, requestedAttachment) {
    const candidate = /** @type {Record<string, any>} */ (eventOrAttachment)
    const attachment = requestedAttachment?.id ? requestedAttachment : candidate?.id ? candidate : null
    if (!attachment?.id || !this.token) return
    this.downloadError = ''
    try {
      const response = await fetch(this.apiUrl(
        `/thread-shares/public/${encodeURIComponent(this.$token)}/attachments/${encodeURIComponent(attachment.id)}`,
      ), { credentials: 'omit', cache: 'no-store', referrerPolicy: 'no-referrer' })
      const data = await response.json()
      if (!response.ok || typeof data?.contentBase64 !== 'string') throw new Error('unavailable')
      const bytes = Uint8Array.from(atob(data.contentBase64), character => character.charCodeAt(0))
      const url = URL.createObjectURL(new Blob([bytes], { type: data.contentType || 'application/octet-stream' }))
      const link = document.createElement('a')
      link.href = url
      link.download = data.filename || attachment.filename || 'attachment'
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch {
      this.downloadError = 'This attachment is unavailable. The link may have expired or been revoked.'
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {string} path */
  apiUrl(path) {
    const base = String(window.CADUCEUS_CONFIG?.apiUrl || '').replace(/\/$/, '')
    return `${base}${path}`
  }
}

/** @param {unknown} value */
function formatBytes(value) {
  const bytes = Number(value) || 0
  if (bytes < 1024) return `${formatNumber(bytes)} B`
  if (bytes < 1024 * 1024) {
    const digits = bytes < 10 * 1024 ? 1 : 0
    return `${formatNumber(bytes / 1024, { minimumFractionDigits: digits, maximumFractionDigits: digits })} KB`
  }
  return `${formatNumber(bytes / (1024 * 1024), { minimumFractionDigits: 1, maximumFractionDigits: 1 })} MB`
}

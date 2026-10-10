export class SearchMatcher {
  static #contains(value, needle) { return String(value ?? '').toLowerCase().includes(needle) }

  /** @param {string} raw @param {Date} now */
  static #parseSearchDate(raw, now) {
    const value = raw.trim().toLowerCase().replace(/^>=\s*/, '')
    const date = new Date(now)
    if (value === 'yesterday') { date.setDate(date.getDate() - 1); return SearchMatcher.#startOfDay(date) }
    const days = value.match(/^(\d+)\s+days?\s+ago$/)
    if (days) { date.setDate(date.getDate() - Number(days[1])); return SearchMatcher.#startOfDay(date) }
    if (value === 'last week') { date.setDate(date.getDate() - 7); return SearchMatcher.#startOfDay(date) }
    if (value === 'last month') { date.setMonth(date.getMonth() - 1); return SearchMatcher.#startOfDay(date) }
    const timestamp = Date.parse(value.replaceAll('/', '-'))
    return Number.isFinite(timestamp) ? new Date(timestamp) : null
  }

  /** @param {Date} date */
  static #startOfDay(date) { date.setHours(0, 0, 0, 0); return date }

  constructor(email, attachments, now) {
    this.email = email || {}; this.attachments = Array.isArray(attachments) ? attachments : []
    this.now = now instanceof Date ? now : new Date()
  }

  /** @param {SearchExpression|null} expression */
  matches(expression) {
    if (!expression) return true
    if (expression.type === 'and') return this.matches(expression.left) && this.matches(expression.right)
    if (expression.type === 'or') return this.matches(expression.left) || this.matches(expression.right)
    if (expression.type === 'not') return !this.matches(expression.expression)
    if (expression.type === 'term') return this.matchesTerm(expression.field, expression.value)
    return false
  }

  matchesTerm(field, rawValue) {
    const value = String(rawValue || '').toLowerCase()
    if (field === 'from') return SearchMatcher.#contains(this.email.sender, value)
    if (field === 'to') return this.recipientText().some(item => SearchMatcher.#contains(item, value))
    if (field === 'subject') return SearchMatcher.#contains(this.email.subject, value)
    if (field === 'body') return [this.email.body, this.email.htmlBody].some(item => SearchMatcher.#contains(item, value))
    if (field === 'in') return this.matchesCategory(value)
    if (field === 'is') return this.matchesStatus(value)
    if (field === 'has') return ['attachment', 'attachments'].includes(value) && this.attachments.length > 0
    if (field === 'before' || field === 'after' || field === 'since') return this.matchesDate(field, value)
    if (field === 'attachment' || field === 'filename') return this.attachments.some(item => SearchMatcher.#contains(item.filename, value))
    if (field === 'text') return this.fullText().some(item => SearchMatcher.#contains(item, value))
    return this.fullText().some(item => SearchMatcher.#contains(item, `${field}:${value}`))
  }

  recipientText() { return [this.email.recipient, ...(this.email.to || []), ...(this.email.cc || []), ...(this.email.bcc || [])] }
  fullText() { return [this.email.sender, ...this.recipientText(), this.email.subject, this.email.snippet, this.email.body, this.email.htmlBody, ...this.attachments.map(item => item.filename)] }

  matchesCategory(value) {
    if (value === 'all') return true
    const folder = String(this.email.systemFolder || this.email.folder || 'inbox').toLowerCase()
    const labels = Array.isArray(this.email.labels) ? this.email.labels.map(item => String(item).toLowerCase()) : []
    return folder === value || labels.includes(value)
  }

  matchesStatus(value) {
    if (value === 'read') return this.email.read === true
    if (value === 'unread') return this.email.read !== true
    if (value === 'starred') return this.email.starred === true
    if (value === 'unstarred') return this.email.starred !== true
    return false
  }

  matchesDate(field, value) {
    const boundary = SearchMatcher.#parseSearchDate(value, this.now)
    if (!boundary) return true
    const timestamp = Date.parse(String(this.email.receivedAt || ''))
    if (!Number.isFinite(timestamp)) return false
    return field === 'before' ? timestamp < boundary.getTime() : timestamp > boundary.getTime()
  }
}

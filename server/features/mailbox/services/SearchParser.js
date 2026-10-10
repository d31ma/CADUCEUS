class SearchLexer {
  /** @param {string} source */
  constructor(source) { this.source = source; this.offset = 0 }

  scan() {
    const tokens = []
    while (this.offset < this.source.length) {
      const token = this.next()
      if (token) tokens.push(token)
    }
    tokens.push({ type: 'eof', value: '' })
    return tokens
  }

  next() {
    this.skipWhitespace()
    if (this.offset >= this.source.length) return null
    const character = this.source[this.offset]
    if (character === '(') return this.single('leftParen')
    if (character === ')') return this.single('rightParen')
    if (character === ':') return this.single('colon')
    if (character === '"') return this.quoted()
    return this.word()
  }

  skipWhitespace() { while (/\s/.test(this.source[this.offset] || '')) this.offset += 1 }
  /** @param {string} type */
  single(type) { return { type, value: this.source[this.offset++] } }

  quoted() {
    const start = ++this.offset
    while (this.offset < this.source.length && this.source[this.offset] !== '"') this.offset += 1
    if (this.offset >= this.source.length) throw new SyntaxError('Expected closing quote for quoted search term')
    const value = this.source.slice(start, this.offset)
    this.offset += 1
    return { type: 'text', value }
  }

  word() {
    const start = this.offset
    while (this.offset < this.source.length && !/[\s():]/.test(this.source[this.offset])) this.offset += 1
    if (start === this.offset) throw new SyntaxError(`Unexpected search character ${this.source[this.offset]}`)
    return { type: 'text', value: this.source.slice(start, this.offset) }
  }
}

class StructuredSearchParser {
  /** @param {Array<{type:string,value:string}>} tokens */
  constructor(tokens) { this.tokens = tokens; this.offset = 0 }

  parse() {
    if (this.peek().type === 'eof') return null
    const expression = this.parseAnd()
    if (this.peek().type !== 'eof') throw new SyntaxError(`Unexpected search token ${this.peek().value}`)
    return expression
  }

  parseAnd() {
    const left = this.parseOr()
    if (['eof', 'rightParen'].includes(this.peek().type)) return left
    if (this.wordIs('AND')) this.take()
    if (this.wordIs('NOT')) {
      this.take()
      return { type: 'and', left, right: { type: 'not', expression: this.parseAnd() } }
    }
    return { type: 'and', left, right: this.parseAnd() }
  }

  parseOr() {
    const left = this.parsePrimary()
    if (!this.wordIs('OR')) return left
    this.take()
    return { type: 'or', left, right: this.parseOr() }
  }

  parsePrimary() {
    if (this.wordIs('NOT')) { this.take(); return { type: 'not', expression: this.parsePrimary() } }
    if (this.peek().type !== 'leftParen') return this.parseTerm()
    this.take()
    const expression = this.parseAnd()
    if (this.peek().type !== 'rightParen') throw new SyntaxError('Expected ) to close search group')
    this.take()
    return expression
  }

  parseTerm() {
    const first = this.takeText('Expected a search term')
    if (this.peek().type !== 'colon') return { type: 'term', field: 'text', value: first }
    this.take()
    return { type: 'term', field: first.toLowerCase(), value: this.takeText(`Expected a value after ${first}:`) }
  }

  /** @param {string} message */
  takeText(message) { if (this.peek().type !== 'text') throw new SyntaxError(message); return this.take().value }
  /** @param {string} value */
  wordIs(value) { return this.peek().type === 'text' && this.peek().value.toUpperCase() === value }
  peek() { return this.tokens[this.offset] }
  take() { return this.tokens[this.offset++] }
}

class SearchMatcher {
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
    if (field === 'from') return contains(this.email.sender, value)
    if (field === 'to') return this.recipientText().some(item => contains(item, value))
    if (field === 'subject') return contains(this.email.subject, value)
    if (field === 'body') return [this.email.body, this.email.htmlBody].some(item => contains(item, value))
    if (field === 'in') return this.matchesCategory(value)
    if (field === 'is') return this.matchesStatus(value)
    if (field === 'has') return ['attachment', 'attachments'].includes(value) && this.attachments.length > 0
    if (field === 'before' || field === 'after' || field === 'since') return this.matchesDate(field, value)
    if (field === 'attachment' || field === 'filename') return this.attachments.some(item => contains(item.filename, value))
    if (field === 'text') return this.fullText().some(item => contains(item, value))
    return this.fullText().some(item => contains(item, `${field}:${value}`))
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
    const boundary = parseSearchDate(value, this.now)
    if (!boundary) return true
    const timestamp = Date.parse(String(this.email.receivedAt || ''))
    if (!Number.isFinite(timestamp)) return false
    return field === 'before' ? timestamp < boundary.getTime() : timestamp > boundary.getTime()
  }
}

/** @param {unknown} value @param {string} needle */
function contains(value, needle) { return String(value ?? '').toLowerCase().includes(needle) }

/** @param {string} raw @param {Date} now */
function parseSearchDate(raw, now) {
  const value = raw.trim().toLowerCase().replace(/^>=\s*/, '')
  const date = new Date(now)
  if (value === 'yesterday') { date.setDate(date.getDate() - 1); return startOfDay(date) }
  const days = value.match(/^(\d+)\s+days?\s+ago$/)
  if (days) { date.setDate(date.getDate() - Number(days[1])); return startOfDay(date) }
  if (value === 'last week') { date.setDate(date.getDate() - 7); return startOfDay(date) }
  if (value === 'last month') { date.setMonth(date.getMonth() - 1); return startOfDay(date) }
  const timestamp = Date.parse(value.replaceAll('/', '-'))
  return Number.isFinite(timestamp) ? new Date(timestamp) : null
}
/** @param {Date} date */
function startOfDay(date) { date.setHours(0, 0, 0, 0); return date }

export class SearchParser {
  /** @param {string} raw @returns {SearchExpression|null} */
  static parse(raw) {
    if (typeof raw !== 'string' || !raw.trim()) return null
    return /** @type {SearchExpression|null} */ (new StructuredSearchParser(new SearchLexer(raw).scan()).parse())
  }

  /** @param {SearchExpression|null} expression @param {Record<string,any>} email @param {Array<Record<string,any>>} [attachments] @param {{now?:Date}} [options] */
  static matches(expression, email, attachments = [], options = {}) {
    return new SearchMatcher(email, attachments, options.now).matches(expression)
  }

  /** @param {string} raw @returns {SearchExpression|null} */
  static parseWithFallback(raw) {
    try { return SearchParser.parse(raw) }
    catch {
      return raw.trim()
        ? /** @type {SearchExpression} */ ({ type: 'term', field: 'text', value: raw.trim() })
        : null
    }
  }

  /** @param {SearchExpression|null} expression @param {string} field */
  static hasField(expression, field) {
    if (!expression) return false
    if (expression.type === 'term') return expression.field === String(field || '').toLowerCase()
    if (expression.type === 'not') return SearchParser.hasField(expression.expression, field)
    if (expression.type === 'and' || expression.type === 'or') {
      return SearchParser.hasField(expression.left, field) || SearchParser.hasField(expression.right, field)
    }
    return false
  }
}

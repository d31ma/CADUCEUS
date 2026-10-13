export class SearchLexer {
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

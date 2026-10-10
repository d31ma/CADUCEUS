export class StructuredSearchParser {
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

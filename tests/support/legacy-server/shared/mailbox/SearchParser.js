import { SearchMatcher } from './SearchMatcher.js'
import { StructuredSearchParser } from './StructuredSearchParser.js'
import { SearchLexer } from './SearchLexer.js'
export class SearchParser {
  /** @param {unknown} value @param {string} needle */

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

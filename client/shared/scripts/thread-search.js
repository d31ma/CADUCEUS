// @ts-check

const TOKEN_NAMES = ['from', 'to', 'in', 'before', 'since', 'after', 'is', 'has', 'subject']
const TOKEN_PATTERN = /(^|\s)(i[ns]?|s[iu]?[nb]?[cj]?e?c?t?|fr?o?m?|to?|ha?s?|be?f?o?r?e?|af?t?e?r?)(?::? ?$|: ?("[^"]*"?|[^\s]+))/gi
const DEFINITIONS = [
  { token: 'from', description: 'an email address' },
  { token: 'to', description: 'an email address' },
  { token: 'in', description: 'folder or label' },
  { token: 'before', description: 'date received or range' },
  { token: 'since', description: 'date received or range' },
  { token: 'after', description: 'date received or range', hidden: true },
  { token: 'is', description: 'unread, read, starred, unstarred' },
  { token: 'has', description: 'attachment' },
  { token: 'subject', description: 'an email subject' },
]
const DATE_VALUES = ['yesterday', '2 days ago', 'last week', 'last month', '>=2018/05/31']

/** @param {string} value */
export function wrapSearchTerm(value) {
  return `"${String(value || '').replaceAll('"', '').trim()}"`
}

/** @param {string} query @param {number} insertionIndex */
export function getCurrentTokenAndTerm(query, insertionIndex) {
  const normalized = String(query || '').replace(/\s/g, ' ')
  const regexp = new RegExp(TOKEN_PATTERN.source, TOKEN_PATTERN.flags)
  let match
  while ((match = regexp.exec(normalized))) {
    const prefixLength = match[1]?.length || 0
    const index = match.index + prefixLength
    const length = match[0].length - prefixLength
    if (index <= insertionIndex && index + length >= insertionIndex) {
      return {
        token: match[2].toLowerCase(),
        term: (match[3] || '').replace(/"$/, '').replace(/^"/, '').toLowerCase(),
        index, length,
      }
    }
  }
  return { token: null, term: null, index: -1, length: 0 }
}

/** @param {string} query */
export function searchTokenSegments(query) {
  const text = String(query || '')
  const pattern = new RegExp(`\\b(${TOKEN_NAMES.join('|')}):`, 'gi')
  const segments = []
  let offset = 0
  let match
  while ((match = pattern.exec(text))) {
    if (match.index > offset) segments.push({ text: text.slice(offset, match.index), token: false })
    segments.push({ text: match[0], token: true })
    offset = match.index + match[0].length
  }
  if (offset < text.length) segments.push({ text: text.slice(offset), token: false })
  return segments
}

/** @param {string} query @param {number} insertionIndex */
export function staticSearchSuggestions(query, insertionIndex) {
  const text = String(query || '')
  if (!text) return DEFINITIONS.filter(item => !item.hidden).map(item => ({ ...item, term: '', submit: false }))
  const current = getCurrentTokenAndTerm(text, insertionIndex)
  if (!current.token) return []
  const exact = DEFINITIONS.find(item => item.token === current.token)
  if (!exact) {
    return DEFINITIONS.filter(item => item.token.startsWith(current.token) && !item.hidden)
      .map(item => ({ ...item, term: '', submit: false }))
  }
  const values = exact.token === 'is'
    ? ['unread', 'read', 'starred', 'unstarred']
    : exact.token === 'has'
      ? ['attachment']
      : ['before', 'since', 'after'].includes(exact.token) ? DATE_VALUES : []
  return values.filter(value => !current.term || (value.startsWith(current.term) && value !== current.term))
    .map(value => ({ token: exact.token, term: value.includes(' ') ? wrapSearchTerm(value) : value, description: value, submit: true }))
}

/** @param {string} query @param {number} insertionIndex @param {{token?:string|null,term:string,submit?:boolean}} suggestion */
export function applySearchSuggestion(query, insertionIndex, suggestion) {
  const text = String(query || '')
  let { index, length } = getCurrentTokenAndTerm(text, insertionIndex)
  if (index === -1 && text.endsWith(' ')) { index = text.length; length = 0 }
  const expression = suggestion.token ? `${suggestion.token}:${suggestion.term}` : suggestion.term
  const next = index === -1
    ? expression
    : [text.slice(0, index).trim(), expression, text.slice(index + length).trim()]
      .filter(Boolean).join(' ')
  return { query: next.endsWith(' ') ? next : `${next} `, submit: Boolean(suggestion.submit) }
}

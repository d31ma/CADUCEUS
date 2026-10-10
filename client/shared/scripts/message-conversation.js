// @ts-check

const QUOTE_START = [
  /^\s*>/,
  /^\s*On .{1,240} wrote:\s*$/i,
  /^\s*-{2,}\s*(?:Original|Forwarded) Message\s*-{2,}\s*$/i,
  /^\s*Begin forwarded message:\s*$/i,
  /^\s*_{5,}\s*$/,
]

const MAX_SEARCH_MATCHES = 1000

/**
 * Split the visible reply from a trailing plain-text quotation without
 * discarding either side. This deliberately recognizes only established mail
 * delimiters; arbitrary prose containing “From:” or “On” remains visible.
 *
 * @param {unknown} value
 * @returns {{ visible: string, quoted: string, hasQuoted: boolean }}
 */
export function splitQuotedText(value) {
  const text = typeof value === 'string' ? value.replace(/\r\n?/g, '\n') : ''
  if (!text) return { visible: '', quoted: '', hasQuoted: false }
  const lines = text.split('\n')
  const index = lines.findIndex(line => QUOTE_START.some(pattern => pattern.test(line)))
  if (index < 0) return { visible: text, quoted: '', hasQuoted: false }

  const visible = lines.slice(0, index).join('\n').trimEnd()
  const quoted = lines.slice(index).join('\n').trim()
  if (!quoted) return { visible: text, quoted: '', hasQuoted: false }
  return { visible, quoted, hasQuoted: true }
}

/** @param {unknown} value */
export function isForwardedSubject(value) {
  return /^\s*fwd?\s*:/i.test(String(value || ''))
}

/**
 * Build bounded, case-insensitive match segments for every visible field in a
 * conversation. Segment text is always plain text; callers render it with
 * ordinary text interpolation and wrap only the explicit match segments.
 *
 * @param {Array<Record<string, any>>} messages
 * @param {unknown} rawTerm
 * @param {number} [requestedIndex]
 */
export function searchConversation(messages, rawTerm, requestedIndex = 0) {
  const term = String(rawTerm || '').trim().slice(0, 200)
  let nextIndex = 0
  const results = []

  const decorate = (message) => {
    const fields = {}
    for (const [name, rawValue] of [
      ['sender', message.sender],
      ['recipient', message.recipient],
      ['subject', message.subject],
      ['body', message.body],
    ]) {
      const text = String(rawValue || '').slice(0, 1024 * 1024)
      fields[name] = segmentMatches(text, term, () => {
        if (nextIndex >= MAX_SEARCH_MATCHES) return -1
        const index = nextIndex++
        results.push({ index, messageId: String(message.id || ''), field: name })
        return index
      })
    }
    return {
      ...message,
      findSenderSegments: fields.sender,
      findRecipientSegments: fields.recipient,
      findSubjectSegments: fields.subject,
      findBodySegments: fields.body,
    }
  }

  let decorated = (Array.isArray(messages) ? messages : []).map(decorate)
  const index = results.length
    ? ((Math.trunc(requestedIndex) % results.length) + results.length) % results.length
    : -1
  decorated = decorated.map(message => ({
    ...message,
    findSenderSegments: markCurrent(message.findSenderSegments, index),
    findRecipientSegments: markCurrent(message.findRecipientSegments, index),
    findSubjectSegments: markCurrent(message.findSubjectSegments, index),
    findBodySegments: markCurrent(message.findBodySegments, index),
  }))
  return { term, messages: decorated, results, index, count: results.length }
}

/** @param {string} text @param {string} term @param {() => number} next */
function segmentMatches(text, term, next) {
  if (!text || !term) return text ? [{ text, match: false, index: -1, current: false }] : []
  const haystack = text.toLocaleLowerCase()
  const needle = term.toLocaleLowerCase()
  const segments = []
  let cursor = 0
  while (cursor < text.length && segments.length < 2000) {
    const found = haystack.indexOf(needle, cursor)
    if (found < 0) break
    const index = next()
    if (index < 0) break
    if (found > cursor) segments.push({ text: text.slice(cursor, found), match: false, index: -1, current: false })
    segments.push({ text: text.slice(found, found + term.length), match: true, index, current: false })
    cursor = found + Math.max(term.length, 1)
  }
  if (cursor < text.length) segments.push({ text: text.slice(cursor), match: false, index: -1, current: false })
  return segments
}

/** @param {Array<Record<string, any>>} segments @param {number} currentIndex */
function markCurrent(segments, currentIndex) {
  return segments.map(segment => ({ ...segment, current: segment.index === currentIndex }))
}

// @ts-check

const HIGHLIGHT_NAME = 'caduceus-grammar-errors'

/** Return the exact text sent to the local checker. */
export function grammarText(root) {
  return Boolean(root?.nodeType) ? root.textContent || '' : ''
}

/** Keep only bounded matches whose offsets fit the current editor text. */
export function normalizeGrammarMatches(value, textLength) {
  if (!Array.isArray(value)) return []
  return value.flatMap(raw => {
    const offset = Number(raw?.offset)
    const length = Number(raw?.length)
    if (!Number.isInteger(offset) || !Number.isInteger(length) || offset < 0 || length < 1 || offset + length > textLength) return []
    const replacements = Array.isArray(raw?.replacements)
      ? raw.replacements.filter(item => typeof item === 'string').slice(0, 5)
      : []
    const ruleId = typeof raw?.ruleId === 'string' && /^[A-Z0-9_-]{1,64}$/.test(raw.ruleId) ? raw.ruleId : ''
    if (!ruleId) return []
    return [{
      offset,
      length,
      message: safeText(raw?.message, 300) || 'Grammar issue',
      shortMessage: safeText(raw?.shortMessage, 80) || 'Grammar',
      category: safeText(raw?.category, 40),
      ruleId,
      replacements,
      context: raw?.context && typeof raw.context === 'object' ? {
        text: safeText(raw.context.text, 500),
        offset: Number(raw.context.offset) || 0,
        length: Number(raw.context.length) || length,
      } : { text: '', offset: 0, length },
    }]
  }).slice(0, 100)
}

/** Resolve plain-text offsets to a DOM Range without changing editor markup. */
export function grammarRange(root, offset, length) {
  if (!(Boolean(root?.nodeType)) || !Number.isInteger(offset) || !Number.isInteger(length) || length < 1) return null
  const documentRef = root.ownerDocument || document
  const walker = documentRef.createTreeWalker(root, NodeFilter.SHOW_TEXT)
  let consumed = 0
  /** @type {Node | null} */
  let startNode = null
  let startOffset = 0
  /** @type {Node | null} */
  let endNode = null
  let endOffset = 0
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const size = node.textContent?.length || 0
    if (!startNode && offset <= consumed + size) {
      startNode = node
      startOffset = Math.max(0, offset - consumed)
    }
    if (startNode && offset + length <= consumed + size) {
      endNode = node
      endOffset = Math.max(0, offset + length - consumed)
      break
    }
    consumed += size
  }
  if (!startNode || !endNode) return null
  const range = documentRef.createRange()
  range.setStart(startNode, startOffset)
  range.setEnd(endNode, endOffset)
  return range
}

/**
 * Paint non-serializing CSS highlights when the browser supports them. The
 * highlight registry belongs to the window of the editor's document.
 * @param {Node} root
 * @param {{ offset: number, length: number }[]} matches
 */
export function paintGrammarHighlights(root, matches) {
  clearGrammarHighlights(root)
  const view = /** @type {any} */ (root?.ownerDocument?.defaultView ?? globalThis)
  if (!view.CSS?.highlights || typeof view.Highlight !== 'function') return false
  const ranges = matches.map(match => grammarRange(root, match.offset, match.length)).filter(Boolean)
  if (!ranges.length) return true
  view.CSS.highlights.set(HIGHLIGHT_NAME, new view.Highlight(...ranges))
  return true
}

/** @param {Node | null} [root] */
export function clearGrammarHighlights(root) {
  const view = /** @type {any} */ (root?.ownerDocument?.defaultView ?? globalThis)
  view.CSS?.highlights?.delete?.(HIGHLIGHT_NAME)
}

/**
 * Apply one correction to the editor through the browser's insertText
 * editing command, and leave the caret after it.
 * @param {HTMLElement} root
 * @param {{ offset: number, length: number }} match
 * @param {string} replacement
 */
export function replaceGrammarRange(root, match, replacement) {
  const range = grammarRange(root, match?.offset, match?.length)
  if (!range || typeof replacement !== 'string') return false
  const selection = root.ownerDocument.getSelection()
  selection?.removeAllRanges()
  selection?.addRange(range)
  root.focus()
  return root.ownerDocument.execCommand('insertText', false, replacement)
}

function safeText(value, max) {
  return typeof value === 'string'
    ? value.replace(/[\0-\x1f\x7f]/g, ' ').replace(/\s+/g, ' ').trim().slice(0, max)
    : ''
}

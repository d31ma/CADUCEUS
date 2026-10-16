// @ts-check

const TEMPLATE_CODE_RE = /<code\b([^>]*)>([\s\S]*?)<\/code\s*>/gi
const UNSAFE_BLOCK_RE = /<(script|style|iframe|frame|frameset|object|embed|applet|form|template|noscript)\b[^>]*>[\s\S]*?<\/\1\s*>/gi
const UNSAFE_SINGLE_RE = /<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*\/?\s*>/gi
const TEMPLATE_BOUNDARY_RE = /<(?:signature\b|blockquote\b[^>]*class\s*=\s*["'][^"']*gmail_quote)|<[^>]+class\s*=\s*["'][^"']*(?:mailspring-signature|caduceus-signature|gmail_quote|quoted-message)[^"']*["']/i

/** @param {string} attributes */
function isVariableAttributes(attributes) {
  return /\bdata-template-var(?:\s*=|\s|$)/i.test(attributes)
    || /\bclass\s*=\s*(["'])[^"']*\bvar\b[^"']*\1/i.test(attributes)
}

/** @param {string} html */
function visibleTemplateText(html) {
  return html
    .replace(/<br\s*\/?\s*>/gi, '')
    .replace(/<[^>]+>/g, '')
    .replace(/&(?:nbsp|#160|#x0*a0);/gi, ' ')
    .replace(/&(?:zwnj|zwj|#8204|#8205);/gi, '')
    .trim()
}

/**
 * Remove executable markup before owned template HTML enters the live composer.
 * The server applies the same policy at rest; this second boundary protects
 * older records and imported data.
 *
 * @param {unknown} input
 * @returns {string}
 */
export function sanitizeTemplateHTML(input) {
  return String(input ?? '')
    .replace(/\u0000/g, '')
    .replace(UNSAFE_BLOCK_RE, '')
    .replace(UNSAFE_SINGLE_RE, '')
    .replace(/\s+on[a-z0-9_-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/\s+(href|src|xlink:href)\s*=\s*(["'])\s*(?:javascript|vbscript):[\s\S]*?\2/gi, ' $1="#"')
}

/**
 * Add stable DOM hooks to Mailspring-compatible `<code class="var">` regions.
 * Empty regions remain visibly distinct until the user fills them.
 *
 * @param {string} input
 * @returns {string}
 */
export function normalizeTemplateVariables(input) {
  return sanitizeTemplateHTML(input).replace(TEMPLATE_CODE_RE, (whole, attributes, body) => {
    if (!isVariableAttributes(attributes)) return whole
    const classes = /\bclass\s*=\s*(["'])([^"']*)\1/i.exec(attributes)?.[2]
      ?.split(/\s+/).filter(Boolean) ?? []
    if (!classes.includes('var')) classes.push('var')
    const empty = !visibleTemplateText(body)
    const nextClasses = [...new Set(classes.filter(name => name !== 'empty'))]
    if (empty) nextClasses.push('empty')
    const withoutClass = attributes.replace(/\s*\bclass\s*=\s*(["'])[^"']*\1/i, '')
      .replace(/\s*\bdata-template-var(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+))?/i, '')
    return `<code${withoutClass} class="${nextClasses.join(' ')}" data-template-var="true">${body}</code>`
  })
}

/** @param {string} input @returns {boolean} */
export function hasEmptyTemplateVariables(input) {
  let empty = false
  normalizeTemplateVariables(input).replace(TEMPLATE_CODE_RE, (whole, attributes, body) => {
    if (isVariableAttributes(attributes) && !visibleTemplateText(body)) empty = true
    return whole
  })
  return empty
}

/**
 * Template highlighting is an authoring affordance and must never be sent.
 * Preserve the user's replacement text while unwrapping every variable node.
 *
 * @param {string} input
 * @returns {string}
 */
export function stripTemplateVariableMarkup(input) {
  return sanitizeTemplateHTML(input).replace(TEMPLATE_CODE_RE, (whole, attributes, body) => (
    isVariableAttributes(attributes) ? body : whole
  ))
}

/**
 * Replace only the editable portion of a reply, retaining its signature and
 * quoted conversation exactly as Mailspring does.
 *
 * @param {string} templateHTML
 * @param {string} currentHTML
 * @returns {string}
 */
export function insertTemplateBeforeProtectedContent(templateHTML, currentHTML) {
  const current = String(currentHTML || '')
  const boundary = current.search(TEMPLATE_BOUNDARY_RE)
  const protectedSuffix = boundary >= 0 ? current.slice(boundary) : ''
  return `${normalizeTemplateVariables(templateHTML)}${protectedSuffix}`
}

/** Remove quoted content and a signature before saving a draft as a template. */
export function templateBodyFromDraft(input) {
  const html = String(input || '')
  const boundary = html.search(TEMPLATE_BOUNDARY_RE)
  return sanitizeTemplateHTML(boundary >= 0 ? html.slice(0, boundary) : html).trim()
}

/** @param {ParentNode} root */
export function decorateTemplateVariables(root) {
  const nodes = Array.from(root.querySelectorAll('code.var, code[data-template-var]'))
  for (const node of nodes) {
    node.classList.add('var')
    node.setAttribute('data-template-var', 'true')
    node.setAttribute('title', 'Template blank — press Tab for the next blank')
    node.classList.toggle('empty', !(node.textContent || '').trim())
  }
  return nodes
}

/** Select a variable's contents so the next keystroke replaces the placeholder. */
export function selectTemplateVariable(node) {
  const selection = node.ownerDocument?.getSelection?.()
  if (!selection) return false
  const range = node.ownerDocument.createRange()
  range.selectNodeContents(node)
  selection.removeAllRanges()
  selection.addRange(range)
  node.scrollIntoView?.({ block: 'nearest' })
  return true
}

/**
 * Move to the next or previous template blank from the current selection.
 *
 * @param {HTMLElement} editor
 * @param {number} direction
 */
export function moveBetweenTemplateVariables(editor, direction = 1) {
  const variables = decorateTemplateVariables(editor)
  if (variables.length === 0) return false
  const anchor = editor.ownerDocument.getSelection?.()?.anchorNode ?? null
  const current = variables.findIndex(node => node === anchor || node.contains(anchor))
  const next = current < 0
    ? (direction < 0 ? variables.length - 1 : 0)
    : (current + direction + variables.length) % variables.length
  return selectTemplateVariable(variables[next])
}

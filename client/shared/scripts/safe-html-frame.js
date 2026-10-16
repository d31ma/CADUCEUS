/** @import { SafeHtmlRevision } from "../../models/SafeHtmlRevision.js" */
/** @import { SafeHtmlMetadataEntry } from "../../models/SafeHtmlMetadataEntry.js" */
/** @import { SafeHtmlMetadata } from "../../models/SafeHtmlMetadata.js" */
/** @import { SafeHtmlFramePackage } from "../../models/SafeHtmlFramePackage.js" */
/** @import { SafeHtmlFrameOptions } from "../../models/SafeHtmlFrameOptions.js" */
/** @import { ProjectedHtmlNode } from "../../models/ProjectedHtmlNode.js" */

export const SAFE_HTML_FRAME_TAGS = Object.freeze(['a', 'abbr', 'address', 'b', 'bdi', 'bdo', 'blockquote', 'br', 'caption', 'center', 'cite', 'code', 'col',
  'colgroup', 'dd', 'del', 'details', 'dfn', 'div', 'dl', 'dt', 'em', 'figcaption', 'figure', 'font', 'h1', 'h2', 'h3', 'h4', 'h5',
  'h6', 'hr', 'i', 'img', 'ins', 'kbd', 'li', 'mark', 'ol', 'p', 'pre', 'q', 's', 'samp', 'small', 'span', 'strike', 'strong', 'sub',
  'summary', 'sup', 'table', 'tbody', 'td', 'tfoot', 'th', 'thead', 'time', 'tr', 'tt', 'u', 'ul', 'var', 'wbr'])
const KEPT = new Set(SAFE_HTML_FRAME_TAGS)
const DROPPED = new Set(['script', 'style', 'iframe', 'frame', 'frameset', 'noframes', 'object', 'embed', 'applet', 'param',
  'template', 'noscript', 'noembed', 'svg', 'math', 'form', 'input', 'button', 'select', 'option', 'optgroup', 'textarea', 'datalist',
  'link', 'meta', 'base', 'title', 'head', 'video', 'audio', 'source', 'track', 'picture', 'canvas', 'dialog', 'slot', 'portal',
  'xmp', 'plaintext', 'listing', 'marquee', 'image'])
const ATTRIBUTES = new Set(['abbr', 'align', 'alt', 'bgcolor', 'border', 'cellpadding', 'cellspacing', 'cite', 'color', 'colspan',
  'datetime', 'dir', 'face', 'headers', 'height', 'lang', 'reversed', 'rowspan', 'scope', 'size', 'span', 'start', 'title', 'type',
  'valign', 'width'])
const STYLE_NAMES = /^(?:color|background-color|font(?:-[a-z]+)?|text-(?:align|decoration|indent|transform)|line-height|letter-spacing|word-spacing|white-space|vertical-align|(?:margin|padding)(?:-(?:top|right|bottom|left))?|border(?:-(?:top|right|bottom|left))?(?:-(?:color|style|width))?|border-(?:collapse|spacing|radius)|width|height|max-width|min-width|max-height|min-height|list-style-type|direction|display|table-layout)$/
const UNSAFE_STYLE = /url\s*\(|expression\s*\(|javascript:|@import|\\|<|>/i
const DATA_IMAGE = /^data:image\/(?:png|gif|jpeg|webp|avif);base64,[a-z0-9+/=\s]*$/i
const MOST_CHARACTERS = 1048576
const MOST_NODES = 20000

/** @param {string} tag @param {readonly string[]} wrappers */
export function safeHtmlFrameKeepsTag(tag, wrappers = []) {
  return KEPT.has(tag) || (['section', 'signature'].includes(tag) && wrappers.includes(tag))
}

/** @param {string} documentHtml @param {SafeHtmlRevision} revision @param {SafeHtmlFrameOptions} options @returns {SafeHtmlFramePackage} */
export function buildSafeHtmlFrame(documentHtml, revision, options = {}) {
  if (!['string', 'number'].includes(typeof revision) || (typeof revision === 'number' && !Number.isSafeInteger(revision))) throw new TypeError('Invalid frame revision.')
  const parsed = new DOMParser().parseFromString(documentHtml, 'text/html')
  if (documentHtml) options.prepare?.(parsed)
  const body = parsed.body.innerHTML
  const reparsed = new DOMParser().parseFromString(body, 'text/html')
  const projection = new FrameProjection(body, revision, options)
  projection.read(reparsed.body)
  return { body, metadata: { revision, entries: projection.entries } }
}

/** @param {string} html @param {readonly string[]} wrappers @returns {string} */
export function safeHtmlFrameContentKey(html, wrappers = []) {
  const parsed = new DOMParser().parseFromString(html, 'text/html')
  return JSON.stringify(new FrameProjection(html, '', { wrappers }).read(parsed.body))
}

class FrameProjection {
  /** @type {SafeHtmlMetadataEntry[]} */
  entries = []
  /** @type {Map<string,number>} */
  indexes = new Map()
  nodes = 0
  characters = 0
  /** @param {string} html @param {SafeHtmlRevision} revision @param {SafeHtmlFrameOptions} options */
  constructor(html, revision, options) {
    if (html.length > MOST_CHARACTERS) throw new RangeError('Tac safe-html content exceeds 1 MiB.')
    if ((options.wrappers || []).some(tag => !['section', 'signature'].includes(tag))) throw new TypeError('Invalid frame wrapper.')
    this.revision = revision
    this.options = options
    this.budget(String(revision))
  }
  /** @param {Element} body @returns {ProjectedHtmlNode[]} */
  read(body) { return [...body.childNodes].flatMap(node => this.node(node)) }
  /** @param {Node} source @returns {ProjectedHtmlNode[]} */
  node(source) {
    if (++this.nodes > MOST_NODES) throw new RangeError('Tac safe-html content exceeds 20,000 nodes.')
    if (source.nodeType === Node.TEXT_NODE) return [{ text: source.nodeValue ?? '' }]
    if (source.nodeType !== Node.ELEMENT_NODE || /** @type {Element} */ (source).namespaceURI !== 'http://www.w3.org/1999/xhtml') return []
    const element = /** @type {Element} */ (source)
    const tag = element.localName
    if (DROPPED.has(tag)) return []
    const kept = safeHtmlFrameKeepsTag(tag, this.options.wrappers)
    const index = this.indexes.get(tag) || 0
    if (kept) this.indexes.set(tag, index + 1)
    const children = [...source.childNodes].flatMap(node => this.node(node))
    if (!kept) return children
    const attributes = projectedAttributes(element)
    const approved = this.options.authorize?.(element)
    if (approved && Object.keys(approved).length) this.metadata(element, index, attributes, children, approved)
    return [{ tag, attributes, children }]
  }
  /** @param {Element} element @param {number} index @param {Record<string,string>} attributes @param {ProjectedHtmlNode[]} children @param {Record<string,string>} approved */
  metadata(element, index, attributes, children, approved) {
    const tag = element.localName
    const expected = { text: projectedText(children) }
    if (['a', 'img'].includes(tag)) {
      const name = tag === 'a' ? 'href' : 'src'
      if (typeof attributes[name] !== 'string') throw new TypeError('Trusted link/image metadata requires a sanitizer-safe URL.')
      Object.assign(expected, { [name]: attributes[name] })
    }
    const declared = this.options.metadataAttributes?.[tag] || []
    for (const [name, value] of Object.entries(approved)) {
      if (!declared.includes(name) || typeof value !== 'string' || !/^(?:class|contenteditable|aria-[a-z][a-z0-9-]*|data-[a-z][a-z0-9-]*)$/.test(name)
        || /^data-(?:tac|tachyon)/.test(name) || (name === 'contenteditable' && !['true', 'false', 'plaintext-only'].includes(value))) throw new TypeError('Undeclared or invalid trusted frame metadata.')
      this.budget(name); this.budget(value)
    }
    this.budget(tag)
    for (const value of Object.values(expected)) this.budget(value)
    this.entries.push({ tag, index, expected, attributes: { ...approved } })
  }
  /** @param {string} value */
  budget(value) { this.characters += value.length; if (this.characters > MOST_CHARACTERS) throw new RangeError('Tac safe-html metadata exceeds 1 MiB.') }
}

/** @param {ProjectedHtmlNode[]} nodes @returns {string} */
function projectedText(nodes) { return nodes.map(node => node.text ?? projectedText(node.children || [])).join('') }

/** @param {Element} element @returns {Record<string,string>} */
function projectedAttributes(element) {
  /** @type {Map<string,string>} */
  const values = new Map()
  for (const attribute of [...element.attributes]) if (ATTRIBUTES.has(attribute.name)) values.set(attribute.name, attribute.value)
  if (element.getAttribute('style')) {
    const style = /** @type {HTMLElement} */ (element).style
    const declarations = [...style].flatMap(name => {
      const value = style.getPropertyValue(name)
      return STYLE_NAMES.test(name) && !UNSAFE_STYLE.test(value) ? [`${name}: ${value}`] : []
    })
    if (declarations.length) values.set('style', declarations.join('; '))
  }
  const tag = element.localName
  const name = tag === 'a' ? 'href' : tag === 'img' ? 'src' : ''
  const raw = name ? element.getAttribute(name) : null
  if (name && raw !== null) {
    const safe = projectedUrl(raw, tag)
    if (safe !== null) {
      values.set(name, safe)
      if (tag === 'a') { values.set('rel', 'noopener noreferrer nofollow'); values.set('target', '_blank') }
    }
  }
  return Object.fromEntries([...values].sort(([a], [b]) => a.localeCompare(b)))
}

/** @param {string} value @param {string} tag @returns {string|null} */
function projectedUrl(value, tag) {
  const trimmed = value.trim()
  if (tag === 'img' && DATA_IMAGE.test(trimmed)) return trimmed
  if (/[\u0000-\u001f\u007f]/.test(trimmed) || /\s/.test(trimmed.match(/^[^:/?#]*:/)?.[0] || '')) return null
  try {
    const url = new URL(trimmed, location.href)
    return (tag === 'img' ? ['http:', 'https:'] : ['http:', 'https:', 'mailto:', 'tel:']).includes(url.protocol) ? url.href : null
  } catch { return null }
}

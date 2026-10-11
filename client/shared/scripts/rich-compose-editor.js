// @ts-check

import { formatPreferenceDate } from './mail-preferences.js'

/**
 * Rich-text editing for the composer's editor document.
 *
 * The editor is the body of a sandboxed iframe whose document the Go
 * ComposeService builds (see EditorUtils.go): Tac replaces a contenteditable
 * element whenever it renders without focus, and a Tac script may not build
 * markup or write styles (TY1364). Every change here therefore goes through
 * the browser's own editing commands (document.execCommand) and attribute
 * writes, and message HTML enters the live editor only through
 * insertEditorHTML.
 */

const INLINE_IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'])
const MAX_PASTED_HTML_LENGTH = 1_000_000
const INLINE_COMMANDS = /** @type {Record<string, string>} */ ({ STRONG: 'bold', EM: 'italic', U: 'underline', S: 'strikeThrough' })
const BLOCK_TAGS = new Set(['DIV', 'P', 'H1', 'H2', 'H3', 'BLOCKQUOTE', 'PRE', 'LI'])

/** Accept only links that are safe to place in outbound HTML. */
export function normalizeComposeLink(value) {
  let source = String(value || '').trim()
  if (!source) return ''
  if (/^www\./i.test(source)) source = `https://${source}`
  try {
    const url = new URL(source)
    return ['http:', 'https:', 'mailto:'].includes(url.protocol) ? url.href : ''
  } catch {
    return ''
  }
}

/** Match Mailspring's theme-safe treatment of source-page default colors. */
export function isNeutralPastedColor(value) {
  const normalized = String(value || '').toLowerCase().replace(/\s/g, '')
  if (['black', 'white', 'transparent'].includes(normalized)) return true

  /** @type {number[] | null} */
  let channels = null
  const shortHex = normalized.match(/^#([0-9a-f])([0-9a-f])([0-9a-f])$/i)
  const longHex = normalized.match(/^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i)
  const rgb = normalized.match(/^rgba?\((\d+(?:\.\d+)?),(\d+(?:\.\d+)?),(\d+(?:\.\d+)?)(?:,[^)]+)?\)$/i)
  if (shortHex) channels = shortHex.slice(1).map(channel => Number.parseInt(channel + channel, 16))
  else if (longHex) channels = longHex.slice(1).map(channel => Number.parseInt(channel, 16))
  else if (rgb) channels = rgb.slice(1).map(Number)
  if (!channels || channels.some(channel => !Number.isFinite(channel) || channel < 0 || channel > 255)) return false

  const darkest = Math.min(...channels)
  const lightest = Math.max(...channels)
  return lightest - darkest <= 12 && (lightest <= 32 || darkest >= 223)
}

/** Normalize every clipboard newline convention. */
export function normalizePlainTextForPaste(value) {
  return String(value || '').replace(/\r\n?/g, '\n')
}

/**
 * Sanitize rich clipboard HTML on a detached document before it enters the
 * composer. Scripts, embeds, forms, images and event handlers go, links keep
 * only safe schemes, and neutral colors that turn unreadable across themes
 * go. Inline CSS goes too: rewriting a style attribute is a style write
 * (TY1364), so pasted text keeps its tags (bold, links, lists, headings) and
 * loses its inline CSS.
 * @param {string} value
 */
export function normalizePastedComposerHTML(value) {
  const parsed = new DOMParser().parseFromString(String(value || '').slice(0, MAX_PASTED_HTML_LENGTH), 'text/html')
  for (const node of parsed.body.querySelectorAll('script,style,iframe,object,embed,form,input,button,textarea,select,svg,math,link,meta,base,img')) node.remove()
  for (const element of parsed.body.querySelectorAll('*')) cleanPastedElement(element)
  return parsed.body.innerHTML
}

/** @param {Element} element */
function cleanPastedElement(element) {
  for (const attribute of Array.from(element.attributes)) {
    const name = attribute.name.toLowerCase()
    if (name.startsWith('on') || ['src', 'srcset', 'srcdoc', 'formaction', 'contenteditable', 'style', 'class', 'id'].includes(name)) element.removeAttribute(attribute.name)
  }
  if (isNeutralPastedColor(element.getAttribute('color'))) element.removeAttribute('color')
  const href = element.getAttribute('href')
  if (href == null) return
  const safeHref = normalizeComposeLink(href)
  if (safeHref) {
    element.setAttribute('href', safeHref)
    element.setAttribute('rel', 'noopener noreferrer')
  } else element.removeAttribute('href')
}

/** @param {{ type?: string }} file */
export function isInlineImageFile(file) {
  return INLINE_IMAGE_TYPES.has(String(file?.type || '').toLowerCase())
}

/** Create a normalized, globally unique RFC Content-ID. */
export function createInlineContentId(uuid = crypto.randomUUID()) {
  return `caduceus.${String(uuid).replace(/[^a-z0-9.-]/gi, '')}@inline`
}

/** Escape untrusted message metadata and text before inserting a reply quote. */
export function quotedMessageHTML(message, mode = 'reply') {
  const sender = escapeHTML(message?.sender || '')
  const subject = escapeHTML(message?.subject || '')
  const date = message?.receivedAt ? escapeHTML(formatPreferenceDate(message.receivedAt)) : ''
  const body = escapeHTML(message?.body || '').replace(/\r?\n/g, '<br>')
  const heading = mode === 'forward'
    ? `<p><strong>---------- Forwarded message ----------</strong><br>From: ${sender}<br>Date: ${date}<br>Subject: ${subject}</p>`
    : `<p>On ${date}, ${sender} wrote:</p>`
  return `<div><br></div><section data-caduceus-quote="${mode}">${heading}<blockquote>${body}</blockquote></section>`
}

/**
 * The one way message HTML enters the live editor: the browser's own
 * insertHTML editing command at the current selection.
 * @param {HTMLElement} editor
 * @param {string} html
 */
export function insertEditorHTML(editor, html) {
  focusEditor(editor)
  return editor.ownerDocument.execCommand('insertHTML', false, html)
}

/**
 * Toggle bold, italic, underline or strikethrough, or wrap the selection in
 * inline code.
 * @param {HTMLElement} editor
 * @param {string} tagName
 */
export function applyInlineFormat(editor, tagName) {
  const tag = String(tagName || '').toUpperCase()
  if (!editorRange(editor)) return false
  if (INLINE_COMMANDS[tag]) return command(editor, INLINE_COMMANDS[tag])
  const text = editor.ownerDocument.getSelection()?.toString() || '​'
  return insertEditorHTML(editor, `<code>${escapeHTML(text)}</code>`)
}

/**
 * Apply a font family, a size step (1-7) or a text color to the selection.
 * @param {HTMLElement} editor
 * @param {'fontName' | 'fontSize' | 'foreColor'} name
 * @param {string} value
 */
export function applyStyleCommand(editor, name, value) {
  if (!editorRange(editor)) return false
  editor.ownerDocument.execCommand('styleWithCSS', false, 'true')
  return command(editor, name, value)
}

/**
 * Apply paragraph, heading, quote or code-block formatting, or toggle the
 * direction of the current block.
 * @param {HTMLElement} editor
 * @param {string} tagName
 * @param {{ dir?: boolean }} [options]
 */
export function applyBlockFormat(editor, tagName, options = {}) {
  const range = editorRange(editor)
  if (!range) return false
  if (!options.dir) return command(editor, 'formatBlock', `<${String(tagName || 'p').toLowerCase()}>`)
  const block = closestBlock(range.startContainer, editor) ?? editor
  if (block.getAttribute('dir') === 'rtl') block.removeAttribute('dir')
  else block.setAttribute('dir', 'rtl')
  return true
}

/**
 * Toggle the current block between a list and a paragraph.
 * @param {HTMLElement} editor
 * @param {boolean} ordered
 */
export function applyListFormat(editor, ordered = false) {
  if (!editorRange(editor)) return false
  return command(editor, ordered ? 'insertOrderedList' : 'insertUnorderedList')
}

/**
 * Turn a just-typed Markdown line prefix into block formatting.
 * @param {HTMLElement} editor
 */
export function applyMarkdownShortcut(editor) {
  const range = editorRange(editor)
  if (!range || !range.collapsed || range.startContainer.nodeType !== Node.TEXT_NODE) return false
  const before = /** @type {Text} */ (range.startContainer).data.slice(0, range.startOffset).replace(/​/g, '')
  const match = before.match(/^(#{1,3}|>|[-*]|1\.)$/)
  if (!match) return false
  selectBeforeCaret(range, range.startOffset)
  command(editor, 'delete')
  if (match[1] === '>') return applyBlockFormat(editor, 'blockquote')
  if (match[1] === '-' || match[1] === '*') return applyListFormat(editor, false)
  if (match[1] === '1.') return applyListFormat(editor, true)
  return applyBlockFormat(editor, `h${match[1].length}`)
}

/**
 * Convert completed inline Markdown marks at the caret.
 * @param {HTMLElement} editor
 */
export function applyInlineMarkdownShortcut(editor) {
  const range = editorRange(editor)
  if (!range?.collapsed || range.startContainer.nodeType !== Node.TEXT_NODE) return false
  const before = /** @type {Text} */ (range.startContainer).data.slice(0, range.startOffset)
  const match = before.match(/(?:^|\s)(\*\*|__)([^\n]+?)\1$|(?:^|\s)(\*|_)([^\n*]+?)\3$|(?:^|\s)`([^\n`]+?)`$/)
  if (!match) return false
  const marker = match[1] || match[3] || '`'
  const content = match[2] || match[4] || match[5]
  const marked = match[0].replace(/^\s/, '')
  const tag = marker === '`' ? 'code' : marker.length === 2 ? 'strong' : 'em'
  selectBeforeCaret(range, marked.length)
  return insertEditorHTML(editor, `<${tag}>${escapeHTML(content)}</${tag}>​`)
}

/**
 * Link the URL or email immediately before the caret on Space or Enter.
 * @param {HTMLElement} editor
 */
export function autoLinkBeforeCaret(editor) {
  const range = editorRange(editor)
  if (!range?.collapsed || range.startContainer.nodeType !== Node.TEXT_NODE) return false
  const node = /** @type {Text} */ (range.startContainer)
  const caret = range.startOffset
  const token = node.data.slice(0, caret).match(/(?:^|\s)((?:https?:\/\/|www\.)[^\s<]+|[\w.+-]+@[\w.-]+\.[a-z]{2,})$/i)?.[1]
  if (!token) return false
  const href = normalizeComposeLink(token.includes('@') && !/^https?:|^www\./i.test(token) ? `mailto:${token}` : token)
  if (!href) return false
  selectBeforeCaret(range, token.length)
  linkSelection(editor, href)
  collapseAt(editor, node, Math.min(caret, node.length))
  return true
}

/**
 * Turn the selection into a link.
 * @param {HTMLElement} editor
 * @param {string} href a normalized link (normalizeComposeLink)
 */
export function linkSelection(editor, href) {
  command(editor, 'createLink', href)
  const anchor = editor.ownerDocument.getSelection()?.anchorNode?.parentElement?.closest?.('a')
  for (const link of anchor && editor.contains(anchor) ? [anchor] : editor.querySelectorAll(`a[href="${CSS.escape(href)}"]:not([rel])`)) {
    link.setAttribute('rel', 'noopener noreferrer')
    link.setAttribute('target', '_blank')
  }
  return true
}

/**
 * Insert a locally visible image whose outbound representation uses cid:.
 * @param {HTMLElement} editor
 * @param {{ id?: string, contentId?: string, contentType?: string, contentBase64?: string, filename?: string }} attachment
 */
export function insertInlineImage(editor, attachment) {
  const contentId = String(attachment?.contentId || '')
  if (!contentId || !isInlineImageFile({ type: attachment?.contentType })) return false
  if (!editorRange(editor)) collapseAt(editor, editor, editor.childNodes.length)
  const source = `data:${escapeHTML(String(attachment.contentType))};base64,${String(attachment.contentBase64 || '').replace(/[^A-Za-z0-9+/=]/g, '')}`
  return insertEditorHTML(editor, `<img src="${source}" data-caduceus-cid="${escapeHTML(contentId)}" data-caduceus-attachment-id="${escapeHTML(String(attachment.id || ''))}" data-caduceus-inline-image="" alt="${escapeHTML(String(attachment.filename || 'Inline image'))}" width="480">`)
}

/**
 * Insert normalized clipboard content at the current editor selection.
 * @param {HTMLElement} editor
 * @param {string} html
 * @param {string} plainText
 */
export function insertPastedComposerContent(editor, html, plainText) {
  if (!editorRange(editor)) return false
  const safeHTML = html ? normalizePastedComposerHTML(html) : ''
  if (safeHTML.trim()) return insertEditorHTML(editor, safeHTML)
  const text = normalizePlainTextForPaste(plainText)
  return Boolean(text) && command(editor, 'insertText', text)
}

/**
 * Serialize editor HTML without local data URLs.
 * @param {HTMLElement} editor
 */
export function serializeComposerHTML(editor) {
  const clone = /** @type {HTMLElement} */ (editor.cloneNode(true))
  const replacements = []
  let index = 0
  for (const image of clone.querySelectorAll('img[data-caduceus-cid]')) {
    const placeholder = `#caduceus-inline-${index++}`
    replacements.push([`src="${placeholder}"`, `src="cid:${escapeHTML(image.getAttribute('data-caduceus-cid') || '')}"`])
    // A placeholder, not the cid: URL: setting an unknown scheme on even a
    // disconnected image makes Chromium log a network error.
    image.setAttribute('src', placeholder)
    image.removeAttribute('data-caduceus-attachment-id')
    image.removeAttribute('data-selected')
  }
  let html = clone.innerHTML
  for (const [placeholder, cid] of replacements) html = html.replace(placeholder, cid)
  return html
}

/**
 * @param {HTMLImageElement | null} image
 * @param {number} width
 */
export function resizeInlineImage(image, width) {
  if (!image?.matches?.('img[data-caduceus-inline-image]')) return false
  image.setAttribute('width', String(Math.max(120, Math.min(960, Number(width) || 480))))
  return true
}

/** @param {HTMLElement} editor */
function focusEditor(editor) {
  editor.ownerDocument.defaultView?.focus()
  editor.focus()
}

/**
 * @param {HTMLElement} editor
 * @param {string} name
 * @param {string} [value]
 */
function command(editor, name, value) {
  focusEditor(editor)
  return editor.ownerDocument.execCommand(name, false, value)
}

/** @param {HTMLElement} editor */
function editorRange(editor) {
  const selection = editor.ownerDocument.getSelection()
  if (!selection?.rangeCount) return null
  const range = selection.getRangeAt(0)
  return editor.contains(range.commonAncestorContainer) ? range : null
}

/**
 * @param {Range} range
 * @param {number} length
 */
function selectBeforeCaret(range, length) {
  const node = range.startContainer
  const selected = node.ownerDocument?.createRange()
  if (!selected) return
  selected.setStart(node, Math.max(0, range.startOffset - length))
  selected.setEnd(node, range.startOffset)
  const selection = node.ownerDocument?.getSelection()
  selection?.removeAllRanges()
  selection?.addRange(selected)
}

/**
 * @param {HTMLElement} editor
 * @param {Node} node
 * @param {number} offset
 */
function collapseAt(editor, node, offset) {
  const range = editor.ownerDocument.createRange()
  range.setStart(node, offset)
  range.collapse(true)
  const selection = editor.ownerDocument.getSelection()
  selection?.removeAllRanges()
  selection?.addRange(range)
}

/**
 * @param {Node} node
 * @param {HTMLElement} editor
 */
function closestBlock(node, editor) {
  let element = node?.nodeType === Node.ELEMENT_NODE ? /** @type {Element} */ (node) : node?.parentElement
  while (element && element !== editor) {
    if (BLOCK_TAGS.has(element.tagName)) return element
    element = element.parentElement
  }
  return null
}

/** @param {string} value */
function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, character => /** @type {Record<string, string>} */ ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[character])
}

// @ts-check

import { formatPreferenceDate } from './mail-preferences.js'

const INLINE_IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'])
const BLOCK_TAGS = new Set(['DIV', 'P', 'H1', 'H2', 'H3', 'BLOCKQUOTE', 'PRE', 'LI'])
const TRANSPARENT_GIF = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw=='
const MAX_PASTED_HTML_LENGTH = 1_000_000
const PASTED_STYLE_ALLOWLIST = new Set([
  'color', 'direction', 'font-family', 'font-size', 'font-style', 'font-weight',
  'letter-spacing', 'line-height', 'text-align', 'text-decoration', 'text-indent',
  'text-transform', 'vertical-align', 'white-space',
])

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

/** Normalize every clipboard newline convention before creating editor nodes. */
export function normalizePlainTextForPaste(value) {
  return String(value || '').replace(/\r\n?/g, '\n')
}

/**
 * Sanitize rich clipboard HTML before it enters the live composer and remove
 * neutral foreground colors that become unreadable when sender and recipient
 * themes differ. Intentional colors and independent font-family styling stay.
 */
export function normalizePastedComposerHTML(value) {
  const template = document.createElement('template')
  template.innerHTML = String(value || '').slice(0, MAX_PASTED_HTML_LENGTH)
  for (const node of template.content.querySelectorAll('script,style,iframe,object,embed,form,input,button,textarea,select,svg,math,link,meta,base,img')) node.remove()
  for (const element of template.content.querySelectorAll('*')) {
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase()
      if (name.startsWith('on') || ['src', 'srcset', 'srcdoc', 'formaction', 'contenteditable'].includes(name)) element.removeAttribute(attribute.name)
    }
    const colorAttribute = element.getAttribute('color')
    if (colorAttribute && isNeutralPastedColor(colorAttribute)) element.removeAttribute('color')
    const href = element.getAttribute('href')
    if (href != null) {
      const safeHref = normalizeComposeLink(href)
      if (safeHref) {
        element.setAttribute('href', safeHref)
        element.setAttribute('rel', 'noopener noreferrer')
      } else element.removeAttribute('href')
    }
    const htmlElement = element instanceof HTMLElement ? element : null
    if (!htmlElement?.style) continue
    for (const property of Array.from(htmlElement.style)) {
      if (!PASTED_STYLE_ALLOWLIST.has(property)) htmlElement.style.removeProperty(property)
    }
    if (isNeutralPastedColor(htmlElement.style.color)) htmlElement.style.removeProperty('color')
    if (!htmlElement.style.length) htmlElement.removeAttribute('style')
  }
  return template.innerHTML
}

/** Insert normalized clipboard content at the current editor selection. */
export function insertPastedComposerContent(editor, html, plainText) {
  const range = editorRange(editor)
  if (!range) return false
  const safeHTML = html ? normalizePastedComposerHTML(html) : ''
  const text = normalizePlainTextForPaste(plainText)
  const fragment = safeHTML
    ? range.createContextualFragment(safeHTML)
    : plainTextFragment(text)
  if (!fragment.childNodes.length) return false
  const tail = fragment.lastChild
  range.deleteContents()
  range.insertNode(fragment)
  if (tail) {
    range.setStartAfter(tail)
    range.collapse(true)
    const selection = getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
  }
  return true
}

/** @param {string} text */
function plainTextFragment(text) {
  const fragment = document.createDocumentFragment()
  const lines = text.split('\n')
  lines.forEach((line, index) => {
    if (index) fragment.append(document.createElement('br'))
    if (line) fragment.append(document.createTextNode(line))
  })
  return fragment
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

/** Replace one editor selection with a semantic inline element. */
export function applyInlineFormat(editor, tagName, style = {}) {
  const range = editorRange(editor)
  if (!range) return false
  const tag = String(tagName || '').toUpperCase()
  const active = closestWithin(range.commonAncestorContainer, tag, editor)
  if (active && Object.keys(style).length === 0) {
    const parent = active.parentNode
    unwrap(active)
    selectNodeContents(parent, editor)
    return true
  }
  const wrapper = document.createElement(tag.toLowerCase())
  for (const [property, value] of Object.entries(style)) wrapper.style.setProperty(property, value)
  if (range.collapsed) {
    wrapper.append(document.createTextNode('\u200b'))
    range.insertNode(wrapper)
    selectNodeContents(wrapper, editor)
  } else {
    wrapper.append(range.extractContents())
    range.insertNode(wrapper)
    selectNodeContents(wrapper, editor)
  }
  return true
}

/** Apply paragraph, heading, quote, code-block, or direction formatting. */
export function applyBlockFormat(editor, tagName, attributes = {}) {
  const range = editorRange(editor)
  if (!range) return false
  const requested = String(tagName || 'p').toUpperCase()
  let block = closestBlock(range.startContainer, editor)
  if (!block) {
    block = document.createElement(requested.toLowerCase())
    if (range.collapsed) block.append(document.createElement('br'))
    else block.append(range.extractContents())
    range.insertNode(block)
  } else if (block.tagName !== requested) {
    const replacement = document.createElement(requested.toLowerCase())
    replacement.append(...block.childNodes)
    block.replaceWith(replacement)
    block = replacement
  }
  for (const [name, value] of Object.entries(attributes)) {
    if (value) block.setAttribute(name, value)
    else block.removeAttribute(name)
  }
  selectNodeContents(block, editor)
  return true
}

/** Toggle the current block between a semantic list and a paragraph. */
export function applyListFormat(editor, ordered = false) {
  const range = editorRange(editor)
  if (!range) return false
  const list = closestWithin(range.startContainer, ordered ? 'OL' : 'UL', editor)
  if (list) {
    const fragment = document.createDocumentFragment()
    for (const item of Array.from(list.children)) {
      const paragraph = document.createElement('p')
      paragraph.append(...item.childNodes)
      fragment.append(paragraph)
    }
    const last = fragment.lastChild
    list.replaceWith(fragment)
    selectNodeContents(last, editor)
    return true
  }
  const block = closestBlock(range.startContainer, editor)
  const container = document.createElement(ordered ? 'ol' : 'ul')
  const item = document.createElement('li')
  if (block) {
    item.append(...block.childNodes)
    block.replaceWith(container)
  } else if (!range.collapsed) {
    item.append(range.extractContents())
    range.insertNode(container)
  } else {
    item.append(document.createElement('br'))
    range.insertNode(container)
  }
  container.append(item)
  selectNodeContents(item, editor)
  return true
}

/** Turn a just-typed Markdown line prefix into rich block formatting. */
export function applyMarkdownShortcut(editor) {
  const range = editorRange(editor)
  if (!range || !range.collapsed) return false
  const block = closestBlock(range.startContainer, editor)
  const source = String(block?.textContent ?? textBeforeCaret(range, editor)).replace(/\u200b/g, '')
  const match = source.match(/^(#{1,3}|>|[-*]|1\.)$/)
  if (!match) return false
  if (block) {
    block.replaceChildren(document.createElement('br'))
    selectNodeContents(block, editor)
  }
  else deleteBeforeCaret(range, match[0].length)
  if (match[1] === '>') return applyBlockFormat(editor, 'blockquote')
  if (match[1] === '-' || match[1] === '*') return applyListFormat(editor, false)
  if (match[1] === '1.') return applyListFormat(editor, true)
  return applyBlockFormat(editor, `h${match[1].length}`)
}

/** Convert completed inline Markdown marks at the caret. */
export function applyInlineMarkdownShortcut(editor) {
  const range = editorRange(editor)
  if (!range?.collapsed || range.startContainer.nodeType !== Node.TEXT_NODE) return false
  const node = /** @type {Text} */ (range.startContainer)
  const before = node.data.slice(0, range.startOffset)
  const match = before.match(/(?:^|\s)(\*\*|__)([^\n]+?)\1$|(?:^|\s)(\*|_)([^\n*]+?)\3$|(?:^|\s)`([^\n`]+?)`$/)
  if (!match) return false
  const marker = match[1] || match[3] || '`'
  const content = match[2] || match[4] || match[5]
  const full = match[0]
  const leading = /^\s/.test(full) ? full[0] : ''
  const start = range.startOffset - full.length
  const replacementRange = document.createRange()
  replacementRange.setStart(node, start)
  replacementRange.setEnd(node, range.startOffset)
  const fragment = document.createDocumentFragment()
  if (leading) fragment.append(document.createTextNode(leading))
  const wrapper = document.createElement(marker === '`' ? 'code' : marker.length === 2 ? 'strong' : 'em')
  wrapper.textContent = content
  fragment.append(wrapper)
  replacementRange.deleteContents()
  replacementRange.insertNode(fragment)
  const next = document.createRange()
  next.setStartAfter(wrapper)
  next.collapse(true)
  const selection = getSelection()
  selection?.removeAllRanges()
  selection?.addRange(next)
  return true
}

/** Link the URL or email immediately before the caret on Space / Enter. */
export function autoLinkBeforeCaret(editor) {
  const range = editorRange(editor)
  if (!range?.collapsed || range.startContainer.nodeType !== Node.TEXT_NODE) return false
  const node = /** @type {Text} */ (range.startContainer)
  const before = node.data.slice(0, range.startOffset)
  const token = before.match(/(?:^|\s)((?:https?:\/\/|www\.)[^\s<]+|[\w.+-]+@[\w.-]+\.[a-z]{2,})$/i)?.[1]
  if (!token) return false
  const href = normalizeComposeLink(token.includes('@') && !/^https?:|^www\./i.test(token) ? `mailto:${token}` : token)
  if (!href) return false
  const linkRange = document.createRange()
  linkRange.setStart(node, range.startOffset - token.length)
  linkRange.setEnd(node, range.startOffset)
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.rel = 'noopener noreferrer'
  anchor.target = '_blank'
  anchor.textContent = token
  linkRange.deleteContents()
  linkRange.insertNode(anchor)
  const next = document.createRange()
  next.setStartAfter(anchor)
  next.collapse(true)
  const selection = getSelection()
  selection?.removeAllRanges()
  selection?.addRange(next)
  return true
}

/** Insert a locally visible image whose outbound representation uses cid:. */
export function insertInlineImage(editor, attachment) {
  const contentId = String(attachment?.contentId || '')
  if (!contentId || !isInlineImageFile({ type: attachment?.contentType })) return false
  const image = document.createElement('img')
  image.src = `data:${attachment.contentType};base64,${attachment.contentBase64}`
  image.dataset.caduceusCid = contentId
  image.dataset.caduceusAttachmentId = String(attachment.id || '')
  image.alt = String(attachment.filename || 'Inline image')
  image.width = 480
  image.setAttribute('data-caduceus-inline-image', '')
  const range = editorRange(editor) || rangeAtEnd(editor)
  range.insertNode(image)
  range.setStartAfter(image)
  range.collapse(true)
  const selection = getSelection()
  selection?.removeAllRanges()
  selection?.addRange(range)
  editor.focus()
  return true
}

/** Hydrate persisted cid images from their matching attachment bytes. */
export function hydrateInlineImages(editor, attachments) {
  const byCid = new Map((attachments || []).filter(item => item.contentId).map(item => [String(item.contentId).toLowerCase(), item]))
  for (const image of editor.querySelectorAll('img')) {
    const cid = String(image.dataset.caduceusCid || image.getAttribute('src') || '').replace(/^cid:/i, '').toLowerCase()
    const attachment = byCid.get(cid)
    if (!attachment || !isInlineImageFile({ type: attachment.contentType })) continue
    image.dataset.caduceusCid = attachment.contentId
    image.dataset.caduceusAttachmentId = attachment.id || ''
    image.setAttribute('data-caduceus-inline-image', '')
    image.src = `data:${attachment.contentType};base64,${attachment.contentBase64}`
  }
}

/** Build connected-editor HTML with cid sources hydrated while still inert. */
export function editableComposerHTML(html, attachments) {
  const byCid = new Map((attachments || []).filter(item => item.contentId).map(item => [String(item.contentId).toLowerCase(), item]))
  const safeSource = String(html || '').replace(/\bsrc\s*=\s*(["'])cid:([^"']+)\1/gi, (_match, quote, rawCid) => {
    const cid = String(rawCid).replace(/^<|>$/g, '')
    const attachment = byCid.get(cid.toLowerCase())
    const source = attachment && isInlineImageFile({ type: attachment.contentType })
      ? `data:${attachment.contentType};base64,${attachment.contentBase64}`
      : `${TRANSPARENT_GIF}#missing-inline-image`
    return `src=${quote}${source}${quote} data-caduceus-cid=${quote}${escapeAttribute(cid)}${quote}`
  })
  const template = document.createElement('template')
  template.innerHTML = safeSource
  hydrateInlineImages(template.content, attachments)
  return template.innerHTML
}

/** Serialize editor HTML without local data URLs or transient collapsed state. */
export function serializeComposerHTML(editor) {
  const clone = editor.cloneNode(true)
  const replacements = []
  let index = 0
  for (const image of clone.querySelectorAll('img[data-caduceus-cid]')) {
    const placeholder = `${TRANSPARENT_GIF}#caduceus-inline-${index++}`
    replacements.push([placeholder, `cid:${escapeAttribute(image.dataset.caduceusCid || '')}`])
    // Assign only a browser-safe placeholder while this disconnected clone is
    // serialized. Setting an unknown cid: scheme directly still emits noisy
    // Chromium network errors even when the node is never connected.
    image.setAttribute('src', placeholder)
    image.removeAttribute('data-caduceus-attachment-id')
  }
  clone.classList.remove('compose-quote-collapsed')
  clone.removeAttribute('data-quote-expanded')
  let html = clone.innerHTML
  for (const [placeholder, cid] of replacements) html = html.replace(placeholder, cid)
  return html
}

export function resizeInlineImage(image, width) {
  if (!image?.matches?.('img[data-caduceus-inline-image]')) return false
  const safeWidth = Math.max(120, Math.min(960, Number(width) || 480))
  image.width = safeWidth
  image.style.maxWidth = '100%'
  image.style.height = 'auto'
  return true
}

function editorRange(editor) {
  const selection = getSelection()
  if (!selection?.rangeCount) return null
  const range = selection.getRangeAt(0)
  return editor.contains(range.commonAncestorContainer) ? range : null
}

function rangeAtEnd(editor) {
  const range = document.createRange()
  range.selectNodeContents(editor)
  range.collapse(false)
  return range
}

function closestWithin(node, tagName, editor) {
  let element = node?.nodeType === Node.ELEMENT_NODE ? node : node?.parentElement
  while (element && element !== editor) {
    if (element.tagName === tagName) return element
    element = element.parentElement
  }
  return null
}

function closestBlock(node, editor) {
  let element = node?.nodeType === Node.ELEMENT_NODE ? node : node?.parentElement
  while (element && element !== editor) {
    if (BLOCK_TAGS.has(element.tagName)) return element
    element = element.parentElement
  }
  return null
}

function unwrap(element) {
  const parent = element.parentNode
  if (!parent) return
  while (element.firstChild) parent.insertBefore(element.firstChild, element)
  element.remove()
}

function selectNodeContents(node, editor) {
  if (!node || !editor.contains(node)) return
  const range = document.createRange()
  range.selectNodeContents(node)
  range.collapse(false)
  const selection = getSelection()
  selection?.removeAllRanges()
  selection?.addRange(range)
  editor.focus()
}

function textBeforeCaret(range, editor) {
  const probe = range.cloneRange()
  probe.selectNodeContents(editor)
  probe.setEnd(range.startContainer, range.startOffset)
  return probe.toString().split(/\r?\n/).pop() || ''
}

function deleteBeforeCaret(range, length) {
  if (range.startContainer.nodeType !== Node.TEXT_NODE) return
  const start = Math.max(0, range.startOffset - length)
  range.setStart(range.startContainer, start)
  range.deleteContents()
  range.collapse(true)
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, character => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[character])
}

function escapeAttribute(value) {
  return String(value).replace(/[&"<>]/g, character => ({ '&': '&amp;', '"': '&quot;', '<': '&lt;', '>': '&gt;' })[character])
}

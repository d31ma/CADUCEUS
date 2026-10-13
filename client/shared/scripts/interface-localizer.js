// @ts-check

import {
  hasInterfaceLiteral,
  loadInterfaceLiteralCatalogs,
  translateInterfaceLiteral,
} from './localization.js'

const LOCALIZED_ATTRIBUTES = Object.freeze(['aria-label', 'title', 'placeholder', 'alt'])
const EXCLUDED = [
  'script', 'style', 'code', 'pre', 'iframe', '[contenteditable="true"]',
  '[data-no-localize]', '.email-sender', '.email-subject', '.email-subject-text',
  '.email-preview', '.email-body', '.email-quoted-plain', '.conversation-message-sender',
  '.conversation-message-subject', '.shared-thread-message', '.contact-notes',
  '.activity-event-copy', '.activity-subjects tbody', '.calendar-event-title',
  '.event-modal-readonly-description', '.contact-row-copy', '.contact-detail-title',
  '.contact-detail-card strong', '.contact-detail-card a', '.contact-chips',
  '.contact-group-checks span', '.team-name', '.team-email', '.attachment-chip-name',
  '.attachment-preview-header h2', '.invite-title', '.invite-sender-response',
  '.email-tracking-event-copy', '.participant-profile-card h2',
  '.participant-profile-card p', '.participant-profile-card a',
].join(',')

/** @type {WeakMap<Text, string>} */
const originalText = new WeakMap()
/** @type {WeakMap<Element, Map<string, string>>} */
const originalAttributes = new WeakMap()
let observer

/** Install the bounded literal localizer once for the browser document. */
export function installInterfaceLocalizer() {
  if (typeof document !== 'object' || observer) return
  localizeInterfaceTree(document.documentElement)
  observer = new MutationObserver(mutations => {
    for (const mutation of mutations) {
      if (mutation.type === 'characterData') localizeText(/** @type {Text} */ (mutation.target))
      for (const node of mutation.addedNodes) localizeInterfaceTree(node)
    }
  })
  observer.observe(document.documentElement, { childList: true, subtree: true, characterData: true })
  globalThis.addEventListener('caduceus:interface-locale', () => localizeInterfaceTree(document.documentElement))
  globalThis.addEventListener('caduceus:interface-catalog-ready', () => localizeInterfaceTree(document.documentElement))
  void loadInterfaceLiteralCatalogs().catch(() => {
    // Curated core catalogs remain available when the immutable optional asset
    // cannot be read; no network fallback or untrusted literal discovery occurs.
  })
}

/** @param {Node} root */
export function localizeInterfaceTree(root) {
  if (root.nodeType === Node.TEXT_NODE) {
    localizeText(/** @type {Text} */ (root))
    return
  }
  if (!(root instanceof Element) || isExcluded(root)) return
  localizeAttributes(root)
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      return node instanceof Element && isExcluded(node)
        ? NodeFilter.FILTER_REJECT
        : NodeFilter.FILTER_ACCEPT
    },
  })
  while (walker.nextNode()) {
    if (walker.currentNode instanceof Text) localizeText(walker.currentNode)
    else if (walker.currentNode instanceof Element) localizeAttributes(walker.currentNode)
  }
}

/** @param {Text} node */
function localizeText(node) {
  if (isExcluded(node.parentElement)) return
  const current = node.data
  const trimmed = current.trim()
  let source = originalText.get(node)
  if (!source || hasInterfaceLiteral(trimmed)) {
    if (!hasInterfaceLiteral(trimmed)) return
    source = trimmed
    originalText.set(node, source)
  }
  const translated = translateInterfaceLiteral(source)
  const leading = current.match(/^\s*/)?.[0] || ''
  const trailing = current.match(/\s*$/)?.[0] || ''
  const next = `${leading}${translated}${trailing}`
  if (next !== current) node.data = next
}

/** @param {Element} element */
function localizeAttributes(element) {
  let originals = originalAttributes.get(element)
  for (const attribute of LOCALIZED_ATTRIBUTES) {
    const current = element.getAttribute(attribute)?.trim() || ''
    let source = originals?.get(attribute)
    if (!source || hasInterfaceLiteral(current)) {
      if (!hasInterfaceLiteral(current)) continue
      source = current
      originals ||= new Map()
      originals.set(attribute, source)
      originalAttributes.set(element, originals)
    }
    const translated = translateInterfaceLiteral(source)
    if (translated !== element.getAttribute(attribute)) element.setAttribute(attribute, translated)
  }
}

/** @param {Element | null} element */
function isExcluded(element) {
  return Boolean(element?.closest?.(EXCLUDED))
}

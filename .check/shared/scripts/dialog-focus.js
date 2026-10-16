/**
 * Browser-owned modal focus containment.
 *
 * Tachyon rerenders after declarative event handlers. Moving focus from a
 * component keydown handler can therefore target a node that is replaced in
 * the same turn. A capture-phase native handler owns Tab navigation so focus
 * always moves between the stable, currently rendered modal controls.
 */

const MODAL_SELECTOR = '[role="dialog"][aria-modal="true"]'
const FOCUSABLE_SELECTOR = [
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  'a[href]',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

function isVisible(element) {
  if (!(element instanceof HTMLElement) || element.hidden) return false
  const style = getComputedStyle(element)
  return style.display !== 'none' && style.visibility !== 'hidden' && element.getClientRects().length > 0
}

function visibleModal() {
  const dialogs = [...document.querySelectorAll(MODAL_SELECTOR)].filter(isVisible)
  return /** @type {HTMLElement | null} */ (dialogs.at(-1) || null)
}

function modalControls(dialog) {
  return [...dialog.querySelectorAll(FOCUSABLE_SELECTOR)]
    .filter(element => isVisible(element) && element.getAttribute('aria-hidden') !== 'true')
}

/** @type {HTMLElement | null} */
let currentModal = null
/** @type {HTMLElement | null} */
let returnFocus = null
/** @type {{ id: string, tagName: string, ariaLabel: string, href: string } | null} */
let returnIdentity = null
let modalLocation = ''

document.addEventListener('focusin', event => {
  const target = event.target
  if (!(target instanceof HTMLElement) || target.closest(MODAL_SELECTOR)) return
  returnFocus = target
  returnIdentity = {
    id: target.id,
    tagName: target.tagName.toLowerCase(),
    ariaLabel: target.getAttribute('aria-label') || '',
    href: target.getAttribute('href') || '',
  }
}, true)

function resolveReturnFocus() {
  if (returnFocus?.isConnected) return returnFocus
  if (!returnIdentity) return null
  const identity = returnIdentity
  if (identity.id) {
    const byId = document.getElementById(identity.id)
    if (byId instanceof HTMLElement) return byId
  }
  const replacement = [...document.querySelectorAll(identity.tagName)].find(candidate =>
    (!identity.ariaLabel || candidate.getAttribute('aria-label') === identity.ariaLabel) &&
    (!identity.href || candidate.getAttribute('href') === identity.href)
  )
  return replacement instanceof HTMLElement ? replacement : null
}

new MutationObserver(() => {
  const next = visibleModal()
  if (next && next !== currentModal) {
    currentModal = next
    modalLocation = location.href
    requestAnimationFrame(() => {
      if (!next.isConnected || next.contains(document.activeElement)) return
      const target = next.querySelector('[autofocus]') || modalControls(next)[0] || next
      if (target instanceof HTMLElement) target.focus()
    })
    return
  }
  if (!next && currentModal) {
    currentModal = null
    const restoreAt = modalLocation
    requestAnimationFrame(() => requestAnimationFrame(() => {
      if (location.href !== restoreAt || visibleModal()) return
      resolveReturnFocus()?.focus()
    }))
  }
}).observe(document.documentElement, { childList: true, subtree: true })

document.addEventListener('keydown', event => {
  if (event.key !== 'Tab') return
  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null
  const dialog = active?.closest(MODAL_SELECTOR) || visibleModal()
  if (!(dialog instanceof HTMLElement) || !isVisible(dialog)) return

  const controls = modalControls(dialog)
  event.preventDefault()
  event.stopImmediatePropagation()
  if (!controls.length) {
    dialog.focus()
    return
  }

  const current = active ? controls.indexOf(active) : -1
  const next = event.shiftKey
    ? current <= 0 ? controls.length - 1 : current - 1
    : current < 0 || current >= controls.length - 1 ? 0 : current + 1
  const target = controls[next]
  if (target instanceof HTMLElement) target.focus()
}, true)

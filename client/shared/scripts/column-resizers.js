/**
 * Column resizers: the sidebar and the message-list separators
 * (`[data-column-resizer="sidebar" | "mail"]`). Drag with a pointer, nudge
 * with ArrowLeft/ArrowRight, reset with Home or a double click. The width is
 * the `--app-sidebar-width` / `--mail-list-width` property on <html>, kept per
 * device in localStorage. Ported from the JavaScript shell; it only moves
 * pixels, so no Go service is involved.
 */

const WIDE = '(min-width: 1025px)'
const STEP = 16
const KINDS = {
  sidebar: { property: '--app-sidebar-width', initial: 232, min: 180, room: 720, cap: 360 },
  mail: { property: '--mail-list-width', initial: 390, min: 280, room: 360, cap: 640 },
}

/** @param {EventTarget | null} target */
function handleOf(target) {
  const handle = target instanceof Element ? target.closest('[data-column-resizer]') : null
  return handle instanceof HTMLElement && KINDS[handle.dataset.columnResizer || ''] ? handle : null
}

/** @param {string} kind */
function current(kind) {
  return Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue(KINDS[kind].property))
}

function inlineDirection() {
  return getComputedStyle(document.documentElement).direction === 'rtl' ? -1 : 1
}

/** @param {string} kind @param {HTMLElement | null} handle */
function bounds(kind, handle) {
  const spec = KINDS[kind]
  const layout = handle?.closest('.mail-layout') || document.querySelector('.mail-layout')
  const available = kind === 'sidebar' ? innerWidth : layout?.getBoundingClientRect().width || innerWidth
  return { min: spec.min, max: Math.max(spec.min, Math.min(spec.cap, available - spec.room)) }
}

/** @param {string} kind @param {number} requested @param {boolean} persist @param {HTMLElement | null} [handle] */
function setWidth(kind, requested, persist, handle = null) {
  const { min, max } = bounds(kind, handle)
  const width = Math.round(Math.min(max, Math.max(min, requested)))
  // Tac accepts a literal custom property name here.
  if (kind === 'sidebar') document.documentElement.style.setProperty('--app-sidebar-width', `${width}px`)
  else document.documentElement.style.setProperty('--mail-list-width', `${width}px`)
  for (const separator of document.querySelectorAll(`[data-column-resizer="${kind}"]`)) {
    separator.setAttribute('aria-valuemin', String(min))
    separator.setAttribute('aria-valuemax', String(Math.round(max)))
    separator.setAttribute('aria-valuenow', String(width))
  }
  if (persist) {
    try { localStorage.setItem(`caduceus-column-${kind}-width`, String(width)) } catch {}
  }
}

function restore() {
  if (!matchMedia(WIDE).matches) return
  for (const kind of Object.keys(KINDS)) {
    let stored = Number.NaN
    try { stored = Number.parseFloat(localStorage.getItem(`caduceus-column-${kind}-width`) || '') } catch {}
    if (Number.isFinite(stored)) setWidth(kind, stored, false)
  }
}

document.addEventListener('pointerdown', event => {
  const handle = handleOf(event.target)
  if (!handle || event.button !== 0 || !matchMedia(WIDE).matches) return
  const kind = handle.dataset.columnResizer || ''
  const panel = kind === 'sidebar' ? document.querySelector('.app-sidebar') : handle.closest('.mail-layout')?.querySelector('.mail-list-panel')
  const startWidth = panel?.getBoundingClientRect().width
  if (!startWidth) return
  event.preventDefault()
  const startX = event.clientX
  const direction = inlineDirection()
  handle.classList.add('is-active')
  document.body.classList.add('column-resize-active')
  /** @param {PointerEvent} moved */
  const move = moved => setWidth(kind, startWidth + (moved.clientX - startX) * direction, false, handle)
  const stop = () => {
    removeEventListener('pointermove', move)
    removeEventListener('pointerup', stop)
    removeEventListener('pointercancel', stop)
    handle.classList.remove('is-active')
    document.body.classList.remove('column-resize-active')
    if (Number.isFinite(current(kind))) setWidth(kind, current(kind), true, handle)
  }
  addEventListener('pointermove', move)
  addEventListener('pointerup', stop)
  addEventListener('pointercancel', stop)
})

document.addEventListener('keydown', event => {
  const handle = handleOf(event.target)
  if (!handle || !['ArrowLeft', 'ArrowRight', 'Home'].includes(event.key)) return
  event.preventDefault()
  const kind = handle.dataset.columnResizer || ''
  const next = event.key === 'Home'
    ? KINDS[kind].initial
    : current(kind) + (event.key === 'ArrowLeft' ? -STEP : STEP) * inlineDirection()
  setWidth(kind, next, true, handle)
})

document.addEventListener('dblclick', event => {
  const handle = handleOf(event.target)
  if (handle) setWidth(handle.dataset.columnResizer || '', KINDS[handle.dataset.columnResizer || ''].initial, true, handle)
})

restore()

// @ts-check

/**
 * Browser-side helpers of the mailbox list (MailboxBrowserService).
 *
 * The list's state and actions are Go services, which run as WebAssembly and
 * never see a DOM event. What needs the event itself stays here: the
 * capture-phase row activation that keeps a row's own controls independent,
 * roving focus, the Backspace and swipe gestures, the context-menu position,
 * drag data, the keys of the search box and the dialogs, and the focus and
 * scroll moves after a render. Every helper reports through plain callbacks;
 * the service turns them into typed events.
 */

const ROW_OPEN = '[data-email-open], [data-thread-open]'
const ROW = '.email-row[data-email-id], .thread-email-row[data-email-id], .thread-summary[data-thread-id]'
const CONTROL = 'a, button, input, select, textarea, label, [role="button"], [role="menuitem"], w-btn, w-chip, w-switch, w-checkbox'
const EDITABLE = 'input, textarea, select, [contenteditable="true"], [role="textbox"], [role="dialog"]'
const SEARCH_INPUT = '[data-thread-search-input]'
const ROVING_KEYS = ['ArrowDown', 'ArrowUp', 'Home', 'End']
const SUGGESTION_PAUSE_MS = 90
const BLUR_GRACE_MS = 140
const SWIPE_DISTANCE = 64
const SWIPE_DRIFT = 48
const MENU_WIDTH = 232
const MENU_HEIGHT = 320
const EDGE = 8
const NARROW = 760

/**
 * Row activation, roving focus and gestures, in the capture phase so a row's
 * checkbox and quick actions keep their own clicks.
 *
 * @param {AbortSignal} signal
 * @param {{ activate: (emailId: string, threadId: string, narrow: boolean) => void, focus: (id: string) => void, gesture: (kind: string, emailId: string, folder: string) => void }} report
 */
export function watchRows(signal, report) {
  /** @type {{ id: string, x: number, y: number, pointerId: number } | null} */
  let swipe = null
  document.addEventListener('click', event => rowClick(event, report), { capture: true, signal })
  document.addEventListener('keydown', event => rowKey(event, report), { capture: true, signal })
  document.addEventListener('pointerdown', event => { swipe = swipeStart(event) ?? swipe }, { capture: true, signal })
  document.addEventListener('pointerup', event => { swipe = swipeEnd(event, swipe, report) }, { capture: true, signal })
}

/** @param {MouseEvent} event @param {{ activate: (emailId: string, threadId: string, narrow: boolean) => void }} report */
function rowClick(event, report) {
  const source = event.target instanceof Element ? event.target : null
  if (!source) return
  let target = source.closest(ROW_OPEN)
  if (!(target instanceof HTMLElement)) {
    if (source.closest(CONTROL)) return
    target = source.closest(ROW)
    if (!(target instanceof HTMLElement)) return
  }
  event.preventDefault()
  event.stopImmediatePropagation()
  const emailId = target.dataset.emailOpen || target.dataset.emailId || ''
  report.activate(emailId, emailId ? '' : target.dataset.threadOpen || target.dataset.threadId || '', innerWidth <= NARROW)
}

/** @param {KeyboardEvent} event @param {{ activate: (emailId: string, threadId: string, narrow: boolean) => void, focus: (id: string) => void, gesture: (kind: string, emailId: string, folder: string) => void }} report */
function rowKey(event, report) {
  const source = event.target instanceof Element ? event.target : null
  if (event.key === 'Backspace' && !source?.closest(EDITABLE) && document.querySelector('.email-row, .thread-email-row')) {
    event.preventDefault()
    event.stopImmediatePropagation()
    const focusedRow = source?.closest('[data-email-open]')?.getAttribute('data-email-open') || ''
    report.gesture('backspace', focusedRow, preferences().backspaceAction === 'trash' ? 'trash' : 'archive')
    return
  }
  const target = source?.closest(ROW_OPEN)
  if (!(target instanceof HTMLElement)) return
  const id = target.dataset.emailOpen || target.dataset.threadOpen || ''
  if (ROVING_KEYS.includes(event.key)) {
    event.preventDefault()
    event.stopImmediatePropagation()
    moveFocus(event.key, id, report)
    return
  }
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  event.stopImmediatePropagation()
  report.activate(target.dataset.emailOpen || '', target.dataset.emailOpen ? '' : id, innerWidth <= NARROW)
}

/** @param {string} key @param {string} id @param {{ focus: (id: string) => void }} report */
function moveFocus(key, id, report) {
  const rows = /** @type {HTMLElement[]} */ ([...document.querySelectorAll('[data-roving-id]')])
  if (rows.length === 0) return
  const ids = rows.map(row => row.dataset.rovingId || '')
  const current = Math.max(0, ids.indexOf(id))
  const index = key === 'Home' ? 0
    : key === 'End' ? ids.length - 1
      : Math.min(ids.length - 1, Math.max(0, current + (key === 'ArrowDown' ? 1 : -1)))
  rows[index].focus()
  report.focus(ids[index])
}

/** @param {PointerEvent} event */
function swipeStart(event) {
  if (!['touch', 'pen'].includes(event.pointerType)) return null
  const row = event.target instanceof Element ? event.target.closest('.email-row[data-email-id], .thread-email-row[data-email-id]') : null
  if (!(row instanceof HTMLElement) || !row.dataset.emailId) return null
  return { id: row.dataset.emailId, x: event.clientX, y: event.clientY, pointerId: event.pointerId }
}

/**
 * @param {PointerEvent} event
 * @param {{ id: string, x: number, y: number, pointerId: number } | null} swipe
 * @param {{ gesture: (kind: string, emailId: string, folder: string) => void }} report
 */
function swipeEnd(event, swipe, report) {
  if (!swipe || swipe.pointerId !== event.pointerId) return swipe
  const dx = event.clientX - swipe.x
  const dy = event.clientY - swipe.y
  if (preferences().swipeEnabled === false || Math.abs(dx) < SWIPE_DISTANCE || Math.abs(dy) > SWIPE_DRIFT) return null
  event.preventDefault()
  event.stopImmediatePropagation()
  report.gesture('swipe', swipe.id, dx > 0 ? 'archive' : 'trash')
  return null
}

/** @returns {{ backspaceAction?: string, swipeEnabled?: boolean }} */
function preferences() {
  try {
    const stored = JSON.parse(sessionStorage.getItem('caduceus_mail_preferences') || '{}')
    return stored && typeof stored === 'object' ? stored : {}
  } catch { return {} }
}

/**
 * The pause in typing and the delayed blur of the search box.
 *
 * @param {AbortSignal} signal
 * @param {{ wanted: (value: string, caret: number) => void, blurred: () => void }} report
 */
export function watchSearch(signal, report) {
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  let pause
  document.addEventListener('input', event => {
    const input = searchInput(event)
    if (!input) return
    clearTimeout(pause)
    pause = setTimeout(() => report.wanted(input.value, input.selectionStart ?? input.value.length), SUGGESTION_PAUSE_MS)
  }, { capture: true, signal })
  document.addEventListener('blur', event => {
    if (searchInput(event)) setTimeout(() => report.blurred(), BLUR_GRACE_MS)
  }, { capture: true, signal })
  signal.addEventListener('abort', () => clearTimeout(pause), { once: true })
}

/** @param {Event} event */
function searchInput(event) {
  return event.target instanceof HTMLInputElement && event.target.matches(SEARCH_INPUT) ? event.target : null
}

/**
 * ArrowDown, ArrowUp (while suggestions show), Escape and Enter in the search
 * box, without the browser's own meaning (Escape clears a search field).
 *
 * @param {KeyboardEvent} event
 * @param {(key: string, value: string, caret: number) => void} report
 */
export function searchKey(event, report) {
  const input = event.target instanceof HTMLInputElement ? event.target : null
  if (!input || event.isComposing) return
  const suggestions = document.querySelectorAll('#thread-search-suggestions [role="option"]').length
  const arrow = (event.key === 'ArrowDown' || event.key === 'ArrowUp') && suggestions > 0
  if (!arrow && event.key !== 'Escape' && event.key !== 'Enter') return
  event.preventDefault()
  report(event.key, input.value, input.selectionStart ?? input.value.length)
}

/**
 * A suggestion pressed with the mouse; the search box keeps the focus.
 *
 * @param {MouseEvent} event
 * @param {(value: string, caret: number) => void} report
 */
export function suggestionPressed(event, report) {
  event.preventDefault()
  const input = /** @type {HTMLInputElement | null} */ (document.querySelector(SEARCH_INPUT))
  report(input?.value ?? '', input?.selectionStart ?? input?.value.length ?? 0)
}

/**
 * Where to open the row menu, kept inside the window.
 *
 * @param {MouseEvent} event
 * @returns {{ x: number, y: number }}
 */
export function menuPosition(event) {
  event.preventDefault()
  return {
    x: Math.max(EDGE, Math.min(Number(event.clientX) || EDGE, innerWidth - MENU_WIDTH)),
    y: Math.max(EDGE, Math.min(Number(event.clientY) || EDGE, innerHeight - MENU_HEIGHT)),
  }
}

/**
 * Drag one row, or every selected row when it is part of a selection.
 *
 * @param {DragEvent} event
 * @param {string} emailId
 * @param {string[] | null | undefined} selected
 */
export function startDrag(event, emailId, selected) {
  const chosen = Array.isArray(selected) ? selected : []
  const ids = chosen.includes(emailId) && chosen.length > 1 ? chosen : [emailId]
  event.dataTransfer?.setData('text/plain', emailId)
  event.dataTransfer?.setData('application/x-caduceus-mail-ids', JSON.stringify(ids))
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

/** @param {KeyboardEvent} event @param {() => void} dismissed */
export function menuKey(event, dismissed) {
  if (event.key !== 'Escape') return
  event.preventDefault()
  dismissed()
}

/**
 * Escape closes the category picker, Tab stays inside it and the arrows walk
 * its options.
 *
 * @param {KeyboardEvent} event
 * @param {() => void} dismissed
 */
export function pickerKey(event, dismissed) {
  if (event.key === 'Escape') {
    event.preventDefault()
    dismissed()
    return
  }
  const dialog = document.querySelector('.category-picker-dialog')
  const focusable = /** @type {HTMLElement[]} */ ([...(dialog?.querySelectorAll('button:not([disabled]), input:not([disabled])') || [])])
  if (event.key === 'Tab' && focusable.length) {
    const current = document.activeElement ? focusable.indexOf(/** @type {HTMLElement} */ (document.activeElement)) : -1
    const next = event.shiftKey
      ? current <= 0 ? focusable.length - 1 : current - 1
      : current >= focusable.length - 1 ? 0 : current + 1
    event.preventDefault()
    focusable[next].focus()
    return
  }
  if (!ROVING_KEYS.includes(event.key)) return
  const options = focusable.filter(element => element.classList.contains('category-picker-option'))
  if (options.length === 0) return
  event.preventDefault()
  const current = document.activeElement ? options.indexOf(/** @type {HTMLElement} */ (document.activeElement)) : -1
  const index = event.key === 'Home' ? 0
    : event.key === 'End' ? options.length - 1
      : event.key === 'ArrowDown' ? Math.min(options.length - 1, current + 1)
        : Math.max(0, current < 0 ? options.length - 1 : current - 1)
  options[index].focus()
}

/**
 * Run once the element the next render brings exists.
 *
 * @param {string} selector
 * @param {(element: HTMLElement) => void} action
 */
export function whenRendered(selector, action) {
  let frames = 0
  const look = () => {
    const element = document.querySelector(selector)
    if (element instanceof HTMLElement) action(element)
    else if (++frames < 60) requestAnimationFrame(look)
  }
  requestAnimationFrame(look)
}

/** @returns {HTMLElement | null} */
export function focusedElement() {
  return document.activeElement instanceof HTMLElement ? document.activeElement : null
}

/** @param {number} top @param {number} [by] */
export function scrollList(top, by) {
  const list = document.querySelector('.inbox-list')
  if (by === undefined) list?.scrollTo({ top, behavior: 'smooth' })
  else if (list) list.scrollBy({ top: by, behavior: 'smooth' })
  else scrollBy({ top: by, behavior: 'smooth' })
}

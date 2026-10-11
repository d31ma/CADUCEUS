/**
 * Escape hatch: the calendar's browser work that a Go service cannot reach.
 *
 * Tachyon gives Go services no DOM events, no native dialogs and no media
 * queries, and its template handlers cannot cancel an event. So this service
 * owns the native HTML drag-and-drop transaction (dragover must call
 * preventDefault before a drop can happen), the delete confirmations, the
 * phone-size viewport check, the chip that holds keyboard focus, scrolling a
 * chip into view, and the accessible names that DuVay's floating labels leave
 * off their inner controls. It reads the DOM and talks to CalendarService in
 * events; every decision stays in Go.
 */
@Service
export default class CalendarBrowserService {
  static _NARROW = '(max-width: 760px)'
  static _EVENT = '[data-calendar-event-id]'
  static _EVENT_ID = 'data-calendar-event-id'
  static _ALL_DAY_DATE = 'data-calendar-all-day-date'
  static _DROP = '[data-calendar-all-day-date], [data-calendar-date]'
  static _DROP_TARGET = 'calendar-drop-target'
  static _SELECTED = 'keyboard-selected'
  static _ARIA_LABEL = 'aria-label'
  static _LABELLED = '.event-modal w-text-field, .event-modal w-textarea, .quick-event-popover w-text-field'

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  ready = false
  _dragEventId = ''
  _pointing = false
  /** @type {Array<[EventTarget, string, (event: any) => void]>} */
  _bindings = []
  /** @type {MutationObserver | null} */
  _observer = null

  @OnStart
  start() {
    const type = CalendarBrowserService
    this._bindings = [
      [window, 'resize', () => this._measure()],
      [document, 'dragstart', event => this._dragStart(event, this._closest(event.target, type._EVENT))],
      [document, 'dragover', event => this._dragOver(event, this._closest(event.target, type._DROP))],
      [document, 'dragleave', event => this._dragLeave(event, this._closest(event.target, type._DROP))],
      [document, 'drop', event => this._drop(event, this._closest(event.target, type._DROP))],
      [document, 'dragend', () => this._clearDrag()],
      [document, 'pointerdown', () => this._pointed()],
      [document, 'focusin', event => this._focusIn(this._closest(event.target, type._EVENT))],
    ]
    this._bindings.forEach(([target, name, handler]) => target.addEventListener(name, handler))
    this._observer = new MutationObserver(() => this._labelFields())
    this._observer.observe(document.body, { childList: true, subtree: true })
    this._measure()
    this.ready = true
  }

  @OnStop
  stop() {
    this._bindings.forEach(([target, name, handler]) => target.removeEventListener(name, handler))
    this._bindings = []
    this._observer?.disconnect()
  }

  /**
   * Scroll the selected event into view after a view switch has rendered.
   * @param {EventRevealRequested} request
   */
  @OnEvent
  reveal(request) {
    setTimeout(() => requestAnimationFrame(() => this._scrollTo(request.eventId)), 60)
  }

  /**
   * Ask before deleting: a recurring event offers this occurrence, then the
   * whole series. The chosen scope goes to CalendarService.
   * @param {string} seriesId
   */
  confirmDelete(seriesId) {
    this._publishScope(this._question(seriesId).call(this))
  }

  _measure() {
    /** @type {ViewportMeasured} */
    const measured = { narrow: window.matchMedia(CalendarBrowserService._NARROW).matches }
    this.applicationEventPublisher.publishEvent(measured, true)
  }

  /** @param {EventTarget | null} target @param {string} selector @returns {HTMLElement | null} */
  _closest(target, selector) {
    if (!(target instanceof Element)) return null
    return /** @type {HTMLElement | null} */ (target.closest(selector))
  }

  /** @param {DragEvent} event @param {HTMLElement | null} chip */
  _dragStart(event, chip) {
    if (!chip) return
    this._dragEventId = String(chip.getAttribute(CalendarBrowserService._EVENT_ID))
    event.dataTransfer?.setData('text/plain', this._dragEventId)
  }

  /** @param {DragEvent} event @param {HTMLElement | null} target */
  _dragOver(event, target) {
    if (!target || !this._dragEventId) return
    event.preventDefault()
    target.classList.add(CalendarBrowserService._DROP_TARGET)
  }

  /** @param {DragEvent} event @param {HTMLElement | null} target */
  _dragLeave(event, target) {
    if (!target || target.contains(/** @type {Node | null} */ (event.relatedTarget))) return
    target.classList.remove(CalendarBrowserService._DROP_TARGET)
  }

  /** @param {DragEvent} event @param {HTMLElement | null} zone */
  _drop(event, zone) {
    const id = this._dragEventId
    this._clearDrag()
    if (![id, zone].every(Boolean)) return
    this._publishDrop(event, id, /** @type {HTMLElement} */ (zone))
  }

  /** @param {DragEvent} event @param {string} id @param {HTMLElement} zone */
  _publishDrop(event, id, zone) {
    event.preventDefault()
    event.stopPropagation()
    /** @type {CalendarEventDropped} */
    const dropped = { id, date: this._zoneDate(zone), allDay: zone.hasAttribute(CalendarBrowserService._ALL_DAY_DATE) }
    this.applicationEventPublisher.publishEvent(dropped, false)
  }

  /** @param {HTMLElement} zone */
  _zoneDate(zone) {
    return String(zone.dataset.calendarAllDayDate ?? zone.dataset.calendarDate)
  }

  _clearDrag() {
    document.querySelectorAll(`.${CalendarBrowserService._DROP_TARGET}`).forEach(node => node.classList.remove(CalendarBrowserService._DROP_TARGET))
    this._dragEventId = ''
  }

  /**
   * Focus only marks the move and resize target, and decorates the chip in
   * place: a render would rebuild the chips, and a click whose mousedown
   * focused a chip would then land on a detached node.
   * @param {HTMLElement | null} chip
   */
  /**
   * A pointer press focuses the chip it lands on. Go hears only keyboard
   * focus: the re-render that a published focus starts replaces the chip
   * before the press ends, and the click would be lost.
   */
  _pointed() {
    this._pointing = true
    setTimeout(() => { this._pointing = false }, 0)
  }

  _focusIn(chip) {
    if ([!chip, this._pointing].includes(true)) return
    const type = CalendarBrowserService
    /** @type {CalendarEventFocused} */
    const focused = { id: String(chip.getAttribute(type._EVENT_ID)) }
    this.applicationEventPublisher.publishEvent(focused, false)
    document.querySelectorAll(`.${type._SELECTED}`).forEach(node => node.classList.remove(type._SELECTED))
    chip.classList.toggle(type._SELECTED, !chip.classList.contains('calendar-event-chip-team'))
  }

  _labelFields() {
    document.querySelectorAll(CalendarBrowserService._LABELLED).forEach(field => this._labelField(field))
  }

  /** DuVay's floating labels are visual siblings, not <label for>; name the real control.
   * @param {Element} field */
  _labelField(field) {
    const control = field.querySelector('input, textarea')
    const label = field.getAttribute('label')
    if (control && label && !control.getAttribute(CalendarBrowserService._ARIA_LABEL)) control.setAttribute(CalendarBrowserService._ARIA_LABEL, label)
  }

  /** @param {string} id */
  _scrollTo(id) {
    const selector = `[${CalendarBrowserService._EVENT_ID}="${CSS.escape(id)}"]`
    const scopes = ['.calendar-agenda-grid ', '.calendar-month-grid ', '.calendar-agenda-list ', '']
    const found = scopes.map(scope => document.querySelector(scope + selector)).find(Boolean)
    found?.scrollIntoView({ block: 'center', inline: 'nearest' })
  }

  /** @param {string} seriesId @returns {() => string | null} */
  _question(seriesId) {
    return seriesId ? this._askSeries : this._askEvent
  }

  /** @returns {string | null} */
  _askSeries() {
    switch (true) {
      case confirm('Delete only this occurrence? The rest of the series will remain.'): return 'instance'
      case confirm('Delete the entire recurring series? This removes every occurrence.'): return 'series'
      default: return null
    }
  }

  /** @returns {string | null} */
  _askEvent() {
    return confirm('Delete this event?') ? '' : null
  }

  /** @param {string | null} scope */
  _publishScope(scope) {
    if (scope === null) return
    /** @type {CalendarDeleteChosen} */
    const chosen = { scope }
    this.applicationEventPublisher.publishEvent(chosen, false)
  }
}

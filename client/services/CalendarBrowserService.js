/**
 * Escape hatch: the calendar's browser work that a Go service cannot reach.
 *
 * Tachyon gives Go services no DOM events, no native dialogs and no media
 * queries, and its template handlers cannot cancel an event. So this service
 * owns the native HTML drag-and-drop transaction (dragover must call
 * preventDefault before a drop can happen), the delete confirmations, the
 * phone-size viewport check, the shell's `caduceus:shortcut:calendar:*` and
 * `caduceus:offline-write-result` window events, the quick-event form's native
 * submit, and the accessible names that DuVay's floating labels leave off
 * their inner controls. It hands every decision to CalendarService.
 */
@Service
export default class CalendarBrowserService {
  static _NARROW = '(max-width: 760px)'
  static _EVENT = '[data-calendar-event-id]'
  static _EVENT_ID = 'data-calendar-event-id'
  static _ALL_DAY = '[data-calendar-all-day-date]'
  static _DATE = '[data-calendar-date]'
  static _DROP = '[data-calendar-all-day-date], [data-calendar-date]'
  static _DROP_TARGET = 'calendar-drop-target'
  static _QUICK_FORM = '.quick-event-popover'
  static _ARIA_LABEL = 'aria-label'
  static _LABELLED = '.event-modal w-text-field, .event-modal w-textarea, .quick-event-popover w-text-field'
  static _SHORTCUTS = [
    'calendar:move-event-up', 'calendar:move-event-down', 'calendar:move-event-left', 'calendar:move-event-right',
    'calendar:resize-event-up', 'calendar:resize-event-down', 'calendar:resize-event-left', 'calendar:resize-event-right',
    'calendar:view-day', 'calendar:view-week', 'calendar:view-month', 'calendar:view-agenda',
    'calendar:go-to-today', 'calendar:navigate-next', 'calendar:navigate-previous', 'calendar:refresh-calendars',
  ]

  /** @type {TacTaskScheduler} */
  @Needs
  taskScheduler
  calendar
  _dragEventId = ''
  _opened = false
  /** @type {Array<[EventTarget, string, (event: any) => void]>} */
  _bindings = []
  /** @type {MutationObserver | null} */
  _observer = null

  @OnStart
  start() {
    const type = CalendarBrowserService
    this._bindings = [
      [window, 'resize', () => this._run(() => this.calendar.viewport(this._narrow()))],
      [window, 'caduceus:offline-write-result', event => this._offlineResult(event.detail)],
      [document, 'dragstart', event => this._dragStart(event, this._closest(event.target, type._EVENT))],
      [document, 'dragover', event => this._dragOver(event, this._closest(event.target, type._DROP))],
      [document, 'dragleave', event => this._dragLeave(event, this._closest(event.target, type._DROP))],
      [document, 'drop', event => this._drop(event, this._closest(event.target, type._ALL_DAY), this._closest(event.target, type._DATE))],
      [document, 'dragend', () => this._clearDrag()],
      [document, 'focusin', event => this._focusIn(this._closest(event.target, type._EVENT))],
      [document, 'submit', event => this._preventQuickSubmit(event, this._closest(event.target, type._QUICK_FORM))],
      ...type._SHORTCUTS.map(name => this._shortcutBinding(name)),
    ]
    this._bindings.forEach(([target, name, handler]) => target.addEventListener(name, handler))
    this._observer = new MutationObserver(() => this._labelFields())
    this._observer.observe(document.body, { childList: true, subtree: true })
  }

  @OnStop
  stop() {
    this._bindings.forEach(([target, name, handler]) => target.removeEventListener(name, handler))
    this._bindings = []
    this._observer?.disconnect()
  }

  /**
   * The first load waits for the session, because the browser transport
   * carries the bearer token only once SessionService has it.
   * @param {SessionChanged} changed
   */
  @OnEvent({"replay":true})
  session(changed) {
    this._openWhenSignedIn(changed)
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
   * whole series.
   * @param {string} seriesId
   */
  confirmDelete(seriesId) {
    const scope = this._deleteScope(seriesId)
    return this._deleteWhenChosen(scope)
  }

  /** @param {SessionChanged} changed */
  _openWhenSignedIn(changed) {
    if (!changed.signedIn || this._opened) return
    this._opened = true
    setTimeout(() => this._run(() => this.calendar.open(changed, this._narrow())), 0)
  }

  /** @param {() => Promise<unknown>} action */
  async _run(action) {
    await Promise.resolve().then(action).catch(() => undefined)
    await this.taskScheduler.render()
  }

  _narrow() {
    return window.matchMedia(CalendarBrowserService._NARROW).matches
  }

  /** @param {string} name @returns {[EventTarget, string, () => void]} */
  _shortcutBinding(name) {
    return [window, `caduceus:shortcut:${name}`, () => this._run(() => this.calendar.shortcut(name))]
  }

  /** @param {EventTarget | null} target @param {string} selector @returns {HTMLElement | null} */
  _closest(target, selector) {
    if (!(target instanceof Element)) return null
    return /** @type {HTMLElement | null} */ (target.closest(selector))
  }

  /** @param {any} detail */
  _offlineResult(detail) {
    if (detail?.kind === 'calendar' && detail?.status === 'accepted') this._run(() => this.calendar.reload())
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

  /** @param {DragEvent} event @param {HTMLElement | null} allDay @param {HTMLElement | null} day */
  _drop(event, allDay, day) {
    const id = this._dragEventId
    this._clearDrag()
    const call = this._dropCall(id, allDay, day)
    this._runDrop(event, call)
  }

  /** @param {string} id @param {HTMLElement | null} allDay @param {HTMLElement | null} day */
  _dropCall(id, allDay, day) {
    switch (true) {
      case !id: return null
      case !!allDay: return () => this.calendar.dropOnAllDay(id, String(allDay?.dataset.calendarAllDayDate))
      case !!day: return () => this.calendar.dropOnDate(id, String(day?.dataset.calendarDate))
      default: return null
    }
  }

  /** @param {DragEvent} event @param {(() => Promise<unknown>) | null} call */
  _runDrop(event, call) {
    if (!call) return
    event.preventDefault()
    event.stopPropagation()
    this._run(call)
  }

  _clearDrag() {
    document.querySelectorAll(`.${CalendarBrowserService._DROP_TARGET}`).forEach(node => node.classList.remove(CalendarBrowserService._DROP_TARGET))
    this._dragEventId = ''
  }

  /** @param {HTMLElement | null} chip */
  _focusIn(chip) {
    if (!chip) return
    this._run(() => this.calendar.focusEvent(String(chip.getAttribute(CalendarBrowserService._EVENT_ID))))
  }

  /** Tachyon still runs the form's on:submit handler; only the page load is cancelled.
   * @param {Event} event @param {HTMLElement | null} form */
  _preventQuickSubmit(event, form) {
    if (form) event.preventDefault()
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

  /** @param {string} seriesId @returns {string | null} */
  _deleteScope(seriesId) {
    const ask = seriesId ? this._askSeries : this._askEvent
    return ask.call(this)
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
  _deleteWhenChosen(scope) {
    if (scope === null) return undefined
    return this.calendar.deleteEditing(scope)
  }
}

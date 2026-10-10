// @ts-check

import {
  addCalendarDays,
  allDayRangeForDate,
  calendarDateValue,
  calendarDaysBetween,
  calendarCursorForEvent,
  eventCoversCalendarDate,
  isCalendarDate,
  localDateTimeValue,
  localIsoDate,
  moveTimedRangeToDate,
  shiftAllDayRangeToDate,
  timedRangeForAllDayDates,
} from '../../shared/scripts/calendar-date.js'
import { formatPreferenceDate } from '../../shared/scripts/mail-preferences.js'

/**
 * Calendar page — month view with create/edit modal.
 *
 * Renders a Sunday-start month grid (6 rows × 7 cols, padded with the
 * trailing days from the previous and leading days from the next month so
 * weeks stay rectangular). Events for the visible month are fetched once
 * per cursor change and bucketed into their starting day's cell.
 *
 * Event sources are surfaced via the `source` field on the API response:
 *   - `manual`  — created via the modal here.
 *   - `invite`  — auto-imported by `server/services/calendar-import.js` when
 *                 an inbound email carried an iCalendar VEVENT.
 *
 * The component never mutates the URL — `cursor` lives in-memory and is
 * advanced via the toolbar buttons. Deep-linking by date is a future PR.
 */
export default class {
  static __tachyonOnMount = ['init', 'bindResponsiveViewMode', 'bindKeyboardShortcuts', 'bindDragAndDrop', 'bindOfflineCalendarWrites']

  /** @type {boolean} */
  get canUseApp() { return !!(window._caduceus?.auth?.isLoggedIn || (typeof sessionStorage !== 'undefined' && sessionStorage.getItem('caduceus_token'))) }
  /** @type {boolean} */
  get canShowLogin() { return !this.canUseApp }

  /** @type {Array<Record<string, any>>} All events for the visible month. */
  $events = []

  /** @type {Array<Record<string, any>>} Writable connected calendar collections. */
  $providerCalendars = []

  /** @type {Array<Record<string, any>>} Same-domain teammates' events for
   *  the visible month — only fetched when {@link $$showTeam} is true. */
  $teamEvents = []

  /** Cursor day (any day in the month being viewed). Persisted via `$$` so
   *  navigating away and back returns to the same month. */
  $$cursor = new Date().toISOString()

  /** Whether to overlay the same-domain teammates' events on the grid.
   *  localStorage-backed so the user's choice survives reloads. */
  $$showTeam = false

  /** @type {'day'|'three-day'|'work-week'|'week'|'month'|'agenda'|'split'} */
  $$viewMode = 'month'

  /** Modal state — open + editing-id + form fields. */
  $modalOpen = false
  /** @type {string} */
  $editingId = ''
  /** @type {string} */
  $editingSeriesId = ''
  /** @type {string} */
  $editingRecurrenceId = ''
  /** @type {'event'|'instance'|'series'} */
  $editScope = 'event'
  $editingReadOnly = false
  /** @type {Record<string, any> | null} */
  $editingOccurrence = null
  /** @type {HTMLElement | null} */
  $modalReturnFocus = null
  /** @type {string} */
  $modalReturnSelector = ''
  /** @type {string} Exact owned event most recently focused for move/resize commands. */
  $keyboardEventId = ''
  $form = { title: '', start: '', end: '', allDay: false, location: '', description: '', repeat: 'none', recurrence: '', timeZone: 'UTC', calendarId: 'local', error: '', saving: false }
  $mutationKey = ''
  $quickCreateOpen = false
  $quickCreateDate = ''
  $quickCreateTitle = ''
  $quickCreateError = ''
  $quickCreateSaving = false
  $dragEventId = ''
  $dragOverDate = ''

  bindOfflineCalendarWrites() {
    if (window._caduceusCalendarOfflineWriteHandler) {
      window.removeEventListener('caduceus:offline-write-result', window._caduceusCalendarOfflineWriteHandler)
    }
    window._caduceusCalendarOfflineWriteHandler = event => {
      const detail = /** @type {CustomEvent<any>} */ (event).detail
      if (detail?.kind === 'calendar' && detail.status === 'accepted') void this.loadEvents()
    }
    window.addEventListener('caduceus:offline-write-result', window._caduceusCalendarOfflineWriteHandler)
  }

  /** Queue only transport failures; HTTP validation/conflict responses remain visible to the caller. */
  async calendarWrite(path, method, body, message, coalesceKey = '') {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return { response: null, queued: false }
    try {
      return {
        response: await apiFetch(path, { method, ...(body ? { body: JSON.stringify(body) } : {}) }),
        queued: false,
      }
    } catch {
      await window._caduceus?.queueOfflineRequest?.({
        kind: 'calendar', path, method, body, message,
        ...(coalesceKey ? { coalesceKey } : {}),
        metadata: { calendar: true },
      })
      return { response: null, queued: true }
    }
  }

  get $quickCreateDateLabel() {
    if (!this.$quickCreateDate) return ''
    return formatPreferenceDate(`${this.$quickCreateDate}T12:00:00`, { weekday: 'long', month: 'long', day: 'numeric' })
  }

  /** @type {string[]} Localised Sunday-first weekday labels for the header row. */
  get $weekdayLabels() {
    // Use a fixed Sunday and format in UTC so the label doesn't drift to
    // Saturday in negative-offset timezones (which it would if we asked
    // toLocaleDateString to render a UTC-midnight date in local time).
    const ref = new Date(Date.UTC(2024, 8, 1)) // 2024-09-01 — a Sunday in UTC
    return Array.from({ length: 7 }, (_, i) => {
      const d = new Date(ref); d.setUTCDate(ref.getUTCDate() + i)
      return formatPreferenceDate(d, { weekday: 'short', timeZone: 'UTC' })
    })
  }

  get $viewModes() {
    return [
      { id: 'day', label: 'Day', className: '' },
      { id: 'three-day', label: '3-Day', className: 'calendar-mobile-only' },
      { id: 'work-week', label: 'Work Week', className: 'calendar-desktop-only' },
      { id: 'week', label: 'Week', className: 'calendar-desktop-only' },
      { id: 'month', label: 'Month', className: '' },
      { id: 'agenda', label: 'Agenda', className: 'calendar-mobile-only' },
      { id: 'split', label: 'Split View', className: 'calendar-desktop-only' },
    ]
  }

  get $calendarOptions() {
    return [
      { id: 'local', label: 'CADUCEUS calendar', color: '' },
      ...this.$providerCalendars.map(calendar => ({
        id: calendar.calendarId,
        label: [calendar.accountLabel, calendar.calendarName].filter(Boolean).join(' · ') || 'Connected calendar',
        color: calendar.calendarColor || calendar.accountColor || '',
      })),
    ]
  }

  get $allEvents() {
    return [
      ...(this.$events || []).map(e => ({
        ...e,
        source: e.source || 'manual',
        ownerLabel: e.source === 'provider'
          ? [e.accountLabel, e.calendarName].filter(Boolean).join(' · ') || 'Connected calendar'
          : 'Me',
      })),
      ...((this.$$showTeam && this.$teamEvents) || []).map(e => ({
        ...e,
        source: e.source || 'team',
        ownerLabel: e.ownerEmail || e.userEmail || e.owner || 'Team',
      })),
    ]
  }

  /** Human-readable title for the toolbar: "May 2026". */
  get $titleLabel() {
    const d = new Date(this.$$cursor)
    if (this.$$viewMode === 'day')
      return formatPreferenceDate(d, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' })
    if (this.$$viewMode === 'month')
      return formatPreferenceDate(d, { month: 'long', year: 'numeric' })
    const days = this.visibleDaysForMode(this.$$viewMode)
    const first = days[0]
    const last = days[days.length - 1]
    const sameMonth = first.getMonth() === last.getMonth()
    const left = formatPreferenceDate(first, { month: 'short', day: 'numeric' })
    const right = formatPreferenceDate(last, { month: sameMonth ? undefined : 'short', day: 'numeric', year: 'numeric' })
    return `${left} - ${right}`
  }

  get $agendaDays() {
    return this.visibleDaysForMode(this.$$viewMode).map(day => {
      const events = this.eventsForDay(day)
      return {
        date: day,
        isoDate: localIsoDate(day),
        label: formatPreferenceDate(day, { weekday: 'short', month: 'short', day: 'numeric' }),
        isToday: sameDay(day, new Date()),
        events,
        allDayEvents: events.filter(event => event.allDay),
        timedEvents: events.filter(event => !event.allDay),
      }
    })
  }

  get $agendaListDays() {
    return this.$agendaDays.filter(day => day.events.length > 0)
  }

  get $splitColumns() {
    const groups = new Map()
    for (const ev of this.$allEvents) {
      const key = ev.ownerLabel || (ev.source === 'team' ? 'Team' : 'Me')
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key).push(this.decorateEvent(ev))
    }
    return [...groups.entries()].map(([owner, events]) => ({
      owner,
      events: events.sort((a, b) => Date.parse(a.start) - Date.parse(b.start)),
    }))
  }

  /**
   * 6 × 7 grid of cells for the month containing `$$cursor`. Each cell has
   * `date`, `isCurrentMonth`, `isToday`, `dayLabel`, and bucketed `events`.
   *
   * The grid always starts on the Sunday of the week containing the 1st of
   * the cursor month, and runs for 42 days. This guarantees a stable
   * 6-row layout regardless of how the month aligns to the calendar.
   */
  get $weeks() {
    const cursor = new Date(this.$$cursor || new Date().toISOString())
    const firstOfMonth = new Date(cursor.getFullYear(), cursor.getMonth(), 1)
    // Walk back to the Sunday of that week.
    const start = new Date(firstOfMonth)
    start.setDate(start.getDate() - start.getDay())
    const today = new Date(); today.setHours(0,0,0,0)
    // Merge own + teammate events, tagging the latter so the chip renderer
    // can apply different styling.
    const events = /** @type {any[]} */ (this.$allEvents)

    /** @type {Array<Array<any>>} */
    const weeks = []
    for (let w = 0; w < 6; w++) {
      /** @type {Array<any>} */
      const row = []
      for (let d = 0; d < 7; d++) {
        const date = new Date(start)
        date.setDate(start.getDate() + (w * 7 + d))
        const dayEvents = events
          .filter(e => eventCoversCalendarDate(e, localIsoDate(date)))
          .map(e => this.decorateEvent(e))
        row.push({
          date,
          dayLabel: String(date.getDate()),
          isCurrentMonth: date.getMonth() === cursor.getMonth(),
          isToday: sameDay(date, today),
          isoDate: localIsoDate(date),
          events: dayEvents,
        })
      }
      weeks.push(row)
    }
    return weeks
  }

  /**
   * Tailwind-ish helper that picks the right modifier class for a cell.
   * @param {any} cell
   */
  cellClass(cell) {
    return [
      'calendar-cell',
      cell.isCurrentMonth ? '' : 'calendar-cell-other',
      cell.isToday ? 'calendar-cell-today' : '',
    ].filter(Boolean).join(' ')
  }

  /** @param {any} ev */
  eventChipClass(ev) {
    const source = ev.source || 'manual'
    return [
      'calendar-event-chip',
      source === 'invite' ? 'calendar-event-chip-invite' :
      source === 'provider' ? 'calendar-event-chip-provider' :
      source === 'team' ? 'calendar-event-chip-team' :
      'calendar-event-chip-manual',
      ev.isRecurring ? 'calendar-event-chip-recurring' : '',
      ev.id === this.$keyboardEventId ? 'keyboard-selected' : '',
    ].join(' ')
  }

  /** @param {Record<string, any>} event */
  eventDraggable(event) { return !this.isReadOnlyEvent(event) }

  get $editingReadOnlyMessage() {
    const event = this.$editingOccurrence
    if (event?.source === 'provider') {
      const origin = [event.accountLabel, event.calendarName].filter(Boolean).join(' · ') || 'the connected calendar'
      return `This event is read-only from ${origin}. Changes made at the provider will synchronize here.`
    }
    return 'This event is read-only because it belongs to a teammate, is an invitation, or has already ended.'
  }

  get $editingWhenLabel() {
    const event = this.$editingOccurrence
    if (!event?.start) return ''
    /** @type {Intl.DateTimeFormatOptions} */
    const options = event.allDay
      ? { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' }
      : { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' }
    const start = formatPreferenceDate(event.start, options)
    const end = event.end ? formatPreferenceDate(event.end, options) : ''
    return end && end !== start ? `${start} – ${end}` : start
  }

  async setViewMode(eventOrMode, explicitMode) {
    const mode = typeof explicitMode === 'string' ? explicitMode : eventOrMode
    if (!this.$viewModes.some(candidate => candidate.id === mode)) return
    const selected = this.$events.find(candidate => candidate.id === this.$keyboardEventId)
    const selectedCursor = calendarCursorForEvent(selected)
    if (selectedCursor) this.$$cursor = selectedCursor.toISOString()
    this.$$viewMode = mode
    if (mode === 'split') this.$$showTeam = true
    await this.loadEvents()
    this.scheduleSelectedEventIntoView()
  }

  visibleDaysForMode(mode = this.$$viewMode) {
    const cursor = new Date(this.$$cursor || new Date().toISOString())
    cursor.setHours(0, 0, 0, 0)
    if (mode === 'day') return [cursor]
    if (mode === 'three-day') return Array.from({ length: 3 }, (_, i) => addDays(cursor, i))
    if (mode === 'agenda') return Array.from({ length: 14 }, (_, i) => addDays(cursor, i))
    const start = new Date(cursor)
    const day = start.getDay()
    if (mode === 'work-week') {
      const mondayOffset = day === 0 ? -6 : 1 - day
      start.setDate(start.getDate() + mondayOffset)
      return Array.from({ length: 5 }, (_, i) => addDays(start, i))
    }
    start.setDate(start.getDate() - day)
    return Array.from({ length: 7 }, (_, i) => addDays(start, i))
  }

  eventsForDay(day) {
    const date = localIsoDate(day)
    return /** @type {any[]} */ (this.$allEvents)
      .filter(e => eventCoversCalendarDate(e, date))
      .map(e => this.decorateEvent(e))
      .sort((a, b) => Date.parse(a.start) - Date.parse(b.start))
  }

  /** @returns {any} */
  decorateEvent(ev) {
    return {
      ...ev,
      timeLabel: ev.allDay ? '' : formatPreferenceDate(ev.start, { hour: '2-digit', minute: '2-digit' }),
      recurrenceLabel: ev.isRecurring ? (ev.isException ? 'Edited occurrence' : 'Recurring event') : '',
    }
  }

  /**
   * Fetch the events covering the visible month plus a day of slack each
   * side (so events that span the boundary still render).
   *
   * @async
   * @returns {Promise<void>}
   */
  async loadEvents() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const { start, end } = this.fetchRange()
    try {
      const res = await apiFetch(`/calendar/events?start=${encodeURIComponent(start.toISOString())}&end=${encodeURIComponent(end.toISOString())}`)
      this.$events = res?.ok ? await res.json() : []
    } catch {
      this.$events = []
    }
    if (this.$$showTeam) await this.loadTeamEvents(start.toISOString(), end.toISOString())
    await this.renderCurrentState()
  }

  async loadProviderCalendars() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const response = await apiFetch('/calendar/calendars')
      const payload = response?.ok ? await response.json() : { calendars: [] }
      this.$providerCalendars = Array.isArray(payload?.calendars) ? payload.calendars.filter(calendar => calendar?.writable !== false) : []
    } catch {
      this.$providerCalendars = []
    }
  }

  fetchRange() {
    const cursor = new Date(this.$$cursor || new Date().toISOString())
    if (this.$$viewMode === 'month') {
      return {
        start: new Date(cursor.getFullYear(), cursor.getMonth() - 1, 25),
        end: new Date(cursor.getFullYear(), cursor.getMonth() + 1, 7, 23, 59, 59),
      }
    }
    const days = this.visibleDaysForMode(this.$$viewMode)
    const start = new Date(days[0]); start.setHours(0, 0, 0, 0)
    const end = new Date(days[days.length - 1]); end.setHours(23, 59, 59, 999)
    return { start, end }
  }

  /**
   * Pull events from same-domain teammates who have opted into sharing.
   * Server applies the privacy redaction per their `calendarVisibility`
   * setting — we just render whatever it returns.
   * @param {string} start ISO
   * @param {string} end   ISO
   */
  async loadTeamEvents(start, end) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.$teamEvents = []; return }
    try {
      const res = await apiFetch(`/calendar/team-events?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`)
      this.$teamEvents = res?.ok ? await res.json() : []
    } catch {
      this.$teamEvents = []
    }
  }

  /**
   * Flip the team-overlay toggle. Loads/clears team events to match the
   * new state without needing to re-fetch own events.
   */
  async toggleShowTeam() {
    this.$$showTeam = !this.$$showTeam
    if (this.$$showTeam) {
      const { start, end } = this.fetchRange()
      await this.loadTeamEvents(start.toISOString(), end.toISOString())
    } else {
      this.$teamEvents = []
    }
  }

  async init() {
    this.syncViewModeForViewport()
    await Promise.all([this.loadProviderCalendars(), this.loadEvents()])
  }

  bindResponsiveViewMode() {
    window.removeEventListener('resize', /** @type {() => void} */ (window._caduceusCalendarViewModeGuard))
    window._caduceusCalendarViewModeGuard = () => {
      if (this.syncViewModeForViewport()) this.loadEvents()
    }
    window.addEventListener('resize', window._caduceusCalendarViewModeGuard)
  }

  bindKeyboardShortcuts() {
    const previous = window._caduceusCalendarShortcutHandlers || {}
    for (const [name, handler] of Object.entries(previous)) {
      window.removeEventListener(`caduceus:shortcut:${name}`, handler)
    }
    const renderAfter = async operation => {
      await operation()
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
    const handlers = {
      'calendar:move-event-up': () => this.moveSelectedEvent('up', false),
      'calendar:move-event-down': () => this.moveSelectedEvent('down', false),
      'calendar:move-event-left': () => this.moveSelectedEvent('left', false),
      'calendar:move-event-right': () => this.moveSelectedEvent('right', false),
      'calendar:resize-event-up': () => this.moveSelectedEvent('up', true),
      'calendar:resize-event-down': () => this.moveSelectedEvent('down', true),
      'calendar:resize-event-left': () => this.moveSelectedEvent('left', true),
      'calendar:resize-event-right': () => this.moveSelectedEvent('right', true),
      'calendar:view-day': () => { void renderAfter(() => this.setViewMode('day')) },
      'calendar:view-week': () => { void renderAfter(() => this.setViewMode('week')) },
      'calendar:view-month': () => { void renderAfter(() => this.setViewMode('month')) },
      'calendar:view-agenda': () => { void renderAfter(() => this.setViewMode('agenda')) },
      'calendar:go-to-today': () => { void renderAfter(() => this.goToToday()) },
      'calendar:navigate-next': () => { void renderAfter(() => this.shiftCursor(1)) },
      'calendar:navigate-previous': () => { void renderAfter(() => this.shiftCursor(-1)) },
      'calendar:refresh-calendars': () => { void renderAfter(() => this.loadEvents()) },
    }
    window._caduceusCalendarShortcutHandlers = handlers
    for (const [name, handler] of Object.entries(handlers)) {
      window.addEventListener(`caduceus:shortcut:${name}`, handler)
    }
    if (window._caduceusCalendarFocusHandler) document.removeEventListener('focusin', window._caduceusCalendarFocusHandler)
    window._caduceusCalendarFocusHandler = event => {
      const target = event.target instanceof Element ? event.target.closest('[data-calendar-event-id]') : null
      const eventId = target?.getAttribute('data-calendar-event-id') || ''
      const calendarEvent = this.$events.find(candidate => candidate.id === eventId)
      if (calendarEvent) this.focusEvent(calendarEvent, event)
    }
    document.addEventListener('focusin', window._caduceusCalendarFocusHandler)
  }

  /** Remember only an owned event as the keyboard move/resize target. */
  focusEvent(event, domEvent) {
    this.$keyboardEventId = event?.source === 'team' ? '' : String(event?.id || '')
    for (const chip of document.querySelectorAll('[data-calendar-event-id].keyboard-selected')) chip.classList.remove('keyboard-selected')
    const target = domEvent?.target instanceof Element
      ? domEvent.target.closest('[data-calendar-event-id]')
      : domEvent?.currentTarget
    if (this.$keyboardEventId && target instanceof HTMLElement) target.classList.add('keyboard-selected')
  }

  /**
   * Match Mailspring calendar semantics: arrows move, Shift+arrows resize;
   * vertical changes are 15 minutes in day/week views and horizontal changes
   * are one calendar day. Month/agenda vertical commands intentionally no-op.
   */
  async moveSelectedEvent(direction, resize) {
    const event = this.$events.find(candidate => candidate.id === this.$keyboardEventId)
    if (!event || this.isReadOnlyEvent(event)) return
    if (event.allDay) {
      if (direction === 'up' || direction === 'down') return
      const amount = direction === 'left' ? -1 : 1
      const current = shiftAllDayRangeToDate({ start: event.start, end: event.end || event.start }, calendarDateValue(event.start))
      const next = resize
        ? { start: current.start, end: addCalendarDays(current.end, amount), allDay: true }
        : shiftAllDayRangeToDate(current, addCalendarDays(current.start, amount))
      if (calendarDaysBetween(next.start, next.end) < 1) return
      await this.persistEventTimes(event, next, resize ? 'Event resized.' : 'Event moved.')
      return
    }
    const start = new Date(event.start)
    const end = new Date(event.end || event.start)
    if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime())) return
    const timedView = ['day', 'three-day', 'work-week', 'week'].includes(this.$$viewMode)
    if ((direction === 'up' || direction === 'down') && !timedView) return
    const delta = direction === 'up' ? -15 : direction === 'down' ? 15 : direction === 'left' ? -1440 : 1440
    const shift = (date, minutes) => {
      const next = new Date(date)
      if (Math.abs(minutes) === 1440) next.setDate(next.getDate() + Math.sign(minutes))
      else next.setMinutes(next.getMinutes() + minutes)
      return next
    }
    const nextStart = resize ? start : shift(start, delta)
    const nextEnd = shift(end, delta)
    if (resize && nextEnd.getTime() - nextStart.getTime() < 15 * 60 * 1000) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const targetId = event.seriesId || event.id
    const query = event.isRecurring
      ? `?scope=instance&occurrenceStart=${encodeURIComponent(event.recurrenceId || event.start)}`
      : ''
    const mutation = { start: nextStart.toISOString(), end: nextEnd.toISOString(), idempotencyKey: crypto.randomUUID() }
    const { response, queued } = await this.calendarWrite(
      `/calendar/events/${encodeURIComponent(targetId)}${query}`, 'PUT', mutation,
      resize ? 'Resize calendar event' : 'Move calendar event', `calendar:${targetId}`,
    )
    if (queued) { window._caduceus?.toast(resize ? 'Event resize queued.' : 'Event move queued.'); return }
    if (!response?.ok) { window._caduceus?.toast('Could not update event.'); return }
    await response.json()
    window._caduceus?.toast(resize ? 'Event resized.' : 'Event moved.')
    await this.loadEvents()
  }

  syncViewModeForViewport() {
    const isMobile = window.matchMedia?.('(max-width: 760px)').matches
    if (!isMobile || !['work-week', 'week', 'split'].includes(this.$$viewMode)) return false
    this.$$viewMode = 'day'
    return true
  }

  /** Move the cursor forward (+1) or backward (-1) one month. */
  async shiftCursor(eventOrDirection, explicitDirection) {
    const direction = typeof explicitDirection === 'number' ? explicitDirection : eventOrDirection
    if (direction !== -1 && direction !== 1) return
    const cursor = new Date(this.$$cursor || new Date().toISOString())
    if (this.$$viewMode === 'month') cursor.setMonth(cursor.getMonth() + direction)
    else if (this.$$viewMode === 'agenda') cursor.setDate(cursor.getDate() + (direction * 14))
    else if (this.$$viewMode === 'three-day') cursor.setDate(cursor.getDate() + (direction * 3))
    else cursor.setDate(cursor.getDate() + (this.$$viewMode === 'day' ? direction : direction * 7))
    this.$$cursor = cursor.toISOString()
    await this.loadEvents()
  }

  /** Jump to today. */
  async goToToday() {
    this.$$cursor = new Date().toISOString()
    await this.loadEvents()
  }

  /** Open the modal in create-mode pre-filled with the clicked cell's day. */
  async onCellClick(domEvent, cell) {
    if (!cell?.date) return
    await this.openQuickCreate(domEvent, cell.date)
  }

  async openQuickCreate(domEvent, date) {
    this.captureModalReturnFocus(domEvent)
    this.$quickCreateDate = localIsoDate(new Date(date))
    this.$quickCreateTitle = ''
    this.$quickCreateError = ''
    this.$quickCreateSaving = false
    this.$quickCreateOpen = true
    await this.renderCurrentState()
    requestAnimationFrame(() => {
      const control = document.querySelector('.quick-event-popover input')
      if (control instanceof HTMLElement) {
        control.setAttribute('aria-label', 'Title')
        control.focus()
      }
    })
  }

  async openCreateForDate(date) {
    const start = new Date(date)
    start.setHours(9, 0, 0, 0)
    const end = new Date(date)
    end.setHours(10, 0, 0, 0)
    this.$editingId = ''
    this.$editingSeriesId = ''
    this.$editingRecurrenceId = ''
    this.$editScope = 'event'
    this.$editingReadOnly = false
    this.$mutationKey = crypto.randomUUID()
    this.$form = {
      title: '',
      start: localDateTimeValue(start),
      end: localDateTimeValue(end),
      allDay: false,
      location: '',
      description: '',
      repeat: 'none',
      recurrence: '',
      timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      calendarId: 'local',
      error: '',
      saving: false,
    }
    this.$modalOpen = true
    await this.renderCurrentState()
    this.stabilizeModalFocus()
  }

  updateQuickCreateTitle(event) {
    this.$quickCreateTitle = String(event?.value ?? event?.target?.value ?? '')
  }

  async createQuickEvent(event) {
    event?.preventDefault?.()
    const title = this.$quickCreateTitle.trim()
    if (!title) { this.$quickCreateError = 'Title is required'; return }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || !this.$quickCreateDate || this.$quickCreateSaving) return
    const start = new Date(`${this.$quickCreateDate}T09:00:00`)
    const end = new Date(start); end.setHours(10)
    this.$quickCreateSaving = true
    const mutation = {
      title, start: start.toISOString(), end: end.toISOString(),
      timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      idempotencyKey: crypto.randomUUID(),
    }
    const { response, queued } = await this.calendarWrite('/calendar/events', 'POST', mutation, 'Create calendar event')
    if (queued) {
      await this.closeQuickCreate(false)
      window._caduceus?.toast('Event creation queued.')
      return
    }
    if (!response?.ok) {
      this.$quickCreateSaving = false
      this.$quickCreateError = 'Create failed'
      return
    }
    await this.closeQuickCreate(false)
    window._caduceus?.toast('Event created.')
    await this.loadEvents()
  }

  async openQuickCreateDetails(event) {
    event?.preventDefault?.()
    const date = new Date(`${this.$quickCreateDate}T12:00:00`)
    await this.closeQuickCreate(false)
    await this.openCreateForDate(date)
  }

  async closeQuickCreate(restoreFocus = true) {
    this.$quickCreateOpen = false
    const returnFocus = this.$modalReturnFocus
    await this.renderCurrentState()
    if (restoreFocus) requestAnimationFrame(() => returnFocus?.isConnected && returnFocus.focus())
  }

  async onQuickCreateKeydown(event) {
    if (event?.key === 'Escape') {
      event.preventDefault()
      await this.closeQuickCreate()
      return
    }
    if (event?.key !== 'Tab') return
    const dialog = event.currentTarget instanceof HTMLElement ? event.currentTarget : document.querySelector('.quick-event-popover')
    if (!(dialog instanceof HTMLElement)) return
    const nodes = this.modalFocusableControls(dialog)
    const head = nodes[0]
    const tail = nodes[nodes.length - 1]
    if (event.shiftKey && document.activeElement === head && tail instanceof HTMLElement) {
      event.preventDefault(); tail.focus()
    } else if (!event.shiftKey && document.activeElement === tail && head instanceof HTMLElement) {
      event.preventDefault(); head.focus()
    }
  }

  onEventDragStart(domEvent, event) {
    domEvent?.stopPropagation?.()
    if (!event?.id || this.isReadOnlyEvent(event)) {
      domEvent?.preventDefault?.()
      return
    }
    this.$dragEventId = event.id
    domEvent.dataTransfer.effectAllowed = 'move'
    domEvent.dataTransfer.setData('text/plain', event.id)
  }

  onEventDragEnd() {
    this.clearDragTarget()
    this.$dragEventId = ''
  }

  /**
   * Native delegated drag listeners keep the source node stable for the whole
   * HTML drag transaction. Tachyon rerenders after declarative handlers, which
   * would otherwise detach the source between dragstart and drop.
   */
  bindDragAndDrop() {
    if (window._caduceusCalendarDragHandlers) {
      for (const [name, handler] of Object.entries(window._caduceusCalendarDragHandlers)) document.removeEventListener(name, handler)
    }
    const handlers = {
      dragstart: event => {
        const chip = event.target instanceof Element ? event.target.closest('[data-calendar-event-id]') : null
        if (!chip) return
        const calendarEvent = /** @type {any[]} */ (this.$allEvents).find(candidate => candidate.id === chip.getAttribute('data-calendar-event-id'))
        this.onEventDragStart(event, calendarEvent)
      },
      dragover: event => {
        const target = event.target instanceof Element ? event.target.closest('[data-calendar-all-day-date], [data-calendar-date]') : null
        if (target) this.setDragTarget(event, target.getAttribute('data-calendar-all-day-date') || target.getAttribute('data-calendar-date'))
      },
      dragleave: event => {
        const target = event.target instanceof Element ? event.target.closest('[data-calendar-all-day-date], [data-calendar-date]') : null
        const date = target?.getAttribute('data-calendar-all-day-date') || target?.getAttribute('data-calendar-date')
        if (target && !target.contains(event.relatedTarget)) this.leaveDragTarget({ ...event, currentTarget: target }, date)
      },
      drop: event => {
        const allDayTarget = event.target instanceof Element ? event.target.closest('[data-calendar-all-day-date]') : null
        if (allDayTarget) {
          void this.dropEventOnAllDay(event, allDayTarget.getAttribute('data-calendar-all-day-date'))
          return
        }
        const target = event.target instanceof Element ? event.target.closest('[data-calendar-date]') : null
        if (target) void this.dropEventOnDate(event, target.getAttribute('data-calendar-date'))
      },
      dragend: () => this.onEventDragEnd(),
    }
    window._caduceusCalendarDragHandlers = handlers
    for (const [name, handler] of Object.entries(handlers)) document.addEventListener(name, handler)
  }

  onCellDragOver(domEvent, cell) { this.setDragTarget(domEvent, cell?.isoDate) }
  onDayDragOver(domEvent, day) { this.setDragTarget(domEvent, day?.isoDate) }
  onCellDragLeave(domEvent, cell) { this.leaveDragTarget(domEvent, cell?.isoDate) }
  onDayDragLeave(domEvent, day) { this.leaveDragTarget(domEvent, day?.isoDate) }
  async onCellDrop(domEvent, cell) { await this.dropEventOnDate(domEvent, cell?.isoDate) }
  async onDayDrop(domEvent, day) { await this.dropEventOnDate(domEvent, day?.isoDate) }

  setDragTarget(domEvent, isoDate) {
    if (!this.$dragEventId || !isoDate) return
    domEvent?.preventDefault?.()
    if (domEvent?.dataTransfer) domEvent.dataTransfer.dropEffect = 'move'
    this.$dragOverDate = isoDate
    domEvent?.currentTarget?.classList?.add('calendar-drop-target')
  }

  leaveDragTarget(domEvent, isoDate) {
    if (this.$dragOverDate === isoDate) this.$dragOverDate = ''
    domEvent?.currentTarget?.classList?.remove('calendar-drop-target')
  }

  clearDragTarget() {
    for (const target of document.querySelectorAll('.calendar-drop-target')) target.classList.remove('calendar-drop-target')
    this.$dragOverDate = ''
  }

  async dropEventOnDate(domEvent, isoDate) {
    domEvent?.preventDefault?.()
    domEvent?.stopPropagation?.()
    const eventId = this.$dragEventId || domEvent?.dataTransfer?.getData('text/plain')
    const event = this.$events.find(candidate => candidate.id === eventId)
    this.clearDragTarget()
    this.$dragEventId = ''
    if (!event || !isoDate || this.isReadOnlyEvent(event)) return
    const range = { start: String(event.start), end: String(event.end || event.start) }
    const next = event.allDay
      ? shiftAllDayRangeToDate(range, isoDate)
      : moveTimedRangeToDate(range, isoDate)
    if (event.allDay ? next.start === calendarDateValue(event.start) : localIsoDate(new Date(event.start)) === isoDate) return
    await this.persistEventTimes(event, next, 'Event moved.')
  }

  async dropEventOnAllDay(domEvent, isoDate) {
    domEvent?.preventDefault?.()
    domEvent?.stopPropagation?.()
    const eventId = this.$dragEventId || domEvent?.dataTransfer?.getData('text/plain')
    const event = this.$events.find(candidate => candidate.id === eventId)
    this.clearDragTarget()
    this.$dragEventId = ''
    if (!event || !isoDate || this.isReadOnlyEvent(event)) return
    // A recurring timed occurrence keeps its representation because converting
    // it would change the recurrence identity and orphan the exception.
    const range = { start: String(event.start), end: String(event.end || event.start) }
    const next = event.allDay
      ? shiftAllDayRangeToDate(range, isoDate)
      : event.isRecurring ? moveTimedRangeToDate(range, isoDate) : allDayRangeForDate(isoDate)
    await this.persistEventTimes(event, next, event.isRecurring && !event.allDay ? 'Event moved.' : 'Event moved to all day.')
  }

  async persistEventTimes(event, range, message) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return false
    const targetId = event.seriesId || event.id
    const query = event.isRecurring ? `?scope=instance&occurrenceStart=${encodeURIComponent(event.recurrenceId || event.start)}` : ''
    const mutation = { start: range.start, end: range.end, allDay: range.allDay, idempotencyKey: crypto.randomUUID() }
    const { response, queued } = await this.calendarWrite(
      `/calendar/events/${encodeURIComponent(targetId)}${query}`, 'PUT', mutation, message, `calendar:${targetId}`,
    )
    if (queued) { window._caduceus?.toast(`${message.replace(/\.$/, '')} queued.`); return true }
    if (!response?.ok) { window._caduceus?.toast('Could not update event.'); return false }
    window._caduceus?.toast(message)
    await this.loadEvents()
    return true
  }

  /** Open the modal in create-mode (no cell context). */
  async openCreateModal(domEvent) {
    this.captureModalReturnFocus(domEvent)
    const start = new Date(); start.setMinutes(0, 0, 0)
    if (start.getHours() < 9) start.setHours(9)
    const end = new Date(start); end.setHours(start.getHours() + 1)
    this.$editingId = ''
    this.$editingSeriesId = ''
    this.$editingRecurrenceId = ''
    this.$editScope = 'event'
    this.$editingReadOnly = false
    this.$mutationKey = crypto.randomUUID()
    this.$form = {
      title: '',
      start: toLocalInputValue(start),
      end: toLocalInputValue(end),
      allDay: false,
      location: '',
      description: '',
      repeat: 'none',
      recurrence: '',
      timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      calendarId: 'local',
      error: '',
      saving: false,
    }
    this.$modalOpen = true
    await this.renderCurrentState()
    this.stabilizeModalFocus()
  }

  /** Open the modal in edit-mode with the clicked event's data. */
  async onEventClick(domEvent, ev) {
    domEvent?.stopPropagation?.()
    if (!ev?.id) return
    this.captureModalReturnFocus(domEvent)
    this.focusEvent(ev, domEvent)
    this.$editingOccurrence = { ...ev }
    this.$editingSeriesId = ev.seriesId || ''
    this.$editingRecurrenceId = ev.recurrenceId || ''
    this.$editScope = ev.isRecurring ? 'instance' : 'event'
    this.$editingId = ev.seriesId || ev.id
    this.$editingReadOnly = this.isReadOnlyEvent(ev)
    this.$mutationKey = crypto.randomUUID()
    this.$form = {
      title: ev.title || '',
      start: ev.allDay ? calendarDateValue(ev.start) : localDateTimeValue(new Date(ev.start)),
      end: ev.allDay ? calendarDateValue(ev.end || ev.start) : localDateTimeValue(new Date(ev.end || ev.start)),
      allDay: !!ev.allDay,
      location: ev.location || '',
      description: ev.description || '',
      repeat: recurrencePreset(ev.recurrence),
      recurrence: ev.recurrence || '',
      timeZone: ev.timeZone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      calendarId: ev.calendarId || 'local',
      error: '',
      saving: false,
    }
    this.$modalOpen = true
    await this.renderCurrentState()
    this.stabilizeModalFocus()
  }

  async closeModal() {
    this.$modalOpen = false
    this.$editingId = ''
    this.$editingSeriesId = ''
    this.$editingRecurrenceId = ''
    this.$editingOccurrence = null
    this.$editingReadOnly = false
    const returnFocus = this.$modalReturnFocus
    const returnSelector = this.$modalReturnSelector
    this.$modalReturnFocus = null
    this.$modalReturnSelector = ''
    await this.renderCurrentState()
    requestAnimationFrame(() => {
      const target = returnSelector ? document.querySelector(returnSelector) : returnFocus
      if (target instanceof HTMLElement && target.isConnected) target.focus()
    })
  }

  /** Prevent backdrop click-to-close when the click originated inside the
   *  dialog itself. */
  stopBackdrop(e) { e?.stopPropagation?.() }

  /** Keep keyboard focus inside the modal and make Escape deterministic from
   *  the moment the dialog enters the DOM. */
  async onModalKeydown(event) {
    if (event?.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      await this.closeModal()
      return
    }
    if (event?.key !== 'Tab') return
    const dialog = event.currentTarget instanceof HTMLElement
      ? event.currentTarget
      : document.querySelector('.event-modal')
    if (!(dialog instanceof HTMLElement)) return
    const nodes = this.modalFocusableControls(dialog)
    if (!nodes.length) return
    const head = nodes[0]
    const tail = nodes[nodes.length - 1]
    if (event.shiftKey && document.activeElement === head && tail instanceof HTMLElement) {
      event.preventDefault()
      tail.focus()
    } else if (!event.shiftKey && document.activeElement === tail && head instanceof HTMLElement) {
      event.preventDefault()
      head.focus()
    }
  }

  /** @param {Event & { value?: unknown }} event @param {'title'|'start'|'end'|'location'|'description'|'repeat'|'calendarId'} field */
  updateFormField(event, field) {
    const target = /** @type {HTMLInputElement | HTMLTextAreaElement | null} */ (event?.target)
    const value = event?.value ?? target?.value ?? ''
    this.$form = { ...this.$form, [field]: String(value) }
  }

  /** @param {any} event */
  updateFormTitle(event) { this.updateFormField(event, 'title') }
  /** @param {any} event */
  updateFormStart(event) { this.updateFormField(event, 'start') }
  /** @param {any} event */
  updateFormEnd(event) { this.updateFormField(event, 'end') }
  /** @param {any} event */
  updateFormAllDay(event) {
    const checked = !!event?.target?.checked
    if (checked === this.$form.allDay) return
    if (checked) {
      const start = String(this.$form.start || '').slice(0, 10)
      const rawEnd = String(this.$form.end || '').slice(0, 10)
      if (!isCalendarDate(start)) return
      const end = isCalendarDate(rawEnd) && rawEnd > start ? rawEnd : addCalendarDays(start, 1)
      this.$form = { ...this.$form, start, end, allDay: true, error: '' }
      this.stabilizeModalFocus(false)
      return
    }
    try {
      this.$form = { ...this.$form, ...timedRangeForAllDayDates(this.$form), error: '' }
    } catch {
      this.$form = { ...this.$form, allDay: false }
    }
    this.stabilizeModalFocus(false)
  }
  /** @param {any} event */
  updateFormLocation(event) { this.updateFormField(event, 'location') }
  /** @param {any} event */
  updateFormDescription(event) { this.updateFormField(event, 'description') }
  /** @param {any} event */
  updateFormRepeat(event) { this.updateFormField(event, 'repeat') }
  /** @param {any} event */
  updateFormCalendar(event) { this.updateFormField(event, 'calendarId') }

  async updateEditScope(event) {
    const next = String(event?.target?.value || event?.value || '')
    if (!['instance', 'series'].includes(next) || !this.$editingSeriesId) return
    this.$editScope = /** @type {'instance'|'series'} */ (next)
    if (next === 'instance' && this.$editingOccurrence) {
      this.applyEventToForm(this.$editingOccurrence)
      this.stabilizeModalFocus(false)
      return
    }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const response = await apiFetch(`/calendar/events/${encodeURIComponent(this.$editingSeriesId)}`)
    if (!response?.ok) {
      this.$form = { ...this.$form, error: 'Could not load the series.' }
      this.stabilizeModalFocus(false)
      return
    }
    const master = await response.json()
    // Keep the selected occurrence's dates in the editor. The server applies
    // their move delta and duration to the master when the whole series saves.
    this.$form = {
      ...this.$form,
      repeat: recurrencePreset(master.recurrence),
      recurrence: master.recurrence || '',
      timeZone: master.timeZone || this.$form.timeZone || 'UTC',
      error: '',
    }
    // Tachyon rerenders after this handler resolves. Re-apply the accessible
    // names DuVay's visual-only floating labels do not retain across that
    // replacement without stealing focus from the scope selector.
    this.stabilizeModalFocus(false)
  }

  applyEventToForm(event) {
    this.$form = {
      ...this.$form,
      title: event.title || '',
      start: event.allDay ? calendarDateValue(event.start) : localDateTimeValue(new Date(event.start)),
      end: event.allDay ? calendarDateValue(event.end || event.start) : localDateTimeValue(new Date(event.end || event.start)),
      allDay: !!event.allDay,
      location: event.location || '',
      description: event.description || '',
      repeat: recurrencePreset(event.recurrence),
      recurrence: event.recurrence || '',
      timeZone: event.timeZone || 'UTC',
      calendarId: event.calendarId || this.$form.calendarId || 'local',
      error: '',
    }
  }

  async saveEvent() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || this.$editingReadOnly) return
    const form = this.$form
    if (!form.title.trim()) { this.$form = { ...form, error: 'Title is required' }; return }
    if (!form.start || !form.end) { this.$form = { ...form, error: 'Start and end are required' }; return }
    let startValue = form.start
    let endValue = form.end
    if (!form.allDay) {
      const startInstant = new Date(form.start)
      const endInstant = new Date(form.end)
      if (!Number.isFinite(startInstant.getTime()) || !Number.isFinite(endInstant.getTime())) {
        this.$form = { ...form, error: 'Start and end must be valid dates' }
        return
      }
      startValue = startInstant.toISOString()
      endValue = endInstant.toISOString()
    }
    const invalidRange = form.allDay
      ? !isCalendarDate(startValue) || !isCalendarDate(endValue) || endValue <= startValue
      : Date.parse(endValue) < Date.parse(startValue)
    if (invalidRange) {
      this.$form = { ...form, error: form.allDay ? 'End must be after start' : 'End must be on or after start' }
      return
    }
    this.$form = { ...form, saving: true, error: '' }
    /** @type {Record<string, any>} */
    const body = {
      title: form.title.trim(),
      start: startValue,
      end: endValue,
      allDay: !!form.allDay,
      location: form.location.trim(),
      description: form.description,
      timeZone: form.timeZone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      idempotencyKey: this.$mutationKey || crypto.randomUUID(),
    }
    if (!this.$editingId && form.calendarId && form.calendarId !== 'local') body.calendarId = form.calendarId
    if (!this.$editingId || this.$editScope === 'series') body.recurrence = recurrenceRule(form.repeat, form.recurrence)
    try {
      const scopeQuery = this.$editingId && this.$editingRecurrenceId
        ? `?scope=${encodeURIComponent(this.$editScope)}&occurrenceStart=${encodeURIComponent(this.$editingRecurrenceId)}`
        : ''
      const url = this.$editingId ? `/calendar/events/${encodeURIComponent(this.$editingId)}${scopeQuery}` : '/calendar/events'
      const method = this.$editingId ? 'PUT' : 'POST'
      const { response: res, queued } = await this.calendarWrite(
        url, method, body, this.$editingId ? 'Update calendar event' : 'Create calendar event',
        this.$editingId ? `calendar:${this.$editingId}` : '',
      )
      if (queued) {
        const wasEditing = !!this.$editingId
        await this.closeModal()
        window._caduceus?.toast(wasEditing ? 'Event update queued.' : 'Event creation queued.')
        return
      }
      if (!res?.ok) { this.$form = { ...this.$form, saving: false, error: 'Save failed' }; return }
      const wasEditing = !!this.$editingId
      const scope = this.$editScope
      await this.closeModal()
      window._caduceus?.toast(wasEditing ? (scope === 'instance' ? 'Occurrence saved.' : 'Event series saved.') : 'Event created.')
      await this.loadEvents()
    } catch {
      this.$form = { ...this.$form, saving: false, error: 'Network error' }
    }
  }

  async deleteEditing() {
    if (!this.$editingId || this.$editingReadOnly) return
    let scope = this.$editScope
    if (this.$editingSeriesId) {
      if (confirm('Delete only this occurrence? The rest of the series will remain.')) scope = 'instance'
      else if (confirm('Delete the entire recurring series? This removes every occurrence.')) scope = 'series'
      else return
    } else if (!confirm('Delete this event?')) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const idempotencyKey = this.$mutationKey || crypto.randomUUID()
    const query = scope === 'instance'
      ? `?scope=instance&occurrenceStart=${encodeURIComponent(this.$editingRecurrenceId)}&idempotencyKey=${encodeURIComponent(idempotencyKey)}`
      : scope === 'series' ? `?scope=series&idempotencyKey=${encodeURIComponent(idempotencyKey)}` : `?idempotencyKey=${encodeURIComponent(idempotencyKey)}`
    const targetId = this.$editingId
    const { response: res, queued } = await this.calendarWrite(
      `/calendar/events/${encodeURIComponent(targetId)}${query}`, 'DELETE', null,
      'Delete calendar event', `calendar:${targetId}`,
    )
    if (queued) {
      await this.closeModal()
      window._caduceus?.toast('Event deletion queued.')
      return
    }
    if (res?.ok) {
      await this.closeModal()
      window._caduceus?.toast(scope === 'instance' ? 'Occurrence deleted. The series remains.' : scope === 'series' ? 'Event series deleted.' : 'Event deleted.')
      await this.loadEvents()
    } else {
      window._caduceus?.toast('Delete failed.')
    }
  }

  isReadOnlyEvent(event) {
    if (event?.readOnly || event?.source === 'team') return true
    if (event?.source === 'invite' && event?.organizer) return true
    if (event?.allDay) return calendarDateValue(event.end || event.start) <= localIsoDate(new Date())
    return Date.parse(event?.end || event?.start || '') < Date.now()
  }

  captureModalReturnFocus(domEvent) {
    const eventTarget = domEvent?.currentTarget instanceof HTMLElement ? domEvent.currentTarget : null
    const active = eventTarget || (document.activeElement instanceof HTMLElement ? document.activeElement : null)
    this.$modalReturnFocus = active
    const create = active?.closest('[data-calendar-create]')
    const event = active?.closest('[data-calendar-event-id]')
    this.$modalReturnSelector = create ? '[data-calendar-create] button' : event?.getAttribute('data-calendar-event-id')
      ? `[data-calendar-event-id="${CSS.escape(event.getAttribute('data-calendar-event-id') || '')}"]`
      : ''
  }

  async renderCurrentState() {
    const render = (/** @type {any} */ (globalThis)).__tc_rerender
    if (typeof render !== 'function') return
    // Tachyon coalesces a render requested while another render is active.
    // Awaiting once and then rendering again guarantees state changed during
    // that active pass is represented by the final DOM.
    await render()
    await render()
  }

  prepareModalFocus(shouldFocus = true) {
    const initialDialog = document.querySelector('.event-modal')
    this.bindModalBackdrop()
    // Focus the dialog synchronously after the awaited render. This gives the
    // declarative key handler an active target before DuVay finishes upgrading
    // its nested controls on the following animation frames.
    if (shouldFocus && initialDialog instanceof HTMLElement) initialDialog.focus()
    requestAnimationFrame(() => requestAnimationFrame(() => {
      const dialog = document.querySelector('.event-modal')
      if (!(dialog instanceof HTMLElement)) return
      this.bindModalBackdrop()
      // DuVay's floating labels are visual siblings rather than native
      // <label for> relationships. Name the actual controls so the calendar
      // dialog remains operable through screen readers and role-based tools.
      for (const field of dialog.querySelectorAll('w-text-field, w-textarea')) {
        const control = field.querySelector('input, textarea')
        const label = field.getAttribute('label')
        if (control && label && !control.getAttribute('aria-label')) control.setAttribute('aria-label', label)
      }
      const first = this.modalFocusableControls(dialog)[0]
      if (shouldFocus && first instanceof HTMLElement) first.focus()
    }))
  }

  bindModalBackdrop() {
    const backdrop = document.querySelector('.event-modal-backdrop')
    if (!(backdrop instanceof HTMLElement) || backdrop.dataset.bound === 'true') return
    backdrop.dataset.bound = 'true'
    backdrop.addEventListener('click', event => {
      if (event.target === backdrop) void this.closeModal()
    })
  }

  scheduleSelectedEventIntoView() {
    if (!this.$keyboardEventId) return
    setTimeout(() => requestAnimationFrame(() => {
      const selector = `[data-calendar-event-id="${CSS.escape(this.$keyboardEventId)}"]`
      const selected = document.querySelector(`.calendar-agenda-grid ${selector}`) ||
        document.querySelector(`.calendar-month-grid ${selector}`) ||
        document.querySelector(`.calendar-agenda-list ${selector}`) ||
        document.querySelector(selector)
      selected?.scrollIntoView?.({ block: 'center', inline: 'nearest' })
    }), 0)
  }

  /**
   * Tachyon performs its declarative post-handler render after a handler
   * resolves. DuVay currently omits native accessible names for floating
   * inputs, so enhance the settled DOM once more without stealing focus.
   */
  stabilizeModalFocus(shouldFocus = true) {
    this.prepareModalFocus(shouldFocus)
    // Do not request another render here: that can continuously detach a
    // control while an assistive technology or browser automation is acting
    // on it. The delayed passes decorate Tachyon's own settled render only.
    setTimeout(() => this.prepareModalFocus(false), 16)
    setTimeout(() => this.prepareModalFocus(false), 80)
  }

  modalFocusableControls(dialog) {
    return [...dialog.querySelectorAll('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])')]
      .filter(node => node instanceof HTMLElement && !node.hidden)
  }
}

/** @param {Date} a @param {Date} b */
function sameDay(a, b) {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
}

function addDays(date, amount) {
  const next = new Date(date)
  next.setDate(next.getDate() + amount)
  return next
}

/** Format a local calendar date without UTC rollover. @param {Date} date */
/**
 * Format a Date as the value expected by `<input type="datetime-local">`
 * (`YYYY-MM-DDTHH:mm`). The input is always interpreted in the browser's
 * local timezone, so we deliberately use local accessors here.
 *
 * @param {Date} d
 */
const toLocalInputValue = localDateTimeValue

function recurrencePreset(rule) {
  if (!rule) return 'none'
  const match = String(rule).match(/^FREQ=(DAILY|WEEKLY|MONTHLY|YEARLY)$/)
  return match ? match[1].toLowerCase() : 'custom'
}

function recurrenceRule(preset, existing) {
  if (preset === 'custom') return existing || ''
  if (preset === 'none') return ''
  return ['daily', 'weekly', 'monthly', 'yearly'].includes(preset) ? `FREQ=${preset.toUpperCase()}` : ''
}

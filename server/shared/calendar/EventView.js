export class EventView {
  // @ts-check

  /**
   * Stable public calendar-event representation shared by collection and item
   * routes. Persistence document ids and owner fields never cross the boundary.
   *
   * @param {Record<string, any>} event
   * @returns {Record<string, any>}
   */
  static toCalendarEventView(event) {
    return {
      id: event.id,
      title: event.title,
      start: event.start,
      end: event.end,
      allDay: !!event.allDay,
      location: event.location || '',
      description: event.description || '',
      attendees: Array.isArray(event.attendees) ? event.attendees : [],
      organizer: event.organizer || '',
      source: event.source || 'manual',
      sourceEmailId: event.sourceEmailId || '',
      icsUid: event.icsUid || '',
      recurrence: event.recurrence || '',
      recurrenceDates: Array.isArray(event.recurrenceDates) ? event.recurrenceDates : [],
      exceptionDates: Array.isArray(event.exceptionDates) ? event.exceptionDates : [],
      timeZone: event.timeZone || 'UTC',
      seriesId: event.seriesId || '',
      recurrenceId: event.recurrenceId || '',
      isRecurring: !!event.isRecurring || !!event.recurrence || !!event.seriesId,
      isException: typeof event.isException === 'boolean' ? event.isException : !!event.seriesId,
      readOnly: !!event.readOnly,
      reminderMinutes: typeof event.reminderMinutes === 'number' ? event.reminderMinutes : null,
      status: event.status || 'confirmed',
      accountId: event.accountId || '',
      accountLabel: event.accountLabel || '',
      accountColor: event.accountColor || '',
      calendarId: event.calendarId || '',
      calendarName: event.calendarName || '',
      calendarColor: event.calendarColor || '',
      providerResourceId: event.providerResourceId || '',
      createdAt: event.createdAt,
      updatedAt: event.updatedAt,
    }
  }
}

// @ts-check
import { createHash, randomUUID } from 'node:crypto'
import { ExternalAccounts } from '../../../shared/accounts/ExternalAccounts.js'
import { Ics } from '../../../shared/calendar/Ics.js'

const MAX_RESOURCES = 10_000
const MAX_EVENTS = 20_000
const MAX_ICALENDAR_BYTES = 2 << 20

/** Parse authenticated CalDAV resources into the normal calendar event model. */
export class ProviderCalendarService {
  /** @param {{ calendarResources(token:string): Promise<any>, mutateCalendar(token:string, id:string, input:Record<string, unknown>):Promise<any>, calendarMutation(token:string, id:string, mutationId:string):Promise<any> }} [boundary] */
  constructor(boundary = ExternalAccounts) {
    this.boundary = boundary
  }

  /** @param {string} token */
  async list(token) {
    const { events } = await this.#load(token)
    return events
  }

  /** @param {string} token */
  async calendars(token) {
    const { calendars } = await this.#load(token)
    return calendars
  }

  /** @param {string} token @param {string} eventId */
  async find(token, eventId) {
    const loaded = await this.#load(token)
    const event = loaded.events.find(candidate => candidate.id === eventId) || null
    if (!event) return null
    const resource = loaded.resources.get(event.providerResourceId) || null
    return resource ? { event, resource } : null
  }

  /** @param {string} token @param {string} calendarId @param {Record<string, any>} event @param {string} idempotencyKey */
  async create(token, calendarId, event, idempotencyKey) {
    const { calendars } = await this.#load(token)
    const calendar = calendars.find(candidate => candidate.calendarId === calendarId && candidate.writable !== false)
    if (!calendar) throw new Error('provider calendar not found')
    const uid = `${randomUUID()}@caduceus.local`
    const mutation = await this.boundary.mutateCalendar(token, calendar.accountId, {
      idempotencyKey, kind: 'create', calendarId, resourceId: '', expectedEtag: '', rawICalendar: Ics.buildCalendarResource(event, uid),
    })
    return this.#awaitMutation(token, calendar.accountId, mutation)
  }

  /** @param {string} token @param {string} eventId @param {Record<string, any>} event @param {string} recurrenceId @param {string} idempotencyKey */
  async update(token, eventId, event, recurrenceId, idempotencyKey) {
    const found = await this.find(token, eventId)
    if (!found) throw new Error('provider calendar event not found')
    /** @type {Record<string, any>} */
    const desired = { ...found.event, ...event, recurrenceId: recurrenceId || '', status: event.status || 'confirmed' }
    if (recurrenceId) {
      desired.recurrence = ''
      desired.recurrenceDates = []
      desired.exceptionDates = []
    }
    const rawICalendar = Ics.updateEventResource(found.resource.rawICalendar, { uid: found.event.icsUid, recurrenceId, event: desired })
    const mutation = await this.boundary.mutateCalendar(token, found.event.accountId, {
      idempotencyKey, kind: 'update', calendarId: '', resourceId: found.resource.id, expectedEtag: found.resource.etag, rawICalendar,
    })
    return this.#awaitMutation(token, found.event.accountId, mutation)
  }

  /** @param {string} token @param {string} eventId @param {string} recurrenceId @param {string} idempotencyKey */
  async delete(token, eventId, recurrenceId, idempotencyKey) {
    const found = await this.find(token, eventId)
    if (!found) throw new Error('provider calendar event not found')
    const rawICalendar = Ics.deleteEventResource(found.resource.rawICalendar, { uid: found.event.icsUid, recurrenceId })
    const mutation = await this.boundary.mutateCalendar(token, found.event.accountId, {
      idempotencyKey,
      kind: rawICalendar == null ? 'delete' : 'update',
      calendarId: '',
      resourceId: found.resource.id,
      expectedEtag: found.resource.etag,
      rawICalendar: rawICalendar || '',
    })
    return this.#awaitMutation(token, found.event.accountId, mutation)
  }

  /** @param {string} token */
  async #load(token) {
    const response = await this.boundary.calendarResources(token)
    const resources = Array.isArray(response?.resources) ? response.resources : []
    const calendars = Array.isArray(response?.calendars) ? response.calendars.filter(candidate => candidate?.calendarId && candidate?.accountId) : []
    if (resources.length > MAX_RESOURCES) throw new Error('provider calendar resource limit exceeded')
    /** @type {Array<Record<string, any>>} */
    const events = []
    /** @type {Map<string, Record<string, any>>} */
    const resourcesById = new Map()
    for (const resource of resources) {
      const raw = typeof resource?.rawICalendar === 'string' ? resource.rawICalendar : ''
      if (!raw || Buffer.byteLength(raw) > MAX_ICALENDAR_BYTES) continue
      const calendar = Ics.parseIcs(raw)
      if (!calendar) continue
      resourcesById.set(resource.id, resource)
      for (const event of calendar.events) {
        if (events.length >= MAX_EVENTS) throw new Error('provider calendar event limit exceeded')
        const masterId = providerEventId(resource.id, event.uid)
        const recurrenceId = event.recurrenceId || ''
        events.push({
          id: recurrenceId ? `${masterId}:exception:${shortHash(recurrenceId)}` : masterId,
          userEmail: resource.mailboxAddress,
          title: event.summary,
          start: event.start,
          end: event.end,
          allDay: !!event.allDay,
          location: event.location || '',
          description: event.description || '',
          attendees: Array.isArray(event.attendees) ? event.attendees : [],
          organizer: event.organizer || resource.mailboxAddress,
          source: 'provider',
          icsUid: event.uid,
          recurrence: event.rrule || '',
          recurrenceDates: Array.isArray(event.recurrenceDates) ? event.recurrenceDates : [],
          exceptionDates: Array.isArray(event.exceptionDates) ? event.exceptionDates : [],
          timeZone: event.timeZone || 'UTC',
          seriesId: recurrenceId ? masterId : '',
          recurrenceId,
          status: event.status || 'confirmed',
          readOnly: false,
          accountId: resource.accountId,
          accountLabel: resource.accountLabel || resource.mailboxAddress,
          accountColor: resource.accountColor || '',
          calendarId: resource.calendarId,
          calendarName: resource.calendarName || 'Calendar',
          calendarColor: resource.calendarColor || resource.accountColor || '',
          providerResourceId: resource.id,
          createdAt: resource.updatedAt,
          updatedAt: resource.updatedAt,
        })
      }
    }
    return { calendars, events, resources: resourcesById }
  }

  /** @param {string} token @param {string} accountId @param {Record<string, any>} mutation */
  async #awaitMutation(token, accountId, mutation) {
    if (!mutation?.id) return mutation
    let current = mutation
    for (let attempt = 0; attempt < 40 && ['queued', 'applying'].includes(current?.state); attempt++) {
      await new Promise(resolve => setTimeout(resolve, 50 + Math.min(attempt * 25, 200)))
      current = await this.boundary.calendarMutation(token, accountId, mutation.id)
    }
    if (current?.state === 'conflict') {
      const error = new Error('the provider calendar changed; refresh and retry')
      // @ts-ignore stable transport discriminator consumed by accountFailure.
      error.code = 'ECONFLICT'
      throw error
    }
    if (current?.state !== 'applied') throw new Error('provider calendar mutation did not complete')
    return current
  }
}

/** Keep imported invitation copies from duplicating an authoritative provider event. */
export function mergeCalendarEvents(localEvents, providerEvents) {
  const providerUIDs = new Set(providerEvents.map(event => event.icsUid).filter(Boolean))
  return [
    ...localEvents.filter(event => event.source !== 'invite' || !providerUIDs.has(event.icsUid)),
    ...providerEvents,
  ]
}

function providerEventId(resourceId, uid) {
  return `provider-${shortHash(`${resourceId}\x00${uid}`)}`
}

function shortHash(value) {
  return createHash('sha256').update(String(value)).digest('base64url').slice(0, 24)
}

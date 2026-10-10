// @ts-check
import { createHash, randomUUID } from 'node:crypto'
import { ExternalAccounts } from '../accounts/ExternalAccounts.js'
import { Ics } from './Ics.js'

export class ProviderCalendarRules {
  static MAX_RESOURCES = 10_000

  static MAX_EVENTS = 20_000

  static MAX_ICALENDAR_BYTES = 2 << 20

  /** Keep imported invitation copies from duplicating an authoritative provider event. */
  static mergeCalendarEvents(localEvents, providerEvents) {
    const providerUIDs = new Set(providerEvents.map(event => event.icsUid).filter(Boolean))
    return [
      ...localEvents.filter(event => event.source !== 'invite' || !providerUIDs.has(event.icsUid)),
      ...providerEvents,
    ]
  }

  static providerEventId(resourceId, uid) {
    return `provider-${ProviderCalendarRules.shortHash(`${resourceId}\x00${uid}`)}`
  }

  static shortHash(value) {
    return createHash('sha256').update(String(value)).digest('base64url').slice(0, 24)
  }
}

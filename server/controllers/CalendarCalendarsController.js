import { Guards } from '../../../middleware/guards.js'
import { getApp } from '../../../app.js'
import { accountFailure, bearerToken } from '../../../platform/accounts/bridge.js'

@RequestRoute("/calendar/calendars")
@Public
export class CalendarCalendarsController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { context }) {
    const app = await getApp()
    if (!app.providerCalendars) return { calendars: [] }
    try {
      return { calendars: await app.providerCalendars.calendars(bearerToken(context)) }
    } catch (error) {
      return accountFailure(error)
    }
  }
}

import { RouteRequest } from '../middleware/RouteRequest.js'
import { getApp } from '../app.js'
import { accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'

@RequestRoute("/calendar/calendars")
@SignedIn
export class CalendarCalendarsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context }) => {
      const app = await getApp()
      if (!app.providerCalendars) return { calendars: [] }
      try {
        return { calendars: await app.providerCalendars.calendars(bearerToken(context)) }
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}

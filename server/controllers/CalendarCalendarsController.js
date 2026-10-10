import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/calendar/calendars")
@SignedIn
export class CalendarCalendarsController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context }) => {
      const app = await getApp()
      if (!app.providerCalendars) return { calendars: [] }
      try {
        return { calendars: await app.providerCalendars.calendars(ExternalAccounts.bearerToken(context)) }
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
    })
  }
}

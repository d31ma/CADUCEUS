import { RouteRequest } from '../shared/http/RouteRequest.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/calendar/calendars")
@SignedIn
export class CalendarCalendarsController {
  /** @type {ProviderCalendarService} */
  @Needs
  providerCalendarService

  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context }) => {
      const app = this
      if (!app.providerCalendarService.available()) return { calendars: [] }
      try {
        return { calendars: await app.providerCalendarService.calendars(ExternalAccounts.bearerToken(context)) }
      } catch (error) {
        return ExternalAccounts.accountFailure(error)
      }
    })
  }
}

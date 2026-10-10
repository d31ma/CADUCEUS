// @ts-check
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'

/** Test-only recipient signal injection for real-browser coverage. */
@RequestRoute("/test/seed/tracking")
@SignedIn
export class TestSeedTrackingController {
  static async recordClick(app, email, recipient, requestedIndex) {
    const linkIndex = Number.isInteger(Number(requestedIndex)) ? Number(requestedIndex) : 0
    const link = Array.isArray(email.trackedLinks)
      ? email.trackedLinks.find(item => item?.index === linkIndex)
      : null
    if (!link?.url) return false
    return await app.tracking.recordClick({
      trackingId: email.trackingId || '',
      recipient,
      url: link.url,
      linkIndex,
    })
  }

  /** @param {any} request */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { testOnly: true, guard: {} }, async ({ body = {}, auth }) => {
      const messageId = typeof body.messageId === 'string' ? body.messageId.trim() : ''
      const type = body.type === 'click' ? 'click' : body.type === 'open' ? 'open' : ''
      if (!messageId || !type) return Respond.r400('messageId and type=open|click required')

      const app = await getApp()
      const [, email] = await app.emails.findById(messageId)
      if (!email || email.folder !== 'sent' || email.ownerEmail?.toLowerCase() !== auth.email.toLowerCase()) {
        return Respond.r404('Sent email not found')
      }
      const recipient = typeof body.recipient === 'string'
        ? body.recipient
        : email.to?.[0] || String(email.recipient || '').split(',')[0]
      const recorded = type === 'open'
        ? await app.tracking.recordOpen({ trackingId: email.trackingId || '', recipient })
        : await TestSeedTrackingController.recordClick(app, email, recipient, body.linkIndex)
      if (!recorded) return Respond.r400(`Message does not have ${type} tracking enabled`)
      return { recorded: true, type, messageId }
    })
  }
}

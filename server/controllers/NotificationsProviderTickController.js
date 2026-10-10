import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'
import { ExternalAccounts, accountFailure } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/notifications/provider/tick")
@Public
export class NotificationsProviderTickController {
  /** Process one leased provider new-mail notification batch. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { signature: 'INBOUND_WEBHOOK_SECRET' }, async () => {
      try {
        const claim = await ExternalAccounts.claimNotifications(`tachyon-${process.pid}`, 50)
        const notifications = Array.isArray(claim?.notifications) ? claim.notifications : []
        if (!claim?.token || notifications.length === 0) return { claimed: 0, delivered: 0, retried: 0 }
        const app = await getApp()
        const delivered = []
        const retried = []
        for (const notification of notifications) {
          const ok = await app.push.notifyProviderNewEmail(notification)
          ;(ok ? delivered : retried).push(notification.id)
        }
        if (delivered.length) await ExternalAccounts.completeNotifications(claim.token, delivered, true)
        if (retried.length) await ExternalAccounts.completeNotifications(claim.token, retried, false, 'push_delivery_failed')
        return { claimed: notifications.length, delivered: delivered.length, retried: retried.length }
      } catch (error) {
        return accountFailure(error)
      }
    })
  }
}

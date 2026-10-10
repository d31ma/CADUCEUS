import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { Actions } from '../shared/auth/Auth.js'
import { getApp } from '../app.js'
import { ThreadShareInputError } from '../features/mailbox/services/ThreadSharingService.js'

@RequestRoute("/thread-shares")
@SignedIn
export class ThreadSharesController {
  /** List exact-owner links, optionally narrowed to one authorized thread. */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_READ } }, async ({ query, auth }) => {
      const emailId = query?.emailId == null ? '' : String(query.emailId).trim()
      if (emailId.length > 500) return Respond.r400('emailId is too long')
      const app = await getApp()
      const result = await app.threadSharing.list(auth.email, emailId, auth.domains)
      if (result == null) return Respond.r404('Conversation not found')
      return result
    })
  }

  /** Mint one expiring link and revoke any prior active link for the thread. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_READ } }, async ({ body, auth }) => {
      try {
        const app = await getApp()
        const result = await app.threadSharing.create({
          ownerEmail: auth.email,
          domains: auth.domains,
          emailId: body?.emailId,
          expiresInDays: body?.expiresInDays,
        })
        return result ?? Respond.r404('Conversation not found')
      } catch (error) {
        if (error instanceof ThreadShareInputError) return Respond.r400(error.message)
        throw error
      }
    })
  }
}

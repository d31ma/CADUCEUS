import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/reminders")
@SignedIn
export class RemindersController {
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
      const app = await getApp()
      const reminders = await app.reminders.listForOwner(auth.email)
      return reminders.map(({ docId, ...item }) => ({
        ...item,
        remindAt: new Date(item.remindAt).toISOString(),
        createdAt: new Date(item.createdAt).toISOString(),
        ...(item.dueAt ? { dueAt: new Date(item.dueAt).toISOString() } : {}),
      }))
    })
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      if (!body?.emailId || typeof body.emailId !== 'string') return Respond.r400('emailId is required')
      if (!body?.remindAt || typeof body.remindAt !== 'string') return Respond.r400('remindAt is required')
      const remindAt = Date.parse(body.remindAt)
      if (isNaN(remindAt)) return Respond.r400('remindAt must be a valid ISO datetime')
      if (remindAt <= Date.now()) return Respond.r400('remindAt must be in the future')

      const app = await getApp()
      const [, email] = await app.emails.findById(body.emailId)
      if (!email || email.sender.toLowerCase() !== auth.email || !auth.domains.includes(email.domain)) {
        return Respond.r404('Sent email not found')
      }
      const reminder = await app.reminders.schedule({
        ownerEmail: auth.email,
        emailId: email.id,
        recipient: email.recipient,
        subject: email.subject,
        threadId: email.threadId,
        remindAt,
      })
      return { id: reminder.id, scheduled: true }
    })
  }

  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      if (!body?.id || typeof body.id !== 'string') return Respond.r400('id is required')
      const app = await getApp()
      if (!await app.reminders.dismiss(body.id, auth.email)) return Respond.r404('Reminder not found')
      return { dismissed: true }
    })
  }
}

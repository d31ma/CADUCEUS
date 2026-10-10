import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { SnoozeError } from '../../../features/mailbox/services/snooze.js'

export class Handler {
  /** @param {{paths?:{id?:string},auth:any}} params */
  @Guards.auth()
  static async DELETE({ paths, auth }) {
    if (!paths?.id) return Respond.r404('Snooze not found')
    const app = await getApp()
    try {
      return await app.snoozeService.undo(paths.id, auth.email)
    } catch (error) {
      if (error instanceof SnoozeError && error.status === 404) return Respond.r404(error.message)
      throw error
    }
  }
}

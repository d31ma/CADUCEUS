// @ts-check
import { Guards } from '../../../../../middleware/guards.js'
import { Respond } from '../../../../../platform/http/respond.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../../../platform/accounts/bridge.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { context, paths }) {
    const id = normalizedID(paths?.id)
    if (!id) return Respond.r404('External account not found')
    try {
      return await ExternalAccounts.get(bearerToken(context), id)
    } catch (error) {
      return accountFailure(error)
    }
  }

  @Guards.auth()
  static async PATCH(/** @type {any} */ { context, paths, body }) {
    const id = normalizedID(paths?.id)
    const revision = normalizedRevision(body?.revision)
    const action = body?.action
    if (!id) return Respond.r404('External account not found')
    if (!revision || !['pause', 'resume', 'update'].includes(action)) {
      return Respond.r400('action and positive revision are required')
    }
    try {
      if (action === 'update') {
        const update = {}
        if ('label' in (body || {})) update.label = body.label
        if ('color' in (body || {})) update.color = body.color
        if ('order' in (body || {})) update.order = body.order
        return await ExternalAccounts.updatePresentation(bearerToken(context), id, revision, update)
      }
      return action === 'pause'
        ? await ExternalAccounts.pause(bearerToken(context), id, revision)
        : await ExternalAccounts.resume(bearerToken(context), id, revision)
    } catch (error) {
      return accountFailure(error)
    }
  }

  @Guards.auth()
  static async DELETE(/** @type {any} */ { context, paths, body }) {
    const id = normalizedID(paths?.id)
    const revision = normalizedRevision(body?.revision)
    if (!id) return Respond.r404('External account not found')
    if (!revision) return Respond.r400('positive revision is required')
    try {
      return await ExternalAccounts.remove(bearerToken(context), id, revision)
    } catch (error) {
      return accountFailure(error)
    }
  }
}

function normalizedID(value) {
  const id = typeof value === 'string' ? value.trim() : ''
  return /^ext_[A-Za-z0-9_-]{20,80}$/.test(id) ? id : ''
}

function normalizedRevision(value) {
  return Number.isSafeInteger(value) && value > 0 ? value : 0
}

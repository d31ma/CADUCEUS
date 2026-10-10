import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { randomUUID } from 'node:crypto'
import { AuthBridgeError } from '../../../platform/auth/bridge.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../../platform/accounts/bridge.js'

export class Handler {
  @Guards.auth()
  static async GET(/** @type {any} */ { paths, auth, context }) {
    const app = await getApp()
    const id = String(paths?.id || '')
    const local = await app.contacts.get(id, auth.email)
    if (local) return local
    const remote = await remoteContact(bearerToken(context), id)
    return remote ?? Respond.r404('Contact not found')
  }

  @Guards.auth()
  static async PUT(/** @type {any} */ { paths, body, auth, context }) {
    try {
      const app = await getApp()
      const id = String(paths?.id || '')
      const local = await app.contacts.update(id, auth.email, body || {})
      if (local) return local
      const token = bearerToken(context)
      const remote = await remoteContact(token, id)
      if (!remote) return Respond.r404('Contact not found')
      const { idempotencyKey, ...contact } = body || {}
      const mutation = await ExternalAccounts.mutateContact(token, remote.accountId, {
        idempotencyKey: validIdempotency(idempotencyKey) || `contact-update-${randomUUID()}`,
        kind: 'update', contactId: remote.id, expectedEtag: remote.etag, contact,
      })
      return { ...remote, ...contact, source: 'carddav', syncState: mutation.state, mutationId: mutation.id }
    } catch (error) {
      if (error instanceof AuthBridgeError) return accountFailure(error)
      return Respond.r400(error instanceof Error ? error.message : 'Invalid contact')
    }
  }

  @Guards.auth()
  static async DELETE(/** @type {any} */ { paths, auth, context, query }) {
    const app = await getApp()
    const id = String(paths?.id || '')
    if (await app.contacts.delete(id, auth.email)) return { deleted: true }
    try {
      const token = bearerToken(context)
      const remote = await remoteContact(token, id)
      if (!remote) return Respond.r404('Contact not found')
      const mutation = await ExternalAccounts.mutateContact(token, remote.accountId, {
        idempotencyKey: validIdempotency(query?.idempotencyKey) || `contact-delete-${randomUUID()}`,
        kind: 'delete', contactId: remote.id, expectedEtag: remote.etag, contact: {},
      })
      return { deleted: false, pending: true, mutationId: mutation.id, accountId: remote.accountId }
    } catch (error) {
      if (error instanceof AuthBridgeError) return accountFailure(error)
      return Respond.r400(error instanceof Error ? error.message : 'Contact could not be deleted')
    }
  }
}

async function remoteContact(token, id) {
  const contacts = (await ExternalAccounts.contacts(token))?.contacts
  return Array.isArray(contacts) ? contacts.find(contact => contact?.id === id) || null : null
}

function validIdempotency(value) {
  const text = typeof value === 'string' ? value.trim() : ''
  return text && text.length <= 128 && !/[\r\n\0]/.test(text) ? text : ''
}

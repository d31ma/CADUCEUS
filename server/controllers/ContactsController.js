import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { randomUUID } from 'node:crypto'
import { AuthBridgeError } from '../../platform/auth/bridge.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../../platform/accounts/bridge.js'

@RequestRoute("/contacts")
@Public
export class ContactsController {
  /** List and search the caller's personal and found-in-mail contacts. */
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { query, context, auth }) {
    const params = query || context?.query || {}
    const source = params.source ? String(params.source) : ''
    if (source && !['manual', 'mail', 'vcard', 'carddav'].includes(source)) return Respond.r400('source is invalid')
    const app = await getApp()
    let providerContacts = []
    try { providerContacts = (await ExternalAccounts.contacts(bearerToken(context)))?.contacts || [] } catch { /* hosted contacts remain available while provider sync recovers */ }
    return await app.contacts.list(auth.email, {
      q: String(params.q || '').slice(0, 200),
      source,
      groupId: String(params.groupId || ''),
    }, providerContacts)
  }

  /** Create one mailbox-owned contact. */
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, auth, context }) {
    try {
	  const { source = '', accountId = '', addressBookId = '', idempotencyKey = '', ...contact } = body || {}
	  if (source === 'carddav') {
		if (!validIdentifier(accountId, 'ext_') || !validIdentifier(addressBookId, 'book_')) return Respond.r400('A valid CardDAV address book is required')
		const mutation = await ExternalAccounts.mutateContact(bearerToken(context), accountId, {
		  idempotencyKey: validIdempotency(idempotencyKey) || `contact-create-${randomUUID()}`,
		  kind: 'create', addressBookId, contact,
		})
		const desired = mutation?.desired || {}
		return {
		  ...desired, id: mutation.contactId, source: 'carddav', accountId, addressBookId,
		  etag: '', groupIds: [], createdAt: mutation.createdAt, updatedAt: mutation.updatedAt,
		  readOnly: false, syncState: mutation.state, mutationId: mutation.id,
		}
	  }
	  if (source) return Respond.r400('source is invalid')
      const app = await getApp()
	  return await app.contacts.create(auth.email, contact)
    } catch (error) {
	  if (error instanceof AuthBridgeError) return accountFailure(error)
      return Respond.r400(error instanceof Error ? error.message : 'Invalid contact')
    }
  }
}

function validIdentifier(value, prefix) {
  const text = typeof value === 'string' ? value.trim() : ''
  return text.startsWith(prefix) && text.length <= 256 && !/[\r\n\0]/.test(text)
}

function validIdempotency(value) {
  const text = typeof value === 'string' ? value.trim() : ''
  return text && text.length <= 128 && !/[\r\n\0]/.test(text) ? text : ''
}

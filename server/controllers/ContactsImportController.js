import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

@RequestRoute("/contacts/import")
@Public
export class ContactsImportController {
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, auth }) {
    if (typeof body?.content !== 'string' || !body.content.trim()) return Respond.r400('vCard content is required')
    try { return await (await getApp()).contacts.importVCard(auth.email, body.content, body.groupIds || []) }
    catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid vCard') }
  }
}

import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'

@RequestRoute("/contacts/groups")
@Public
export class ContactsGroupsController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { auth }) {
    return await (await getApp()).contacts.listGroups(auth.email)
  }

  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, auth }) {
    try { return await (await getApp()).contacts.createGroup(auth.email, body?.name) }
    catch (error) { return Respond.r400(error instanceof Error ? error.message : 'Invalid group') }
  }
}

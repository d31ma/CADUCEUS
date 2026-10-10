import { Buffer } from 'node:buffer'
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/contacts/export")
@SignedIn
export class ContactsExportController {
  /** @type {ContactsService} */
  @Needs
  contactsService

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      if (body?.ids !== undefined && !Array.isArray(body.ids)) return Respond.r400('ids must be an array')
      const result = await this.contactsService.exportVCard(auth.email, body?.ids || [])
      return {
        filename: result.count === 1 ? 'contact.vcf' : 'contacts.vcf',
        contentType: 'text/vcard; charset=utf-8',
        contentBase64: Buffer.from(result.content, 'utf8').toString('base64'),
        count: result.count,
      }
    })
  }
}

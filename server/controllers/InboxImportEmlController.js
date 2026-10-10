import { Buffer } from 'node:buffer'
import { Guards } from '../../../../middleware/guards.js'
import { Actions } from '../../../../platform/auth/bridge.js'
import { Respond } from '../../../../platform/http/respond.js'
import { Security } from '../../../../platform/security/security.js'
import { getApp } from '../../../../app.js'

const MAX_EML_BYTES = 50 * 1024 * 1024
const BASE64 = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/

/** Authenticated EML import into one explicitly authorized mailbox. */
@RequestRoute("/inbox/import/eml")
@Public
export class InboxImportEmlController {
  @Guards.auth({ action: Actions.MAILBOX_WRITE, mailbox: request => request.body?.mailbox })
  @PostRoute
  async post(/** @type {any} */ { body, auth }) {
    const mailbox = Security.normalizeEmailAddress(body?.mailbox)
    if (!mailbox) return Respond.r400('mailbox is required')
    const domain = mailbox.split('@')[1]
    if (domain !== auth.domain) return Respond.r403('Access denied')

    const folder = InboxImportEmlController._folder(body?.folder)
    if (folder instanceof Error) return Respond.r400(folder.message)
    const source = InboxImportEmlController._source(body?.contentBase64)
    if (source instanceof Error) return Respond.r400(source.message)

    try {
      const app = await getApp()
      const mappedFolder = await app.categoryMappings.pathFor(auth.email, domain, folder) || folder
      const imported = await app.emlImport.importOne({ source, domain, mailbox, folder: mappedFolder })
      return { imported: true, ...imported }
    } catch {
      return Respond.r503('The EML file could not be imported')
    }
  }

  /** @param {unknown} value */
  static _folder(value) {
    if (typeof value !== 'string' || !value.trim()) return new Error('folder is required')
    const folder = value.trim().toLowerCase()
    if (folder === 'drafts') return new Error('EML files cannot be imported as drafts')
    if (!/^[a-z0-9_-]{1,60}$/.test(folder)) return new Error('folder is invalid')
    return folder
  }

  /** @param {unknown} value */
  static _source(value) {
    if (typeof value !== 'string' || !value || !BASE64.test(value)) {
      return new Error('contentBase64 must contain one EML file')
    }
    if (value.length > Math.ceil(MAX_EML_BYTES / 3) * 4) return new Error('EML file exceeds 50 MiB')
    const source = Buffer.from(value, 'base64')
    if (source.byteLength === 0 || source.byteLength > MAX_EML_BYTES) return new Error('EML file exceeds 50 MiB')
    if (!InboxImportEmlController._looksLikeEml(source)) return new Error('File is not a recognizable RFC 822 message')
    return source
  }

  /** @param {Uint8Array} source */
  static _looksLikeEml(source) {
    const header = Buffer.from(source).subarray(0, Math.min(source.byteLength, 256 * 1024)).toString('latin1')
    const separator = header.search(/\r?\n\r?\n/)
    if (separator <= 0) return false
    const block = header.slice(0, separator).replace(/\r?\n[ \t]+/g, ' ')
    return /^(?:from|to|cc|date|subject|message-id|mime-version|content-type):/im.test(block)
  }
}

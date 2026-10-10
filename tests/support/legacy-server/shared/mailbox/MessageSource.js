import MailComposer from '../../deps/npm/nodemailer-mail-composer.mjs'
import { Buffer } from 'node:buffer'

export class MessageSource {
  /**
   * Build the standards-compliant RFC 822 source retained beside a local Sent
   * copy. Nodemailer's same MIME composer is used by SMTP delivery, with all
   * filesystem and URL access disabled so request data cannot dereference a
   * server-side path or network resource.
   *
   * @param {string} from
   * @param {import('../../types').SendRequest & { attachments?: import('../../types').SendAttachment[] }} message
   * @param {string} messageId
   * @param {Date} [date]
   * @returns {Promise<Buffer>}
   */
  static buildOutboundMessageSource(from, message, messageId, date = new Date()) {
    const attachments = (message.attachments || []).map(attachment => ({
      filename: attachment.filename,
      contentType: attachment.contentType,
      content: Buffer.from(attachment.contentBase64, 'base64'),
      contentDisposition: attachment.disposition || 'attachment',
      cid: attachment.contentId,
    }))
    const composer = new MailComposer({
      from,
      to: message.to,
      cc: message.cc,
      bcc: message.bcc,
      replyTo: message.replyTo,
      subject: message.subject,
      text: message.text,
      html: message.html,
      attachments,
      messageId: MessageSource.#normalizeMessageId(message.messageId || messageId, from),
      inReplyTo: message.inReplyTo,
      references: message.references,
      date,
      disableFileAccess: true,
      disableUrlAccess: true,
    })
    return new Promise((resolve, reject) => {
      composer.compile().build((error, source) => error ? reject(error) : resolve(source))
    })
  }

  /** @param {string} value @param {string} from */
  static #normalizeMessageId(value, from) {
    const clean = String(value || '').replace(/[<>\r\n]/g, '').trim()
    if (clean.includes('@')) return `<${clean}>`
    const domain = String(from).split('@')[1] || 'localhost'
    return `<${clean || `caduceus-${Date.now()}`}@${domain}>`
  }
}

import { readFileSync } from 'node:fs'
import { TrackingTokens } from './TrackingTokens.js'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

export class ConsoleSmtpAdapter {
  /** @param {SendRequest} msg */
  async send(msg) {
    console.log(`[smtp] to=${msg.to.join(', ')} subject="${msg.subject}" attachments=${msg.attachments?.length ?? 0}`)
    return { messageId: msg.messageId || `console-${Date.now()}` }
  }

  /** @param {string} from @param {SendRequest} msg */
  async sendEmail(from, msg) {
    console.log(`[smtp] from=${from} to=${msg.to.join(', ')} subject="${msg.subject}" attachments=${msg.attachments?.length ?? 0}`)
    return { messageId: msg.messageId || `console-${Date.now()}` }
  }
}

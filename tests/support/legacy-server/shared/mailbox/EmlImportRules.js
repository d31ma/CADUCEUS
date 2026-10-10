import { randomBytes } from 'node:crypto'
import { PhishingDetection } from '../delivery/PhishingDetection.js'
import { ListUnsubscribe } from '../delivery/ListUnsubscribe.js'
import { Threading } from './Threading.js'

export class EmlImportRules {
  static MAX_SUBJECT_LENGTH = 500

  /** @param {unknown} value */
  static safeSubject(value) {
    return String(value || '(no subject)')
      .replace(/[\u0000-\u001f\u007f]+/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
      .slice(0, EmlImportRules.MAX_SUBJECT_LENGTH) || '(no subject)'
  }

  /** @param {unknown} value */
  static validDate(value) {
    const time = Date.parse(String(value || ''))
    return Number.isFinite(time) ? new Date(time).toISOString() : ''
  }
}

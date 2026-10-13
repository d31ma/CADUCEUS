import { Fylo } from '../persistence/Fylo.js'

export class ThreadSharesRecords {
  /** @param {ThreadShareRecord} share */
  static serialize(share) {
    const { emailIds, ...stored } = share
    return { ...stored, emailIdsJson: JSON.stringify(emailIds) }
  }

  /** @param {any} value @returns {ThreadShareRecord} */
  static deserialize(value) {
    let emailIds = []
    try {
      const parsed = JSON.parse(value.emailIdsJson || '[]')
      if (Array.isArray(parsed)) emailIds = parsed.filter(item => typeof item === 'string')
    } catch {}
    const result = { ...value, emailIds }
    delete result.emailIdsJson
    return result
  }
}

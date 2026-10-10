export class FollowUpReminderRules {
  /** @param {string} subject */
  static threadKey(subject) {
    return String(subject || '')
      .replace(/^(?:(?:re|fw|fwd):\s*)+/i, '')
      .trim()
      .toLowerCase()
  }

  /** @param {string} value */
  static recipientSet(value) {
    return new Set(String(value || '').split(',').map(item => item.trim().toLowerCase()).filter(Boolean))
  }
}

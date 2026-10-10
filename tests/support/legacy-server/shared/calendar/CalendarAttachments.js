export class CalendarAttachments {
  // @ts-check

  /**
   * Return the attachment MailSpring's event package would use as the canonical
   * calendar payload. Providers disagree about MIME type, so filename is the
   * strongest signal: .ics, then text/calendar, then legacy .vcs. The final
   * application/ics fallback keeps older Exchange messages usable.
   *
   * @template {{ filename?: string, contentType?: string }} T
   * @param {T[]} attachments
   * @returns {T | null}
   */
  static bestCalendarAttachment(attachments) {
    /** @type {T | null} */
    let best = null
    let bestRank = Infinity
    for (const attachment of attachments || []) {
      const rank = CalendarAttachments.#calendarAttachmentRank(attachment)
      if (rank < bestRank) {
        best = attachment
        bestRank = rank
      }
    }
    return best
  }

  /**
   * Hide the selected calendar file from the ordinary attachment rail because
   * the invite card owns its download action. Providers commonly attach the
   * same bytes twice under text/calendar and application/ics; suppress that
   * duplicate only when its byte size matches the selected file.
   *
   * @template {{ id?: string, filename?: string, contentType?: string, size?: number }} T
   * @param {T[]} attachments
   * @param {T | null} selected
   * @returns {T[]}
   */
  static withoutCalendarCardAttachments(attachments, selected) {
    if (!selected) return attachments
    return attachments.filter(attachment => {
      if (attachment === selected || (attachment.id && attachment.id === selected.id)) return false
      const contentType = CalendarAttachments.#normalizedContentType(attachment.contentType)
      const duplicateType = contentType === 'text/calendar' || contentType === 'application/ics'
      return !(duplicateType && Number.isFinite(attachment.size) && attachment.size === selected.size)
    })
  }

  /** @param {{ filename?: string, contentType?: string }} attachment */
  static isCalendarAttachment(attachment) {
    return CalendarAttachments.#calendarAttachmentRank(attachment) < Infinity
  }

  /** @param {{ filename?: string, contentType?: string }} attachment */
  static #calendarAttachmentRank(attachment) {
    const filename = String(attachment?.filename || '').toLowerCase()
    const contentType = CalendarAttachments.#normalizedContentType(attachment?.contentType)
    if (filename.endsWith('.ics')) return 0
    if (contentType === 'text/calendar') return 1
    if (filename.endsWith('.vcs')) return 2
    if (contentType === 'application/ics') return 3
    return Infinity
  }

  /** @param {string | undefined} value */
  static #normalizedContentType(value) {
    return String(value || '').split(';', 1)[0].trim().toLowerCase()
  }
}

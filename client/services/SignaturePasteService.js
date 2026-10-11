import { signatureHTMLFromClipboard } from '../shared/scripts/composer-signatures.js'

/**
 * Turns a rich clipboard paste into sanitized HTML source in the signature
 * editor. Reading the clipboard event and editing the text selection are
 * page work, so this stays in JavaScript; the input event it raises lets
 * SignatureEditorService keep the edited text.
 */
@Service
export default class SignaturePasteService {
  /** @param {ClipboardEvent} event */
  paste(event) {
    const textarea = SignaturePasteService._textarea(event)
    const rich = signatureHTMLFromClipboard(SignaturePasteService._flavor(event, 'text/plain'), SignaturePasteService._flavor(event, 'text/html'))
    SignaturePasteService._insert(event, textarea, rich)
  }

  /** @param {Event} event @returns {HTMLTextAreaElement | null} */
  static _textarea(event) {
    return event.target instanceof Element ? event.target.closest('[data-signature-html] textarea, textarea[data-signature-html]') : null
  }

  /** @param {ClipboardEvent} event @param {string} type */
  static _flavor(event, type) {
    return [event.clipboardData?.getData(type)].join('')
  }

  /** @param {ClipboardEvent} event @param {HTMLTextAreaElement | null} textarea @param {string} rich */
  static _insert(event, textarea, rich) {
    if (!textarea || !rich) return
    event.preventDefault()
    textarea.setRangeText(rich, textarea.selectionStart, textarea.selectionEnd, 'end')
    textarea.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertFromPaste', data: rich }))
  }
}

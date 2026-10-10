// @ts-check
/** @import { ComposerOpened } from '/models/ComposerOpened.ts' */
/** @import { SessionChanged } from '/models/SessionChanged.ts' */
/** @import { ComposeLoaded } from '/models/ComposeLoaded.ts' */
/** @import { ComposeChanged } from '/models/ComposeChanged.ts' */
/** @import { ComposeAttachment } from '/models/ComposeAttachment.ts' */
/** @import { EditorPreferences } from '/models/EditorPreferences.ts' */
/** @import { SignatureApplyRequested } from '/models/SignatureApplyRequested.ts' */
/** @import { EditorSignatureChanged } from '/models/EditorSignatureChanged.ts' */
/** @import { TemplateInsertRequested } from '/models/TemplateInsertRequested.ts' */
/** @import { TemplateInserted } from '/models/TemplateInserted.ts' */
/** @import { TemplateDraftRequested } from '/models/TemplateDraftRequested.ts' */
/** @import { TemplateDraftReady } from '/models/TemplateDraftReady.ts' */
/** @import { AttachmentsOffered } from '/models/AttachmentsOffered.ts' */
/** @import { AttachmentsAccepted } from '/models/AttachmentsAccepted.ts' */
/** @import { AttachmentAcceptance } from '/models/AttachmentAcceptance.ts' */
/** @import { AttachmentsAdded } from '/models/AttachmentsAdded.ts' */
/** @import { AttachmentRemoved } from '/models/AttachmentRemoved.ts' */
/** @import { InlineImageRemoved } from '/models/InlineImageRemoved.ts' */
/** @import { RecipientQueryChanged } from '/models/RecipientQueryChanged.ts' */
/** @import { RecipientSuggestionsDismissed } from '/models/RecipientSuggestionsDismissed.ts' */
/** @import { RecipientSuggestionsShown } from '/models/RecipientSuggestionsShown.ts' */
/** @import { RecipientChosen } from '/models/RecipientChosen.ts' */
/** @import { GrammarTextChanged } from '/models/GrammarTextChanged.ts' */
/** @import { GrammarTextWanted } from '/models/GrammarTextWanted.ts' */
/** @import { GrammarHighlightsChanged } from '/models/GrammarHighlightsChanged.ts' */
/** @import { GrammarReplacementRequested } from '/models/GrammarReplacementRequested.ts' */
/** @import { GrammarFocusRequested } from '/models/GrammarFocusRequested.ts' */
/** @import { ComposeShortcut } from '/models/ComposeShortcut.ts' */
/** @import { SendRequested } from '/models/SendRequested.ts' */
/** @import { SendFinished } from '/models/SendFinished.ts' */
/** @import { DraftSaveRequested } from '/models/DraftSaveRequested.ts' */
/** @import { DraftSaveDue } from '/models/DraftSaveDue.ts' */
/** @import { DraftSaveFinished } from '/models/DraftSaveFinished.ts' */
/** @import { MessageEnvelope } from '/models/MessageEnvelope.ts' */
/** @import { NavigationRequested } from '/models/NavigationRequested.ts' */
/** @import { ToastRequested } from '/models/ToastRequested.ts' */
/** @import { SoundRequested } from '/models/SoundRequested.ts' */

import {
  decorateTemplateVariables,
  hasEmptyTemplateVariables,
  insertTemplateBeforeProtectedContent,
  moveBetweenTemplateVariables,
  selectTemplateVariable,
  stripTemplateVariableMarkup,
  templateBodyFromDraft,
} from './composer-templates.js'
import { applySignature, currentSignatureId } from './composer-signatures.js'
import {
  clearGrammarHighlights,
  grammarRange,
  grammarText,
  paintGrammarHighlights,
  replaceGrammarRange,
} from './grammar-check.js'
import { TRANSLATION_LANGUAGES } from './translation-languages.js'
import {
  applyBlockFormat,
  applyInlineFormat,
  applyInlineMarkdownShortcut,
  applyListFormat,
  applyMarkdownShortcut,
  autoLinkBeforeCaret,
  clearEditorContent,
  insertInlineImage,
  insertPastedComposerContent,
  isInlineImageFile,
  normalizeComposeLink,
  replaceEditorContent,
  resizeInlineImage,
  serializeComposerHTML,
} from './rich-compose-editor.js'
import { formatPreferenceDate } from './mail-preferences.js'
import { composeLaunchPrefill } from './pwa-launch.js'
import {
  discardOfflineWrites,
  loadOfflineSnapshot,
  queueOfflineWrite,
  removeOfflineSnapshot,
  saveOfflineSnapshot,
} from './offline-write-vault.js'
import { mailRequest } from './mail-transport.js'

/**
 * The browser-only half of the composer, used by ComposeEditorService.
 *
 * This plain module holds what a Go service cannot do in the page: the
 * contenteditable rich-text editor (selection, formatting, paste, inline
 * images, quoted text and template blanks), the grammar highlights, file
 * reading, confirm and prompt dialogs, debounce timers, on-device
 * translation, the encrypted offline vault and the requests that carry the
 * message body and attachment bytes (see mail-transport.js). The Go
 * ComposeService, SenderService, TemplateService and GrammarService own
 * every other part of the composer and talk to it through events.
 */
/**
 * The preferred translation target, like preferredTranslationLanguage in
 * on-device-translation.js. That module (and its language detector) loads
 * only when the person asks for a translation.
 */
export function preferredTranslationTarget() {
  const primary = String(navigator.language || 'en').trim().toLowerCase().split('-')[0]
  return Object.hasOwn(TRANSLATION_LANGUAGES, primary) ? primary : 'en'
}

export class ComposeEditor {
  /** @param {any} view the ComposeEditorService whose fields the view reads */
  constructor(view) {
    this._view = view
  }

  static _EDITOR = '[data-compose-editor]'
  static _PANEL = '.compose-panel'
  static _RECIPIENT_FIELD = '[data-recipient-field]'
  static _RECOVERY = 'compose-recovery'
  static _LEGACY_RECOVERY = 'caduceus-compose-recovery'
  static _PREFILL = 'caduceus_compose_prefill'
  static _UNDO = 'caduceus_pending_send_undo'
  static _TRANSLATION_TARGET = 'caduceus-translation-target'
  static _SAVE_DELAY = 750
  static _GRAMMAR_DELAY = 700
  static _GRAMMAR_COMPOSITION_DELAY = 300
  static _RECIPIENT_DELAY = 140
  static _BLUR_DELAY = 120
  static _SAVED = 'Saved'
  static _CONFLICT = 'Newer saved version detected'
  static _DRAFT = 'draft'
  static _SEND = 'send'
  static _ARCHIVE = 'archive'
  static _SCHEDULE_EDIT = 'schedule-edit'
  static _INLINE = 'inline'
  static _ATTACHMENT = 'attachment'
  static _QUOTE = '[data-caduceus-quote]'
  static _INLINE_IMAGE = 'img[data-caduceus-inline-image]'
  static _SELECTED = 'data-selected'
  static _EXPANDED = 'aria-expanded'
  static _COLLAPSED = 'compose-quote-collapsed'
  static _QUOTE_EXPANDED = 'data-quote-expanded'
  static _FALSE = 'false'
  static _TRUE = 'true'



  _token = ''
  _owner = ''
  _started = false
  _ready = false
  _exit = false
  _richText = true
  _html = ''
  /** @type {HTMLElement | null} */
  _liveEditor = null
  /** @type {Map<string, Record<string, any>>} */
  _attachments = new Map()
  /** @type {Map<number, File>} */
  _offered = new Map()
  _offeredCount = 0
  _draftId = ''
  _draftClientId = ''
  _draftVersion = 0
  _lastSnapshot = ''
  _saving = false
  _saveAgain = false
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _saveTimer
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _grammarTimer
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _recipientTimer
  _composing = false
  _recipient = { field: '', count: 0, active: -1 }
  /** @type {Range | null} */
  _savedRange = null
  /** @type {HTMLImageElement | null} */
  _selectedImage = null
  _selectedImageId = ''
  /** @type {AbortController | null} */
  _listeners = null
  /** @type {MutationObserver | null} */
  _observer = null

  get translationLanguages() {
    return Object.entries(TRANSLATION_LANGUAGES).map(([id, label]) => ({ id, label }))
  }

  start() {
    this._restoreTranslationTarget()
    this._bindDocument()
    this._observeEditor()
    /** @type {ComposerOpened} */
    const opened = { route: location.pathname }
    this._view.publishComposerOpened(opened)
  }

  stop() {
    this._listeners?.abort()
    this._observer?.disconnect()
    clearTimeout(this._saveTimer)
    clearTimeout(this._grammarTimer)
    clearTimeout(this._recipientTimer)
  }

  /** @param {SessionChanged} changed */
  async sessionChanged(changed) {
    this._token = changed.token
    this._owner = String(changed.email).toLowerCase()
    await this._openOnce()
  }

  /** @param {EditorPreferences} preferences */
  editorPreferences(preferences) {
    this._richText = preferences.richText
    this._applyEditorPreferences(preferences)
  }

  /** @param {ComposeChanged} changed */
  async composeChanged(changed) {
    this._followChange(changed.reason)
    await this._render()
  }

  /** @param {SignatureApplyRequested} requested */
  signatureApplyRequested(requested) {
    this._html = applySignature(this._currentHTML(), this._signature(requested))
    this._syncEditor()
  }

  /** @param {TemplateInsertRequested} requested */
  templateInsertRequested(requested) {
    this._insertTemplate(requested, this._templateAllowed(requested.confirm))
  }

  /** @param {TemplateDraftRequested} requested */
  templateDraftRequested(requested) {
    this._offerTemplate(templateBodyFromDraft(this._currentHTML()), requested.suggestedName)
  }

  /** @param {AttachmentsAccepted} accepted */
  async attachmentsAccepted(accepted) {
    const added = await this._readAccepted(accepted.accepted)
    this._insertInlineImages(added)
    this._offered.clear()
    /** @type {AttachmentsAdded} */
    const event = { attachments: added.map(item => this._metadata(item)) }
    this._view.publishAttachmentsAdded(event)
  }

  /** @param {AttachmentRemoved} removed */
  attachmentRemoved(removed) {
    this._attachments.delete(removed.id)
    this._removeInlineImages(removed.contentId)
  }

  /** @param {RecipientSuggestionsShown} shown */
  recipientSuggestionsShown(shown) {
    this._recipient = { field: shown.field, count: shown.count, active: shown.active }
    this._describeRecipientInputs()
  }

  /** @param {RecipientChosen} chosen */
  recipientChosen(chosen) {
    const input = this._recipientInput(chosen.field)
    this._fillRecipientInput(input, chosen.value)
  }

  /** @param {GrammarTextWanted} wanted */
  grammarTextWanted(wanted) {
    this._scheduleGrammar(0)
  }

  /** @param {GrammarHighlightsChanged} changed */
  grammarHighlightsChanged(changed) {
    this._paintGrammar(this._editor(), changed.ranges)
  }

  /** @param {GrammarReplacementRequested} requested */
  grammarReplacementRequested(requested) {
    this._replaceGrammar(this._editor(), requested)
  }

  /** @param {GrammarFocusRequested} requested */
  grammarFocusRequested(requested) {
    this._focusRange(this._editor(), requested.offset, requested.length)
  }

  /** @param {SendRequested} requested */
  async sendRequested(requested) {
    const finished = await this._deliver(requested, this._blanksAccepted())
    this._view.publishSendFinished(finished)
  }

  /** @param {DraftSaveRequested} requested */
  async draftSaveRequested(requested) {
    await this._saveDraft(requested, this._mayStartSave())
    await this._render()
  }

  /** @param {NavigationRequested} requested */
  async navigationRequested(requested) {
    await this._leave(requested)
  }

  /** Bold the selection (toolbar and Ctrl+B). */
  bold() { this._format('strong') }

  italic() { this._format('em') }

  underline() { this._format('u') }

  strike() { this._format('s') }

  code() { this._format('code') }

  bulletList() { this._list(false) }

  numberedList() { this._list(true) }

  /** Turn the selection into a link after asking for its address. */
  link() {
    this._askLink(this._restoreSelection())
  }

  /** @param {Event} event */
  changeBlockStyle(event) {
    const select = /** @type {HTMLSelectElement} */ (event.target)
    this._block(select.value)
    select.value = 'p'
  }

  /** @param {Event} event */
  changeFontFamily(event) {
    this._styled('font-family', /** @type {HTMLSelectElement} */ (event.target).value, ['Arial, sans-serif', 'Georgia, serif', 'Tahoma, sans-serif', 'Times New Roman, serif', 'Verdana, sans-serif', 'monospace'])
  }

  /** @param {Event} event */
  changeFontSize(event) {
    this._styled('font-size', /** @type {HTMLSelectElement} */ (event.target).value, ['12px', '14px', '16px', '18px', '24px', '32px'])
  }

  /** @param {Event} event */
  changeTextColor(event) {
    this._color(/** @type {HTMLInputElement} */ (event.target).value)
  }

  toggleRightToLeft() {
    this._direction(this._restoreSelection())
  }

  toggleQuotedText() {
    this._view.quoteExpanded = !this._view.quoteExpanded
    this._renderQuote(this._editor())
  }

  removeQuotedText() {
    this._removeQuote(this._editor())
  }

  pickAttachments() { this._click('[data-attachment-input]') }

  pickInlineImages() { this._click('[data-inline-image-input]') }

  /** @param {Event} event */
  addAttachments(event) {
    this._offerFiles(this._inputFiles(/** @type {HTMLInputElement} */ (event.target)), false)
  }

  /** @param {Event} event */
  addInlineImages(event) {
    this._offerFiles(this._inputFiles(/** @type {HTMLInputElement} */ (event.target)), true)
  }

  /** @param {DragEvent} event */
  dragOver(event) {
    this._acceptDrag(event, Array.from(event.dataTransfer?.types ?? []).includes('Files'))
  }

  /** @param {DragEvent} event */
  dragLeave(event) {
    this._leaveDrag(/** @type {Node | null} */ (event.currentTarget), /** @type {Node | null} */ (event.relatedTarget))
  }

  /** @param {DragEvent} event */
  drop(event) {
    event.preventDefault()
    this._view.dragActive = false
    this._offerMixed(this._transferFiles(event.dataTransfer))
  }

  /** @param {ClipboardEvent} event */
  paste(event) {
    this._paste(event, this._transferFiles(event.clipboardData))
  }

  /**
   * @param {Event} _event
   * @param {number} width
   */
  resizeSelectedInlineImage(_event, width) {
    this._resizeImage(this._selectedImage, Number(width))
  }

  removeSelectedInlineImage() {
    this._removeSelectedImage(this._selectedImage)
  }

  /** @param {Event} event */
  changeTranslationTarget(event) {
    this._keepTranslationTarget(String(/** @type {HTMLSelectElement} */ (event.target).value), TRANSLATION_LANGUAGES)
  }

  /** Replace the draft with an on-device translation the person asked for. */
  async translateDraft() {
    await this._translate(this._editor(), this._view.translationBusy)
    await this._render()
  }

  /** Replace local edits with the newer saved draft after a conflict. */
  async reloadSavedDraft() {
    await this._reload(this._draftId !== '' && confirm('Replace local changes with the newer saved draft?'))
    await this._render()
  }

  /** Keep local edits by saving them as a new draft after a conflict. */
  async saveDraftCopy() {
    await discardOfflineWrites(this._owner, `draft:${this._draftClientId}`)
    this._draftId = ''
    this._draftVersion = 0
    this._draftClientId = crypto.randomUUID()
    this._lastSnapshot = ''
    this._view.draftConflict = false
    /** @type {DraftSaveFinished} */
    const copied = { status: 'copy', draftId: '', draftClientId: this._draftClientId, draftVersion: 0, message: '' }
    this._view.publishDraftSaveFinished(copied)
    this._requestSave(0)
  }

  /**
   * Format a stored ISO time with the mailbox's clock preference.
   * @param {string} value
   */
  formatDate(value) {
    return formatPreferenceDate(value)
  }

  /**
   * The ISO time of a follow-up reminder preset, in local wall-clock time.
   * @param {string} preset
   */
  reminderPreset(preset) {
    return this._reminderDate(preset).toISOString()
  }

  /**
   * The ISO time of a send-later preset, in local wall-clock time.
   * @param {string} preset
   */
  sendLaterPreset(preset) {
    return this._sendLaterDate(preset).toISOString()
  }

  /**
   * Convert a datetime-local value to an ISO time ('' when invalid).
   * @param {string} value
   */
  localTime(value) {
    return this._isoOrEmpty(new Date(String(value)))
  }

  async _render() {
    await this._view.taskScheduler.render()
    this._syncEditor()
  }

  /** @param {Date} date */
  _isoOrEmpty(date) {
    return Number.isFinite(date.getTime()) ? date.toISOString() : ''
  }

  /** @param {string} preset */
  _reminderDate(preset) {
    const date = new Date()
    if (preset === 'tomorrow') {
      date.setDate(date.getDate() + 1)
      date.setHours(9, 0, 0, 0)
      return date
    }
    const days = { 'three-days': 3, 'next-week': 7 }[preset] ?? 7
    return new Date(Date.now() + days * 24 * 60 * 60 * 1000)
  }

  /** @param {string} preset */
  _sendLaterDate(preset) {
    const date = new Date()
    const nextMonday = (8 - date.getDay()) % 7
    const days = { 'next-monday': nextMonday === 0 ? 7 : nextMonday }[preset] ?? 1
    date.setDate(date.getDate() + days)
    date.setHours(preset === 'tomorrow-afternoon' ? 14 : 8, 0, 0, 0)
    return date
  }

  _restoreTranslationTarget() {
    try {
      this._keepTranslationTarget(String(localStorage.getItem(ComposeEditor._TRANSLATION_TARGET)), TRANSLATION_LANGUAGES)
    } catch { /* storage unavailable */ }
  }

  /**
   * @param {string} target
   * @param {Record<string, string>} languages
   */
  _keepTranslationTarget(target, languages) {
    if (!Object.hasOwn(languages, target)) return
    this._view.translationTarget = target
    this._storeTranslationTarget(target)
  }

  /** @param {string} target */
  _storeTranslationTarget(target) {
    try { localStorage.setItem(ComposeEditor._TRANSLATION_TARGET, target) } catch { /* storage unavailable */ }
  }

  _bindDocument() {
    this._listeners = new AbortController()
    const options = { capture: true, signal: this._listeners.signal }
    document.addEventListener('input', event => this._onInput(event), options)
    document.addEventListener('keydown', event => this._onKeydown(event), options)
    document.addEventListener('selectionchange', () => this._onSelection(), options)
    document.addEventListener('mousedown', event => this._keepToolbarSelection(event), options)
    document.addEventListener('click', event => this._onClick(event), options)
    document.addEventListener('focusin', event => this._onFocusIn(event), options)
    document.addEventListener('focusout', event => this._onFocusOut(event), options)
    document.addEventListener('pointerdown', event => this._keepRecipientFocus(event), options)
    document.addEventListener('compositionstart', event => this._composition(event, true), options)
    document.addEventListener('compositionend', event => this._composition(event, false), options)
    document.addEventListener('visibilitychange', () => this._onVisibility(), { signal: this._listeners.signal })
    window.addEventListener('caduceus:offline-write-result', event => this._onOfflineWrite(/** @type {CustomEvent<any>} */ (event).detail), { signal: this._listeners.signal })
    window.addEventListener('caduceus:shortcut:composer:send', () => this._shortcut(ComposeEditor._SEND), { signal: this._listeners.signal })
    window.addEventListener('caduceus:shortcut:composer:discard', () => this._shortcut('discard'), { signal: this._listeners.signal })
  }

  /** Restore the rich editor after Tachyon replaced its node in a render. */
  _observeEditor() {
    this._observer = new MutationObserver(() => this._syncEditor())
    this._observer.observe(document.body, { childList: true, subtree: true })
  }

  /** @returns {HTMLElement | null} */
  _editor() {
    return document.querySelector(ComposeEditor._EDITOR)
  }

  /** Write the HTML model into a newly created editor node. */
  _syncEditor() {
    const editor = this._editor()
    if (!editor || editor === this._liveEditor) return
    this._liveEditor = editor
    this._hydrate(editor)
  }

  /** @param {HTMLElement} editor */
  _hydrate(editor) {
    this._observer?.disconnect()
    replaceEditorContent(editor, this._html, Array.from(this._attachments.values()))
    this._decorate(editor)
    this._renderQuote(editor)
    this._reselectImage(editor)
    this._observer?.observe(document.body, { childList: true, subtree: true })
  }

  /** Rewrite the live editor from the model (after a signature or template change). */
  _rewriteEditor() {
    this._liveEditor = null
    this._syncEditor()
  }

  /** @param {HTMLElement} editor */
  _decorate(editor) {
    const variables = decorateTemplateVariables(editor)
    const empty = variables.filter(node => node.classList.contains('empty')).length
    this._view.templateStatus = this._templateStatusText(variables.length, empty)
  }

  /**
   * @param {number} count
   * @param {number} empty
   */
  _templateStatusText(count, empty) {
    if (count === 0) return ''
    return `Template blanks: ${count}${empty ? ` (${empty} empty)` : ''}. Press Tab or Shift+Tab to move between blanks.`
  }

  /** @param {HTMLElement} editor */
  _reselectImage(editor) {
    const image = /** @type {HTMLImageElement | null} */ (editor.querySelector(`img[data-caduceus-attachment-id="${CSS.escape(this._selectedImageId)}"]`))
    this._selectImage(this._selectedImageId ? image : null)
  }

  /** @returns {string} */
  _currentHTML() {
    const editor = this._editor()
    return this._serialized(editor, this._richText && editor !== null)
  }

  /**
   * @param {HTMLElement | null} editor
   * @param {boolean} rich
   */
  _serialized(editor, rich) {
    if (!editor) return this._html
    return rich ? serializeComposerHTML(editor) : editor.innerText
  }

  _currentText() {
    return this._editor()?.textContent ?? ''
  }

  /** Keep the model in step after any change inside the editor. */
  _editorChanged() {
    this._html = this._currentHTML()
    this._renderQuote(this._editor())
    this._requestSave(0)
  }

  /** @param {string} reason */
  _followChange(reason) {
    switch (reason) {
      case 'ready':
        this._ready = true
        this._rewriteEditor()
        break
      case 'save-now':
        this._requestSave(0)
        break
      case 'save-soon':
        this._requestSave(ComposeEditor._SAVE_DELAY)
        break
    }
  }

  /** @param {EditorPreferences} preferences */
  _applyEditorPreferences(preferences) {
    for (const target of [this._editor(), document.querySelector('[data-compose-subject] input')]) {
      this._applySpelling(/** @type {HTMLElement | null} */ (target), preferences)
    }
  }

  /**
   * @param {HTMLElement | null} target
   * @param {EditorPreferences} preferences
   */
  _applySpelling(target, preferences) {
    if (!target) return
    target.spellcheck = preferences.spellcheck
    target.toggleAttribute('lang', Boolean(preferences.spellcheckLanguage))
    target.lang = preferences.spellcheckLanguage
  }

  /** @param {SignatureApplyRequested} requested */
  _signature(requested) {
    return requested.signatureId ? { id: requested.signatureId, text: requested.text } : null
  }

  async _openOnce() {
    if (this._started) return
    this._started = true
    const loaded = await this._open(this._consumePrefill())
    this._view.publishComposeLoaded(loaded)
  }

  /** @returns {Record<string, any>} */
  _consumePrefill() {
    const stored = this._storedPrefill()
    sessionStorage.removeItem(ComposeEditor._PREFILL)
    return { ...stored, ...composeLaunchPrefill(location.search) }
  }

  /** @returns {Record<string, any>} */
  _storedPrefill() {
    try {
      const stored = JSON.parse(String(sessionStorage.getItem(ComposeEditor._PREFILL)))
      return stored && typeof stored === 'object' ? stored : {}
    } catch { return {} }
  }

  /**
   * Seed the model from the prefill, the draft it names or the local
   * recovery record, like initFromPrefill in the former compose view.
   * @param {Record<string, any>} prefill
   * @returns {Promise<ComposeLoaded>}
   */
  async _open(prefill) {
    const headers = this._prefillHeaders(prefill)
    this._html = String(prefill.body ?? '')
    this._keepAttachments(prefill.attachments)
    const state = await this._restore(prefill, headers, this._hasExplicitPrefill(prefill))
    this._draftClientId = this._draftClientId || crypto.randomUUID()
    return this._loadedEvent('open', state.headers, state.recovered, state.status)
  }

  /** @param {Record<string, any>} prefill */
  _prefillHeaders(prefill) {
    return {
      from: String(prefill.from ?? ''), to: String(prefill.to ?? ''), cc: String(prefill.cc ?? ''), bcc: String(prefill.bcc ?? ''),
      replyTo: String(prefill.replyTo ?? ''), subject: String(prefill.subject ?? ''), inReplyTo: String(prefill.inReplyTo ?? ''),
      references: Array.isArray(prefill.references) ? prefill.references.map(String) : [],
      scheduledId: String(prefill.scheduledId ?? ''), sendAt: String(prefill.sendAt ?? ''),
      sourceEmailId: String(prefill.sourceEmailId ?? ''), remindAt: String(prefill.remindAt ?? ''),
      openTracking: prefill.openTracking === true, linkTracking: prefill.linkTracking === true,
    }
  }

  /** @param {unknown} attachments */
  _keepAttachments(attachments) {
    this._attachments.clear()
    for (const attachment of Array.isArray(attachments) ? attachments : []) {
      const id = String(attachment.id || crypto.randomUUID())
      this._attachments.set(id, { ...attachment, id })
    }
  }

  /** @param {Record<string, any>} prefill */
  _hasExplicitPrefill(prefill) {
    return Boolean(prefill.scheduledId || prefill.sourceEmailId || prefill.from || prefill.to || prefill.cc || prefill.bcc || prefill.replyTo || prefill.subject || prefill.body || prefill.inReplyTo || prefill.references?.length)
  }

  /**
   * @param {Record<string, any>} prefill
   * @param {Record<string, any>} headers
   * @param {boolean} explicit
   */
  async _restore(prefill, headers, explicit) {
    if (prefill.draftId) return await this._openDraft(String(prefill.draftId), headers)
    if (explicit) return { headers, recovered: false, status: '' }
    return await this._recover(headers)
  }

  /**
   * @param {string} id
   * @param {Record<string, any>} headers
   */
  async _openDraft(id, headers) {
    this._draftId = id
    const loaded = await this._loadDraft(id)
    return { headers: { ...headers, ...loaded }, recovered: false, status: this._view.draftStatus }
  }

  /**
   * Fetch a draft and seed the model from it; answers its header fields.
   * @param {string} id
   */
  async _loadDraft(id) {
    try {
      const response = await mailRequest(`/drafts/${id}`, { token: this._token })
      return this._applyDraft(response.ok ? response.data : null)
    } catch { return {} }
  }

  /** @param {Record<string, any> | null} draft */
  _applyDraft(draft) {
    if (!draft) return {}
    this._html = String(draft.htmlBody || draft.body || this._html)
    this._keepAttachments(draft.attachments)
    this._draftId = String(draft.id)
    this._draftClientId = String(draft.draftClientId || this._draftClientId || crypto.randomUUID())
    this._draftVersion = Number(draft.draftVersion || 0)
    this._lastSnapshot = JSON.stringify(this._savedSnapshot(draft))
    this._view.draftStatus = this._draftVersion ? ComposeEditor._SAVED : ''
    this._persistRecovery(this._savedSnapshot(draft))
    return this._draftHeaders(draft)
  }

  /** @param {Record<string, any>} draft */
  _draftHeaders(draft) {
    const list = (/** @type {unknown} */ value) => Array.isArray(value) ? value.join(', ') : String(value || '')
    return {
      from: String(draft.sender || ''), to: list(draft.to || draft.recipient), cc: list(draft.cc), bcc: list(draft.bcc),
      replyTo: Array.isArray(draft.replyTo) ? String(draft.replyTo[0] || '') : String(draft.replyTo || ''),
      subject: String(draft.subject || ''), inReplyTo: String(draft.inReplyTo || ''),
      references: Array.isArray(draft.references) ? draft.references : [], sourceEmailId: String(draft.sourceEmailId || ''),
    }
  }

  /**
   * Restore a server-backed draft pointer, falling back to the encrypted
   * local snapshot after a crash.
   * @param {Record<string, any>} headers
   */
  async _recover(headers) {
    const recovery = await this._recoveryRecord()
    const source = recovery && typeof recovery === 'object' ? recovery : {}
    this._draftClientId = String(source.clientId || '')
    this._draftVersion = Number(source.version || 0)
    const restored = await this._recoverDraft(String(source.draftId || ''))
    return this._recoveredState(headers, restored, source.snapshot)
  }

  async _recoveryRecord() {
    const stored = await loadOfflineSnapshot(this._owner, ComposeEditor._RECOVERY)
    return stored ?? await this._migrateLegacyRecovery()
  }

  /** One-time migration from the former plaintext recovery record. */
  async _migrateLegacyRecovery() {
    try {
      const legacy = JSON.parse(String(localStorage.getItem(ComposeEditor._LEGACY_RECOVERY)))
      localStorage.removeItem(ComposeEditor._LEGACY_RECOVERY)
      await saveOfflineSnapshot(this._owner, ComposeEditor._RECOVERY, legacy)
      return legacy
    } catch { return null }
  }

  /** @param {string} id */
  async _recoverDraft(id) {
    if (!id) return null
    const headers = await this._loadDraft(id)
    return Object.keys(headers).length ? headers : null
  }

  /**
   * @param {Record<string, any>} headers
   * @param {Record<string, any> | null} restored
   * @param {any} snapshot
   */
  _recoveredState(headers, restored, snapshot) {
    if (restored) return { headers: { ...headers, ...restored }, recovered: true, status: this._view.draftStatus }
    if (!snapshot || typeof snapshot !== 'object') return { headers, recovered: false, status: '' }
    this._html = String(snapshot.html || snapshot.text || '')
    this._view.draftStatus = 'Recovered locally'
    return { headers: { ...headers, ...this._snapshotHeaders(snapshot) }, recovered: true, status: this._view.draftStatus }
  }

  /** @param {Record<string, any>} snapshot */
  _snapshotHeaders(snapshot) {
    const list = (/** @type {unknown} */ value) => Array.isArray(value) ? value.join(', ') : ''
    return {
      from: String(snapshot.from || ''), to: list(snapshot.to), cc: list(snapshot.cc), bcc: list(snapshot.bcc),
      replyTo: Array.isArray(snapshot.replyTo) ? String(snapshot.replyTo[0] || '') : '',
      subject: String(snapshot.subject || ''), inReplyTo: String(snapshot.inReplyTo || ''),
      references: Array.isArray(snapshot.references) ? snapshot.references : [], sourceEmailId: String(snapshot.sourceEmailId || ''),
    }
  }

  /**
   * @param {string} reason
   * @param {Record<string, any>} headers
   * @param {boolean} recovered
   * @param {string} status
   * @returns {ComposeLoaded}
   */
  _loadedEvent(reason, headers, recovered, status) {
    const attachments = Array.from(this._attachments.values()).map(item => this._metadata(item))
    return {
      reason, from: headers.from, to: headers.to, cc: headers.cc, bcc: headers.bcc, replyTo: headers.replyTo,
      subject: headers.subject, inReplyTo: headers.inReplyTo, references: headers.references,
      scheduledId: String(headers.scheduledId ?? ''), sendAt: String(headers.sendAt ?? ''),
      sourceEmailId: headers.sourceEmailId, remindAt: String(headers.remindAt ?? ''),
      openTracking: headers.openTracking === true, linkTracking: headers.linkTracking === true,
      draftId: this._draftId, draftClientId: this._draftClientId, draftVersion: this._draftVersion, draftStatus: status,
      savedSnapshot: this._lastSnapshot !== '', explicitPrefill: false, recovered,
      meaningful: this._meaningful(headers, attachments.length), signatureId: currentSignatureId(this._html), attachments,
    }
  }

  /**
   * @param {Record<string, any>} headers
   * @param {number} attachments
   */
  _meaningful(headers, attachments) {
    return Boolean(headers.to || headers.cc || headers.replyTo || headers.subject || this._html.trim() || attachments)
  }

  /** @param {Record<string, any>} attachment @returns {ComposeAttachment} */
  _metadata(attachment) {
    return {
      id: String(attachment.id), filename: String(attachment.filename), contentType: String(attachment.contentType),
      size: Number(attachment.size), disposition: String(attachment.disposition || ComposeEditor._ATTACHMENT),
      contentId: String(attachment.contentId || ''),
    }
  }

  /** @param {boolean} allowed */
  async _reload(allowed) {
    if (!allowed) return
    const headers = await this._loadDraft(this._draftId)
    this._view.draftConflict = false
    this._view.draftStatus = 'Saved version loaded'
    this._rewriteEditor()
    this._view.publishComposeLoaded(this._loadedEvent('reload', headers, false, this._view.draftStatus))
  }

  /** @param {Record<string, any> | null} snapshot */
  _persistRecovery(snapshot) {
    void saveOfflineSnapshot(this._owner, ComposeEditor._RECOVERY, {
      draftId: this._draftId, clientId: this._draftClientId, version: this._draftVersion, snapshot, savedAt: new Date().toISOString(),
    }).catch(() => { /* server autosave remains authoritative */ })
  }

  async _clearRecovery() {
    await removeOfflineSnapshot(this._owner, ComposeEditor._RECOVERY).catch(() => {})
    this._removeLegacyRecovery()
  }

  _removeLegacyRecovery() {
    try { localStorage.removeItem(ComposeEditor._LEGACY_RECOVERY) } catch { /* storage unavailable */ }
  }

  /** Debounce a draft save; the Go service decides whether one may run. */
  /** @param {number} delay */
  _requestSave(delay) {
    if (!this._ready || this._exit) return
    clearTimeout(this._saveTimer)
    this._view.draftStatus = this._pendingStatus(this._saving)
    this._saveTimer = setTimeout(() => this._publishSaveDue(delay), delay)
  }

  /** @param {boolean} saving */
  _pendingStatus(saving) {
    if (saving) {
      this._saveAgain = true
      return this._view.draftStatus
    }
    return 'Unsaved changes'
  }

  /** @param {number} delay */
  _publishSaveDue(delay) {
    /** @type {DraftSaveDue} */
    const due = { delay }
    this._view.publishDraftSaveDue(due)
  }

  _mayStartSave() {
    if (this._saving) {
      this._saveAgain = true
      return false
    }
    return !this._view.draftConflict && !this._exit
  }

  /**
   * Persist one complete versioned snapshot.
   * @param {DraftSaveRequested} requested
   * @param {boolean} allowed
   */
  async _saveDraft(requested, allowed) {
    if (!allowed) return
    const snapshot = this._snapshot(requested.envelope, true)
    const fingerprint = JSON.stringify(snapshot)
    this._persistRecovery(this._snapshot(requested.envelope, false))
    await this._sendDraft(snapshot, fingerprint, this._draftNeeded(snapshot, fingerprint))
  }

  /**
   * @param {Record<string, any>} snapshot
   * @param {string} fingerprint
   */
  _draftNeeded(snapshot, fingerprint) {
    if (fingerprint === this._lastSnapshot) {
      this._view.draftStatus = ComposeEditor._SAVED
      return false
    }
    return this._snapshotMeaningful(snapshot) || this._draftId !== ''
  }

  /** @param {Record<string, any>} snapshot */
  _snapshotMeaningful(snapshot) {
    return Boolean(snapshot.to.length || snapshot.cc.length || snapshot.replyTo.length || snapshot.subject || snapshot.text.trim() || snapshot.html.trim() || snapshot.attachments.length)
  }

  /**
   * Capture a complete server snapshot; local recovery omits attachment bytes.
   * @param {MessageEnvelope} envelope
   * @param {boolean} withBytes
   */
  _snapshot(envelope, withBytes) {
    return {
      from: envelope.from, to: envelope.to, cc: envelope.cc, bcc: envelope.bcc, replyTo: envelope.replyTo,
      subject: envelope.subject, text: this._currentText(), html: this._currentHTML(),
      ...(envelope.inReplyTo ? { inReplyTo: envelope.inReplyTo } : {}),
      references: [...envelope.references],
      ...(envelope.sourceEmailId ? { sourceEmailId: envelope.sourceEmailId } : {}),
      attachments: Array.from(this._attachments.values()).map(attachment => ({
        filename: attachment.filename, contentType: attachment.contentType, size: attachment.size,
        disposition: attachment.disposition || ComposeEditor._ATTACHMENT,
        ...(attachment.contentId ? { contentId: attachment.contentId } : {}),
        ...(withBytes ? { contentBase64: attachment.contentBase64 } : {}),
      })),
    }
  }

  /** @param {Record<string, any>} draft */
  _savedSnapshot(draft) {
    const list = (/** @type {unknown} */ value) => Array.isArray(value) ? value : []
    return {
      from: draft.sender, to: list(draft.to), cc: list(draft.cc), bcc: list(draft.bcc), replyTo: list(draft.replyTo),
      subject: draft.subject || '', text: draft.body || '', html: draft.htmlBody || '',
      ...(draft.inReplyTo ? { inReplyTo: draft.inReplyTo } : {}),
      references: list(draft.references),
      ...(draft.sourceEmailId ? { sourceEmailId: draft.sourceEmailId } : {}),
      attachments: list(draft.attachments).map(attachment => ({
        filename: attachment.filename, contentType: attachment.contentType, size: attachment.size,
        contentBase64: attachment.contentBase64, disposition: attachment.disposition || ComposeEditor._ATTACHMENT,
        ...(attachment.contentId ? { contentId: attachment.contentId } : {}),
      })),
    }
  }

  /**
   * @param {Record<string, any>} snapshot
   * @param {string} fingerprint
   * @param {boolean} needed
   */
  async _sendDraft(snapshot, fingerprint, needed) {
    if (!needed) return
    const creating = this._draftId === ''
    const request = this._draftRequest(snapshot, creating)
    this._saving = true
    this._view.draftStatus = 'Saving…'
    await this._render()
    const outcome = await this._attempt(request.path, request.method, request.body)
    await this._afterDraft(outcome, request, fingerprint)
    this._finishSave()
  }

  /**
   * @param {Record<string, any>} snapshot
   * @param {boolean} creating
   */
  _draftRequest(snapshot, creating) {
    const body = { ...snapshot, clientId: this._draftClientId, ...(creating ? {} : { version: this._draftVersion }) }
    return creating
      ? { path: '/drafts', method: 'POST', body }
      : { path: `/drafts/${this._draftId}`, method: 'PUT', body }
  }

  /**
   * One request through the large-payload carrier.
   * @param {string} path
   * @param {string} method
   * @param {unknown} body
   */
  async _attempt(path, method, body) {
    try {
      const response = await mailRequest(path, { method, body, token: this._token })
      return { network: false, ...response }
    } catch {
      return { network: true, ok: false, status: 0, data: null }
    }
  }

  /**
   * @param {{ network: boolean, ok: boolean, status: number, data: any }} outcome
   * @param {{ path: string, method: string, body: any }} request
   * @param {string} fingerprint
   */
  async _afterDraft(outcome, request, fingerprint) {
    switch (true) {
      case outcome.network:
        await this._queueDraft(request)
        break
      case outcome.status === 409 || outcome.data?.conflict === true:
        this._view.draftConflict = true
        this._view.draftStatus = ComposeEditor._CONFLICT
        break
      case !outcome.ok:
        this._view.draftStatus = String(outcome.data?.error || 'Autosave failed')
        break
      default:
        await this._draftSaved(outcome.data, fingerprint)
    }
  }

  /** @param {{ path: string, method: string, body: any }} request */
  async _queueDraft(request) {
    this._view.draftStatus = await this._queued({
      kind: ComposeEditor._DRAFT, path: request.path, method: request.method, body: request.body,
      message: 'Draft autosave', coalesceKey: `draft:${this._draftClientId}`,
      metadata: { clientId: this._draftClientId, draftId: this._draftId },
    }, 'Offline — encrypted autosave queued', 'Offline — changes kept in this session')
  }

  /**
   * Encrypt and persist a write for the shell to replay after reconnecting.
   * @param {Record<string, any>} write
   * @param {string} queuedMessage
   * @param {string} failedMessage
   */
  async _queued(write, queuedMessage, failedMessage) {
    try {
      await queueOfflineWrite({ ...write, owner: this._owner })
      this._toast('Change queued securely and will synchronize when this device reconnects.')
      return queuedMessage
    } catch { return failedMessage }
  }

  /**
   * @param {Record<string, any>} data
   * @param {string} fingerprint
   */
  async _draftSaved(data, fingerprint) {
    this._draftId = String(data.id)
    this._draftClientId = String(data.draftClientId || this._draftClientId)
    this._draftVersion = Number(data.draftVersion || this._draftVersion || 1)
    this._lastSnapshot = JSON.stringify(this._savedSnapshot(data))
    this._view.draftStatus = ComposeEditor._SAVED
    this._view.draftConflict = false
    this._saveAgain = this._saveAgain || (data.recovered === true && this._lastSnapshot !== fingerprint)
    this._publishDraftSaved()
    await discardOfflineWrites(this._owner, `draft:${this._draftClientId}`)
    window.dispatchEvent(new Event('caduceus:refresh-inbox'))
  }

  _publishDraftSaved() {
    /** @type {DraftSaveFinished} */
    const finished = { status: 'saved', draftId: this._draftId, draftClientId: this._draftClientId, draftVersion: this._draftVersion, message: '' }
    this._view.publishDraftSaveFinished(finished)
  }

  _finishSave() {
    this._saving = false
    this._rewriteEditor()
    this._repeatSave(this._saveAgain)
  }

  /** @param {boolean} again */
  _repeatSave(again) {
    if (!again) return
    this._saveAgain = false
    this._requestSave(0)
  }

  /** @param {any} detail */
  _onOfflineWrite(detail) {
    if (detail?.kind !== ComposeEditor._DRAFT || detail?.metadata?.clientId !== this._draftClientId) return
    this._offlineWriteSettled(detail)
  }

  /** @param {any} detail */
  _offlineWriteSettled(detail) {
    switch (true) {
      case detail.status === 'conflict':
        this._view.draftConflict = true
        this._view.draftStatus = ComposeEditor._CONFLICT
        break
      case detail.status === 'accepted' && Boolean(detail.response?.id):
        this._draftId = String(detail.response.id)
        this._draftVersion = Number(detail.response.draftVersion || this._draftVersion || 1)
        this._lastSnapshot = JSON.stringify(this._savedSnapshot(detail.response))
        this._view.draftConflict = false
        this._view.draftStatus = ComposeEditor._SAVED
        this._publishDraftSaved()
        window.dispatchEvent(new Event('caduceus:refresh-inbox'))
        break
    }
    void this._render()
  }

  _onVisibility() {
    if (document.visibilityState === 'hidden') this._publishSaveDue(0)
  }

  /** @param {string} action */
  _shortcut(action) {
    /** @type {ComposeShortcut} */
    const shortcut = { action }
    this._view.publishComposeShortcut(shortcut)
  }

  /** Confirm sending with empty template blanks, as Mailspring does. */
  _blanksAccepted() {
    if (!hasEmptyTemplateVariables(this._currentHTML())) return true
    return confirm('The message contains an empty template area. Send anyway?')
  }

  /**
   * @param {SendRequested} requested
   * @param {boolean} accepted
   * @returns {Promise<SendFinished>}
   */
  async _deliver(requested, accepted) {
    if (!accepted) return this._declinedBlanks(requested.mode)
    this._sound(requested.mode, 'hit-send')
    const request = this._sendRequest(requested)
    const outcome = await this._attempt(request.path, request.method, request.body)
    return await this._sendOutcome(requested, request, outcome)
  }

  /** @param {string} mode @returns {SendFinished} */
  _declinedBlanks(mode) {
    const first = document.querySelector('[data-compose-editor] code.empty[data-template-var]')
    if (first) selectTemplateVariable(first)
    return { mode, status: 'cancelled', undoId: '', archived: false, message: 'Fill every highlighted template blank before sending.', sendAt: '' }
  }

  /**
   * @param {string} mode
   * @param {string} sound
   */
  _sound(mode, sound) {
    if (mode !== ComposeEditor._SEND && mode !== ComposeEditor._ARCHIVE) return
    /** @type {SoundRequested} */
    const requested = { sound }
    this._view.publishSoundRequested(requested)
  }

  /**
   * Build the clean send request. The server adds per-recipient tracking at
   * the SMTP boundary; drafts and Sent copies never hold tracking URLs.
   * @param {SendRequested} requested
   */
  _sendRequest(requested) {
    const envelope = requested.envelope
    const body = { ...this._payload(envelope), ...this._modeFields(requested) }
    return requested.mode === ComposeEditor._SCHEDULE_EDIT
      ? { path: `/send/scheduled/${requested.scheduledId}`, method: 'PUT', body }
      : { path: '/send', method: 'POST', body }
  }

  /** @param {MessageEnvelope} envelope */
  _payload(envelope) {
    const html = stripTemplateVariableMarkup(this._currentHTML())
    const attachments = Array.from(this._attachments.values())
    return {
      from: envelope.from, to: envelope.to, cc: envelope.cc, bcc: envelope.bcc,
      ...(envelope.replyTo.length ? { replyTo: [envelope.replyTo.join(', ')] } : {}),
      ...(envelope.inReplyTo ? { inReplyTo: envelope.inReplyTo } : {}),
      ...(envelope.references.length ? { references: envelope.references } : {}),
      subject: envelope.subject,
      ...(this._richText ? { html } : {}),
      text: this._currentText(),
      ...(attachments.length ? { attachments: attachments.map(({ filename, contentType, contentBase64, disposition, contentId }) => ({
        filename, contentType, contentBase64, disposition: disposition || ComposeEditor._ATTACHMENT, ...(contentId ? { contentId } : {}),
      })) } : {}),
      ...(envelope.openTracking ? { openTracking: true } : {}),
      ...(envelope.linkTracking ? { linkTracking: true } : {}),
      ...(envelope.remindAt ? { remindAt: envelope.remindAt } : {}),
    }
  }

  /** @param {SendRequested} requested */
  _modeFields(requested) {
    const envelope = requested.envelope
    switch (requested.mode) {
      case ComposeEditor._ARCHIVE:
        return { archive: true, archiveSourceId: envelope.sourceEmailId, delayMs: requested.delayMs, idempotencyKey: requested.idempotencyKey }
      case ComposeEditor._SCHEDULE_EDIT:
        return {
          sendAt: requested.sendAt, idempotencyKey: requested.idempotencyKey, remindAt: envelope.remindAt || null,
          replyTo: envelope.replyTo.length ? [envelope.replyTo.join(', ')] : null,
          openTracking: envelope.openTracking, linkTracking: envelope.linkTracking,
        }
      case 'schedule':
        return { sendAt: requested.sendAt, idempotencyKey: requested.idempotencyKey }
      default:
        return { delayMs: requested.delayMs, idempotencyKey: requested.idempotencyKey }
    }
  }

  /**
   * @param {SendRequested} requested
   * @param {{ path: string, method: string, body: any }} request
   * @param {{ network: boolean, ok: boolean, status: number, data: any }} outcome
   * @returns {Promise<SendFinished>}
   */
  async _sendOutcome(requested, request, outcome) {
    const scheduled = requested.mode.startsWith('schedule')
    const finished = { mode: requested.mode, status: '', undoId: '', archived: false, message: '', sendAt: requested.sendAt }
    switch (true) {
      case outcome.network:
        return { ...finished, ...await this._queueSend(requested, request) }
      case !outcome.ok:
        return { ...finished, status: 'failed', message: String(outcome.data?.error || (scheduled ? 'Schedule failed.' : 'Send failed.')) }
      default:
        this._sound(requested.mode, ComposeEditor._SEND)
        this._clearBody()
        return { ...finished, status: scheduled ? 'scheduled' : 'sent', undoId: String(outcome.data?.undoId || ''), archived: outcome.data?.archived === true }
    }
  }

  /**
   * @param {SendRequested} requested
   * @param {{ path: string, method: string, body: any }} request
   */
  async _queueSend(requested, request) {
    const editing = requested.mode === ComposeEditor._SCHEDULE_EDIT
    const status = await this._queued({
      kind: ComposeEditor._SEND, path: request.path, method: request.method, body: request.body,
      message: this._queuedSendMessage(requested.mode),
      coalesceKey: editing ? `scheduled:${requested.scheduledId}` : undefined,
      metadata: requested.mode.startsWith('schedule') ? { scheduled: true, editing } : { draftId: this._draftId, draftClientId: this._draftClientId },
    }, 'queued', 'failed')
    return this._queuedOutcome(status)
  }

  /** @param {string} status */
  _queuedOutcome(status) {
    if (status === 'queued') {
      this._clearBody()
      return { status }
    }
    return { status, message: 'Network error.' }
  }

  /** @param {string} mode */
  _queuedSendMessage(mode) {
    return {
      [ComposeEditor._ARCHIVE]: 'Send and archive message',
      schedule: 'Scheduled send',
      [ComposeEditor._SCHEDULE_EDIT]: 'Scheduled-send update',
    }[mode] ?? 'Send message'
  }

  _clearBody() {
    this._html = ''
    this._attachments.clear()
    const editor = this._editor()
    if (editor) clearEditorContent(editor)
  }

  /**
   * Leave the composer: forget the recovery record when the draft is done,
   * hand the undo-send offer to the shell, then navigate.
   * @param {NavigationRequested} requested
   */
  async _leave(requested) {
    await this._finishDraft(requested.clearRecovery)
    await this._queueDraftDelete(requested.discardDraft)
    this._offerUndo(requested)
    this._announce(requested.toast, requested.toastDate)
    this._navigate(requested.route)
  }

  /** @param {boolean} done */
  async _finishDraft(done) {
    if (!done) return
    this._exit = true
    clearTimeout(this._saveTimer)
    await this._clearRecovery()
    await discardOfflineWrites(this._owner, `draft:${this._draftClientId}`)
  }

  /** @param {string} id */
  async _queueDraftDelete(id) {
    if (!id) return
    await this._queued({
      kind: ComposeEditor._DRAFT, path: `/drafts/${id}`, method: 'DELETE', body: null,
      message: 'Discard sent draft', coalesceKey: `draft-delete:${id}`, metadata: { draftId: id },
    }, '', '')
  }

  /** @param {NavigationRequested} requested */
  _offerUndo(requested) {
    if (!requested.undoId) {
      this._announce(requested.undoLabel.replace('Undo', '').trim(), '')
      return
    }
    sessionStorage.setItem(ComposeEditor._UNDO, JSON.stringify({
      undoId: requested.undoId, label: requested.undoLabel, owner: this._owner, expiresAt: Date.now() + requested.undoDelay,
    }))
    window.dispatchEvent(new Event('caduceus:pending-send-undo'))
  }

  /**
   * @param {string} message
   * @param {string} date
   */
  _announce(message, date) {
    if (message) this._toast(message + (date ? formatPreferenceDate(date) : ''))
  }

  /** @param {string} message */
  _toast(message) {
    /** @type {ToastRequested} */
    const toast = { message }
    this._view.publishToastRequested(toast)
  }

  /** @param {string} route */
  _navigate(route) {
    if (route) location.assign(`/${route}`)
  }

  /** @param {boolean} confirmNeeded */
  _templateAllowed(confirmNeeded) {
    const html = this._currentHTML()
    const editable = templateBodyFromDraft(html) !== '' && this._currentText().trim() !== ''
    if (!confirmNeeded && !editable) return true
    return confirm('Loading a template will replace the editable draft contents. Signature and quoted mail will be kept. Continue?')
  }

  /**
   * @param {TemplateInsertRequested} requested
   * @param {boolean} allowed
   */
  _insertTemplate(requested, allowed) {
    this._resetTemplatePicker()
    if (!allowed) return
    this._html = insertTemplateBeforeProtectedContent(requested.text, this._currentHTML())
    this._rewriteEditor()
    requestAnimationFrame(() => this._selectFirstVariable())
    /** @type {TemplateInserted} */
    const inserted = { templateId: requested.templateId }
    this._view.publishTemplateInserted(inserted)
  }

  _resetTemplatePicker() {
    const picker = /** @type {HTMLInputElement | null} */ (document.querySelector('.compose-template-select'))
    if (picker) picker.value = ''
  }

  _selectFirstVariable() {
    const first = document.querySelector('[data-compose-editor] code.var, [data-compose-editor] code[data-template-var]')
    if (first) selectTemplateVariable(first)
  }

  /**
   * @param {string} content
   * @param {string} suggestedName
   */
  _offerTemplate(content, suggestedName) {
    if (!content) {
      this._toast('Add message content before saving a template.')
      return
    }
    this._publishTemplate(String(prompt('Template name:', suggestedName) ?? '').trim(), content)
  }

  /**
   * @param {string} name
   * @param {string} text
   */
  _publishTemplate(name, text) {
    if (!name) return
    /** @type {TemplateDraftReady} */
    const ready = { name, text }
    this._view.publishTemplateDraftReady(ready)
  }

  /** @param {HTMLInputElement} input */
  _inputFiles(input) {
    const files = Array.from(input.files ?? [])
    input.value = ''
    return files
  }

  /** @param {DataTransfer | null} transfer */
  _transferFiles(transfer) {
    const files = Array.from(transfer?.files ?? [])
    if (files.length) return files
    return Array.from(transfer?.items ?? []).map(item => item.getAsFile()).filter(file => file instanceof File)
  }

  /** @param {File[]} files */
  _offerMixed(files) {
    this._offerFiles([...files.filter(isInlineImageFile), ...files.filter(file => !isInlineImageFile(file))], null)
  }

  /**
   * Ask the Go service which files pass the attachment limits.
   * @param {File[]} files
   * @param {boolean | null} inline
   */
  _offerFiles(files, inline) {
    if (!files.length) return
    this._offered.clear()
    /** @type {AttachmentsOffered} */
    const offered = { files: files.map((file, index) => this._offer(file, index, inline)) }
    this._view.publishAttachmentsOffered(offered)
  }

  /**
   * @param {File} file
   * @param {number} index
   * @param {boolean | null} inline
   */
  _offer(file, index, inline) {
    this._offered.set(index, file)
    return { index, filename: file.name, contentType: file.type || 'application/octet-stream', size: file.size, inline: inline ?? isInlineImageFile(file) }
  }

  /** @param {AttachmentAcceptance[]} accepted */
  async _readAccepted(accepted) {
    const added = []
    for (const acceptance of accepted) added.push(...await this._readOne(acceptance, this._offered.get(acceptance.index)))
    return added
  }

  /**
   * @param {AttachmentAcceptance} acceptance
   * @param {File | undefined} file
   */
  async _readOne(acceptance, file) {
    try {
      const attachment = {
        id: acceptance.id, filename: file?.name, contentType: file?.type || 'application/octet-stream', size: file?.size,
        contentBase64: await this._base64(/** @type {File} */ (file)),
        disposition: acceptance.contentId ? ComposeEditor._INLINE : ComposeEditor._ATTACHMENT,
        ...(acceptance.contentId ? { contentId: acceptance.contentId } : {}),
      }
      this._attachments.set(attachment.id, attachment)
      return [attachment]
    } catch {
      this._toast(`${file?.name} could not be read.`)
      return []
    }
  }

  /** @param {File} file @returns {Promise<string>} */
  _base64(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result).split(',').pop() ?? '')
      reader.onerror = () => reject(reader.error)
      reader.readAsDataURL(file)
    })
  }

  /** @param {Record<string, any>[]} added */
  _insertInlineImages(added) {
    const editor = this._editor()
    for (const attachment of editor ? added.filter(item => item.contentId) : []) insertInlineImage(/** @type {HTMLElement} */ (editor), attachment)
    this._html = this._currentHTML()
  }

  /** @param {string} contentId */
  _removeInlineImages(contentId) {
    for (const image of contentId ? Array.from(this._editor()?.querySelectorAll('img[data-caduceus-cid]') ?? []) : []) {
      this._removeImageWithCid(/** @type {HTMLElement} */ (image), contentId)
    }
    this._html = this._currentHTML()
  }

  /**
   * @param {HTMLElement} image
   * @param {string} contentId
   */
  _removeImageWithCid(image, contentId) {
    if (image.dataset.caduceusCid === contentId) image.remove()
  }

  /**
   * @param {DragEvent} event
   * @param {boolean} files
   */
  _acceptDrag(event, files) {
    if (!files) return
    event.preventDefault()
    this._view.dragActive = true
  }

  /**
   * @param {Node | null} current
   * @param {Node | null} related
   */
  _leaveDrag(current, related) {
    this._view.dragActive = Boolean(current && related && current.contains(related))
  }

  /**
   * @param {ClipboardEvent} event
   * @param {File[]} files
   */
  _paste(event, files) {
    switch (true) {
      case files.length > 0:
        event.preventDefault()
        this._offerMixed(files)
        break
      case !this._richText:
        this._pastePlain(event, String(event.clipboardData?.getData('text/plain') ?? ''))
        break
      default:
        this._pasteRich(event, String(event.clipboardData?.getData('text/html') ?? ''), String(event.clipboardData?.getData('text/plain') ?? ''))
    }
  }

  /**
   * @param {ClipboardEvent} event
   * @param {string} text
   */
  _pastePlain(event, text) {
    const selection = getSelection()
    if (!text || !selection?.rangeCount) return
    event.preventDefault()
    const range = selection.getRangeAt(0)
    range.deleteContents()
    const node = document.createTextNode(text)
    range.insertNode(node)
    range.setStartAfter(node)
    range.collapse(true)
    selection.removeAllRanges()
    selection.addRange(range)
    this._editorChanged()
  }

  /**
   * @param {ClipboardEvent} event
   * @param {string} html
   * @param {string} text
   */
  _pasteRich(event, html, text) {
    const editor = this._editor()
    if ((!html && !text) || !editor) return
    event.preventDefault()
    this._changedWhen(insertPastedComposerContent(editor, html, text))
  }

  /** @param {boolean} changed */
  _changedWhen(changed) {
    if (changed) this._editorChanged()
  }

  /** @param {string} tag */
  _format(tag) {
    const editor = this._restoreSelection()
    this._changedWhen(Boolean(editor) && applyInlineFormat(/** @type {HTMLElement} */ (editor), tag))
  }

  /** @param {boolean} ordered */
  _list(ordered) {
    const editor = this._restoreSelection()
    this._changedWhen(Boolean(editor) && applyListFormat(/** @type {HTMLElement} */ (editor), ordered))
  }

  /** @param {string} tag */
  _block(tag) {
    const editor = ['p', 'h1', 'h2', 'h3', 'blockquote', 'pre'].includes(tag) ? this._restoreSelection() : null
    this._changedWhen(Boolean(editor) && applyBlockFormat(/** @type {HTMLElement} */ (editor), tag))
  }

  /**
   * @param {string} property
   * @param {string} value
   * @param {string[]} allowed
   */
  _styled(property, value, allowed) {
    const editor = allowed.includes(value) ? this._restoreSelection() : null
    this._changedWhen(Boolean(editor) && applyInlineFormat(/** @type {HTMLElement} */ (editor), 'span', { [property]: value }))
  }

  /** @param {string} value */
  _color(value) {
    const editor = /^#[0-9a-f]{6}$/i.test(value) ? this._restoreSelection() : null
    this._changedWhen(Boolean(editor) && applyInlineFormat(/** @type {HTMLElement} */ (editor), 'span', { color: value }))
  }

  /** @param {HTMLElement | null} editor */
  _direction(editor) {
    if (!editor) return
    const current = getSelection()?.anchorNode?.parentElement?.closest?.('[dir]')
    const tag = current?.tagName?.toLowerCase() ?? 'p'
    this._changedWhen(applyBlockFormat(editor, tag, { dir: current?.getAttribute('dir') === 'rtl' ? '' : 'rtl' }))
  }

  /** @param {HTMLElement | null} editor */
  _askLink(editor) {
    if (editor) this._insertLink(editor, normalizeComposeLink(String(prompt('Enter a web or email URL:') ?? '')))
  }

  /**
   * @param {HTMLElement} editor
   * @param {string} url
   */
  _insertLink(editor, url) {
    switch (true) {
      case !url:
        this._toast('Use an http, https, or mailto link.')
        return
      case applyInlineFormat(editor, 'a', {}):
        this._linkSelection(editor, url)
    }
  }

  /**
   * @param {HTMLElement} editor
   * @param {string} url
   */
  _linkSelection(editor, url) {
    const anchor = /** @type {HTMLAnchorElement | null | undefined} */ (getSelection()?.anchorNode?.parentElement?.closest?.('a'))
    if (anchor && editor.contains(anchor)) Object.assign(anchor, { href: url, rel: 'noopener noreferrer', target: '_blank' })
    this._editorChanged()
  }

  /** @returns {HTMLElement | null} */
  _restoreSelection() {
    const editor = this._editor()
    editor?.focus()
    this._reapplyRange(editor, this._savedRange)
    return editor
  }

  /**
   * @param {HTMLElement | null} editor
   * @param {Range | null} range
   */
  _reapplyRange(editor, range) {
    if (!editor || !range || !editor.contains(range.commonAncestorContainer)) return
    const selection = getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
  }

  /** @param {string} selector */
  _click(selector) {
    /** @type {HTMLInputElement | null} */ (document.querySelector(selector))?.click()
  }

  /** @param {HTMLElement | null} editor */
  _renderQuote(editor) {
    const quote = editor?.querySelector(ComposeEditor._QUOTE)
    this._view.quotePresent = Boolean(quote)
    editor?.classList.toggle(ComposeEditor._COLLAPSED, this._view.quotePresent && !this._view.quoteExpanded)
    editor?.toggleAttribute(ComposeEditor._QUOTE_EXPANDED, this._view.quotePresent && this._view.quoteExpanded)
  }

  /** @param {HTMLElement | null} editor */
  _removeQuote(editor) {
    const quote = editor?.querySelector(ComposeEditor._QUOTE)
    if (!quote) return
    quote.remove()
    this._view.quoteExpanded = false
    this._editorChanged()
  }

  /** @param {HTMLImageElement | null} image */
  _selectImage(image) {
    this._selectedImage?.removeAttribute(ComposeEditor._SELECTED)
    this._selectedImage = image
    this._selectedImageId = image?.dataset.caduceusAttachmentId ?? ''
    image?.setAttribute(ComposeEditor._SELECTED, ComposeEditor._TRUE)
    this._view.imageSelected = Boolean(image)
    this._view.selectedImageName = image?.alt || 'Inline image'
  }

  /**
   * @param {HTMLImageElement | null} image
   * @param {number} width
   */
  _resizeImage(image, width) {
    this._changedWhen(Boolean(image) && resizeInlineImage(image, width))
  }

  /** @param {HTMLImageElement | null} image */
  _removeSelectedImage(image) {
    if (!image) return
    const id = String(image.dataset.caduceusAttachmentId ?? '')
    this._selectImage(null)
    this._attachments.delete(id)
    image.remove()
    this._editorChanged()
    /** @type {InlineImageRemoved} */
    const removed = { id }
    this._view.publishInlineImageRemoved(removed)
  }

  /**
   * @param {HTMLElement | null} editor
   * @param {boolean} busy
   */
  async _translate(editor, busy) {
    switch (true) {
      case busy:
        return
      case !editor || !editor.innerText.trim():
        this._translationProblem('Write a message before translating it.')
        return
      default:
        await this._runTranslation(/** @type {HTMLElement} */ (editor))
    }
  }

  /** @param {string} message */
  _translationProblem(message) {
    this._view.translationStatus = message
    this._view.translationError = true
  }

  /** @param {HTMLElement} editor */
  async _runTranslation(editor) {
    this._view.translationBusy = true
    this._translationProgress(0)
    await this._render()
    try {
      const { translateHtml } = await import('./on-device-translation.js')
      const result = await translateHtml(editor.innerHTML, { targetLanguage: this._view.translationTarget, onProgress: loaded => this._translationProgress(loaded) })
      this._translated(result)
    } catch (error) {
      this._translationProblem(error instanceof Error ? error.message : 'Translation failed.')
    }
    this._view.translationBusy = false
  }

  /** @param {number} loaded */
  _translationProgress(loaded) {
    this._view.translationError = false
    this._view.translationStatus = loaded > 0 && loaded < 1 ? `Downloading language model… ${Math.round(loaded * 100)}%` : 'Translating on this device…'
  }

  /** @param {{ html: string, sourceLanguage: string, targetLanguage: string }} result */
  _translated(result) {
    const languages = /** @type {Record<string, string>} */ (TRANSLATION_LANGUAGES)
    this._html = result.html
    this._rewriteEditor()
    this._view.translationStatus = `Translated from ${languages[result.sourceLanguage] ?? 'detected language'} to ${languages[result.targetLanguage] ?? 'the selected language'}.`
    /** @type {EditorSignatureChanged} */
    const changed = { signatureId: currentSignatureId(result.html) }
    this._view.publishEditorSignatureChanged(changed)
    this._requestSave(0)
    this._scheduleGrammar(0)
    this._editor()?.focus()
  }

  /** @param {Event} event */
  _onInput(event) {
    const target = /** @type {HTMLElement} */ (event.target)
    const inPanel = event.isTrusted && Boolean(target?.closest?.(ComposeEditor._PANEL))
    if (!inPanel) return
    this._routeInput(target)
  }

  /** @param {HTMLElement} target */
  _routeInput(target) {
    const editor = /** @type {HTMLElement | null} */ (target.closest(ComposeEditor._EDITOR))
    this._recipientTyped(target)
    this._editorTyped(editor)
    this._requestSave(ComposeEditor._SAVE_DELAY)
  }

  /** @param {HTMLElement | null} editor */
  _editorTyped(editor) {
    if (!editor) return
    this._applyInlineShortcut(editor, this._richText && applyInlineMarkdownShortcut(editor))
    this._html = this._currentHTML()
    this._decorate(editor)
    this._scheduleGrammar(ComposeEditor._GRAMMAR_DELAY)
  }

  /**
   * @param {HTMLElement} editor
   * @param {boolean} applied
   */
  _applyInlineShortcut(editor, applied) {
    if (applied) this._requestSave(0)
  }

  /** @param {number} delay */
  _scheduleGrammar(delay) {
    clearTimeout(this._grammarTimer)
    if (this._composing) return
    this._grammarTimer = setTimeout(() => this._publishGrammarText(), delay)
  }

  _publishGrammarText() {
    /** @type {GrammarTextChanged} */
    const changed = { text: grammarText(this._editor()) }
    this._view.publishGrammarTextChanged(changed)
  }

  /**
   * @param {Event} event
   * @param {boolean} starting
   */
  _composition(event, starting) {
    if (!/** @type {HTMLElement} */ (event.target)?.closest?.(ComposeEditor._EDITOR)) return
    this._composing = starting
    this._compositionEnded(starting)
  }

  /** @param {boolean} starting */
  _compositionEnded(starting) {
    if (!starting) this._scheduleGrammar(ComposeEditor._GRAMMAR_COMPOSITION_DELAY)
  }

  /**
   * @param {HTMLElement | null} editor
   * @param {{ offset: number, length: number }[]} ranges
   */
  _paintGrammar(editor, ranges) {
    if (editor && ranges.length) paintGrammarHighlights(editor, ranges)
    else clearGrammarHighlights()
  }

  /**
   * @param {HTMLElement | null} editor
   * @param {GrammarReplacementRequested} requested
   */
  _replaceGrammar(editor, requested) {
    if (!editor || !replaceGrammarRange(editor, requested, requested.replacement)) return
    editor.focus()
    this._editorChanged()
    this._scheduleGrammar(0)
  }

  /**
   * @param {HTMLElement | null} editor
   * @param {number} offset
   * @param {number} length
   */
  _focusRange(editor, offset, length) {
    const range = editor ? grammarRange(editor, offset, length) : null
    if (!editor || !range) return
    const selection = document.getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
    editor.focus()
  }

  /** @param {Event} event @returns {string} */
  _recipientField(event) {
    const target = event.target
    const field = target instanceof HTMLInputElement ? target.closest(ComposeEditor._RECIPIENT_FIELD)?.getAttribute('data-recipient-field') : ''
    return ['to', 'cc', 'bcc'].includes(String(field)) ? String(field) : ''
  }

  /** @param {HTMLElement} target */
  _recipientTyped(target) {
    const field = this._recipientField(/** @type {Event} */ ({ target }))
    if (!field) return
    const value = String(/** @type {HTMLInputElement} */ (target).value)
    clearTimeout(this._recipientTimer)
    this._recipientTimer = setTimeout(() => this._publishRecipientQuery(field, value), ComposeEditor._RECIPIENT_DELAY)
  }

  /** @param {FocusEvent} event */
  _onFocusIn(event) {
    const field = this._recipientField(event)
    if (field) this._publishRecipientQuery(field, String(/** @type {HTMLInputElement} */ (event.target).value))
  }

  /**
   * @param {string} field
   * @param {string} value
   */
  _publishRecipientQuery(field, value) {
    /** @type {RecipientQueryChanged} */
    const changed = { field, value }
    this._view.publishRecipientQueryChanged(changed)
  }

  /** @param {FocusEvent} event */
  _onFocusOut(event) {
    if (!this._recipientField(event)) return
    setTimeout(() => this._dismissRecipients(Boolean(document.activeElement?.closest?.(ComposeEditor._RECIPIENT_FIELD))), ComposeEditor._BLUR_DELAY)
  }

  /** @param {boolean} stillInside */
  _dismissRecipients(stillInside) {
    if (stillInside) return
    /** @type {RecipientSuggestionsDismissed} */
    const dismissed = { field: this._recipient.field }
    this._view.publishRecipientSuggestionsDismissed(dismissed)
  }

  /** @param {Event} event */
  _keepRecipientFocus(event) {
    const option = event.target instanceof Element ? event.target.closest('[data-recipient-option]') : null
    if (option) event.preventDefault()
  }

  /** @param {string} field */
  _recipientInput(field) {
    return /** @type {HTMLInputElement | null} */ (document.querySelector(`[data-recipient-field="${field}"] input`))
  }

  _describeRecipientInputs() {
    for (const field of ['to', 'cc', 'bcc']) this._describeRecipientInput(this._recipientInput(field), field === this._recipient.field && this._recipient.count > 0)
  }

  /**
   * @param {HTMLInputElement | null} input
   * @param {boolean} expanded
   */
  _describeRecipientInput(input, expanded) {
    if (!input) return
    const list = `compose-${this._recipient.field}-suggestions`
    input.setAttribute(ComposeEditor._EXPANDED, String(expanded))
    input.setAttribute('role', 'combobox')
    input.setAttribute('aria-autocomplete', 'list')
    input.toggleAttribute('aria-controls', expanded)
    input.toggleAttribute('aria-activedescendant', expanded)
    input.setAttribute('aria-controls', list)
    input.setAttribute('aria-activedescendant', `${list}-${this._recipient.active}`)
  }

  /**
   * @param {HTMLInputElement | null} input
   * @param {string} value
   */
  _fillRecipientInput(input, value) {
    if (!input) return
    input.value = value
    input.focus()
  }

  /** @param {KeyboardEvent} event */
  _onKeydown(event) {
    const target = /** @type {HTMLElement} */ (event.target)
    const command = (event.ctrlKey || event.metaKey) && !event.altKey
    const inComposer = Boolean(target?.closest?.(ComposeEditor._PANEL))
    this._dispatchKey(event, target, command && inComposer)
  }

  /**
   * @param {KeyboardEvent} event
   * @param {HTMLElement} target
   * @param {boolean} command
   */
  _dispatchKey(event, target, command) {
    switch (true) {
      case command && event.shiftKey && this._focusField(event, event.key.toLowerCase()):
        return
      case command && event.key === 'Enter':
        event.preventDefault()
        this._shortcut(ComposeEditor._SEND)
        return
      case Boolean(target?.closest?.('[role="toolbar"][data-compose-toolbar]')):
        this._toolbarKey(event, /** @type {HTMLElement} */ (target.closest('[data-compose-toolbar]')))
        return
      case Boolean(this._recipientField(event)):
        this._recipientKey(event)
        return
      default:
        this._editorKey(event, /** @type {HTMLElement | null} */ (target?.closest?.(ComposeEditor._EDITOR)))
    }
  }

  /**
   * Ctrl+Shift+T, C, B and F focus To, CC, BCC and From.
   * @param {KeyboardEvent} event
   * @param {string} key
   */
  _focusField(event, key) {
    const selector = /** @type {Record<string, string>} */ ({ t: '[data-compose-to]', c: '[data-compose-cc]', b: '[data-compose-bcc]', f: '[data-compose-from]' })[key]
    const host = selector ? document.querySelector(selector) : null
    const control = host?.matches('input, select') ? host : host?.querySelector('input, select')
    if (!selector) return false
    event.preventDefault()
    /** @type {HTMLElement | null | undefined} */ (control)?.focus()
    return true
  }

  /**
   * Roving focus through the formatting toolbar.
   * @param {KeyboardEvent} event
   * @param {HTMLElement} toolbar
   */
  _toolbarKey(event, toolbar) {
    const controls = /** @type {HTMLElement[]} */ (Array.from(toolbar.querySelectorAll('button:not([hidden]):not([disabled]), select:not([hidden]):not([disabled]), input:not([hidden]):not([disabled])')))
    const current = controls.indexOf(/** @type {HTMLElement} */ (event.target))
    const next = { Home: 0, End: controls.length - 1, ArrowRight: current + 1, ArrowLeft: current - 1 }[event.key]
    if (next === undefined || current < 0) return
    event.preventDefault()
    const index = (next + controls.length) % controls.length
    controls.forEach((control, position) => control.tabIndex = position === index ? 0 : -1)
    controls[index].focus()
  }

  /** @param {KeyboardEvent} event */
  _recipientKey(event) {
    const open = this._recipient.count > 0 && this._recipientField(event) === this._recipient.field
    const handled = event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Escape' || (event.key === 'Enter' && this._recipient.active >= 0)
    if (open && handled) event.preventDefault()
  }

  /**
   * Markdown shortcuts, auto-links and formatting keys inside the editor.
   * @param {KeyboardEvent} event
   * @param {HTMLElement | null} editor
   */
  _editorKey(event, editor) {
    if (!editor || event.isComposing || !this._richText) return
    this._templateTab(event, editor)
    this._markdownKey(event, editor)
    this._formattingKey(event, (event.ctrlKey || event.metaKey) && !event.altKey)
  }

  /**
   * @param {KeyboardEvent} event
   * @param {HTMLElement} editor
   */
  _templateTab(event, editor) {
    const plainTab = event.key === 'Tab' && !event.altKey && !event.ctrlKey && !event.metaKey
    if (plainTab && moveBetweenTemplateVariables(editor, event.shiftKey ? -1 : 1)) event.preventDefault()
  }

  /**
   * @param {KeyboardEvent} event
   * @param {HTMLElement} editor
   */
  _markdownKey(event, editor) {
    switch (true) {
      case event.key === ' ' && applyMarkdownShortcut(editor):
        event.preventDefault()
        this._editorChanged()
        break
      case (event.key === ' ' || event.key === 'Enter') && autoLinkBeforeCaret(editor):
        this._html = serializeComposerHTML(editor)
        break
    }
  }

  /**
   * @param {KeyboardEvent} event
   * @param {boolean} command
   */
  _formattingKey(event, command) {
    const key = `${event.shiftKey ? 'shift+' : ''}${event.key.toLowerCase()}`
    const action = /** @type {Record<string, () => void>} */ ({
      b: () => this.bold(), i: () => this.italic(), u: () => this.underline(), k: () => this.link(),
      'shift+x': () => this.strike(), 'shift+7': () => this.numberedList(), 'shift+8': () => this.bulletList(),
    })[key]
    if (!command || !action) return
    event.preventDefault()
    action()
  }

  _onSelection() {
    const selection = getSelection()
    const range = selection?.rangeCount ? selection.getRangeAt(0) : null
    const editor = this._editor()
    if (!range || !editor?.contains(range.commonAncestorContainer)) return
    this._savedRange = range.cloneRange()
    this._pressCommands(selection?.anchorNode ?? null)
  }

  /** @param {Node | null} anchor */
  _pressCommands(anchor) {
    const element = /** @type {Element | null} */ (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement ?? null)
    for (const button of document.querySelectorAll('[data-compose-command]')) {
      button.setAttribute('aria-pressed', String(Boolean(element?.closest?.(String(button.getAttribute('data-compose-command'))))))
    }
  }

  /** Keep the editor selection while a formatting button is clicked. @param {MouseEvent} event */
  _keepToolbarSelection(event) {
    const target = /** @type {HTMLElement} */ (event.target)?.closest?.('[data-compose-format]')
    if (target && !target.matches('select, input')) event.preventDefault()
  }

  /** @param {MouseEvent} event */
  _onClick(event) {
    const target = /** @type {HTMLElement} */ (event.target)
    const image = /** @type {HTMLImageElement | null} */ (target?.closest?.(ComposeEditor._INLINE_IMAGE))
    this._clickImage(event, image, Boolean(target?.closest?.('[data-inline-image-controls]')))
  }

  /**
   * @param {MouseEvent} event
   * @param {HTMLImageElement | null} image
   * @param {boolean} onControls
   */
  _clickImage(event, image, onControls) {
    switch (true) {
      case Boolean(image):
        event.preventDefault()
        this._selectImage(image)
        break
      case !onControls:
        this._selectImage(null)
        break
    }
  }
}

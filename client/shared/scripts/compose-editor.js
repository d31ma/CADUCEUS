// @ts-check
/** @import { ComposeLoaded } from '/models/ComposeLoaded.js' */
/** @import { ComposeChanged } from '/models/ComposeChanged.js' */
/** @import { ComposeAttachment } from '/models/ComposeAttachment.js' */
/** @import { ComposeEditorSnapshot } from '/models/ComposeEditorSnapshot.js' */
/** @import { EditorPreferences } from '/models/EditorPreferences.js' */
/** @import { EditorReplaced } from '/models/EditorReplaced.js' */
/** @import { SignatureApplyRequested } from '/models/SignatureApplyRequested.js' */
/** @import { EditorSignatureChanged } from '/models/EditorSignatureChanged.js' */
/** @import { TemplateInsertRequested } from '/models/TemplateInsertRequested.js' */
/** @import { TemplateInserted } from '/models/TemplateInserted.js' */
/** @import { TemplateDraftRequested } from '/models/TemplateDraftRequested.js' */
/** @import { TemplateDraftReady } from '/models/TemplateDraftReady.js' */
/** @import { AttachmentsOffered } from '/models/AttachmentsOffered.js' */
/** @import { AttachmentsAccepted } from '/models/AttachmentsAccepted.js' */
/** @import { AttachmentAcceptance } from '/models/AttachmentAcceptance.js' */
/** @import { AttachmentsAdded } from '/models/AttachmentsAdded.js' */
/** @import { AttachmentRemoved } from '/models/AttachmentRemoved.js' */
/** @import { InlineImageRemoved } from '/models/InlineImageRemoved.js' */
/** @import { RecipientQueryChanged } from '/models/RecipientQueryChanged.js' */
/** @import { RecipientSuggestionsDismissed } from '/models/RecipientSuggestionsDismissed.js' */
/** @import { RecipientSuggestionsShown } from '/models/RecipientSuggestionsShown.js' */
/** @import { RecipientChosen } from '/models/RecipientChosen.js' */
/** @import { GrammarTextChanged } from '/models/GrammarTextChanged.js' */
/** @import { GrammarTextWanted } from '/models/GrammarTextWanted.js' */
/** @import { GrammarHighlightsChanged } from '/models/GrammarHighlightsChanged.js' */
/** @import { GrammarReplacementRequested } from '/models/GrammarReplacementRequested.js' */
/** @import { GrammarFocusRequested } from '/models/GrammarFocusRequested.js' */
/** @import { ShortcutPressed } from '/models/ShortcutPressed.js' */
/** @import { SendRequested } from '/models/SendRequested.js' */
/** @import { SendBodyReady } from '/models/SendBodyReady.js' */
/** @import { DraftSaveDue } from '/models/DraftSaveDue.js' */
/** @import { ToastRequested } from '/models/ToastRequested.js' */
/** @import { SoundRequested } from '/models/SoundRequested.js' */

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
import { prepareComposerFrame, templateVariableKeys } from './composer-frame.js'
/** @import { QuoteProof } from '/models/QuoteProof.js' */
/** @import { MatchedQuote } from '/models/MatchedQuote.js' */
import { matchQuoteProof } from './composer-quote-proof.js'
/** @import { SafeHtmlFramePackage } from '../../models/SafeHtmlFramePackage.js' */
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
  applyStyleCommand,
  autoLinkBeforeCaret,
  insertInlineImage,
  insertPastedComposerContent,
  isInlineImageFile,
  linkSelection,
  normalizeComposeLink,
  resizeInlineImage,
  serializeComposerHTML,
} from './rich-compose-editor.js'
import { formatPreferenceDate } from './mail-preferences.js'
import { playNotificationSound } from './notification-sounds.js'

/**
 * The preferred translation target, like preferredTranslationLanguage in
 * on-device-translation.js. That module (and its language detector) loads
 * only when the person asks for a translation.
 */
export function preferredTranslationTarget() {
  const primary = String(navigator.language || 'en').trim().toLowerCase().split('-')[0]
  return Object.hasOwn(TRANSLATION_LANGUAGES, primary) ? primary : 'en'
}

/**
 * The browser-only half of the composer, used by ComposeEditorService.
 *
 * The message body is the contenteditable body of the editor frame, whose
 * document the Go ComposeService builds (EditorUtils.go). This module edits it
 * with the browser's own editing commands, reads files, asks the confirm and
 * prompt questions, runs the debounce timers and the on-device translation,
 * paints grammar highlights, and hands the body to Go in events. It sends no
 * request, stores nothing and decides nothing about the message: Go owns the
 * fields, the drafts, the sending and the navigation.
 */
export class ComposeEditor {
  /** @param {any} view the ComposeEditorService whose fields the view reads */
  constructor(view) {
    this._view = view
  }

  static _FRAME = '[data-compose-editor-frame]'
  static _PANEL = '.compose-panel'
  static _RECIPIENT_FIELD = '[data-recipient-field]'
  static _PREPARE_ERROR = 'The message editor could not prepare safely. Reload to retry.'
  static _TRANSLATION_TARGET = 'caduceus-translation-target'
  static _SAVE_DELAY = 750
  static _GRAMMAR_DELAY = 700
  static _GRAMMAR_COMPOSITION_DELAY = 300
  static _RECIPIENT_DELAY = 140
  static _BLUR_DELAY = 120
  /** Room for one event: the transport carries 64 KiB, JSON escapes included. */
  static _LARGEST_EVENT = 60 * 1024
  static _QUOTE = '[data-caduceus-quote]'
  static _INLINE_IMAGE = 'img[data-caduceus-inline-image]'
  static _SELECTED = 'data-selected'
  static _EXPANDED = 'aria-expanded'
  static _COLLAPSED = 'compose-quote-collapsed'

  /** @type {SafeHtmlFramePackage | null} */
  _safeFrame = null
  _preparingRevision = -1
  /** @type {Promise<void>|null} */
  _framePreparation = null
  _stopped = false
  _saveCapture = 0
  /** @type {QuoteProof|null} */
  _quoteProof = null
  /** Content keys originate only in the template service’s catalog-backed insertion event. @type {Set<string>} */
  _templateVariables = new Set()

  _ready = false
  _opened = false
  _richText = true
  _spellcheck = true
  _spellcheckLanguage = ''
  /** The body Go last handed over, used until the frame shows it. */
  _html = ''
  /** True between a whole-body replacement and the frame's next load. */
  _stale = false
  _savePending = false
  /** @type {HTMLIFrameElement | null} */
  _frame = null
  /** @type {HTMLElement | null} */
  _body = null
  /** @type {SignatureApplyRequested | null} */
  _pendingSignature = null
  /** @type {Array<() => void>} */
  _afterLoad = []
  /** @type {Map<number, File>} */
  _offered = new Map()
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _saveTimer
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _grammarTimer
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _recipientTimer
  _composing = false
  _recipient = { field: '', count: 0, active: -1 }
  /** @type {HTMLImageElement | null} */
  _selectedImage = null
  _selectedImageId = ''
  /** @type {AbortController | null} */
  _listeners = null
  /** @type {AbortController | null} */
  _frameListeners = null
  /** @type {MutationObserver | null} */
  _observer = null

  /**
   * Same-revision renders must preserve the live document and selection.
   * @param {ComposeEditorSnapshot} snapshot
   * @param {Array<{id:string,text:string}>} signatures
   * @returns {SafeHtmlFramePackage}
   */
  frame(snapshot, signatures) {
    if (!this._safeFrame || this._safeFrame.metadata.revision !== snapshot.revision) throw new Error('Editor frame has not completed safe preparation.')
    return this._safeFrame
  }

  /**
   * Complete proof validation before the first frame for a revision is published.
   * @param {ComposeEditorSnapshot} snapshot
   * @param {Array<{id:string,text:string}>} signatures
   * @returns {boolean}
   */
  frameReady(snapshot, signatures) {
    if (this._stopped) return false
    if (this._safeFrame?.metadata.revision === snapshot.revision) return true
    if (this._preparingRevision !== snapshot.revision) {
      this._preparingRevision = snapshot.revision
      this._view.editorError = ''
      this._framePreparation = prepareComposerFrame(snapshot, signatures, this._templateVariables).then(prepared => {
        if (this._stopped || this._preparingRevision !== snapshot.revision) return
        this._safeFrame = prepared.frame
        this._quoteProof = prepared.quoteProof
        this._rerender()
      }).catch(() => {
        if (this._stopped || this._preparingRevision !== snapshot.revision) return
        this._preparationFailed()
      })
    }
    return false
  }

  get translationLanguages() {
    return Object.entries(TRANSLATION_LANGUAGES).map(([id, label]) => ({ id, label }))
  }

  start() {
    this._stopped = false
    this._restoreTranslationTarget()
    this._bindDocument()
    this._observer = new MutationObserver(() => this._findFrame())
    this._observer.observe(document.body, { childList: true, subtree: true })
    this._findFrame()
  }

  stop() {
    this._stopped = true
    this._saveCapture++
    this._listeners?.abort()
    this._frameListeners?.abort()
    this._observer?.disconnect()
    clearTimeout(this._saveTimer)
    clearTimeout(this._grammarTimer)
    clearTimeout(this._recipientTimer)
  }

  /** @param {ComposeLoaded} loaded */
  composeLoaded(loaded) {
    this._html = loaded.html
    this._opened = true
    const pending = this._pendingSignature
    this._pendingSignature = null
    if (pending) this.signatureApplyRequested(pending)
  }

  /** @param {EditorPreferences} preferences */
  editorPreferences(preferences) {
    this._richText = preferences.richText
    this._spellcheck = preferences.spellcheck
    this._spellcheckLanguage = preferences.spellcheckLanguage
    this._applyPreferences()
  }

  /** @param {ComposeChanged} changed */
  composeChanged(changed) {
    if (changed.reason === 'ready') this._ready = true
    if (changed.reason === 'save-now') this._requestSave(0)
    if (changed.reason === 'save-soon') this._requestSave(ComposeEditor._SAVE_DELAY)
  }

  /** @param {SignatureApplyRequested} requested */
  signatureApplyRequested(requested) {
    if (!this._opened) {
      this._pendingSignature = requested
      return
    }
    const signature = requested.signatureId ? { id: requested.signatureId, text: requested.text } : null
    this._replaceBody(applySignature(this._currentHTML(), signature), 'signature')
  }

  /** @param {TemplateInsertRequested} requested */
  templateInsertRequested(requested) {
    if (!this._templateAllowed(requested.confirm)) return
    for (const key of templateVariableKeys(requested.text)) this._templateVariables.add(key)
    this._replaceBody(insertTemplateBeforeProtectedContent(requested.text, this._currentHTML()), 'template')
    this._afterLoad.push(() => this._selectFirstVariable())
    /** @type {TemplateInserted} */
    const inserted = { templateId: requested.templateId, name: requested.name, to: requested.to, cc: requested.cc, subject: requested.subject }
    this._view.publishTemplateInserted(inserted)
  }

  /** @param {TemplateDraftRequested} requested */
  templateDraftRequested(requested) {
    const content = templateBodyFromDraft(this._currentHTML())
    if (!content) {
      this._toast('Add message content before saving a template.')
      return
    }
    const name = String(prompt('Template name:', requested.suggestedName) ?? '').trim()
    if (!name) return
    /** @type {TemplateDraftReady} */
    const ready = { name, text: content }
    this._view.publishTemplateDraftReady(ready)
  }

  /** @param {AttachmentsAccepted} accepted */
  async attachmentsAccepted(accepted) {
    const added = []
    for (const acceptance of accepted.accepted) added.push(...await this._readOne(acceptance, this._offered.get(acceptance.index)))
    this._offered.clear()
    for (const attachment of added) if (attachment.contentId && this._live()) insertInlineImage(/** @type {HTMLElement} */ (this._body), attachment)
    /** @type {AttachmentsAdded} */
    const event = { attachments: added }
    if (added.length) this._view.publishAttachmentsAdded(event)
  }

  /** @param {AttachmentRemoved} removed */
  attachmentRemoved(removed) {
    for (const image of removed.contentId ? Array.from(this._body?.querySelectorAll('img[data-caduceus-cid]') ?? []) : []) {
      if (image.getAttribute('data-caduceus-cid') === removed.contentId) image.remove()
    }
  }

  /** @param {RecipientSuggestionsShown} shown */
  recipientSuggestionsShown(shown) {
    this._recipient = { field: shown.field, count: shown.count, active: shown.active }
    for (const field of ['to', 'cc', 'bcc']) this._describeRecipientInput(this._recipientInput(field), field === shown.field && shown.count > 0)
  }

  /** @param {RecipientChosen} chosen */
  recipientChosen(chosen) {
    const input = this._recipientInput(chosen.field)
    if (!input) return
    input.value = chosen.value
    input.focus()
  }

  /** @param {GrammarTextWanted} _wanted */
  grammarTextWanted(_wanted) {
    this._scheduleGrammar(0)
  }

  /** @param {GrammarHighlightsChanged} changed */
  grammarHighlightsChanged(changed) {
    if (this._body && changed.ranges.length) paintGrammarHighlights(this._body, changed.ranges)
    else clearGrammarHighlights(this._body)
  }

  /** @param {GrammarReplacementRequested} requested */
  grammarReplacementRequested(requested) {
    if (!this._body || !replaceGrammarRange(this._body, requested, requested.replacement)) return
    this._scheduleGrammar(0)
  }

  /** @param {GrammarFocusRequested} requested */
  grammarFocusRequested(requested) {
    const range = this._body ? grammarRange(this._body, requested.offset, requested.length) : null
    if (!this._body || !range) return
    const selection = this._body.ownerDocument.getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
    this._focusEditor()
  }

  /**
   * Hand Go the body of a send. A body with an empty template blank goes
   * only when the person confirms it, as in Mailspring.
   * @param {SendRequested} _requested
   * @returns {Promise<void>}
   */
  async sendRequested(_requested) {
    if (!this._live()) {
      this._afterLoad.push(() => { void this.sendRequested(_requested) })
      return
    }
    const html = this._currentHTML()
    const accepted = !hasEmptyTemplateVariables(html) || confirm('The message contains an empty template area. Send anyway?')
    if (!accepted) this._selectFirstVariable('code.empty[data-template-var]')
    const body = { html: stripTemplateVariableMarkup(html), text: this._currentText() }
    const revision = this._safeFrame?.metadata.revision
    const tooLarge = this._tooLarge(body)
    /** @type {MatchedQuote|null} */
    let proof = null
    try {
      proof = tooLarge ? null : await matchQuoteProof(body.html, this._quoteProof)
    } catch {
      if (this._stopped) return
      this._preparationFailed()
      /** @type {SendBodyReady} */
      const failed = {html:'',text:'',quoteProof:null,accepted:false,tooLarge:false}
      this._view.publishSendBodyReady(failed)
      return
    }
    if (this._stopped) return
    const unchanged = this._safeFrame?.metadata.revision === revision && this._live() && stripTemplateVariableMarkup(this._currentHTML()) === body.html && this._currentText() === body.text
    if (!unchanged) this._toast('The message changed while preparing. Send again.')
    /** @type {SendBodyReady} */
    const ready = { html: tooLarge ? '' : body.html, text: tooLarge ? '' : body.text, quoteProof:proof?.proof || null, accepted:accepted && unchanged, tooLarge }
    this._view.publishSendBodyReady(ready)
  }

  /** @param {SoundRequested} requested */
  soundRequested(requested) {
    void playNotificationSound(requested.sound)
  }

  /** @param {string} tag strong, em, u, s or code */
  format(tag) {
    this._changedWhen(Boolean(this._body) && applyInlineFormat(/** @type {HTMLElement} */ (this._body), tag))
  }

  /** @param {boolean} ordered */
  list(ordered) {
    this._changedWhen(Boolean(this._body) && applyListFormat(/** @type {HTMLElement} */ (this._body), ordered))
  }

  /** @param {string} kind 'attachment' or 'inline-image' */
  pick(kind) {
    /** @type {HTMLInputElement | null} */ (document.querySelector(`[data-${kind === 'inline-image' ? 'inline-image' : 'attachment'}-input]`))?.click()
  }

  /** Turn the selection into a link after asking for its address. */
  link() {
    if (!this._body) return
    const url = normalizeComposeLink(String(prompt('Enter a web or email URL:') ?? ''))
    if (!url) {
      this._toast('Use an http, https, or mailto link.')
      return
    }
    this._changedWhen(linkSelection(this._body, url))
  }

  /** @param {Event} event */
  changeBlockStyle(event) {
    const select = /** @type {HTMLSelectElement} */ (event.target)
    if (this._body && ['p', 'h1', 'h2', 'h3', 'blockquote', 'pre'].includes(select.value)) this._changedWhen(applyBlockFormat(this._body, select.value))
    select.value = 'p'
  }

  /** @param {Event} event */
  changeFontFamily(event) {
    this._style('fontName', /** @type {HTMLSelectElement} */ (event.target).value, ['Arial, sans-serif', 'Georgia, serif', 'Tahoma, sans-serif', 'Times New Roman, serif', 'Verdana, sans-serif', 'monospace'])
  }

  /** @param {Event} event */
  changeFontSize(event) {
    this._style('fontSize', /** @type {HTMLSelectElement} */ (event.target).value, ['2', '3', '5', '6'])
  }

  /** @param {Event} event */
  changeTextColor(event) {
    const value = /** @type {HTMLInputElement} */ (event.target).value
    this._style('foreColor', value, /^#[0-9a-f]{6}$/i.test(value) ? [value] : [])
  }

  toggleRightToLeft() {
    this._changedWhen(Boolean(this._body) && applyBlockFormat(/** @type {HTMLElement} */ (this._body), 'p', { dir: true }))
  }

  toggleQuotedText() {
    this._view.quoteExpanded = !this._view.quoteExpanded
    this._renderQuote()
  }

  removeQuotedText() {
    const quote = this._body?.querySelector(ComposeEditor._QUOTE)
    if (!quote) return
    quote.remove()
    this._view.quoteExpanded = false
    this._editorChanged()
  }

  /**
   * @param {Event} event
   * @param {string} kind 'attachment' or 'inline-image'
   */
  add(event, kind) {
    this._offerFiles(this._inputFiles(/** @type {HTMLInputElement} */ (event.target)), kind === 'inline-image')
  }

  /**
   * @param {Event} event
   * @param {string} kind block, font, size or color
   */
  style(event, kind) {
    const choices = /** @type {Record<string, (event: Event) => void>} */ ({
      block: changed => this.changeBlockStyle(changed), font: changed => this.changeFontFamily(changed),
      size: changed => this.changeFontSize(changed), color: changed => this.changeTextColor(changed),
    })
    choices[kind]?.(event)
  }

  /** @param {DragEvent} event */
  dragOver(event) {
    if (!Array.from(event.dataTransfer?.types ?? []).includes('Files')) return
    event.preventDefault()
    if (!this._view.dragActive) this._rerender()
    this._view.dragActive = true
  }

  /** @param {DragEvent} event */
  dragLeave(event) {
    const current = /** @type {Node | null} */ (event.currentTarget)
    const related = /** @type {Node | null} */ (event.relatedTarget)
    this._view.dragActive = Boolean(current && related && current.contains(related))
  }

  /** @param {DragEvent} event */
  drop(event) {
    event.preventDefault()
    this._view.dragActive = false
    this._offerMixed(this._transferFiles(event.dataTransfer))
    this._rerender()
  }

  /**
   * @param {Event} _event
   * @param {number} width
   */
  resizeSelectedInlineImage(_event, width) {
    this._changedWhen(resizeInlineImage(this._selectedImage, Number(width)))
  }

  removeSelectedInlineImage() {
    const image = this._selectedImage
    if (!image) return
    const id = String(image.getAttribute('data-caduceus-attachment-id') ?? '')
    this._selectImage(null)
    image.remove()
    this._editorChanged()
    /** @type {InlineImageRemoved} */
    const removed = { id }
    this._view.publishInlineImageRemoved(removed)
  }

  /** @param {Event} event */
  changeTranslationTarget(event) {
    this._keepTranslationTarget(String(/** @type {HTMLSelectElement} */ (event.target).value))
  }

  /** Replace the draft with an on-device translation the person asked for. */
  async translateDraft() {
    if (this._view.translationBusy) return
    if (!this._body?.innerText.trim()) {
      this._translationProblem('Write a message before translating it.')
      return
    }
    this._view.translationBusy = true
    this._translationProgress(0)
    await this._view.taskScheduler.render()
    try {
      const { translateHtml } = await import('./on-device-translation.js')
      const result = await translateHtml(this._currentHTML(), { targetLanguage: this._view.translationTarget, onProgress: (/** @type {number} */ loaded) => this._translationProgress(loaded) })
      this._translated(result)
    } catch (error) {
      this._translationProblem(error instanceof Error ? error.message : 'Translation failed.')
    }
    this._view.translationBusy = false
    await this._view.taskScheduler.render()
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
    const date = new Date()
    if (preset === 'tomorrow') {
      date.setDate(date.getDate() + 1)
      date.setHours(9, 0, 0, 0)
      return date.toISOString()
    }
    const days = /** @type {Record<string, number>} */ ({ 'three-days': 3, 'next-week': 7 })[preset] ?? 7
    return new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString()
  }

  /**
   * The ISO time of a send-later preset, in local wall-clock time.
   * @param {string} preset
   */
  sendLaterPreset(preset) {
    const date = new Date()
    const nextMonday = (8 - date.getDay()) % 7
    const days = /** @type {Record<string, number>} */ ({ 'next-monday': nextMonday === 0 ? 7 : nextMonday })[preset] ?? 1
    date.setDate(date.getDate() + days)
    date.setHours(preset === 'tomorrow-afternoon' ? 14 : 8, 0, 0, 0)
    return date.toISOString()
  }

  /**
   * Convert a datetime-local value to an ISO time ('' when invalid).
   * @param {string} value
   */
  localTime(value) {
    const date = new Date(String(value))
    return Number.isFinite(date.getTime()) ? date.toISOString() : ''
  }

  _restoreTranslationTarget() {
    try {
      this._keepTranslationTarget(String(localStorage.getItem(ComposeEditor._TRANSLATION_TARGET)))
    } catch { /* storage unavailable */ }
  }

  /** @param {string} target */
  _keepTranslationTarget(target) {
    if (!Object.hasOwn(TRANSLATION_LANGUAGES, target)) return
    this._view.translationTarget = target
    try { localStorage.setItem(ComposeEditor._TRANSLATION_TARGET, target) } catch { /* storage unavailable */ }
  }

  _bindDocument() {
    this._listeners = new AbortController()
    const options = { capture: true, signal: this._listeners.signal }
    document.addEventListener('input', event => this._onHeaderInput(event), options)
    document.addEventListener('keydown', event => this._onHeaderKeydown(event), options)
    document.addEventListener('mousedown', event => this._keepToolbarSelection(event), options)
    document.addEventListener('focusin', event => this._onFocusIn(event), options)
    document.addEventListener('focusout', event => this._onFocusOut(event), options)
    document.addEventListener('pointerdown', event => this._keepRecipientFocus(event), options)
    document.addEventListener('visibilitychange', () => this._onVisibility(), { signal: this._listeners.signal })
  }

  /** Bind the editor frame, which Tac may render after this service starts. */
  _findFrame() {
    const frame = /** @type {HTMLIFrameElement | null} */ (document.querySelector(ComposeEditor._FRAME))
    if (!frame || frame === this._frame) return
    this._frame = frame
    frame.addEventListener('load', () => this._frameLoaded(frame))
    if (frame.contentDocument?.readyState === 'complete') this._frameLoaded(frame)
  }

  /** @param {HTMLIFrameElement} frame */
  _frameLoaded(frame) {
    if (this._stopped || frame !== this._frame || !frame.isConnected) return
    const doc = frame.contentDocument
    const body = /** @type {HTMLElement | null} */ (doc?.querySelector('body[data-compose-editor]') ?? null)
    const revision = doc?.querySelector('meta[name="tachyon-safe-html-revision"]')?.getAttribute('content')
    if (!doc || !body || !this._safeFrame || revision !== String(this._safeFrame.metadata.revision) || body === this._body) return
    this._frameListeners?.abort()
    this._frameListeners = new AbortController()
    this._body = body
    this._stale = false
    this._copyThemeAttributes(doc)
    this._applyPreferences()
    this._bindFrame(doc)
    decorateTemplateVariables(body)
    this._templateStatus()
    this._renderQuote()
    this._selectImage(null)
    const steps = this._afterLoad.splice(0)
    for (const step of steps) step()
    this._scheduleGrammar(0)
    if (this._savePending) this._publishSaveDue()
    this._rerender()
  }

  /** Report actual preparation failure; no unvalidated frame or save is substituted. @returns {void} */
  _preparationFailed() {
    this._view.editorError = ComposeEditor._PREPARE_ERROR
    this._rerender()
  }

  /** Render the view after a change that came from a DOM event. */
  _rerender() {
    void this._view.taskScheduler.render()
  }

  /**
   * Theme, accent and composer font are attributes on <html> keyed in
   * themes.css; the editor document wears the same ones.
   * @param {Document} doc
   */
  _copyThemeAttributes(doc) {
    for (const attribute of Array.from(document.documentElement.attributes)) {
      if (attribute.name.startsWith('data-') || attribute.name === 'w-theme' || attribute.name === 'lang') doc.documentElement.setAttribute(attribute.name, attribute.value)
    }
  }

  /** @param {Document} doc */
  _bindFrame(doc) {
    const signal = /** @type {AbortController} */ (this._frameListeners).signal
    doc.addEventListener('input', event => this._onEditorInput(event), { signal })
    doc.addEventListener('keydown', event => this._onEditorKeydown(event), { signal })
    doc.addEventListener('paste', event => this._paste(event), { signal })
    doc.addEventListener('selectionchange', () => this._pressCommands(), { signal })
    doc.addEventListener('click', event => this._onEditorClick(event), { signal })
    doc.addEventListener('compositionstart', () => { this._composing = true }, { signal })
    doc.addEventListener('compositionend', () => this._compositionEnded(), { signal })
    doc.addEventListener('dragover', event => this.dragOver(event), { signal })
    doc.addEventListener('drop', event => this.drop(event), { signal })
  }

  _live() {
    return !this._stopped && this._safeFrame?.metadata.revision === this._preparingRevision && Boolean(this._frame?.isConnected && this._body?.isConnected) && !this._stale
  }

  /** @returns {string} */
  _currentHTML() {
    if (!this._live()) return this._html
    return this._richText ? serializeComposerHTML(/** @type {HTMLElement} */ (this._body)) : String(this._body?.innerText ?? '')
  }

  _currentText() {
    if (!this._live()) return this._html.replace(/<[^>]*>/g, '')
    return String(this._richText ? this._body?.textContent : this._body?.innerText)
  }

  /**
   * Ask Go to show a new whole body. The frame reloads with it, so saves wait
   * for that load.
   * @param {string} html
   * @param {string} reason
   */
  _replaceBody(html, reason) {
    if (html.length > ComposeEditor._LARGEST_EVENT) {
      this._toast('This message is too large to change here.')
      return
    }
    this._html = html
    this._stale = true
    /** @type {EditorReplaced} */
    const replaced = { html, reason }
    this._view.publishEditorReplaced(replaced)
  }

  _applyPreferences() {
    for (const target of [this._body, document.querySelector('[data-compose-subject] input')]) {
      const element = /** @type {HTMLElement | null} */ (target)
      if (!element) continue
      element.spellcheck = this._spellcheck
      if (this._spellcheckLanguage) element.setAttribute('lang', this._spellcheckLanguage)
      else element.removeAttribute('lang')
    }
    this._body?.setAttribute('contenteditable', this._richText ? 'true' : 'plaintext-only')
  }

  /** Keep the model in step after a change inside the editor. */
  _editorChanged() {
    this._renderQuote()
    this._requestSave(0)
  }

  /** @param {boolean} changed */
  _changedWhen(changed) {
    if (changed) this._editorChanged()
  }

  /**
   * @param {'fontName' | 'fontSize' | 'foreColor'} name
   * @param {string} value
   * @param {string[]} allowed
   */
  _style(name, value, allowed) {
    if (!this._body || !allowed.includes(value)) return
    this._changedWhen(applyStyleCommand(this._body, name, value))
  }

  _focusEditor() {
    this._frame?.contentWindow?.focus()
    this._body?.focus()
  }

  _renderQuote() {
    const present = Boolean(this._body?.querySelector(ComposeEditor._QUOTE))
    this._view.quotePresent = present
    this._body?.classList.toggle(ComposeEditor._COLLAPSED, present && !this._view.quoteExpanded)
  }

  _templateStatus() {
    const variables = this._body ? decorateTemplateVariables(this._body) : []
    const empty = variables.filter(node => node.classList.contains('empty')).length
    this._view.templateStatus = variables.length
      ? `Template blanks: ${variables.length}${empty ? ` (${empty} empty)` : ''}. Press Tab or Shift+Tab to move between blanks.`
      : ''
  }

  /** @param {string} [selector] */
  _selectFirstVariable(selector = 'code.var, code[data-template-var]') {
    const first = this._body?.querySelector(selector)
    if (!first) return
    this._focusEditor()
    selectTemplateVariable(first)
  }

  /** @param {boolean} confirmNeeded */
  _templateAllowed(confirmNeeded) {
    const editable = templateBodyFromDraft(this._currentHTML()) !== '' && this._currentText().trim() !== ''
    if (!confirmNeeded && !editable) return true
    return confirm('Loading a template will replace the editable draft contents. Signature and quoted mail will be kept. Continue?')
  }

  /** @param {{ html: string, text: string }} body */
  _tooLarge(body) {
    return body.html.length + body.text.length > ComposeEditor._LARGEST_EVENT
  }

  /** Debounce a draft save; the Go service decides whether one may run. @param {number} delay */
  _requestSave(delay) {
    if (!this._ready) return
    clearTimeout(this._saveTimer)
    this._saveTimer = setTimeout(() => this._publishSaveDue(), delay)
  }

  /** Capture proof and body as one revision; superseded captures never publish. @returns {Promise<void>} */
  async _publishSaveDue() {
    this._savePending = !this._live()
    if (this._savePending) return
    const body = { html: this._currentHTML(), text: this._currentText() }
    const revision = this._safeFrame?.metadata.revision
    const capture = ++this._saveCapture
    const tooLarge = this._tooLarge(body)
    /** @type {MatchedQuote|null} */
    let proof = null
    try {
      proof = tooLarge ? null : await matchQuoteProof(body.html, this._quoteProof)
    } catch {
      if (this._stopped) return
      this._preparationFailed()
      return
    }
    if (this._stopped || capture !== this._saveCapture) return
    if (this._safeFrame?.metadata.revision !== revision || !this._live() || this._currentHTML() !== body.html || this._currentText() !== body.text) {
      this._requestSave(0)
      return
    }
    this._quoteProof = proof?.proof || null
    /** @type {DraftSaveDue} */
    const due = { html: tooLarge ? '' : body.html, text: tooLarge ? '' : body.text, quoteProof:this._quoteProof, tooLarge }
    this._view.publishDraftSaveDue(due)
  }

  _onVisibility() {
    if (document.visibilityState === 'hidden' && this._ready) this._publishSaveDue()
  }

  /** @param {string} message */
  _toast(message) {
    /** @type {ToastRequested} */
    const toast = { message, actionLabel: '', action: '', subject: '', durationMs: 0 }
    this._view.publishToastRequested(toast)
  }

  /** @param {string} command */
  _shortcut(command) {
    /** @type {ShortcutPressed} */
    const pressed = { command }
    this._view.publishShortcutPressed(pressed)
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
    return Array.from(transfer?.items ?? []).map(item => item.getAsFile()).filter(file => file !== null)
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
    const offered = {
      files: files.map((file, index) => {
        this._offered.set(index, file)
        return { index, filename: file.name, contentType: file.type || 'application/octet-stream', size: file.size, inline: inline ?? isInlineImageFile(file) }
      }),
    }
    this._view.publishAttachmentsOffered(offered)
  }

  /**
   * @param {AttachmentAcceptance} acceptance
   * @param {File | undefined} file
   * @returns {Promise<ComposeAttachment[]>}
   */
  async _readOne(acceptance, file) {
    try {
      return [{
        id: acceptance.id, filename: String(file?.name), contentType: file?.type || 'application/octet-stream', size: Number(file?.size),
        contentBase64: await this._base64(/** @type {File} */ (file)),
        disposition: acceptance.contentId ? 'inline' : 'attachment', contentId: acceptance.contentId,
      }]
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

  /** @param {ClipboardEvent} event */
  _paste(event) {
    const files = this._transferFiles(event.clipboardData)
    if (!this._body) return
    event.preventDefault()
    if (files.length) {
      this._offerMixed(files)
      return
    }
    const html = this._richText ? String(event.clipboardData?.getData('text/html') ?? '') : ''
    this._changedWhen(insertPastedComposerContent(this._body, html, String(event.clipboardData?.getData('text/plain') ?? '')))
  }

  /**
   * @param {{ html: string, sourceLanguage: string, targetLanguage: string }} result
   */
  _translated(result) {
    const languages = /** @type {Record<string, string>} */ (TRANSLATION_LANGUAGES)
    this._replaceBody(result.html, 'translation')
    this._view.translationStatus = `Translated from ${languages[result.sourceLanguage] ?? 'detected language'} to ${languages[result.targetLanguage] ?? 'the selected language'}.`
    /** @type {EditorSignatureChanged} */
    const changed = { signatureId: currentSignatureId(result.html) }
    this._view.publishEditorSignatureChanged(changed)
    this._afterLoad.push(() => this._focusEditor())
    this._requestSave(0)
  }

  /** @param {string} message */
  _translationProblem(message) {
    this._view.translationStatus = message
    this._view.translationError = true
  }

  /** @param {number} loaded */
  _translationProgress(loaded) {
    this._view.translationError = false
    this._view.translationStatus = loaded > 0 && loaded < 1 ? `Downloading language model… ${Math.round(loaded * 100)}%` : 'Translating on this device…'
  }

  /** @param {Event} event */
  _onEditorInput(event) {
    if (!event.isTrusted || !this._body) return
    if (this._richText && applyInlineMarkdownShortcut(this._body)) this._requestSave(0)
    const before = `${this._view.templateStatus}|${this._view.quotePresent}`
    this._templateStatus()
    this._renderQuote()
    this._scheduleGrammar(ComposeEditor._GRAMMAR_DELAY)
    this._requestSave(ComposeEditor._SAVE_DELAY)
    if (before !== `${this._view.templateStatus}|${this._view.quotePresent}`) this._rerender()
  }

  /** @param {Event} event */
  _onHeaderInput(event) {
    const target = /** @type {HTMLElement} */ (event.target)
    if (!event.isTrusted || !target?.closest?.(ComposeEditor._PANEL)) return
    this._recipientTyped(target)
    this._requestSave(ComposeEditor._SAVE_DELAY)
  }

  /** @param {number} delay */
  _scheduleGrammar(delay) {
    clearTimeout(this._grammarTimer)
    if (this._composing) return
    this._grammarTimer = setTimeout(() => this._publishGrammarText(), delay)
  }

  _publishGrammarText() {
    const text = grammarText(this._body)
    if (text.length > ComposeEditor._LARGEST_EVENT) return
    /** @type {GrammarTextChanged} */
    const changed = { text }
    this._view.publishGrammarTextChanged(changed)
  }

  _compositionEnded() {
    this._composing = false
    this._scheduleGrammar(ComposeEditor._GRAMMAR_COMPOSITION_DELAY)
  }

  /** @param {Event} event @returns {string} */
  _recipientField(event) {
    const target = /** @type {HTMLElement | null} */ (event.target)
    const field = target?.tagName === 'INPUT' ? target.closest(ComposeEditor._RECIPIENT_FIELD)?.getAttribute('data-recipient-field') : ''
    return ['to', 'cc', 'bcc'].includes(String(field)) ? String(field) : ''
  }

  /** @param {HTMLElement} target */
  _recipientTyped(target) {
    const field = this._recipientField(/** @type {Event} */ (/** @type {unknown} */ ({ target })))
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
    setTimeout(() => {
      if (document.activeElement?.closest?.(ComposeEditor._RECIPIENT_FIELD)) return
      /** @type {RecipientSuggestionsDismissed} */
      const dismissed = { field: this._recipient.field }
      this._view.publishRecipientSuggestionsDismissed(dismissed)
    }, ComposeEditor._BLUR_DELAY)
  }

  /** @param {Event} event */
  _keepRecipientFocus(event) {
    const target = /** @type {HTMLElement | null} */ (event.target)
    if (target?.closest?.('[data-recipient-option]')) event.preventDefault()
  }

  /** @param {string} field */
  _recipientInput(field) {
    return /** @type {HTMLInputElement | null} */ (document.querySelector(`[data-recipient-field="${field}"] input`))
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
    input.setAttribute('aria-controls', list)
    input.setAttribute('aria-activedescendant', `${list}-${this._recipient.active}`)
    if (!expanded) input.removeAttribute('aria-activedescendant')
  }

  /** @param {KeyboardEvent} event */
  _onHeaderKeydown(event) {
    const target = /** @type {HTMLElement} */ (event.target)
    if (!target?.closest?.(ComposeEditor._PANEL)) return
    const command = (event.ctrlKey || event.metaKey) && !event.altKey
    if (command && event.shiftKey && this._focusField(event, event.key.toLowerCase())) return
    const toolbar = /** @type {HTMLElement | null} */ (target.closest('[role="toolbar"][data-compose-toolbar]'))
    if (toolbar) this._toolbarKey(event, toolbar)
    else if (this._recipientField(event)) this._recipientKey(event)
  }

  /**
   * Ctrl+Enter sends; Ctrl+Shift+T, C, B and F focus To, CC, BCC and From.
   * Keys typed in the editor frame never reach the page's keyboard runtime.
   * @param {KeyboardEvent} event
   */
  _onEditorKeydown(event) {
    const command = (event.ctrlKey || event.metaKey) && !event.altKey
    if (command && event.key === 'Enter') {
      event.preventDefault()
      this._shortcut('composer:send')
      return
    }
    if (command && event.shiftKey && this._focusField(event, event.key.toLowerCase())) return
    if (!this._body || event.isComposing || !this._richText) return
    const plainTab = event.key === 'Tab' && !event.altKey && !event.ctrlKey && !event.metaKey
    if (plainTab && moveBetweenTemplateVariables(this._body, event.shiftKey ? -1 : 1)) event.preventDefault()
    this._markdownKey(event, this._body)
    this._formattingKey(event, command)
  }

  /**
   * @param {KeyboardEvent} event
   * @param {string} key
   */
  _focusField(event, key) {
    const selector = /** @type {Record<string, string>} */ ({ t: '[data-compose-to]', c: '[data-compose-cc]', b: '[data-compose-bcc]', f: '[data-compose-from]' })[key]
    if (!selector) return false
    const host = document.querySelector(selector)
    const control = host?.matches('input, select') ? host : host?.querySelector('input, select')
    event.preventDefault();
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
    const next = /** @type {Record<string, number>} */ ({ Home: 0, End: controls.length - 1, ArrowRight: current + 1, ArrowLeft: current - 1 })[event.key]
    if (next === undefined || current < 0) return
    event.preventDefault()
    const index = (next + controls.length) % controls.length
    controls.forEach((control, position) => { control.tabIndex = position === index ? 0 : -1 })
    controls[index].focus()
  }

  /** @param {KeyboardEvent} event */
  _recipientKey(event) {
    const open = this._recipient.count > 0 && this._recipientField(event) === this._recipient.field
    const handled = ['ArrowDown', 'ArrowUp', 'Escape'].includes(event.key) || (event.key === 'Enter' && this._recipient.active >= 0)
    if (open && handled) event.preventDefault()
  }

  /**
   * Markdown line prefixes on Space, and links typed before Space or Enter.
   * @param {KeyboardEvent} event
   * @param {HTMLElement} editor
   */
  _markdownKey(event, editor) {
    if (event.key === ' ' && applyMarkdownShortcut(editor)) {
      event.preventDefault()
      this._editorChanged()
    } else if (event.key === ' ' || event.key === 'Enter') {
      if (autoLinkBeforeCaret(editor)) this._requestSave(ComposeEditor._SAVE_DELAY)
    }
  }

  /**
   * @param {KeyboardEvent} event
   * @param {boolean} command
   */
  _formattingKey(event, command) {
    const key = `${event.shiftKey ? 'shift+' : ''}${event.key.toLowerCase()}`
    const action = /** @type {Record<string, () => void>} */ ({
      b: () => this.format('strong'), i: () => this.format('em'), u: () => this.format('u'), k: () => this.link(),
      'shift+x': () => this.format('s'), 'shift+7': () => this.list(true), 'shift+8': () => this.list(false),
    })[key]
    if (!command || !action) return
    event.preventDefault()
    action()
  }

  _pressCommands() {
    const doc = this._body?.ownerDocument
    const anchor = doc?.getSelection()?.anchorNode ?? null
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
  _onEditorClick(event) {
    const image = /** @type {HTMLImageElement | null} */ (/** @type {HTMLElement} */ (event.target)?.closest?.(ComposeEditor._INLINE_IMAGE))
    if (image) event.preventDefault()
    this._selectImage(image)
    this._rerender()
  }

  /** @param {HTMLImageElement | null} image */
  _selectImage(image) {
    this._selectedImage?.removeAttribute(ComposeEditor._SELECTED)
    this._selectedImage = image
    this._selectedImageId = image?.getAttribute('data-caduceus-attachment-id') ?? ''
    image?.setAttribute(ComposeEditor._SELECTED, 'true')
    this._view.imageSelected = Boolean(image)
    this._view.selectedImageName = image?.alt || 'Inline image'
  }
}

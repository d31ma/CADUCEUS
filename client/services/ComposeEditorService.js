// @ts-check
/** @import { SessionChanged } from '/models/SessionChanged.js' */
/** @import { SafeHtmlMetadata } from '../models/SafeHtmlMetadata.js' */
/** @import { ComposeEditorSnapshot } from '/models/ComposeEditorSnapshot.js' */
/** @import { ComposeLoaded } from '/models/ComposeLoaded.js' */
/** @import { ComposeChanged } from '/models/ComposeChanged.js' */
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
/** @import { ComposeEditor as Editor } from '../shared/scripts/compose-editor.js' */

import { ComposeEditor, preferredTranslationTarget } from '../shared/scripts/compose-editor.js'
import { setTranslationAssetToken } from '../shared/scripts/chunked-asset-fetch.js'

/**
 * Escape hatch: the composer's rich-text editor. The message body is the
 * contenteditable body of an iframe whose document ComposeService builds, and
 * editing it needs the DOM: the selection, the browser's editing commands,
 * paste, file reading, confirm and prompt, debounce timers, on-device
 * translation, grammar highlights and audio. This service does those and
 * talks to the Go services in events; Go owns the message, the drafts, the
 * sending and the navigation.
 */
@Service
export default class ComposeEditorService {
  /** @type {ApplicationEventPublisher | undefined} Injected before lifecycle methods run. */
  applicationEventPublisher
  /** @type {{ render: () => Promise<void> } | undefined} Injected before lifecycle methods run. */
  @Needs
  taskScheduler

  /** Whether the editor holds a quoted reply. */
  quotePresent = false
  /** Whether the quoted reply is expanded. */
  quoteExpanded = false
  /** Whether an inline image is selected for the sizing controls. */
  imageSelected = false
  selectedImageName = 'Inline image'
  editorError = ''
  templateStatus = ''
  translationStatus = ''
  translationError = false
  translationBusy = false
  translationTarget = preferredTranslationTarget()
  /** Whether a file drag is over the compose surface. */
  dragActive = false

  /** @type {Editor} */
  _editor = new ComposeEditor(this)

  /** The framework injects this resource before lifecycle methods run. @returns {ApplicationEventPublisher} */
  get _publisher() {
    return /** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher)
  }

  /**
   * @param {ComposeEditorSnapshot} snapshot
   * @param {Array<{id:string,text:string}>} signatures
   * @returns {string}
   */
  frameBody(snapshot, signatures) {
    return this._editor.frame(snapshot, signatures).body
  }

  /**
   * @param {ComposeEditorSnapshot} snapshot
   * @param {Array<{id:string,text:string}>} signatures
   * @returns {SafeHtmlMetadata}
   */
  frameMetadata(snapshot, signatures) {
    return this._editor.frame(snapshot, signatures).metadata
  }

  /** @param {ComposeEditorSnapshot} snapshot @param {Array<{id:string,text:string}>} signatures @returns {boolean} */
  frameReady(snapshot, signatures) {
    return this._editor.frameReady(snapshot, signatures)
  }

  // Typed publishers for the events the editor module raises; the compiler
  // names each event after its model type.

  /** @param {AttachmentsAdded} attachmentsAdded */
  publishAttachmentsAdded(attachmentsAdded) {
    this._publisher.publishEvent(attachmentsAdded)
  }

  /** @param {AttachmentsOffered} attachmentsOffered */
  publishAttachmentsOffered(attachmentsOffered) {
    this._publisher.publishEvent(attachmentsOffered)
  }

  /** @param {DraftSaveDue} draftSaveDue */
  publishDraftSaveDue(draftSaveDue) {
    this._publisher.publishEvent(draftSaveDue)
  }

  /** @param {EditorReplaced} editorReplaced */
  publishEditorReplaced(editorReplaced) {
    this._publisher.publishEvent(editorReplaced)
  }

  /** @param {EditorSignatureChanged} editorSignatureChanged */
  publishEditorSignatureChanged(editorSignatureChanged) {
    this._publisher.publishEvent(editorSignatureChanged)
  }

  /** @param {GrammarTextChanged} grammarTextChanged */
  publishGrammarTextChanged(grammarTextChanged) {
    this._publisher.publishEvent(grammarTextChanged)
  }

  /** @param {InlineImageRemoved} inlineImageRemoved */
  publishInlineImageRemoved(inlineImageRemoved) {
    this._publisher.publishEvent(inlineImageRemoved)
  }

  /** @param {RecipientQueryChanged} recipientQueryChanged */
  publishRecipientQueryChanged(recipientQueryChanged) {
    this._publisher.publishEvent(recipientQueryChanged)
  }

  /** @param {RecipientSuggestionsDismissed} recipientSuggestionsDismissed */
  publishRecipientSuggestionsDismissed(recipientSuggestionsDismissed) {
    this._publisher.publishEvent(recipientSuggestionsDismissed)
  }

  /** @param {SendBodyReady} sendBodyReady */
  publishSendBodyReady(sendBodyReady) {
    this._publisher.publishEvent(sendBodyReady)
  }

  /** @param {ShortcutPressed} shortcutPressed */
  publishShortcutPressed(shortcutPressed) {
    this._publisher.publishEvent(shortcutPressed)
  }

  /** @param {TemplateDraftReady} templateDraftReady */
  publishTemplateDraftReady(templateDraftReady) {
    this._publisher.publishEvent(templateDraftReady)
  }

  /** @param {TemplateInserted} templateInserted */
  publishTemplateInserted(templateInserted) {
    this._publisher.publishEvent(templateInserted)
  }

  /** @param {ToastRequested} toastRequested */
  publishToastRequested(toastRequested) {
    this._publisher.publishEvent(toastRequested)
  }

  get translationLanguages() {
    return this._editor.translationLanguages
  }

  /** @param {SessionChanged} changed */
  @OnEvent({"replay":true})
  session(changed) {
    setTranslationAssetToken(this._assetToken(changed))
  }

  /** @param {SessionChanged} changed @returns {string} */
  _assetToken(changed) {
    if (!changed.signedIn) return ''
    return changed.token
  }

  @OnStart
  start() {
    return this._editor.start()
  }

  @OnStop
  stop() {
    return this._editor.stop()
  }

  /** @param {ComposeLoaded} loaded */
  @OnEvent({"replay":true})
  composeLoaded(loaded) {
    return this._editor.composeLoaded(loaded)
  }

  /** @param {EditorPreferences} preferences */
  @OnEvent
  editorPreferences(preferences) {
    return this._editor.editorPreferences(preferences)
  }

  /** @param {ComposeChanged} changed */
  @OnEvent
  composeChanged(changed) {
    return this._editor.composeChanged(changed)
  }

  /** @param {SignatureApplyRequested} requested */
  @OnEvent
  signatureApplyRequested(requested) {
    return this._editor.signatureApplyRequested(requested)
  }

  /** @param {TemplateInsertRequested} requested */
  @OnEvent
  templateInsertRequested(requested) {
    return this._editor.templateInsertRequested(requested)
  }

  /** @param {TemplateDraftRequested} requested */
  @OnEvent
  templateDraftRequested(requested) {
    return this._editor.templateDraftRequested(requested)
  }

  /** @param {AttachmentsAccepted} accepted */
  @OnEvent
  async attachmentsAccepted(accepted) {
    return this._editor.attachmentsAccepted(accepted)
  }

  /** @param {AttachmentRemoved} removed */
  @OnEvent
  attachmentRemoved(removed) {
    return this._editor.attachmentRemoved(removed)
  }

  /** @param {RecipientSuggestionsShown} shown */
  @OnEvent
  recipientSuggestionsShown(shown) {
    return this._editor.recipientSuggestionsShown(shown)
  }

  /** @param {RecipientChosen} chosen */
  @OnEvent
  recipientChosen(chosen) {
    return this._editor.recipientChosen(chosen)
  }

  /** @param {GrammarTextWanted} wanted */
  @OnEvent
  grammarTextWanted(wanted) {
    return this._editor.grammarTextWanted(wanted)
  }

  /** @param {GrammarHighlightsChanged} changed */
  @OnEvent
  grammarHighlightsChanged(changed) {
    return this._editor.grammarHighlightsChanged(changed)
  }

  /** @param {GrammarReplacementRequested} requested */
  @OnEvent
  grammarReplacementRequested(requested) {
    return this._editor.grammarReplacementRequested(requested)
  }

  /** @param {GrammarFocusRequested} requested */
  @OnEvent
  grammarFocusRequested(requested) {
    return this._editor.grammarFocusRequested(requested)
  }

  /** @param {SendRequested} requested */
  @OnEvent
  sendRequested(requested) {
    return this._editor.sendRequested(requested)
  }

  /** @param {SoundRequested} requested */
  @OnEvent
  soundRequested(requested) {
    return this._editor.soundRequested(requested)
  }

  /**
   * Toggle an inline format on the selection: strong, em, u, s or code.
   * @param {Event} _event
   * @param {string} tag
   */
  format(_event, tag) {
    return this._editor.format(tag)
  }

  /**
   * Toggle a bulleted or numbered list.
   * @param {Event} _event
   * @param {boolean} ordered
   */
  list(_event, ordered) {
    return this._editor.list(ordered)
  }

  /**
   * Open the file picker for attachments or for inline images.
   * @param {Event} _event
   * @param {string} kind 'attachment' or 'inline-image'
   */
  pick(_event, kind) {
    return this._editor.pick(kind)
  }

  /** Turn the selection into a link after asking for its address. */
  link() {
    return this._editor.link()
  }

  /**
   * Apply the toolbar choice that changed: block, font, size or color.
   * @param {Event} event
   * @param {string} kind
   */
  style(event, kind) {
    return this._editor.style(event, kind)
  }

  toggleRightToLeft() {
    return this._editor.toggleRightToLeft()
  }

  toggleQuotedText() {
    return this._editor.toggleQuotedText()
  }

  removeQuotedText() {
    return this._editor.removeQuotedText()
  }

  /**
   * Offer the files picked for attachments or for inline images.
   * @param {Event} event
   * @param {string} kind 'attachment' or 'inline-image'
   */
  add(event, kind) {
    return this._editor.add(event, kind)
  }

  /** @param {DragEvent} event */
  dragOver(event) {
    return this._editor.dragOver(event)
  }

  /** @param {DragEvent} event */
  dragLeave(event) {
    return this._editor.dragLeave(event)
  }

  /** @param {DragEvent} event */
  drop(event) {
    return this._editor.drop(event)
  }

  /**
   * @param {Event} _event
   * @param {number} width
   */
  resizeSelectedInlineImage(_event, width) {
    return this._editor.resizeSelectedInlineImage(_event, width)
  }

  removeSelectedInlineImage() {
    return this._editor.removeSelectedInlineImage()
  }

  /** @param {Event} event */
  changeTranslationTarget(event) {
    return this._editor.changeTranslationTarget(event)
  }

  /** Replace the draft with an on-device translation the person asked for. */
  async translateDraft() {
    return this._editor.translateDraft()
  }

  /**
   * Format a stored ISO time with the mailbox's clock preference.
   * @param {string} value
   */
  formatDate(value) {
    return this._editor.formatDate(value)
  }

  /**
   * The ISO time of a follow-up reminder preset, in local wall-clock time.
   * @param {string} preset
   */
  reminderPreset(preset) {
    return this._editor.reminderPreset(preset)
  }

  /**
   * The ISO time of a send-later preset, in local wall-clock time.
   * @param {string} preset
   */
  sendLaterPreset(preset) {
    return this._editor.sendLaterPreset(preset)
  }

  /**
   * Convert a datetime-local value to an ISO time ('' when invalid).
   * @param {string} value
   */
  localTime(value) {
    return this._editor.localTime(value)
  }
}

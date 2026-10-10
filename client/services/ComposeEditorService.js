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
/** @import { ComposeEditor as Editor } from '../shared/scripts/compose-editor.js' */

import { ComposeEditor } from '../shared/scripts/compose-editor.js'
import { preferredTranslationLanguage } from '../shared/scripts/on-device-translation.js'

/**
 * The browser-only half of the composer: the contenteditable rich-text
 * editor, its selection, paste, inline images, quoted text and template
 * blanks, the grammar highlights, file reading, confirm and prompt dialogs,
 * the debounce timers, on-device translation, the encrypted offline vault and
 * the requests that carry the message body and attachment bytes.
 *
 * Everything else (headers, identities, signatures, templates, recipients,
 * reminders, scheduling, validation and the send envelope) lives in the Go
 * ComposeService and GrammarService. Go services cannot reach the DOM, and
 * a Go call, an event and a browser HTTP request each carry at most 64 KiB, so
 * this service keeps the body and the bytes and talks to Go through events.
 */
@Service
export default class ComposeEditorService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher
  /** @type {{ render: () => Promise<void> }} */
  @Needs
  taskScheduler

  /** Human-readable autosave state shown in the live region. */
  draftStatus = ''
  /** Whether another tab saved a newer draft version. */
  draftConflict = false
  /** Whether the editor holds a quoted reply. */
  quotePresent = false
  /** Whether the quoted reply is expanded. */
  quoteExpanded = false
  /** Whether an inline image is selected for the sizing controls. */
  imageSelected = false
  selectedImageName = 'Inline image'
  templateStatus = ''
  translationStatus = ''
  translationError = false
  translationBusy = false
  translationTarget = preferredTranslationLanguage()
  /** Whether a file drag is over the compose surface. */
  dragActive = false

  /** @type {Editor} */
  _editor = new ComposeEditor(this)

  get translationLanguages() {
    return this._editor.translationLanguages
  }

  @OnStart
  start() {
    return this._editor.start()
  }

  @OnStop
  stop() {
    return this._editor.stop()
  }

  /** @param {SessionChanged} changed */
  @OnEvent({"replay":true})
  async sessionChanged(changed) {
    return this._editor.sessionChanged(changed)
  }

  /** @param {EditorPreferences} preferences */
  @OnEvent
  editorPreferences(preferences) {
    return this._editor.editorPreferences(preferences)
  }

  /** @param {ComposeChanged} changed */
  @OnEvent
  async composeChanged(changed) {
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
  async sendRequested(requested) {
    return this._editor.sendRequested(requested)
  }

  /** @param {DraftSaveRequested} requested */
  @OnEvent
  async draftSaveRequested(requested) {
    return this._editor.draftSaveRequested(requested)
  }

  /** @param {NavigationRequested} requested */
  @OnEvent
  async navigationRequested(requested) {
    return this._editor.navigationRequested(requested)
  }

  /** Bold the selection (toolbar and Ctrl+B). */
  bold() {
    return this._editor.bold()
  }

  italic() {
    return this._editor.italic()
  }

  underline() {
    return this._editor.underline()
  }

  strike() {
    return this._editor.strike()
  }

  code() {
    return this._editor.code()
  }

  bulletList() {
    return this._editor.bulletList()
  }

  numberedList() {
    return this._editor.numberedList()
  }

  /** Open the file picker for attachments. */
  pickAttachments() {
    return this._editor.pickAttachments()
  }

  /** Open the file picker for inline images. */
  pickInlineImages() {
    return this._editor.pickInlineImages()
  }

  /** Turn the selection into a link after asking for its address. */
  link() {
    return this._editor.link()
  }

  /** @param {Event} event */
  changeBlockStyle(event) {
    return this._editor.changeBlockStyle(event)
  }

  /** @param {Event} event */
  changeFontFamily(event) {
    return this._editor.changeFontFamily(event)
  }

  /** @param {Event} event */
  changeFontSize(event) {
    return this._editor.changeFontSize(event)
  }

  /** @param {Event} event */
  changeTextColor(event) {
    return this._editor.changeTextColor(event)
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

  /** @param {Event} event */
  addAttachments(event) {
    return this._editor.addAttachments(event)
  }

  /** @param {Event} event */
  addInlineImages(event) {
    return this._editor.addInlineImages(event)
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

  /** @param {ClipboardEvent} event */
  paste(event) {
    return this._editor.paste(event)
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

  /** Replace local edits with the newer saved draft after a conflict. */
  async reloadSavedDraft() {
    return this._editor.reloadSavedDraft()
  }

  /** Keep local edits by saving them as a new draft after a conflict. */
  async saveDraftCopy() {
    return this._editor.saveDraftCopy()
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

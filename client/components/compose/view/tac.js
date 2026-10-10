// @ts-check

import {
  decorateTemplateVariables,
  hasEmptyTemplateVariables,
  insertTemplateBeforeProtectedContent,
  moveBetweenTemplateVariables,
  normalizeTemplateVariables,
  selectTemplateVariable,
  stripTemplateVariableMarkup,
  templateBodyFromDraft,
} from '../../../shared/scripts/composer-templates.js'
import {
  applySignature,
  currentSignatureId,
  defaultSignatureForAddress,
  sanitizeSignatureHTML,
} from '../../../shared/scripts/composer-signatures.js'
import {
  clearGrammarHighlights,
  grammarRange,
  grammarText,
  normalizeGrammarMatches,
  paintGrammarHighlights,
  replaceGrammarRange,
} from '../../../shared/scripts/grammar-check.js'
import {
  preferredTranslationLanguage,
  translateHtml,
  TRANSLATION_LANGUAGES,
} from '../../../shared/scripts/on-device-translation.js'
import {
  applyBlockFormat,
  applyInlineFormat,
  applyInlineMarkdownShortcut,
  applyListFormat,
  applyMarkdownShortcut,
  autoLinkBeforeCaret,
  createInlineContentId,
  editableComposerHTML,
  insertInlineImage,
  insertPastedComposerContent,
  isInlineImageFile,
  normalizeComposeLink,
  resizeInlineImage,
  serializeComposerHTML,
} from '../../../shared/scripts/rich-compose-editor.js'
import {
  DEFAULT_MAIL_PREFERENCES,
  formatPreferenceDate,
  normalizeMailPreferences,
} from '../../../shared/scripts/mail-preferences.js'
import { externalSenderIdentities } from '../../../shared/scripts/external-accounts-ui.js'
import {
  discardOfflineWrites,
  loadOfflineSnapshot,
  removeOfflineSnapshot,
  saveOfflineSnapshot,
} from '../../../shared/scripts/offline-write-vault.js'

/**
 * Email composition component.
 *
 * Renders a rich-text compose form with support for:
 * - Semantic rich-text formatting, inline CID images, and reply quotes
 * - Message templates (saved and loaded from the API)
 * - Default signatures loaded from the API
 * - Send now, send later (scheduled), and send & archive
 * - Read-receipt tracking pixels and click-tracking link rewriting
 * - Keyboard shortcut bindings for send and discard
 *
 * Accepts an optional `prefill` prop to seed the To, Subject, and body fields
 * (e.g. when replying or forwarding). It also checks `window._caduceus` for a
 * route-based prefill payload.
 *
 *
 * @prop {ComposePrefill} [prefill] - Optional prefill data for the compose form
 */
export default class {
  static __tachyonOnMount = ['initFromPrefill', 'bindShortcuts', 'bindDraftAutosave', 'bindRichComposer', 'bindRecipientSuggestions', 'bindTemplateVariables', 'bindGrammarCheck', 'bindOnDeviceTranslation']

  /** @type {(ComposePrefill & {draftId?: string, remindAt?: string, openTracking?: boolean, linkTracking?: boolean}) | undefined} Optional component prefill prop. */
  prefill
  /** @type {string} Exact primary address or alias used as the RFC From identity. */
  $from = ''
  /** @type {string[]} */
  @SessionState
  senderAddresses = []
  /** @type {Array<{ address: string, label: string, accountId: string, provider: string, color: string }>} */
  $externalSenderIdentities = []
  /** @type {string} */
  $to = ''
  /** @type {string} */
  $cc = ''
  /** @type {string} */
  $bcc = ''
  /** @type {string} */
  $replyTo = ''
  /** @type {string} RFC parent Message-ID carried invisibly through reply composition. */
  $inReplyTo = ''
  /** @type {string[]} Ordered RFC References chain carried through reply composition. */
  $references = []
  /** @type {string} */
  $subject = ''
  /** @type {string} */
  $text = ''
  /** @type {boolean} */
  @SessionState
  loading = false
  /** @type {boolean} Prevent interaction until asynchronous defaults settle. */
  @SessionState
  ready = false
  /** @type {string} */
  $error = ''
  /** @type {Array<{ id: string, name: string, subject?: string, text?: string, to?: string, cc?: string }>} */
  $templates = []
  /** @type {Array<{ id: string, domain: string, name: string, text: string, defaultFor: string[] }>} */
  $signatures = []
  /** @type {string} */
  @SessionState
  activeSignatureId = ''
  /** @type {string} Template currently inserted into the editor. */
  @SessionState
  activeTemplateName = ''
  /** @type {boolean} Whether to append a read-receipt tracking pixel */
  @SessionState
  requestReadReceipt = false
  /** @type {boolean} Whether to rewrite outbound links through the click-tracking proxy */
  @SessionState
  trackLinks = false
  /** @type {string} ISO timestamp for a no-reply follow-up reminder. */
  $remindAt = ''
  /** @type {boolean} Whether the accessible follow-up chooser is expanded. */
  @SessionState
  reminderPickerOpen = false
  /** @type {boolean} Whether the accessible send-time chooser is expanded. */
  @SessionState
  schedulePickerOpen = false
  /** @type {string} Existing scheduled message id, when rescheduling. */
  @SessionState
  scheduledEditId = ''
  /** @type {string} Existing or selected ISO delivery timestamp. */
  @SessionState
  scheduledSendAt = ''
  /** @type {string} Source message whose conversation can be archived after send. */
  $sourceEmailId = ''
  /** @type {Array<{ id: string, filename: string, contentType: string, size: number, contentBase64: string, disposition?: 'inline'|'attachment', contentId?: string }>} */
  $attachments = []
  /** @type {boolean} Whether a file drag is currently over the compose surface. */
  @SessionState
  attachmentDragActive = false
  /** @type {string} ID of the draft being edited, if any. When set, sending
   * removes the original draft so it does not linger in the Drafts folder. */
  $draftId = ''
  /** @type {string} Idempotency key retained across a lost create response. */
  $draftClientId = ''
  /** @type {number} Optimistic-concurrency version last acknowledged by FYLO. */
  $draftVersion = 0
  /** @type {string} Human-readable autosave state. */
  @SessionState
  draftStatus = ''
  /** @type {string} Imperative live-region value; deliberately non-reactive so autosave cannot replace the editor. */
  _draftStatusValue = ''
  /** @type {boolean} Whether another tab saved a newer draft version. */
  @SessionState
  draftConflict = false
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _draftSaveTimer
  _draftSaving = false
  _draftSaveAgain = false
  _lastDraftSnapshot = ''
  _intentionalExit = false
  /** @type {Array<{ contactId: string, name: string, email: string, source: string, interactionCount: number, lastContactedAt: string }>} */
  _recipientSuggestions = []
  /** @type {'to'|'cc'|'bcc'|''} */
  _recipientField = ''
  _recipientIndex = -1
  _recipientRequest = 0
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _recipientTimer
  _grammarEnabled = false
  _grammarLanguage = 'auto'
  _grammarDisabledRules = []
  _grammarIssues = []
  _grammarChecking = false
  _grammarStatus = 'Grammar check is off.'
  _grammarRequest = 0
  _grammarComposing = false
  _translationTarget = preferredTranslationLanguage()
  _translationBusy = false
  _translationStatus = ''
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _grammarTimer
  /** @type {Range | null} Last valid editor selection, retained while a toolbar select has focus. */
  _savedEditorRange = null
  /** @type {HTMLImageElement | null} Inline image currently selected for accessible sizing controls. */
  _selectedInlineImage = null
  /** @type {string} Non-reactive canonical HTML for surviving Tachyon node replacement without caret churn. */
  _editorHTML = ''
  _editorModelReady = false
  /** @type {HTMLElement | null} */
  _liveEditor = null
  _quoteExpanded = false
  _selectedInlineAttachmentId = ''
  /** @type {{ from: string, to: string, cc: string, bcc: string, replyTo: string, subject: string } | null} */
  _nativeFieldValues = null
  /** @type {string[]} Recipients introduced by sender automation, replaced when From changes. */
  _automaticCc = []
  /** @type {string[]} */
  _automaticBcc = []

  get mailPreferences() {
    return normalizeMailPreferences(window._caduceus?.mailPreferences || DEFAULT_MAIL_PREFERENCES)
  }

  get translationLanguageOptions() {
    return Object.entries(TRANSLATION_LANGUAGES).map(([id, label]) => ({ id, label }))
  }

  /**
   * Initialise the compose form from route prefill data or component props.
   *
   * Checks `window._caduceus.consumeComposePrefill()` first (route-level prefill),
   * then falls back to the auto-bound `prefill` prop. After seeding fields it loads
   * templates and the default signature, then syncs the rich-text editor.
   *
   * @async
   * @returns {Promise<void>}
   */
  async initFromPrefill() {
    try {
      const target = localStorage.getItem('caduceus-translation-target') || ''
      if (Object.hasOwn(TRANSLATION_LANGUAGES, target)) this._translationTarget = target
    } catch {}
    const routePrefill = window._caduceus?.consumeComposePrefill?.() || {}
    const prefill = this.prefill || routePrefill
    this.$from = prefill.from || window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || ''
    this.$to = prefill.to || ''
    this.$cc = prefill.cc || ''
    this.$bcc = prefill.bcc || ''
    this.$replyTo = prefill.replyTo || ''
    this.$inReplyTo = prefill.inReplyTo || ''
    this.$references = Array.isArray(prefill.references) ? [...prefill.references] : []
    this.$subject = prefill.subject || ''
    this.$text = prefill.body || ''
    this._editorHTML = this.$text
    this._editorModelReady = true
    this.scheduledEditId = prefill.scheduledId || ''
    this.scheduledSendAt = prefill.sendAt || ''
    this.$sourceEmailId = prefill.sourceEmailId || ''
    this.$remindAt = prefill.remindAt || ''
    this.requestReadReceipt = prefill.openTracking === true
    this.trackLinks = prefill.linkTracking === true
    this.$attachments = Array.isArray(prefill.attachments)
      ? prefill.attachments.map(attachment => ({ ...attachment, id: attachment.id || crypto.randomUUID() }))
      : []
    // Editing an existing draft: fetch its full content and seed the form. The
    // draft id is retained so `send()` can remove the original afterwards.
    let recoveredDraft = false
    if (prefill.draftId) {
      this.$draftId = prefill.draftId
      await this._loadDraft(prefill.draftId)
    } else if (!this._hasExplicitPrefill(prefill)) {
      await this._restoreDraftRecovery()
      recoveredDraft = Boolean(this.$draftId || this.draftStatus === 'Recovered locally')
    }
    if (!this.$draftClientId) this.$draftClientId = crypto.randomUUID()
    await Promise.all([this.loadTemplates(), this.loadSignatures(), this.loadGrammarPreferences()])
    const newComposition = !this.$draftId && !this.scheduledEditId && !recoveredDraft
    if (newComposition && !prefill.from && this.mailPreferences.defaultFrom
      && this.senderAddresses.includes(this.mailPreferences.defaultFrom)) {
      this.$from = this.mailPreferences.defaultFrom
    }
    if (newComposition) this.applyAutomaticRecipients(this.$from)
    // New messages, replies, and forwards all receive the exact From address's
    // default before quoted content. Existing drafts retain their saved choice.
    if (!this.$draftId && !this.scheduledEditId && !currentSignatureId(this.$text)) {
      this.$text = applySignature(this.$text, defaultSignatureForAddress(this.$signatures, this.$from))
      this._editorHTML = this.$text
    }
    this.activeSignatureId = currentSignatureId(this.$text)
    this._rememberReactiveFields()
    this._syncEditorContent()
    this._applyEditorPreferences()
    this._syncNativeFields()
    this._setDraftStatus(this._draftStatusValue || this.draftStatus)
    this._renderGrammarUI()
    if (this._grammarEnabled) this._scheduleGrammarCheck(0)
    this.ready = true
    // A server-backed draft already has an authoritative fingerprint. Do not
    // dirty it merely because Tachyon remounted the compose view (for example
    // after an application-level refresh). New prefill and local-only recovery
    // have no fingerprint and still need their first durable save.
    if (!this.scheduledEditId && this._hasMeaningfulDraft() && !this._lastDraftSnapshot) this.scheduleDraftSave()
    setTimeout(async () => {
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      this._syncEditorContent()
      this._applyEditorPreferences()
      this._syncNativeFields()
      this._setDraftStatus(this._draftStatusValue || this.draftStatus)
      this._renderTemplateOptions()
      this._renderGrammarUI()
    }, 0)
  }

  /** Bind the target picker without a Tachyon render that would replace the live rich editor. */
  bindOnDeviceTranslation() {
    if (window._caduceusTranslationTargetHandler) {
      document.removeEventListener('change', window._caduceusTranslationTargetHandler, true)
    }
    window._caduceusTranslationTargetHandler = event => {
      const select = event.target?.closest?.('.compose-translation-language select')
      if (!select) return
      const value = String(select.value || '')
      if (!Object.hasOwn(TRANSLATION_LANGUAGES, value)) return
      this._translationTarget = value
      try { localStorage.setItem('caduceus-translation-target', value) } catch {}
    }
    document.addEventListener('change', window._caduceusTranslationTargetHandler, true)
    if (window._caduceusTranslationClickHandler) {
      document.removeEventListener('click', window._caduceusTranslationClickHandler, true)
    }
    window._caduceusTranslationClickHandler = event => {
      if (!event.target?.closest?.('[data-translate-draft]')) return
      event.preventDefault()
      void this.translateDraft()
    }
    document.addEventListener('click', window._caduceusTranslationClickHandler, true)
  }

  /** Replace the editable draft body with an explicitly requested local translation. */
  async translateDraft() {
    if (this._translationBusy) return
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    if (!editor || !editor.innerText.trim()) {
      this._setTranslationStatus('Write a message before translating it.', true)
      return
    }
    const button = /** @type {HTMLButtonElement | null} */ (document.querySelector('[data-translate-draft]'))
    this._translationBusy = true
    if (button) {
      button.disabled = true
      button.setAttribute('aria-busy', 'true')
    }
    this._setTranslationStatus('Translating on this device…')
    try {
      const result = await translateHtml(editor.innerHTML, {
        targetLanguage: this._translationTarget,
        onProgress: loaded => this._setTranslationStatus(
          loaded > 0 && loaded < 1
            ? `Downloading language model… ${Math.round(loaded * 100)}%`
            : 'Translating on this device…',
        ),
      })
      editor.innerHTML = result.html
      this.$text = result.html
      this._editorHTML = this.$text
      this.activeSignatureId = currentSignatureId(result.html)
      this._setTranslationStatus(`Translated from ${TRANSLATION_LANGUAGES[result.sourceLanguage] || 'detected language'} to ${TRANSLATION_LANGUAGES[result.targetLanguage] || 'the selected language'}.`)
      this.scheduleDraftSave(0)
      if (this._grammarEnabled) this._scheduleGrammarCheck(0)
      editor.focus()
    } catch (error) {
      this._setTranslationStatus(error instanceof Error ? error.message : 'Translation failed.', true)
    } finally {
      this._translationBusy = false
      if (button) {
        button.disabled = false
        button.removeAttribute('aria-busy')
      }
    }
  }

  /** @param {string} message @param {boolean} [error] */
  _setTranslationStatus(message, error = false) {
    this._translationStatus = message
    const status = document.querySelector('[data-translation-status]')
    if (!status) return
    status.textContent = message
    status.classList.toggle('compose-translation-error', error)
    status.removeAttribute('hidden')
  }

  /** Populate native compose inputs after asynchronous prefill has settled. */
  _syncNativeFields() {
    requestAnimationFrame(() => {
      const values = this._nativeFieldValues || {
        from: this.$from, to: this.$to, cc: this.$cc, bcc: this.$bcc, replyTo: this.$replyTo, subject: this.$subject,
      }
      for (const [selector, value] of [
        ['[data-compose-to]', values.to],
        ['[data-compose-cc]', values.cc],
        ['[data-compose-bcc]', values.bcc],
        ['[data-compose-reply-to]', values.replyTo],
        ['[data-compose-subject]', values.subject],
      ]) {
        const host = document.querySelector(selector)
        const input = /** @type {HTMLInputElement | null} */ (
          host?.matches('input') ? host : host?.querySelector('input')
        )
        if (input) input.value = String(value || '')
      }
      const from = /** @type {HTMLSelectElement | null} */ (document.querySelector('[data-compose-from]'))
      if (from) from.value = values.from
      const signature = /** @type {HTMLSelectElement | null} */ (document.querySelector('[data-compose-signature]'))
      if (signature) signature.value = this.activeSignatureId
    })
  }

  _rememberReactiveFields() {
    this._nativeFieldValues = {
      from: this.$from, to: this.$to, cc: this.$cc, bcc: this.$bcc, replyTo: this.$replyTo, subject: this.$subject,
    }
  }

  _captureNativeFields() {
    const read = (selector, fallback) => {
      const host = document.querySelector(selector)
      const control = host?.matches('input, textarea, select') ? host : host?.querySelector('input, textarea, select')
      return String(control?.value ?? fallback ?? '').trim()
    }
    const previous = this._nativeFieldValues
    this._nativeFieldValues = {
      from: read('[data-compose-from]', previous?.from ?? this.$from),
      to: read('[data-compose-to]', previous?.to ?? this.$to),
      cc: read('[data-compose-cc]', previous?.cc ?? this.$cc),
      bcc: read('[data-compose-bcc]', previous?.bcc ?? this.$bcc),
      replyTo: read('[data-compose-reply-to]', previous?.replyTo ?? this.$replyTo),
      subject: read('[data-compose-subject]', previous?.subject ?? this.$subject),
    }
  }

  /**
   * Fetch a draft by id and seed the To / CC / Subject / body fields from it.
   *
   * @param {string} id - The draft email ID.
   * @returns {Promise<boolean>}
   */
  async _loadDraft(id) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return false
    try {
      const res = await apiFetch(`/drafts/${id}`)
      if (!res?.ok) return false
      const draft = await res.json()
      this.$to = Array.isArray(draft.to) ? draft.to.join(', ') : draft.to || draft.recipient || this.$to
      this.$from = draft.sender || this.$from
      this.$cc = Array.isArray(draft.cc) ? draft.cc.join(', ') : draft.cc || this.$cc
      this.$bcc = Array.isArray(draft.bcc) ? draft.bcc.join(', ') : draft.bcc || this.$bcc
      this.$replyTo = Array.isArray(draft.replyTo) ? draft.replyTo[0] || '' : draft.replyTo || this.$replyTo
      this.$inReplyTo = draft.inReplyTo || this.$inReplyTo
      this.$references = Array.isArray(draft.references) ? draft.references : this.$references
      this.$sourceEmailId = draft.sourceEmailId || this.$sourceEmailId
      this.$subject = draft.subject || this.$subject
      this.$text = draft.htmlBody || draft.body || this.$text
      this._editorHTML = this.$text
      this._editorModelReady = true
      this.$attachments = Array.isArray(draft.attachments)
        ? draft.attachments.map(attachment => ({ ...attachment, id: attachment.id || crypto.randomUUID() }))
        : []
      this.$draftId = draft.id
      this.$draftClientId = draft.draftClientId || this.$draftClientId || crypto.randomUUID()
      this.$draftVersion = Number(draft.draftVersion || 0)
      this._lastDraftSnapshot = JSON.stringify(this._snapshotFromSavedDraft(draft))
      this.draftStatus = this.$draftVersion ? 'Saved' : ''
      this._draftStatusValue = this.draftStatus
      this._rememberReactiveFields()
      this._persistDraftRecovery()
      return true
    } catch { /* leave whatever the prefill provided */ }
    return false
  }

  /** Whether route data represents a deliberate new compose action. */
  _hasExplicitPrefill(prefill) {
    return Boolean(prefill?.scheduledId || prefill?.sourceEmailId || prefill?.from || prefill?.to || prefill?.cc || prefill?.bcc || prefill?.replyTo || prefill?.subject || prefill?.body || prefill?.inReplyTo || prefill?.references?.length)
  }

  /** Restore a server-backed draft pointer, falling back to the local text snapshot after a crash. */
  async _restoreDraftRecovery() {
    const owner = String(window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || '').toLowerCase()
    let recovery = owner ? await loadOfflineSnapshot(owner, 'compose-recovery') : null
    // One-time migration from the former plaintext recovery record.
    if (!recovery) {
      try { recovery = JSON.parse(localStorage.getItem('caduceus-compose-recovery') || 'null') } catch { recovery = null }
      if (recovery && owner) {
        await saveOfflineSnapshot(owner, 'compose-recovery', recovery).catch(() => {})
        try { localStorage.removeItem('caduceus-compose-recovery') } catch {}
      }
    }
    if (!recovery || typeof recovery !== 'object') return
    this.$draftClientId = String(recovery.clientId || '')
    this.$draftVersion = Number(recovery.version || 0)
    if (recovery.draftId && await this._loadDraft(String(recovery.draftId))) return
    const snapshot = recovery.snapshot
    if (!snapshot || typeof snapshot !== 'object') return
    this.$to = Array.isArray(snapshot.to) ? snapshot.to.join(', ') : ''
    this.$from = String(snapshot.from || this.$from)
    this.$cc = Array.isArray(snapshot.cc) ? snapshot.cc.join(', ') : ''
    this.$bcc = Array.isArray(snapshot.bcc) ? snapshot.bcc.join(', ') : ''
    this.$replyTo = Array.isArray(snapshot.replyTo) ? snapshot.replyTo[0] || '' : ''
    this.$subject = String(snapshot.subject || '')
    this.$text = String(snapshot.html || snapshot.text || '')
    this._editorHTML = this.$text
    this._editorModelReady = true
    this.$inReplyTo = String(snapshot.inReplyTo || '')
    this.$references = Array.isArray(snapshot.references) ? snapshot.references : []
    this.$sourceEmailId = String(snapshot.sourceEmailId || '')
    this.draftStatus = 'Recovered locally'
    this._draftStatusValue = this.draftStatus
  }

  /** Capture a complete server snapshot; local recovery omits attachment bytes. */
  _draftSnapshot(includeAttachmentContent = true) {
    const split = value => String(value || '').split(',').map(item => item.trim()).filter(Boolean)
    return {
      from: this.$from,
      to: split(this.$to),
      cc: split(this.$cc),
      bcc: split(this.$bcc),
      replyTo: split(this.$replyTo),
      subject: this.$subject || '',
      text: this._getEditorText(),
      html: this.ready ? this._getEditorHTML() : this.$text || '',
      ...(this.$inReplyTo ? { inReplyTo: this.$inReplyTo } : {}),
      references: [...this.$references],
      ...(this.$sourceEmailId ? { sourceEmailId: this.$sourceEmailId } : {}),
      attachments: this.$attachments.map(attachment => ({
        filename: attachment.filename,
        contentType: attachment.contentType,
        size: attachment.size,
        disposition: attachment.disposition || 'attachment',
        ...(attachment.contentId ? { contentId: attachment.contentId } : {}),
        ...(includeAttachmentContent ? { contentBase64: attachment.contentBase64 } : {}),
      })),
    }
  }

  _hasMeaningfulDraft(snapshot = this._draftSnapshot(false)) {
    return Boolean(snapshot.to.length || snapshot.cc.length || snapshot.replyTo.length || snapshot.subject || snapshot.text.trim() || snapshot.html.trim() || snapshot.attachments.length)
  }

  _persistDraftRecovery(snapshot = this._draftSnapshot(false)) {
    const owner = String(window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || '').toLowerCase()
    if (!owner) return
    void saveOfflineSnapshot(owner, 'compose-recovery', {
        draftId: this.$draftId,
        clientId: this.$draftClientId,
        version: this.$draftVersion,
        snapshot,
        savedAt: new Date().toISOString(),
      }).catch(() => { /* server autosave remains authoritative when storage is unavailable */ })
  }

  _clearDraftRecovery() {
    const owner = String(window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || '').toLowerCase()
    if (owner) void removeOfflineSnapshot(owner, 'compose-recovery')
    try { localStorage.removeItem('caduceus-compose-recovery') } catch {}
  }

  /**
   * Publish autosave state immediately. Timer- and network-driven state
   * changes happen outside a Tachyon event transaction, so updating the
   * rendered live region explicitly prevents it from displaying stale state.
   */
  _setDraftStatus(status) {
    this._draftStatusValue = status
    const liveRegion = document.querySelector('.compose-draft-status')
    if (liveRegion) liveRegion.textContent = status
  }

  /** Rerender structural draft UI such as the conflict resolution panel. */
  async _rerenderDraftUI() {
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    this._syncEditorContent()
    this._syncNativeFields()
    this._setDraftStatus(this._draftStatusValue || this.draftStatus)
  }

  /** Bind one capture listener that survives Tachyon replacing compose controls. */
  bindDraftAutosave() {
    if (window._caduceusDraftInputHandler) {
      document.removeEventListener('input', window._caduceusDraftInputHandler, true)
    }
    window._caduceusDraftInputHandler = event => {
      // DuVay may dispatch synthetic input events while a reactive render
      // reconnects its native control. Only a real user edit should dirty the
      // draft; otherwise the status render itself can create an autosave loop.
      if (!event.isTrusted || !event.target?.closest?.('.compose-panel')) return
      this._captureNativeFields()
      if (event.target?.closest?.('[data-compose-editor]')) {
        this._editorHTML = this._getEditorHTML()
        this._editorModelReady = true
      }
      this.scheduleDraftSave()
    }
    document.addEventListener('input', window._caduceusDraftInputHandler, true)
    if (window._caduceusDraftVisibilityHandler) {
      document.removeEventListener('visibilitychange', window._caduceusDraftVisibilityHandler)
    }
    window._caduceusDraftVisibilityHandler = () => {
      if (document.visibilityState === 'hidden') void this.saveDraft()
    }
    document.addEventListener('visibilitychange', window._caduceusDraftVisibilityHandler)
    if (window._caduceusDraftOfflineWriteHandler) {
      window.removeEventListener('caduceus:offline-write-result', window._caduceusDraftOfflineWriteHandler)
    }
    window._caduceusDraftOfflineWriteHandler = event => {
      const detail = /** @type {CustomEvent<any>} */ (event).detail
      if (detail?.kind !== 'draft' || detail?.metadata?.clientId !== this.$draftClientId) return
      if (detail.status === 'conflict') {
        this.draftConflict = true
        this._setDraftStatus('Newer saved version detected')
        void this._rerenderDraftUI()
        return
      }
      if (detail.status !== 'accepted' || !detail.response?.id) return
      this.$draftId = String(detail.response.id)
      this.$draftVersion = Number(detail.response.draftVersion || this.$draftVersion || 1)
      this._lastDraftSnapshot = JSON.stringify(this._snapshotFromSavedDraft(detail.response))
      this.draftConflict = false
      this._setDraftStatus('Saved')
      this._persistDraftRecovery()
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
    }
    window.addEventListener('caduceus:offline-write-result', window._caduceusDraftOfflineWriteHandler)
  }

  /**
   * Bind selection-safe rich-editor behavior through delegated listeners so it
   * survives Tachyon replacing the compose subtree during a reactive render.
   */
  bindRichComposer() {
    for (const [type, key] of [
      ['selectionchange', '_caduceusComposeSelectionHandler'],
      ['mousedown', '_caduceusComposeToolbarPointerHandler'],
      ['keydown', '_caduceusComposeRichKeyHandler'],
      ['click', '_caduceusComposeImageSelectionHandler'],
      ['input', '_caduceusComposeRichInputHandler'],
    ]) {
      const previous = window[key]
      if (previous) document.removeEventListener(type, previous, true)
    }
    window._caduceusComposeSelectionHandler = () => {
      const selection = getSelection()
      if (!selection?.rangeCount) return
      const editor = document.querySelector('[data-compose-editor]')
      const range = selection.getRangeAt(0)
      if (editor?.contains(range.commonAncestorContainer)) {
        this._savedEditorRange = range.cloneRange()
        const anchor = /** @type {Element | null | undefined} */ (
          selection.anchorNode?.nodeType === Node.ELEMENT_NODE ? selection.anchorNode : selection.anchorNode?.parentElement
        )
        for (const button of document.querySelectorAll('[data-compose-command]')) {
          const tag = button.getAttribute('data-compose-command')
          button.setAttribute('aria-pressed', String(Boolean(tag && anchor?.closest?.(tag))))
        }
      }
    }
    window._caduceusComposeToolbarPointerHandler = event => {
      const target = event.target?.closest?.('[data-compose-format]')
      if (!target || target.matches('select, input')) return
      // Keep the editor's selection alive while a formatting button is clicked.
      event.preventDefault()
    }
    window._caduceusComposeRichKeyHandler = event => {
      const inComposer = event.target?.closest?.('.compose-panel')
      const command = (event.ctrlKey || event.metaKey) && !event.altKey
      if (inComposer && command && event.shiftKey) {
        const selector = ({ t: '[data-compose-to]', c: '[data-compose-cc]', b: '[data-compose-bcc]', f: '[data-compose-from]' })[event.key.toLowerCase()]
        if (selector) {
          event.preventDefault()
          const host = document.querySelector(selector)
          const control = host?.matches('input, select') ? host : host?.querySelector('input, select')
          control?.focus()
          return
        }
      }
      if (inComposer && command && event.key === 'Enter') {
        event.preventDefault()
        void this.sendDefault()
        return
      }
      const toolbar = event.target?.closest?.('[role="toolbar"][data-compose-toolbar]')
      if (toolbar && ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
        const controls = Array.from(toolbar.querySelectorAll('button:not([hidden]), select:not([hidden]), input:not([hidden])'))
          .filter(control => !control.disabled)
        const current = controls.indexOf(event.target)
        if (current < 0 || controls.length === 0) return
        event.preventDefault()
        const next = event.key === 'Home' ? 0
          : event.key === 'End' ? controls.length - 1
            : (current + (event.key === 'ArrowRight' ? 1 : -1) + controls.length) % controls.length
        controls.forEach((control, index) => control.tabIndex = index === next ? 0 : -1)
        controls[next].focus()
        return
      }
      const editor = event.target?.closest?.('[data-compose-editor]')
      if (!editor || event.isComposing) return
      if (!this.mailPreferences.richText) return
      if (event.key === ' ') {
        if (applyMarkdownShortcut(editor)) {
          event.preventDefault()
          this._editorChanged()
          return
        }
        if (autoLinkBeforeCaret(editor)) {
          this._editorHTML = serializeComposerHTML(editor)
          this._editorModelReady = true
        }
      } else if (event.key === 'Enter' && autoLinkBeforeCaret(editor)) {
        this._editorHTML = serializeComposerHTML(editor)
        this._editorModelReady = true
      }
      if (!command) return
      const shortcut = event.key.toLowerCase()
      if (shortcut === 'b') { event.preventDefault(); this.execBold() }
      else if (shortcut === 'i') { event.preventDefault(); this.execItalic() }
      else if (shortcut === 'u') { event.preventDefault(); this.execUnderline() }
      else if (shortcut === 'k') { event.preventDefault(); this.execLink() }
      else if (event.shiftKey && shortcut === 'x') { event.preventDefault(); this.execStrike() }
      else if (event.shiftKey && shortcut === '7') { event.preventDefault(); this.execNumberedList() }
      else if (event.shiftKey && shortcut === '8') { event.preventDefault(); this.execBulletList() }
    }
    window._caduceusComposeImageSelectionHandler = event => {
      const image = event.target?.closest?.('img[data-caduceus-inline-image]')
      if (!image) {
        if (!event.target?.closest?.('[data-inline-image-controls]')) this._selectInlineImage(null)
        return
      }
      event.preventDefault()
      this._selectInlineImage(image)
    }
    window._caduceusComposeRichInputHandler = event => {
      const editor = /** @type {HTMLElement | null} */ (event.target?.closest?.('[data-compose-editor]'))
      if (!editor || !applyInlineMarkdownShortcut(editor)) return
      this._editorHTML = serializeComposerHTML(editor)
      this._editorModelReady = true
      this.scheduleDraftSave(0)
    }
    document.addEventListener('selectionchange', window._caduceusComposeSelectionHandler, true)
    document.addEventListener('mousedown', window._caduceusComposeToolbarPointerHandler, true)
    document.addEventListener('keydown', window._caduceusComposeRichKeyHandler, true)
    document.addEventListener('click', window._caduceusComposeImageSelectionHandler, true)
    document.addEventListener('input', window._caduceusComposeRichInputHandler, true)
    window._caduceusComposeEditorObserver?.disconnect()
    window._caduceusComposeEditorObserver = new MutationObserver(() => {
      const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
      if (!editor) return
      if (editor === this._liveEditor) {
        this._syncNativeFields()
        return
      }
      this._liveEditor = editor
      const source = this._editorModelReady ? this._editorHTML : this.$text
      editor.innerHTML = editableComposerHTML(source, this.$attachments)
      decorateTemplateVariables(editor)
      this._applyEditorPreferences(editor)
      if (this._selectedInlineAttachmentId) {
        this._selectInlineImage(/** @type {HTMLImageElement | null} */ (editor.querySelector(`img[data-caduceus-attachment-id="${CSS.escape(this._selectedInlineAttachmentId)}"]`)))
      }
      this._renderQuotedTextControl()
      this._syncNativeFields()
    })
    window._caduceusComposeEditorObserver.observe(document.body, { childList: true, subtree: true })
    this._liveEditor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    this._applyEditorPreferences(this._liveEditor)
    this._renderQuotedTextControl()
  }

  _restoreEditorSelection() {
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    if (!editor) return null
    editor.focus()
    if (this._savedEditorRange && editor.contains(this._savedEditorRange.commonAncestorContainer)) {
      const selection = getSelection()
      selection?.removeAllRanges()
      selection?.addRange(this._savedEditorRange)
    }
    return editor
  }

  _editorChanged() {
    this.$text = this._getEditorHTML()
    this._editorHTML = this.$text
    this._editorModelReady = true
    this.scheduleDraftSave(0)
    this._renderQuotedTextControl()
    // `$text` is reactive even though the editor's HTML is managed
    // imperatively. Rehydrate after Tachyon commits the event render so rich
    // markup and the caret are not replaced by the template's empty div.
    this._syncEditorContent()
  }

  /** @param {HTMLImageElement | null} image */
  _selectInlineImage(image) {
    this._selectedInlineImage?.removeAttribute('data-selected')
    this._selectedInlineImage = image
    this._selectedInlineAttachmentId = image?.dataset.caduceusAttachmentId || ''
    image?.setAttribute('data-selected', 'true')
    const controls = /** @type {HTMLElement | null} */ (document.querySelector('[data-inline-image-controls]'))
    if (controls) controls.hidden = !image
    const name = document.querySelector('[data-inline-image-name]')
    if (name) name.textContent = image?.alt || 'Inline image'
  }

  /**
   * Keep Mailspring-compatible template blanks navigable without tying the
   * rich editor selection to a Tachyon render cycle.
   */
  bindTemplateVariables() {
    if (window._caduceusTemplateKeyHandler) {
      document.removeEventListener('keydown', window._caduceusTemplateKeyHandler, true)
    }
    if (window._caduceusTemplateInputHandler) {
      document.removeEventListener('input', window._caduceusTemplateInputHandler, true)
    }
    window._caduceusTemplateKeyHandler = event => {
      const editor = /** @type {HTMLElement | null} */ (event.target?.closest?.('[data-compose-editor]'))
      if (!editor || event.key !== 'Tab' || event.altKey || event.ctrlKey || event.metaKey) return
      if (moveBetweenTemplateVariables(editor, event.shiftKey ? -1 : 1)) event.preventDefault()
    }
    window._caduceusTemplateInputHandler = event => {
      const editor = event.target?.closest?.('[data-compose-editor]')
      if (!editor) return
      decorateTemplateVariables(editor)
      this._updateTemplateStatus()
    }
    document.addEventListener('keydown', window._caduceusTemplateKeyHandler, true)
    document.addEventListener('input', window._caduceusTemplateInputHandler, true)
  }

  /** Load the mailbox's opt-in local grammar-check preferences. */
  async loadGrammarPreferences() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const response = await apiFetch('/users/me/preferences')
      if (!response?.ok) return
      const preferences = await response.json()
      this._grammarEnabled = preferences?.grammarCheckEnabled === true
      this._grammarLanguage = ['auto', 'en-US', 'en-GB'].includes(preferences?.grammarCheckLanguage)
        ? preferences.grammarCheckLanguage
        : 'auto'
      this._grammarDisabledRules = Array.isArray(preferences?.grammarCheckDisabledRules)
        ? preferences.grammarCheckDisabledRules.filter(rule => /^[A-Z0-9_-]{1,64}$/.test(rule)).slice(0, 100)
        : []
      this._grammarStatus = this._grammarEnabled ? 'Waiting for text…' : 'Grammar check is off.'
      this._renderGrammarUI()
    } catch { /* grammar checking remains off when preferences are unavailable */ }
  }

  /** Debounce editor changes and defer results while IME composition is active. */
  bindGrammarCheck() {
    for (const [type, key] of [
      ['input', '_caduceusGrammarInputHandler'],
      ['compositionstart', '_caduceusGrammarCompositionStart'],
      ['compositionend', '_caduceusGrammarCompositionEnd'],
    ]) {
      const previous = window[key]
      if (previous) document.removeEventListener(type, previous, true)
    }
    window._caduceusGrammarInputHandler = event => {
      if (!event.isTrusted || !event.target?.closest?.('[data-compose-editor]') || !this._grammarEnabled || this._grammarComposing) return
      this._scheduleGrammarCheck(700)
    }
    window._caduceusGrammarCompositionStart = event => {
      if (event.target?.closest?.('[data-compose-editor]')) this._grammarComposing = true
    }
    window._caduceusGrammarCompositionEnd = event => {
      if (!event.target?.closest?.('[data-compose-editor]')) return
      this._grammarComposing = false
      if (this._grammarEnabled) this._scheduleGrammarCheck(300)
    }
    document.addEventListener('input', window._caduceusGrammarInputHandler, true)
    document.addEventListener('compositionstart', window._caduceusGrammarCompositionStart, true)
    document.addEventListener('compositionend', window._caduceusGrammarCompositionEnd, true)
  }

  async toggleGrammarCheck() {
    const previous = this._grammarEnabled
    this._grammarEnabled = !previous
    if (!await this._saveGrammarPreferences({ grammarCheckEnabled: this._grammarEnabled })) {
      this._grammarEnabled = previous
      this._grammarStatus = 'Could not save grammar preference.'
    } else if (this._grammarEnabled) {
      this._grammarStatus = 'Waiting for text…'
      this._scheduleGrammarCheck(0)
    } else {
      clearTimeout(this._grammarTimer)
      this._grammarRequest += 1
      this._grammarIssues = []
      this._grammarStatus = 'Grammar check is off.'
      clearGrammarHighlights()
    }
    this._renderGrammarUI()
  }

  async changeGrammarLanguage(event) {
    const language = String(event?.target?.value || '')
    if (!['auto', 'en-US', 'en-GB'].includes(language)) return
    const previous = this._grammarLanguage
    this._grammarLanguage = language
    if (!await this._saveGrammarPreferences({ grammarCheckLanguage: language })) this._grammarLanguage = previous
    this._renderGrammarUI()
    if (this._grammarEnabled) this._scheduleGrammarCheck(0)
  }

  async _saveGrammarPreferences(patch) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return false
    try {
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify(patch),
      })
      return Boolean(response?.ok)
    } catch { return false }
  }

  _scheduleGrammarCheck(delay = 700) {
    clearTimeout(this._grammarTimer)
    if (!this._grammarEnabled || this._grammarComposing) return
    this._grammarTimer = setTimeout(() => void this._runGrammarCheck(), delay)
  }

  async _runGrammarCheck() {
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    const apiFetch = window._caduceus?.apiFetch
    if (!editor || !apiFetch || !this._grammarEnabled || this._grammarComposing) return
    const text = grammarText(editor)
    const request = ++this._grammarRequest
    if (!text.trim()) {
      this._grammarIssues = []
      this._grammarStatus = 'No text to check.'
      clearGrammarHighlights()
      this._renderGrammarUI()
      return
    }
    this._grammarChecking = true
    this._grammarStatus = 'Checking locally…'
    this._renderGrammarUI()
    try {
      const response = await apiFetch('/grammar/check', {
        method: 'POST',
        body: JSON.stringify({
          text,
          language: this._grammarLanguage,
          disabledRules: this._grammarDisabledRules,
        }),
      })
      const data = await response?.json()
      if (request !== this._grammarRequest || grammarText(editor) !== text) return
      if (!response?.ok) throw new Error(data?.error || 'Grammar check unavailable.')
      this._grammarIssues = normalizeGrammarMatches(data?.matches, text.length)
      this._grammarStatus = this._grammarIssues.length
        ? `${this._grammarIssues.length} grammar ${this._grammarIssues.length === 1 ? 'issue' : 'issues'}.`
        : data?.detectedLanguage === 'und' ? 'This language is not supported by the local checker.' : 'No grammar issues.'
      paintGrammarHighlights(editor, this._grammarIssues)
    } catch (error) {
      if (request !== this._grammarRequest) return
      this._grammarIssues = []
      clearGrammarHighlights()
      this._grammarStatus = error instanceof Error ? error.message : 'Grammar check unavailable.'
    } finally {
      if (request === this._grammarRequest) {
        this._grammarChecking = false
        this._renderGrammarUI()
      }
    }
  }

  _renderGrammarUI() {
    const toggle = /** @type {HTMLButtonElement | null} */ (document.querySelector('[data-grammar-toggle]'))
    const panel = /** @type {HTMLElement | null} */ (document.querySelector('[data-grammar-panel]'))
    const status = document.querySelector('[data-grammar-status]')
    const language = /** @type {HTMLSelectElement | null} */ (document.querySelector('[data-grammar-language]'))
    const count = /** @type {HTMLElement | null} */ (document.querySelector('[data-grammar-count]'))
    const issues = document.querySelector('[data-grammar-issues]')
    if (toggle) {
      const title = this._grammarEnabled
        ? `Grammar check: ${this._grammarChecking ? 'checking' : `${this._grammarIssues.length} issues`}`
        : 'Enable grammar check'
      toggle.setAttribute('aria-pressed', String(this._grammarEnabled))
      toggle.setAttribute('aria-label', title)
      toggle.title = title
      toggle.classList.toggle('enabled', this._grammarEnabled)
    }
    if (panel) panel.hidden = !this._grammarEnabled
    if (status) status.textContent = this._grammarStatus
    if (language) language.value = this._grammarLanguage
    if (count) {
      count.hidden = this._grammarIssues.length === 0
      count.textContent = String(this._grammarIssues.length)
    }
    if (!issues) return
    issues.replaceChildren()
    if (!this._grammarEnabled || this._grammarIssues.length === 0) return
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    const text = editor ? grammarText(editor) : ''
    this._grammarIssues.forEach((issue, index) => {
      const article = document.createElement('article')
      article.className = 'grammar-issue'
      const heading = document.createElement('div')
      heading.className = 'grammar-issue-heading'
      const focus = document.createElement('button')
      focus.type = 'button'
      focus.textContent = `“${text.slice(issue.offset, issue.offset + issue.length)}”`
      focus.addEventListener('click', () => this._focusGrammarIssue(index))
      const category = document.createElement('span')
      category.textContent = issue.shortMessage
      heading.append(focus, category)
      const message = document.createElement('p')
      message.textContent = issue.message
      const actions = document.createElement('div')
      actions.className = 'grammar-issue-actions'
      issue.replacements.forEach(replacement => {
        const button = document.createElement('button')
        button.type = 'button'
        button.textContent = replacement
        button.addEventListener('click', () => this._applyGrammarReplacement(index, replacement))
        actions.append(button)
      })
      const ignore = document.createElement('button')
      ignore.type = 'button'
      ignore.className = 'grammar-ignore'
      ignore.textContent = 'Ignore rule'
      ignore.addEventListener('click', () => void this._ignoreGrammarRule(issue.ruleId))
      actions.append(ignore)
      article.append(heading, message, actions)
      issues.append(article)
    })
  }

  _focusGrammarIssue(index) {
    const issue = this._grammarIssues[index]
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    const range = editor && issue ? grammarRange(editor, issue.offset, issue.length) : null
    if (!editor || !range) return
    const selection = document.getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
    editor.focus()
  }

  _applyGrammarReplacement(index, replacement) {
    const issue = this._grammarIssues[index]
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    if (!editor || !issue || !replaceGrammarRange(editor, issue, replacement)) return
    editor.focus()
    this.scheduleDraftSave(0)
    this._scheduleGrammarCheck(0)
  }

  async _ignoreGrammarRule(ruleId) {
    if (!/^[A-Z0-9_-]{1,64}$/.test(ruleId) || this._grammarDisabledRules.includes(ruleId)) return
    const previous = [...this._grammarDisabledRules]
    this._grammarDisabledRules.push(ruleId)
    if (!await this._saveGrammarPreferences({ grammarCheckDisabledRules: this._grammarDisabledRules })) {
      this._grammarDisabledRules = previous
      return
    }
    this._grammarIssues = this._grammarIssues.filter(issue => issue.ruleId !== ruleId)
    const editor = document.querySelector('[data-compose-editor]')
    if (editor) paintGrammarHighlights(editor, this._grammarIssues)
    this._grammarStatus = this._grammarIssues.length
      ? `${this._grammarIssues.length} grammar ${this._grammarIssues.length === 1 ? 'issue' : 'issues'}.`
      : 'No grammar issues.'
    this._renderGrammarUI()
  }

  /**
   * Bind an accessible recipient combobox that survives Tachyon replacing the
   * underlying DuVay text-field controls. Suggestions are painted into a
   * permanent DOM container instead of triggering a document rerender on each
   * keystroke, preserving the native input caret and rich-text selection.
   */
  bindRecipientSuggestions() {
    for (const [type, key] of [
      ['input', '_caduceusRecipientInputHandler'],
      ['focusin', '_caduceusRecipientFocusHandler'],
      ['keydown', '_caduceusRecipientKeyHandler'],
      ['pointerdown', '_caduceusRecipientPointerHandler'],
      ['focusout', '_caduceusRecipientBlurHandler'],
    ]) {
      const previous = window[key]
      if (previous) document.removeEventListener(type, previous, true)
    }

    window._caduceusRecipientInputHandler = event => {
      const field = recipientFieldForEvent(event)
      if (!field) return
      const value = String(/** @type {HTMLInputElement} */ (event.target).value || '')
      if (field === 'to') this.$to = value
      else if (field === 'cc') this.$cc = value
      else this.$bcc = value
      this._scheduleRecipientSuggestions(field, value)
    }
    window._caduceusRecipientFocusHandler = event => {
      const field = recipientFieldForEvent(event)
      if (!field) return
      this._scheduleRecipientSuggestions(field, String(/** @type {HTMLInputElement} */ (event.target).value || ''), 0)
    }
    window._caduceusRecipientKeyHandler = event => {
      const field = recipientFieldForEvent(event)
      if (!field || this._recipientField !== field || this._recipientSuggestions.length === 0) return
      const key = /** @type {KeyboardEvent} */ (event).key
      if (key === 'ArrowDown' || key === 'ArrowUp') {
        event.preventDefault()
        const direction = key === 'ArrowDown' ? 1 : -1
        this._recipientIndex = (this._recipientIndex + direction + this._recipientSuggestions.length) % this._recipientSuggestions.length
        this._renderRecipientSuggestions()
      } else if (key === 'Enter' && this._recipientIndex >= 0) {
        event.preventDefault()
        this._selectRecipient(this._recipientIndex)
      } else if (key === 'Escape') {
        event.preventDefault()
        this._hideRecipientSuggestions()
      }
    }
    window._caduceusRecipientPointerHandler = event => {
      const option = /** @type {Element | null} */ (event.target instanceof Element ? event.target.closest('[data-recipient-option]') : null)
      if (!option) return
      event.preventDefault()
      this._selectRecipient(Number(option.getAttribute('data-recipient-option')))
    }
    window._caduceusRecipientBlurHandler = event => {
      if (!recipientFieldForEvent(event)) return
      setTimeout(() => {
        if (!document.activeElement?.closest?.('[data-recipient-field]')) this._hideRecipientSuggestions()
      }, 120)
    }

    document.addEventListener('input', window._caduceusRecipientInputHandler, true)
    document.addEventListener('focusin', window._caduceusRecipientFocusHandler, true)
    document.addEventListener('keydown', window._caduceusRecipientKeyHandler, true)
    document.addEventListener('pointerdown', window._caduceusRecipientPointerHandler, true)
    document.addEventListener('focusout', window._caduceusRecipientBlurHandler, true)
  }

  _scheduleRecipientSuggestions(field, value, delay = 140) {
    clearTimeout(this._recipientTimer)
    const query = currentRecipientToken(value)
    if (!query) { this._hideRecipientSuggestions(); return }
    this._recipientTimer = setTimeout(() => void this._loadRecipientSuggestions(field, query), delay)
  }

  async _loadRecipientSuggestions(field, query) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const request = ++this._recipientRequest
    try {
      const response = await apiFetch(`/contacts/suggestions?q=${encodeURIComponent(query)}`)
      if (request !== this._recipientRequest) return
      const suggestions = response?.ok ? await response.json() : []
      const used = new Set(`${this.$to},${this.$cc},${this.$bcc}`.split(',').map(value => value.trim().toLowerCase()).filter(Boolean))
      this._recipientSuggestions = suggestions.filter(item => !used.has(String(item.email).toLowerCase())).slice(0, 8)
      this._recipientField = field
      this._recipientIndex = this._recipientSuggestions.length ? 0 : -1
      this._renderRecipientSuggestions()
    } catch {
      if (request === this._recipientRequest) this._hideRecipientSuggestions()
    }
  }

  _renderRecipientSuggestions() {
    for (const container of document.querySelectorAll('[data-recipient-suggestions]')) {
      container.replaceChildren()
      if (container instanceof HTMLElement) container.hidden = true
    }
    const container = /** @type {HTMLElement | null} */ (document.querySelector(`[data-recipient-suggestions="${this._recipientField}"]`))
    const input = this._recipientInput(this._recipientField)
    if (!container || !input || this._recipientSuggestions.length === 0) {
      if (input) input.setAttribute('aria-expanded', 'false')
      return
    }
    const listId = `compose-${this._recipientField}-suggestions`
    container.id = listId
    this._recipientSuggestions.forEach((suggestion, index) => {
      const option = document.createElement('button')
      option.type = 'button'
      option.className = `recipient-suggestion${index === this._recipientIndex ? ' active' : ''}`
      option.id = `${listId}-${index}`
      option.setAttribute('role', 'option')
      option.setAttribute('aria-selected', String(index === this._recipientIndex))
      option.setAttribute('data-recipient-option', String(index))

      const avatar = document.createElement('span')
      avatar.className = 'recipient-suggestion-avatar'
      avatar.textContent = recipientInitials(suggestion)
      const copy = document.createElement('span')
      copy.className = 'recipient-suggestion-copy'
      const name = document.createElement('strong')
      name.textContent = suggestion.name || suggestion.email
      const detail = document.createElement('span')
      detail.textContent = suggestion.name ? suggestion.email : suggestion.source === 'mail' ? 'Found in mail' : 'Contact'
      copy.append(name, detail)
      option.append(avatar, copy)
      container.append(option)
    })
    container.hidden = false
    input.setAttribute('role', 'combobox')
    input.setAttribute('aria-autocomplete', 'list')
    input.setAttribute('aria-controls', listId)
    input.setAttribute('aria-expanded', 'true')
    input.setAttribute('aria-activedescendant', `${listId}-${this._recipientIndex}`)
  }

  _selectRecipient(index) {
    const suggestion = this._recipientSuggestions[index]
    const field = this._recipientField
    const input = this._recipientInput(field)
    if (!suggestion || !field || !input) return
    const parts = input.value.split(',')
    parts[parts.length - 1] = suggestion.email
    const value = parts.map(part => part.trim()).filter(Boolean).join(', ')
    input.value = value
    if (field === 'to') this.$to = value
    else if (field === 'cc') this.$cc = value
    else this.$bcc = value
    this._captureNativeFields()
    this.scheduleDraftSave()
    this._hideRecipientSuggestions()
    input.focus()
  }

  _hideRecipientSuggestions() {
    this._recipientRequest += 1
    this._recipientSuggestions = []
    this._recipientIndex = -1
    for (const container of document.querySelectorAll('[data-recipient-suggestions]')) {
      container.replaceChildren()
      if (container instanceof HTMLElement) container.hidden = true
    }
    for (const field of ['to', 'cc', 'bcc']) this._recipientInput(field)?.setAttribute('aria-expanded', 'false')
    this._recipientField = ''
  }

  _recipientInput(field) {
    if (!field) return null
    return /** @type {HTMLInputElement | null} */ (document.querySelector(`[data-recipient-field="${field}"] input`))
  }

  /** Debounce typing bursts while keeping a local recovery pointer immediately. */
  scheduleDraftSave(delay = 750) {
    if (!this.ready || this._intentionalExit || this.scheduledEditId) return
    this._persistDraftRecovery()
    if (this._draftSaving) { this._draftSaveAgain = true; return }
    clearTimeout(this._draftSaveTimer)
    this._setDraftStatus('Unsaved changes')
    this._draftSaveTimer = setTimeout(() => void this.saveDraft(), delay)
  }

  /** Persist one complete versioned snapshot to FYLO. */
  async saveDraft() {
    if (!this.ready || this.loading || this._intentionalExit || this.draftConflict || this.scheduledEditId) return
    if (this._draftSaving) { this._draftSaveAgain = true; return }
    this._syncFormValues()
    const snapshot = this._draftSnapshot(true)
    if (!this._hasMeaningfulDraft(snapshot) && !this.$draftId) return
    const fingerprint = JSON.stringify(snapshot)
    if (fingerprint === this._lastDraftSnapshot) { this._setDraftStatus('Saved'); return }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    const creating = !this.$draftId
    const requestPath = creating ? '/drafts' : `/drafts/${this.$draftId}`
    const requestMethod = creating ? 'POST' : 'PUT'
    const requestBody = {
      ...snapshot,
      clientId: this.$draftClientId,
      ...(!creating ? { version: this.$draftVersion } : {}),
    }
    this._draftSaving = true
    this._setDraftStatus('Saving…')
    try {
      const response = await apiFetch(requestPath, {
        method: requestMethod,
        body: JSON.stringify(requestBody),
      })
      const data = await response?.json()
      if (response?.status === 409 || data?.conflict) {
        this.draftConflict = true
        this._setDraftStatus('Newer saved version detected')
        this._persistDraftRecovery(this._draftSnapshot(false))
        await this._rerenderDraftUI()
        return
      }
      if (!response?.ok) {
        this._setDraftStatus(data?.error || 'Autosave failed')
        this._persistDraftRecovery(this._draftSnapshot(false))
        return
      }
      // The request was in flight while the user could keep typing. Capture
      // those live values before draft-id/version assignments trigger another
      // Tachyon render, then the queued follow-up save persists them.
      this._syncFormValues()
      this.$draftId = String(data.id)
      this.$draftClientId = String(data.draftClientId || this.$draftClientId)
      this.$draftVersion = Number(data.draftVersion || this.$draftVersion || 1)
      const serverFingerprint = JSON.stringify(this._snapshotFromSavedDraft(data))
      this._lastDraftSnapshot = serverFingerprint
      this._setDraftStatus('Saved')
      this.draftConflict = false
      this._persistDraftRecovery(this._draftSnapshot(false))
      const owner = String(window._caduceus?.auth?.email || '').toLowerCase()
      if (owner) void discardOfflineWrites(owner, `draft:${this.$draftClientId}`)
      // Only an idempotent create retry can legitimately return an older
      // server snapshot than the one the browser just submitted. Normal
      // create/update responses may normalize representation details, which
      // must not create a no-op autosave loop.
      if (data.recovered && serverFingerprint !== fingerprint) this._draftSaveAgain = true
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
    } catch {
      try {
        await window._caduceus?.queueOfflineRequest?.({
          kind: 'draft', path: requestPath, method: requestMethod, body: requestBody,
          message: 'Draft autosave', coalesceKey: `draft:${this.$draftClientId}`,
          metadata: { clientId: this.$draftClientId, draftId: this.$draftId },
        })
        this._setDraftStatus('Offline — encrypted autosave queued')
      } catch {
        this._setDraftStatus('Offline — changes kept in this session')
      }
      this._persistDraftRecovery(this._draftSnapshot(false))
    } finally {
      this._draftSaving = false
      // Draft id/version updates are reactive and may replace the
      // contenteditable after the request settles. Rehydrate from the captured
      // model so autosave is visually lossless.
      this._syncEditorContent()
      this._syncNativeFields()
      if (this._draftSaveAgain) {
        this._draftSaveAgain = false
        this.scheduleDraftSave(0)
      }
    }
  }

  _snapshotFromSavedDraft(draft) {
    return {
      from: draft.sender || this.$from,
      to: Array.isArray(draft.to) ? draft.to : [],
      cc: Array.isArray(draft.cc) ? draft.cc : [],
      bcc: Array.isArray(draft.bcc) ? draft.bcc : [],
      replyTo: Array.isArray(draft.replyTo) ? draft.replyTo : [],
      subject: draft.subject || '',
      text: draft.body || '',
      html: draft.htmlBody || '',
      ...(draft.inReplyTo ? { inReplyTo: draft.inReplyTo } : {}),
      references: Array.isArray(draft.references) ? draft.references : [],
      ...(draft.sourceEmailId ? { sourceEmailId: draft.sourceEmailId } : {}),
      attachments: Array.isArray(draft.attachments) ? draft.attachments.map(attachment => ({
        filename: attachment.filename,
        contentType: attachment.contentType,
        size: attachment.size,
        contentBase64: attachment.contentBase64,
        disposition: attachment.disposition || 'attachment',
        ...(attachment.contentId ? { contentId: attachment.contentId } : {}),
      })) : [],
    }
  }

  async reloadSavedDraft() {
    if (!this.$draftId || !confirm('Replace local changes with the newer saved draft?')) return
    if (!await this._loadDraft(this.$draftId)) return
    this.draftConflict = false
    this._setDraftStatus('Saved version loaded')
    await this._rerenderDraftUI()
  }

  async saveDraftCopy() {
    const previousClientId = this.$draftClientId
    const owner = String(window._caduceus?.auth?.email || '').toLowerCase()
    if (owner && previousClientId) await discardOfflineWrites(owner, `draft:${previousClientId}`)
    this.$draftId = ''
    this.$draftVersion = 0
    this.$draftClientId = crypto.randomUUID()
    this.draftConflict = false
    this._lastDraftSnapshot = ''
    await this.saveDraft()
    await this._rerenderDraftUI()
  }

  /** Load exact owned From identities and mailbox-owned signatures. */
  async loadSignatures() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const [signatureResponse, aliasResponse, externalResponse] = await Promise.all([
        apiFetch('/signatures'), apiFetch('/users/aliases'), apiFetch('/api/v1/external-accounts'),
      ])
      const signatures = signatureResponse?.ok ? await signatureResponse.json() : []
      const aliases = aliasResponse?.ok ? await aliasResponse.json() : {}
      const external = externalResponse?.ok ? await externalResponse.json() : {}
      const primary = String(aliases?.email || window._caduceus?.auth?.email || this.$from || '').toLowerCase()
      this.$externalSenderIdentities = externalSenderIdentities(external?.accounts)
      this.senderAddresses = [...new Set([primary, ...(Array.isArray(aliases?.aliases) ? aliases.aliases : []), ...this.$externalSenderIdentities.map(identity => identity.address)]
        .map(address => String(address || '').toLowerCase()).filter(Boolean))]
      if (!this.senderAddresses.includes(this.$from.toLowerCase())) this.$from = primary
      this.$signatures = Array.isArray(signatures) ? signatures.map(signature => ({
        ...signature,
        text: sanitizeSignatureHTML(signature.text),
        defaultFor: Array.isArray(signature.defaultFor) ? signature.defaultFor : [],
      })) : []
      window._caduceusSignatureCache = this.$signatures
    } catch { /* signatures unavailable */ }
  }

  senderIdentityLabel(address) {
    const external = this.$externalSenderIdentities.find(identity => identity.address === address)
    return external ? `${address} — ${external.label}` : address
  }

  /** Switch From identity and atomically replace its default signature. */
  changeSender(event) {
    const address = String(event?.target?.value || '').toLowerCase()
    if (!this.senderAddresses.includes(address)) return
    this._captureNativeFields()
    if (this._nativeFieldValues) {
      this.$cc = this._nativeFieldValues.cc
      this.$bcc = this._nativeFieldValues.bcc
    }
    this.$text = applySignature(this._getEditorHTML(), defaultSignatureForAddress(this.$signatures, address))
    this._editorHTML = this.$text
    this.$from = address
    this.applyAutomaticRecipients(address)
    if (this._nativeFieldValues) this._nativeFieldValues.from = address
    this.activeSignatureId = currentSignatureId(this.$text)
    this._syncEditorContent()
    this._syncNativeFields()
    this.scheduleDraftSave(0)
  }

  /** Apply exact-From automatic CC/BCC rules without duplicating recipients. */
  applyAutomaticRecipients(sender) {
    const preferences = this.mailPreferences
    const stripAndMerge = (current, previous, next) => {
      const previousSet = new Set(previous.map(value => value.toLowerCase()))
      const kept = String(current || '').split(',').map(value => value.trim()).filter(value => value && !previousSet.has(value.toLowerCase()))
      return [...new Set([...kept, ...next].map(value => value.toLowerCase()))].join(', ')
    }
    const normalized = String(sender || '').trim().toLowerCase()
    const nextCc = preferences.autoCcBySender[normalized] || []
    const nextBcc = preferences.autoBccBySender[normalized] || []
    this.$cc = stripAndMerge(this.$cc, this._automaticCc, nextCc)
    this.$bcc = stripAndMerge(this.$bcc, this._automaticBcc, nextBcc)
    this._automaticCc = [...nextCc]
    this._automaticBcc = [...nextBcc]
    if (this._nativeFieldValues) {
      this._nativeFieldValues.cc = this.$cc
      this._nativeFieldValues.bcc = this.$bcc
    }
  }

  /** Keep native editing semantics aligned with account spellcheck settings. */
  _applyEditorPreferences(editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))) {
    if (!editor) return
    const preferences = this.mailPreferences
    editor.spellcheck = preferences.spellcheck
    if (preferences.spellcheckLanguage) editor.lang = preferences.spellcheckLanguage
    else editor.removeAttribute('lang')
    const subjectHost = document.querySelector('[data-compose-subject]')
    const subject = /** @type {HTMLInputElement | null} */ (subjectHost?.matches('input') ? subjectHost : subjectHost?.querySelector('input'))
    if (subject) {
      subject.spellcheck = preferences.spellcheck
      if (preferences.spellcheckLanguage) subject.lang = preferences.spellcheckLanguage
      else subject.removeAttribute('lang')
    }
  }

  /** Select a named signature or remove the active signature. */
  changeSignature(event) {
    const id = String(event?.target?.value || '')
    const signature = this.$signatures.find(item => item.id === id) ?? null
    this.$text = applySignature(this._getEditorHTML(), signature)
    this._editorHTML = this.$text
    this.activeSignatureId = signature?.id || ''
    this._syncEditorContent()
    this._syncNativeFields()
    this.scheduleDraftSave(0)
  }

  manageSignatures() { window._caduceus?.navigate('settings') }

  /**
   * Push the current `$text` value into the rich-text editor's `innerHTML`.
   *
   * Scheduled on the next animation frame so the DOM element exists by the
   * time it is queried.
   *
   * @returns {void}
   */
  _syncEditorContent() {
    const apply = () => {
      const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
      if (editor) {
        this._liveEditor = editor
        const source = this._editorModelReady ? this._editorHTML : this.$text
        editor.innerHTML = editableComposerHTML(source, this.$attachments)
        decorateTemplateVariables(editor)
        if (this._selectedInlineAttachmentId) {
          this._selectInlineImage(/** @type {HTMLImageElement | null} */ (editor.querySelector(`img[data-caduceus-attachment-id="${CSS.escape(this._selectedInlineAttachmentId)}"]`)))
        }
        this._updateTemplateStatus()
        this._renderQuotedTextControl()
      }
    }
    // Apply immediately. The replacement observer owns any later Tachyon node
    // swap; repeated RAF writes here would invalidate a live caret or template
    // variable selection.
    apply()
  }

  /**
   * Read the current HTML content from the rich-text editor.
   *
   * Falls back to the reactive `$text` field if the editor element is not in
   * the DOM.
   *
   * @returns {string} The editor's innerHTML or the `$text` fallback
   */
  _getEditorHTML() {
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    if (!this.mailPreferences.richText) return editor?.innerText || editor?.textContent || this.$text || ''
    if (editor && (editor === this._liveEditor || editor.innerHTML)) return serializeComposerHTML(editor)
    return this._editorModelReady ? this._editorHTML : (this.$text || '')
  }

  /** Read compose fields from DuVay's native controls before an action. */
  _syncFormValues() {
    const value = (selector, fallback) => {
      const host = document.querySelector(selector)
      const control = host?.matches('input, textarea, select') ? host : host?.querySelector('input, textarea, select')
      return String(/** @type {HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null} */ (control)?.value ?? fallback ?? '').trim()
    }
    // Read every DOM value before assigning any reactive property: each
    // assignment may synchronously render and replace the remaining controls.
    const snapshot = {
      // `changeSender` owns and validates this identity. A delayed render can
      // briefly replace the native select with its first option, so reading
      // that transient DOM value here could silently revert an owned alias at
      // send time even though the reactive and draft state still hold it.
      from: this.$from,
      to: value('[data-compose-to]', this.$to),
      cc: value('[data-compose-cc]', this.$cc),
      bcc: value('[data-compose-bcc]', this.$bcc),
      replyTo: value('[data-compose-reply-to]', this.$replyTo),
      subject: value('[data-compose-subject]', this.$subject),
      text: this._getEditorHTML(),
    }
    this._nativeFieldValues = {
      from: snapshot.from, to: snapshot.to, cc: snapshot.cc, bcc: snapshot.bcc, replyTo: snapshot.replyTo, subject: snapshot.subject,
    }
    this.$from = snapshot.from
    this.$to = snapshot.to
    this.$cc = snapshot.cc
    this.$bcc = snapshot.bcc
    this.$replyTo = snapshot.replyTo
    this.$subject = snapshot.subject
    this.$text = snapshot.text
    this._editorHTML = snapshot.text
    this._editorModelReady = true
    // The assignments above may synchronously replace the contenteditable.
    // Restore the captured HTML after that render boundary.
    this._syncEditorContent()
  }

  /**
   * Preserve every compose control around Tachyon's reactive checkbox render.
   * Without this boundary, changing a tracking option replaces the rich-text
   * node and can discard a message the user already wrote.
   *
   * @param {Event} event
   */
  changeTrackingOption(event) {
    const input = /** @type {HTMLInputElement} */ (event.target)
    const openTracking = input.matches('[data-open-tracking]')
    const linkTracking = input.matches('[data-link-tracking]')
    if (!openTracking && !linkTracking) return
    this._syncFormValues()
    if (openTracking) this.requestReadReceipt = input.checked
    else this.trackLinks = input.checked
    this._syncEditorContent()
    this._syncNativeFields()
    this.scheduleDraftSave(0)
  }

  /**
   * Read the current plain-text content from the rich-text editor.
   *
   * Uses `innerText` (preferred) or `textContent` for cross-browser support.
   *
   * @returns {string} The editor's plain-text content or the `$text` fallback
   */
  _getEditorText() {
    const editor = document.querySelector('[data-compose-editor]')
    return editor ? (editor.textContent || '') : (this.$text || '')
  }

  /**
   * Clear the rich-text editor content and reset the reactive text field.
   *
   * @returns {void}
   */
  _clearEditor() {
    const editor = document.querySelector('[data-compose-editor]')
    if (editor) editor.innerHTML = ''
    this.$text = ''
    this._editorHTML = ''
    this._editorModelReady = true
  }

  /** Permanently discard the server draft and its local recovery pointer. */
  async discard() {
    this._intentionalExit = true
    clearTimeout(this._draftSaveTimer)
    const apiFetch = window._caduceus?.apiFetch
    if (this.$draftId && apiFetch) {
      try { await apiFetch(`/drafts/${this.$draftId}`, { method: 'DELETE' }) } catch {}
    }
    this._clearDraftRecovery()
    window._caduceus?.navigate(this.scheduledEditId ? 'scheduled' : 'inbox')
  }

  /** Apply semantic inline formatting without deprecated execCommand APIs. */
  execBold() {
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 'strong')) this._editorChanged()
  }

  execItalic() {
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 'em')) this._editorChanged()
  }

  execUnderline() {
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 'u')) this._editorChanged()
  }

  execStrike() {
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 's')) this._editorChanged()
  }

  execCode() {
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 'code')) this._editorChanged()
  }

  execBulletList() {
    const editor = this._restoreEditorSelection()
    if (editor && applyListFormat(editor, false)) this._editorChanged()
  }

  execNumberedList() {
    const editor = this._restoreEditorSelection()
    if (editor && applyListFormat(editor, true)) this._editorChanged()
  }

  execLink() {
    const editor = this._restoreEditorSelection()
    if (!editor) return
    const url = normalizeComposeLink(prompt('Enter a web or email URL:') || '')
    if (!url) {
      window._caduceus?.toast('Use an http, https, or mailto link.')
      return
    }
    if (applyInlineFormat(editor, 'a', { })) {
      const selection = getSelection()
      const anchor = selection?.anchorNode?.parentElement?.closest?.('a')
      if (anchor && editor.contains(anchor)) {
        anchor.href = url
        anchor.rel = 'noopener noreferrer'
        anchor.target = '_blank'
      }
      this._editorChanged()
    }
  }

  changeBlockStyle(event) {
    const value = String(event?.target?.value || 'p')
    if (!['p', 'h1', 'h2', 'h3', 'blockquote', 'pre'].includes(value)) return
    const editor = this._restoreEditorSelection()
    if (editor && applyBlockFormat(editor, value)) this._editorChanged()
    event.target.value = 'p'
  }

  changeFontFamily(event) {
    const value = String(event?.target?.value || '')
    const allowed = ['Arial, sans-serif', 'Georgia, serif', 'Tahoma, sans-serif', 'Times New Roman, serif', 'Verdana, sans-serif', 'monospace']
    if (!allowed.includes(value)) return
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 'span', { 'font-family': value })) this._editorChanged()
  }

  changeFontSize(event) {
    const value = String(event?.target?.value || '')
    if (!['12px', '14px', '16px', '18px', '24px', '32px'].includes(value)) return
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 'span', { 'font-size': value })) this._editorChanged()
  }

  changeTextColor(event) {
    const value = String(event?.target?.value || '')
    if (!/^#[0-9a-f]{6}$/i.test(value)) return
    const editor = this._restoreEditorSelection()
    if (editor && applyInlineFormat(editor, 'span', { color: value })) this._editorChanged()
  }

  toggleRTL() {
    const editor = this._restoreEditorSelection()
    if (!editor) return
    const selection = getSelection()
    const current = selection?.anchorNode?.parentElement?.closest?.('[dir]')
    const rtl = current?.getAttribute('dir') !== 'rtl'
    if (applyBlockFormat(editor, current?.tagName?.toLowerCase() || 'p', { dir: rtl ? 'rtl' : '' })) this._editorChanged()
  }

  _renderQuotedTextControl() {
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    const control = /** @type {HTMLElement | null} */ (document.querySelector('[data-quoted-text-control]'))
    if (!editor || !control) return
    const quote = editor.querySelector('[data-caduceus-quote]')
    control.hidden = !quote
    if (!quote) {
      editor.classList.remove('compose-quote-collapsed')
      return
    }
    editor.classList.toggle('compose-quote-collapsed', !this._quoteExpanded)
    if (this._quoteExpanded) editor.setAttribute('data-quote-expanded', 'true')
    else editor.removeAttribute('data-quote-expanded')
    const toggle = control.querySelector('[data-toggle-quote]')
    if (toggle) {
      const expanded = !editor.classList.contains('compose-quote-collapsed')
      toggle.textContent = expanded ? 'Hide quoted text' : 'Show quoted text'
      toggle.setAttribute('aria-expanded', String(expanded))
    }
  }

  toggleQuotedText() {
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    if (!editor?.querySelector('[data-caduceus-quote]')) return
    this._quoteExpanded = !this._quoteExpanded
    this._renderQuotedTextControl()
  }

  removeQuotedText() {
    const editor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    const quote = editor?.querySelector('[data-caduceus-quote]')
    if (!editor || !quote) return
    quote.remove()
    this._quoteExpanded = false
    editor.classList.remove('compose-quote-collapsed')
    editor.removeAttribute('data-quote-expanded')
    this._editorChanged()
  }

  /**
   * Open the native file picker for outbound attachments.
   *
   * @returns {void}
   */
  pickAttachments() {
    const input = document.querySelector('[data-attachment-input]')
    if (input) /** @type {HTMLInputElement} */ (input).click()
  }

  pickInlineImages() {
    const input = document.querySelector('[data-inline-image-input]')
    if (input) /** @type {HTMLInputElement} */ (input).click()
  }

  /**
   * Add files from the native picker to the pending send payload.
   *
   * @param {Event} event
   * @returns {Promise<void>}
   */
  async addAttachments(event) {
    const input = /** @type {HTMLInputElement} */ (event.target)
    const files = Array.from(input.files || [])
    input.value = ''
    await this.addAttachmentFiles(files)
  }

  async addInlineImages(event) {
    const input = /** @type {HTMLInputElement} */ (event.target)
    const files = Array.from(input.files || [])
    input.value = ''
    await this.addAttachmentFiles(files, { inlineImages: true })
  }

  /** Apply one attachment policy to picker, paste, and drag/drop inputs. */
  async addAttachmentFiles(files, options = {}) {
    if (files.length === 0) return
    const inlineImages = options.inlineImages === true
    const existingHTML = this._getEditorHTML()
    const currentSize = this.$attachments.reduce((sum, item) => sum + item.size, 0)
    let nextSize = currentSize
    const additions = []
    for (const file of files) {
      if (this.$attachments.length + additions.length >= 10) {
        window._caduceus?.toast('You can attach up to 10 files.')
        break
      }
      if (file.size > 10 * 1024 * 1024) {
        window._caduceus?.toast(`${file.name} is larger than 10MB.`)
        continue
      }
      if (nextSize + file.size > 25 * 1024 * 1024) {
        window._caduceus?.toast('Attachments cannot exceed 25MB total.')
        continue
      }
      try {
        const inline = inlineImages && isInlineImageFile(file)
        if (inlineImages && !inline) {
          window._caduceus?.toast(`${file.name} is not a supported inline image.`)
          continue
        }
        additions.push({
          id: crypto.randomUUID(),
          filename: file.name,
          contentType: file.type || 'application/octet-stream',
          size: file.size,
          contentBase64: await this._fileToBase64(file),
          disposition: inline ? 'inline' : 'attachment',
          ...(inline ? { contentId: createInlineContentId() } : {}),
        })
      } catch {
        window._caduceus?.toast(`${file.name} could not be read.`)
        continue
      }
      nextSize += file.size
    }
    if (!additions.length) return
    const liveEditor = /** @type {HTMLElement | null} */ (document.querySelector('[data-compose-editor]'))
    if (liveEditor) {
      // An autosave may have settled while FileReader was running. Restore the
      // captured draft before inserting, then serialize once after all images.
      if (!liveEditor.innerHTML && existingHTML) liveEditor.innerHTML = existingHTML
      for (const addition of additions.filter(item => item.disposition === 'inline')) insertInlineImage(liveEditor, addition)
      this.$text = serializeComposerHTML(liveEditor)
      this._editorHTML = this.$text
      this._editorModelReady = true
    }
    this.$attachments = [...this.$attachments, ...additions]
    this._syncEditorContent()
    this.scheduleDraftSave(0)
  }

  onAttachmentDragOver(event) {
    if (!Array.from(event?.dataTransfer?.types || []).includes('Files')) return
    event.preventDefault()
    if (!this.attachmentDragActive) this.attachmentDragActive = true
  }

  onAttachmentDragLeave(event) {
    const current = /** @type {Node | null} */ (event?.currentTarget || null)
    const related = /** @type {Node | null} */ (event?.relatedTarget || null)
    if (current && related && current.contains(related)) return
    this.attachmentDragActive = false
  }

  async dropAttachments(event) {
    event?.preventDefault?.()
    this.attachmentDragActive = false
    const files = transferFiles(event?.dataTransfer)
    const images = files.filter(isInlineImageFile)
    const attachments = files.filter(file => !isInlineImageFile(file))
    if (images.length) await this.addAttachmentFiles(images, { inlineImages: true })
    if (attachments.length) await this.addAttachmentFiles(attachments)
  }

  async pasteAttachments(event) {
    const files = transferFiles(event?.clipboardData)
    if (!files.length) {
      const text = String(event?.clipboardData?.getData('text/plain') || '')
      if (!this.mailPreferences.richText) {
        if (!text) return
        event.preventDefault()
        const selection = getSelection()
        if (!selection?.rangeCount) return
        const range = selection.getRangeAt(0)
        range.deleteContents()
        const node = document.createTextNode(text)
        range.insertNode(node)
        range.setStartAfter(node)
        range.collapse(true)
        selection.removeAllRanges()
        selection.addRange(range)
        this._editorChanged()
      } else {
        const html = String(event?.clipboardData?.getData('text/html') || '')
        if (!html && !text) return
        const editor = event?.currentTarget instanceof HTMLElement
          ? event.currentTarget
          : document.querySelector('[data-compose-editor]')
        if (!(editor instanceof HTMLElement)) return
        event.preventDefault()
        if (insertPastedComposerContent(editor, html, text)) this._editorChanged()
      }
      return
    }
    event.preventDefault()
    const images = files.filter(isInlineImageFile)
    const attachments = files.filter(file => !isInlineImageFile(file))
    if (images.length) await this.addAttachmentFiles(images, { inlineImages: true })
    if (attachments.length) await this.addAttachmentFiles(attachments)
  }

  /**
   * Remove a pending attachment from the compose form.
   *
   * @param {Event | null} _event
   * @param {string} id
   * @returns {void}
   */
  removeAttachment(_event, id) {
    const attachment = this.$attachments.find(item => item.id === id)
    const editor = document.querySelector('[data-compose-editor]')
    if (attachment?.contentId && editor) {
      for (const image of editor.querySelectorAll('img[data-caduceus-cid]')) {
        if ((/** @type {HTMLElement} */ (image)).dataset.caduceusCid === attachment.contentId) image.remove()
      }
      this.$text = this._getEditorHTML()
      this._editorHTML = this.$text
    }
    this.$attachments = this.$attachments.filter(item => item.id !== id)
    this.scheduleDraftSave(0)
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  resizeSelectedInlineImage(_event, width) {
    if (!this._selectedInlineImage || !resizeInlineImage(this._selectedInlineImage, Number(width))) return
    this._editorChanged()
  }

  removeSelectedInlineImage() {
    const image = this._selectedInlineImage
    if (!image) return
    const id = image.dataset.caduceusAttachmentId
    this._selectInlineImage(null)
    if (id) this.removeAttachment(null, id)
    else { image.remove(); this._editorChanged() }
  }

  /**
   * @param {File} file
   * @returns {Promise<string>}
   */
  _fileToBase64(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => {
        const result = String(reader.result || '')
        resolve(result.includes(',') ? result.split(',').pop() || '' : result)
      }
      reader.onerror = () => reject(reader.error)
      reader.readAsDataURL(file)
    })
  }

  /**
   * @param {number} size
   * @returns {string}
   */
  formatAttachmentSize(size) {
    if (size < 1024) return `${size} B`
    if (size < 1024 * 1024) return `${Math.round(size / 1024)} KB`
    return `${(size / 1024 / 1024).toFixed(1)} MB`
  }

  /**
   * Choose when this conversation should resurface if nobody replies.
   * The server owns reply detection and durable delivery of the reminder.
   */
  async chooseFollowUpReminder() {
    this._syncFormValues()
    this.reminderPickerOpen = !this.reminderPickerOpen
    await this._restoreComposeAfterOptionRender()
  }

  /** @param {Event} _event @param {'tomorrow'|'three-days'|'next-week'} preset */
  async setFollowUpPreset(_event, preset) {
    let date
    if (preset === 'tomorrow') {
      date = new Date()
      date.setDate(date.getDate() + 1)
      date.setHours(9, 0, 0, 0)
    } else {
      date = new Date(Date.now() + (preset === 'three-days' ? 3 : 7) * 24 * 60 * 60 * 1000)
    }
    await this._commitFollowUpReminder(date)
  }

  async setCustomFollowUpReminder() {
    const input = /** @type {HTMLInputElement | null} */ (document.querySelector('[data-follow-up-custom]'))
    await this._commitFollowUpReminder(new Date(String(input?.value || '')))
  }

  /** @param {Date} date */
  async _commitFollowUpReminder(date) {
    this._syncFormValues()
    if (!Number.isFinite(date.getTime()) || date.getTime() <= Date.now()) {
      window._caduceus?.toast('Please choose a future reminder time.')
      await this._restoreComposeAfterOptionRender()
      return
    }
    this.$remindAt = date.toISOString()
    this.reminderPickerOpen = false
    await this._restoreComposeAfterOptionRender()
    this.scheduleDraftSave(0)
  }

  async closeFollowUpReminder() {
    this._syncFormValues()
    this.reminderPickerOpen = false
    await this._restoreComposeAfterOptionRender()
  }

  async clearFollowUpReminder() {
    this._syncFormValues()
    this.$remindAt = ''
    this.reminderPickerOpen = false
    await this._restoreComposeAfterOptionRender()
    this.scheduleDraftSave(0)
  }

  async _restoreComposeAfterOptionRender() {
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    this._syncEditorContent()
    this._syncNativeFields()
  }

  get followUpReminderLabel() {
    return this.$remindAt
      ? `Remind ${formatPreferenceDate(this.$remindAt)}`
      : 'Remind me if no reply'
  }

  /**
   * Fetch saved message templates from the `/templates` API.
   *
   * @async
   * @returns {Promise<void>}
   */
  async loadTemplates() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const res = await apiFetch('/templates')
      if (res?.ok) {
        const templates = await res.json()
        this.$templates = Array.isArray(templates) ? templates : []
        window._caduceusTemplateCache = this.$templates
        requestAnimationFrame(() => this._renderTemplateOptions())
      }
    } catch { /* templates unavailable */ }
  }

  /** Paint API-backed options into DuVay's native select after async startup. */
  _renderTemplateOptions(query = '') {
    const host = document.querySelector('.compose-template-select')
    if (!host) return
    for (const option of host.querySelectorAll('w-option[data-template-dynamic]')) option.remove()
    const templates = window._caduceusTemplateCache || this.$templates
    for (const template of templates.filter(item => !query || item.name.toLocaleLowerCase().startsWith(query))) {
      const option = document.createElement('w-option')
      option.setAttribute('value', template.id)
      option.setAttribute('data-template-dynamic', '')
      option.textContent = template.name
      host.append(option)
    }
  }

  /**
   * Save the current compose form as a reusable template.
   *
   * Prompts the user for a template name, then persists the current To, CC,
   * Subject, and editor HTML to the `/templates` API.
   *
   * @async
   * @returns {Promise<void>}
   */
  async saveTemplate() {
    this._syncFormValues()
    const content = templateBodyFromDraft(this._getEditorHTML())
    if (!content) {
      window._caduceus?.toast('Add message content before saving a template.')
      return
    }
    const suggestedName = this.$subject.replace(/[^\p{L}\p{N}_\- ]+/gu, '').trim().slice(0, 100)
    const name = prompt('Template name:', suggestedName)
    if (!name || !name.trim()) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const res = await apiFetch('/templates', { method: 'POST', body: JSON.stringify({ name: name.trim(), subject: this.$subject, text: content, to: this.$to, cc: this.$cc }) })
      if (res?.ok) {
        const saved = await res.json()
        window._caduceus?.toast(`Template “${saved.name}” saved.`)
        await this.loadTemplates()
      } else {
        window._caduceus?.toast('Failed to save template.')
      }
    } catch { window._caduceus?.toast('Network error.') }
  }

  /**
   * Load a selected template's values into the compose form.
   *
   * When the form already has content, the user is prompted to confirm before
   * the existing fields are overwritten.
   *
   * @param {Event} event - The change event from the template `<select>` element
   * @returns {void}
   */
  loadTemplate(event) {
    const id = /** @type {HTMLInputElement} */ (event?.target)?.value
    if (!id) return
    const tpl = (window._caduceusTemplateCache || this.$templates).find(t => t.id === id)
    if (!tpl) return
    const currentHTML = this._getEditorHTML()
    const protectedOnly = templateBodyFromDraft(currentHTML) === ''
    if (this.$to || this.$subject || (!protectedOnly && this._getEditorText().trim())) {
      if (!confirm('Loading a template will replace the editable draft contents. Signature and quoted mail will be kept. Continue?')) {
        const targetEl2 = /** @type {HTMLInputElement} */ (event.target);
        targetEl2.value = '';
        return
      }
    }
    this.$to = tpl.to || this.$to
    this.$cc = tpl.cc || this.$cc
    this.$subject = tpl.subject || this.$subject
    this.$text = insertTemplateBeforeProtectedContent(tpl.text || '', currentHTML)
    this._editorHTML = this.$text
    this._rememberReactiveFields()
    this.activeTemplateName = tpl.name
    const targetEl = /** @type {HTMLInputElement} */ (event.target);
    targetEl.value = '';
    this._syncEditorContent()
    this._syncNativeFields()
    this.scheduleDraftSave(0)
    requestAnimationFrame(() => {
      const first = document.querySelector('[data-compose-editor] code.var, [data-compose-editor] code[data-template-var]')
      if (first) selectTemplateVariable(first)
    })
  }

  /** Prefix-filter the existing picker options without rerendering the editor. */
  filterTemplates(event) {
    const query = String(event?.target?.value || '').trim().toLocaleLowerCase()
    this._renderTemplateOptions(query)
  }

  manageTemplates() { window._caduceus?.navigate('settings') }

  _updateTemplateStatus() {
    const editor = document.querySelector('[data-compose-editor]')
    const status = /** @type {HTMLElement | null} */ (document.querySelector('[data-template-status]'))
    if (!status || !editor) return
    const variables = decorateTemplateVariables(editor)
    const empty = variables.filter(node => node.classList.contains('empty')).length
    status.hidden = variables.length === 0
    status.textContent = variables.length
      ? `Template blanks: ${variables.length}${empty ? ` (${empty} empty)` : ''}. Press Tab or Shift+Tab to move between blanks.`
      : ''
  }

  /** Display Mailspring's empty-area warning and focus the first blank. */
  _confirmCompleteTemplate() {
    const html = this.$text || this._getEditorHTML()
    if (!hasEmptyTemplateVariables(html)) return true
    const proceed = confirm('The message contains an empty template area. Send anyway?')
    if (proceed) return true
    this.$error = 'Fill every highlighted template blank before sending.'
    const first = document.querySelector('[data-compose-editor] code.empty[data-template-var]')
    if (first) selectTemplateVariable(first)
    return false
  }

  /**
   * Schedule the email to be sent at a future time.
   *
   * Presents the user with preset times (tomorrow morning/afternoon, next
   * Monday) or a custom date/time input. The chosen time is sent as the
   * `sendAt` field in the payload.
   *
   * @async
   * @returns {Promise<void>}
   */
  async chooseSendLater() {
    this._syncFormValues()
    if (!this.$to || !this.$subject) { this.$error = 'To and Subject are required.'; return }
    if (!this._confirmCompleteTemplate()) return
    this.schedulePickerOpen = !this.schedulePickerOpen
    this.reminderPickerOpen = false
    await this._restoreComposeAfterOptionRender()
  }

  /** @param {Event} _event @param {'tomorrow-morning'|'tomorrow-afternoon'|'next-monday'} preset */
  async setSendLaterPreset(_event, preset) {
    const date = new Date()
    if (preset === 'next-monday') {
      date.setDate(date.getDate() + ((8 - date.getDay()) % 7 || 7))
      date.setHours(8, 0, 0, 0)
    } else {
      date.setDate(date.getDate() + 1)
      date.setHours(preset === 'tomorrow-afternoon' ? 14 : 8, 0, 0, 0)
    }
    await this._commitScheduledSend(date)
  }

  async setCustomSendLater() {
    const input = /** @type {HTMLInputElement | null} */ (document.querySelector('[data-send-later-custom]'))
    await this._commitScheduledSend(new Date(String(input?.value || '')))
  }

  async closeSendLater() {
    this._syncFormValues()
    this.schedulePickerOpen = false
    await this._restoreComposeAfterOptionRender()
  }

  /** @param {Date} date */
  async _commitScheduledSend(date) {
    this._syncFormValues()
    if (!Number.isFinite(date.getTime()) || date.getTime() <= Date.now()) {
      window._caduceus?.toast('Please choose a future time.')
      await this._restoreComposeAfterOptionRender()
      return
    }
    const sendAt = date.toISOString()
    const payload = this._buildPayload({
      sendAt,
      idempotencyKey: `send:${crypto.randomUUID()}`,
      ...(this.scheduledEditId ? {
        remindAt: this.$remindAt || null,
        replyTo: this.$replyTo ? [this.$replyTo] : null,
        openTracking: this.requestReadReceipt,
        linkTracking: this.trackLinks,
      } : {}),
    })
    this.loading = true; this.$error = ''
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.loading = false; return }
    try {
      const editing = Boolean(this.scheduledEditId)
      const res = await apiFetch(editing ? `/send/scheduled/${this.$scheduledEditId}` : '/send', { method: editing ? 'PUT' : 'POST', body: JSON.stringify(payload) })
      if (res?.ok) {
        this.$to = this.$cc = this.$bcc = this.$replyTo = this.$subject = this.$remindAt = ''; this.$inReplyTo = ''; this.$references = []; this.$attachments = []; this.requestReadReceipt = false; this.trackLinks = false; this._rememberReactiveFields(); this._clearEditor()
        if (!editing) await this._consumeDraft()
        window._caduceus?.toast((editing ? 'Schedule updated for ' : 'Scheduled for ') + formatPreferenceDate(sendAt))
        window._caduceus?.navigate('scheduled')
      } else {
        const data = await res?.json(); this.$error = data?.error || 'Schedule failed.'
      }
    } catch {
      try {
        const editing = Boolean(this.scheduledEditId)
        await window._caduceus?.queueOfflineRequest?.({
          kind: 'send',
          path: editing ? `/send/scheduled/${this.$scheduledEditId}` : '/send',
          method: editing ? 'PUT' : 'POST',
          body: payload,
          message: editing ? 'Scheduled-send update' : 'Scheduled send',
          coalesceKey: editing ? `scheduled:${this.$scheduledEditId}` : undefined,
          metadata: { scheduled: true, editing },
        })
        this.$to = this.$cc = this.$bcc = this.$replyTo = this.$subject = this.$remindAt = ''; this.$inReplyTo = ''; this.$references = []; this.$attachments = []; this.requestReadReceipt = false; this.trackLinks = false; this._rememberReactiveFields(); this._clearEditor()
        if (!editing) await this._consumeDraft()
        window._caduceus?.navigate('scheduled')
      } catch { this.$error = 'Network error.' }
    }
    finally { this.loading = false }
  }

  get scheduleTimeZoneLabel() {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'local time'
  }

  get scheduledSendLabel() {
    return formatPreferenceDate(this.scheduledSendAt)
  }

  /**
   * Bind global keyboard-shortcut listeners for the composer.
   *
   * - `caduceus:shortcut:composer:send` triggers {@link send}
   * - `caduceus:shortcut:composer:discard` discards the working draft
   *
   * @returns {void}
   */
  bindShortcuts() {
    window.addEventListener('caduceus:shortcut:composer:send', () => this.sendDefault())
    window.addEventListener('caduceus:shortcut:composer:discard', () => {
      void this.discard()
    })
  }


  /**
   * Build a clean send request. The server performs per-recipient tracking
   * transformation only at the SMTP boundary; drafts and Sent copies never
   * contain capability URLs.
   *
   * @param {Record<string, any>} extra - Extra fields to merge into the payload
   * @returns {Record<string, any>}
   */
  _buildPayload(extra = {}) {
    const split = s => s.split(',').map(x => x.trim()).filter(Boolean)
    // `_syncFormValues()` snapshots the editor before `$loading` triggers a
    // Tachyon render, so the send payload never reads from a recreated node.
    const html = stripTemplateVariableMarkup(this.$text || this._getEditorHTML())
    return {
      from: this.$from,
      to: split(this.$to),
      cc: split(this.$cc),
      bcc: split(this.$bcc),
      ...(this.$replyTo ? { replyTo: [this.$replyTo] } : {}),
      ...(this.$inReplyTo ? { inReplyTo: this.$inReplyTo } : {}),
      ...(this.$references.length ? { references: this.$references } : {}),
      subject: this.$subject,
      ...(this.mailPreferences.richText ? { html } : {}),
      text: this._getEditorText(),
      ...(this.$attachments.length ? {
        attachments: this.$attachments.map(({ filename, contentType, contentBase64, disposition, contentId }) => ({
          filename,
          contentType,
          contentBase64,
          disposition: disposition || 'attachment',
          ...(contentId ? { contentId } : {}),
        }))
      } : {}),
      ...(this.requestReadReceipt ? { openTracking: true } : {}),
      ...(this.trackLinks ? { linkTracking: true } : {}),
      ...(this.$remindAt ? { remindAt: this.$remindAt } : {}),
      ...extra,
    }
  }

  /**
   * After a draft is sent, remove its working copy and recovery pointer so it
   * cannot linger as a duplicate.
   *
   * @returns {Promise<void>}
   */
  async _consumeDraft() {
    const id = this.$draftId
    const clientId = this.$draftClientId
    this._intentionalExit = true
    clearTimeout(this._draftSaveTimer)
    this.$draftId = ''
    this._clearDraftRecovery()
    const owner = String(window._caduceus?.auth?.email || '').toLowerCase()
    if (owner && clientId) await discardOfflineWrites(owner, `draft:${clientId}`)
    if (!id) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      await apiFetch(`/drafts/${id}`, { method: 'DELETE' })
    } catch {
      await window._caduceus?.queueOfflineRequest?.({
        kind: 'draft', path: `/drafts/${id}`, method: 'DELETE', body: null,
        message: 'Discard sent draft', coalesceKey: `draft-delete:${id}`, metadata: { draftId: id },
      }).catch(() => {})
    }
  }

  /**
   * Send the email immediately.
   *
   * Validates that To and Subject are filled, builds the payload (with a 10 s
   * delay for undo support), posts to `/send`, and clears the form on success.
   * Displays an undo toast so the user can recall the message within the delay
   * window.
   *
   * @async
   * @returns {Promise<void>}
   */
  async send() {
    this._syncFormValues()
    if (!this.$to || !this.$subject) { this.$error = 'To and Subject are required.'; return }
    if (!this._confirmCompleteTemplate()) return
    void window._caduceus?.playSound?.('hit-send')
    const payload = this._buildPayload({
      delayMs: this.mailPreferences.undoSendDelay,
      idempotencyKey: `send:${crypto.randomUUID()}`,
    })
    this.loading = true; this.$error = ''
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.loading = false; return }
    try {
      const res = await apiFetch('/send', { method: 'POST', body: JSON.stringify(payload) })
      if (res?.ok) {
        const data = await res.json()
        void window._caduceus?.playSound?.('send')
        this.$to = this.$cc = this.$bcc = this.$replyTo = this.$subject = this.$remindAt = ''; this.$inReplyTo = ''; this.$references = []; this.$attachments = []; this.requestReadReceipt = false; this.trackLinks = false; this._rememberReactiveFields(); this._clearEditor()
        await this._consumeDraft()
        window._caduceus?.navigate('inbox')
        this._showUndoToast(data.undoId, 'Message sent. Undo')
      } else { const data = await res?.json(); this.$error = data?.error || 'Send failed.' }
    } catch {
      try {
        await window._caduceus?.queueOfflineRequest?.({
          kind: 'send', path: '/send', method: 'POST', body: payload,
          message: 'Send message', metadata: { draftId: this.$draftId, draftClientId: this.$draftClientId },
        })
        this.$to = this.$cc = this.$bcc = this.$replyTo = this.$subject = this.$remindAt = ''; this.$inReplyTo = ''; this.$references = []; this.$attachments = []; this.requestReadReceipt = false; this.trackLinks = false; this._rememberReactiveFields(); this._clearEditor()
        await this._consumeDraft()
        window._caduceus?.navigate('inbox')
      } catch { this.$error = 'Network error.' }
    }
    finally { this.loading = false }
  }

  /**
   * Send the email immediately AND archive the thread in one action.
   *
   * Same as {@link send} but includes `archive: true` in the payload.
   *
   * @async
   * @returns {Promise<void>}
   */
  async sendAndArchive() {
    this._syncFormValues()
    if (!this.$to || !this.$subject) { this.$error = 'To and Subject are required.'; return }
    if (!this._confirmCompleteTemplate()) return
    if (!this.$sourceEmailId) { this.$error = 'Send & Archive is available only from a source conversation.'; return }
    void window._caduceus?.playSound?.('hit-send')
    const payload = this._buildPayload({
      archive: true,
      archiveSourceId: this.$sourceEmailId,
      delayMs: this.mailPreferences.undoSendDelay,
      idempotencyKey: `send:${crypto.randomUUID()}`,
    })
    this.loading = true; this.$error = ''
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) { this.loading = false; return }
    try {
      const res = await apiFetch('/send', { method: 'POST', body: JSON.stringify(payload) })
      if (res?.ok) {
        const data = await res.json()
        void window._caduceus?.playSound?.('send')
        this.$to = this.$cc = this.$bcc = this.$replyTo = this.$subject = this.$remindAt = ''; this.$inReplyTo = ''; this.$references = []; this.$attachments = []; this.requestReadReceipt = false; this.trackLinks = false; this._rememberReactiveFields(); this._clearEditor()
        await this._consumeDraft()
        window._caduceus?.navigate('inbox')
        this._showUndoToast(data.undoId, data.archived ? 'Sent & archived. Undo' : 'Message sent; source was not archived. Undo')
      } else { const data = await res?.json(); this.$error = data?.error || 'Send failed.' }
    } catch {
      try {
        await window._caduceus?.queueOfflineRequest?.({
          kind: 'send', path: '/send', method: 'POST', body: payload,
          message: 'Send and archive message', metadata: { draftId: this.$draftId, draftClientId: this.$draftClientId },
        })
        this.$to = this.$cc = this.$bcc = this.$replyTo = this.$subject = this.$remindAt = ''; this.$inReplyTo = ''; this.$references = []; this.$attachments = []; this.requestReadReceipt = false; this.trackLinks = false; this._rememberReactiveFields(); this._clearEditor()
        await this._consumeDraft()
        window._caduceus?.navigate('inbox')
      } catch { this.$error = 'Network error.' }
    }
    finally { this.loading = false }
  }

  sendDefault() {
    if (this.mailPreferences.defaultSendAction === 'send-and-archive' && this.$sourceEmailId) return this.sendAndArchive()
    return this.send()
  }

  /**
   * Display an undo toast that lets the user recall a sent message.
   *
   * If no `undoId` is provided, fall back to a plain toast. The toast is
   * automatically dismissed after 10 seconds.
   *
   * @param {string} undoId - The server-side undo identifier for the sent message
   * @param {string} label - Label text for the toast button
   * @returns {void}
   */
  _showUndoToast(undoId, label) {
    if (!undoId) {
      window._caduceus?.toast(label.replace('Undo', '').trim())
      return
    }
    sessionStorage.setItem('caduceus_pending_send_undo', JSON.stringify({
      undoId,
      label,
      owner: window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || '',
      expiresAt: Date.now() + this.mailPreferences.undoSendDelay,
    }))
    window.dispatchEvent(new Event('caduceus:pending-send-undo'))
  }
}

function recipientFieldForEvent(event) {
  const target = event.target
  if (!(target instanceof HTMLInputElement)) return ''
  const field = target.closest('[data-recipient-field]')?.getAttribute('data-recipient-field')
  return field === 'to' || field === 'cc' || field === 'bcc' ? field : ''
}

/** Firefox exposes synthetic and some OS clipboard files through items only. */
function transferFiles(transfer) {
  const files = Array.from(transfer?.files || [])
  if (files.length) return files
  return Array.from(transfer?.items || [])
    .filter(item => item.kind === 'file')
    .map(item => item.getAsFile())
    .filter(file => file instanceof File)
}

function currentRecipientToken(value) {
  return String(value || '').split(',').pop()?.trim() || ''
}

function recipientInitials(suggestion) {
  const source = String(suggestion.name || suggestion.email || '?').trim()
  const parts = source.split(/[@\s._-]+/).filter(Boolean)
  return (parts.length > 1 ? `${parts[0][0]}${parts[1][0]}` : source.slice(0, 2)).toUpperCase()
}

// @ts-check

import { buildUnsubscribeAction } from '../../../shared/scripts/unsubscribe.js'
import { safeGitHubTarget } from '../../../shared/scripts/github-notification.js'
import {
  buildProtectedEmailDocument,
  safeExternalLinkTarget,
} from '../../../shared/scripts/email-content-protection.js'
import {
  isForwardedSubject,
  searchConversation,
  splitQuotedText,
} from '../../../shared/scripts/message-conversation.js'
import {
  attachmentPreviewDataUrl,
  attachmentPreviewKind,
  decodeAttachmentText,
  isRasterImage,
  normalizeCidReference,
} from '../../../shared/scripts/attachment-content.js'
import { resolveSnoozePreset, snoozePresetLabel } from '../../../shared/scripts/snooze-presets.js'
import {
  loadPrintableConversation,
  openPrintConversation,
  renderPrintDocument,
} from '../../../shared/scripts/print-conversation.js'
import {
  detectLanguage,
  preferredTranslationLanguage,
  translationDetectionTextFromHtml,
  translateHtml,
  translateText,
  TRANSLATION_LANGUAGES,
} from '../../../shared/scripts/on-device-translation.js'
import {
  resetTranslationPreferences,
  setTranslationPreference,
  translationPreferenceFor,
} from '../../../shared/scripts/translation-preferences.js'
import {
  loadCachedMessageTranslation,
  saveCachedMessageTranslation,
  setCachedMessageTranslationEnabled,
} from '../../../shared/scripts/translation-cache.js'
import { quotedMessageHTML } from '../../../shared/scripts/rich-compose-editor.js'
import {
  buildCategoryPickerItems,
  categoryLabelPatch,
  normalizePickerPath,
} from '../../../shared/scripts/category-picker.js'
import {
  DEFAULT_MAIL_PREFERENCES,
  formatPreferenceDate,
  normalizeMailPreferences,
} from '../../../shared/scripts/mail-preferences.js'
import {
  cacheOfflineAttachment,
  cacheOfflineMailboxMessage,
  loadOfflineAttachment,
  loadOfflineMailboxMessage,
} from '../../../shared/scripts/offline-mailbox-index.js'

function offlineMailboxOwner() {
  return String(window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || '').toLowerCase()
}

/**
 * Email detail / reader component.
 *
 * Displays a single email message with full headers, body, and attachments.
 * Supports the following actions:
 * - Reply, reply-all, forward
 * - Star / unstar
 * - Archive, trash, snooze
 * - Mark read / unread
 * - List-Unsubscribe one-click, preference-page, and mailto actions
 * - Privacy-safe conversation printing and browser PDF export
 * - Attachment download
 * - Inline quick-reply
 *
 * The email to display can come from a parent component (3-panel layout) via
 * props, from the URL path (`/email/<id>`), or from query parameters.
 *
 * Keyboard shortcuts are bound via `caduceus:shortcut:email:*` events.
 *
 *
 * @prop {EmailDetailProps} [props] - Props for the email detail view
 */
export default class {
  static __tachyonOnMount = ['init', 'bindSelectionListener', 'bindMailActionListener', 'bindShortcuts']

  /**
   * Tachyon 26.33 passes component properties to the companion constructor.
   * Initialize declared fields here because the runtime deliberately avoids
   * overwriting fields that already exist on an owner instance.
   *
   * @param {EmailDetailProps} [props]
   */
  constructor(props = {}) {
    this.emailId = String(props.emailId || props['email-id'] || props.emailid || '')
    this.emailid = String(props.emailid || '')
    this.standalone = Boolean(props.standalone)
    if (props.email) this.$email = props.email
  }

  /** @type {string} Auto-bound `email-id` / `emailId` prop. */
  emailId = ''
  /** @type {string} Backward-compatible lowercase email ID prop. */
  emailid = ''
  /** @type {boolean} True on the dedicated /email/:id route. */
  standalone = false
  /** @type {Record<string, any>|null} The loaded email object */
  $email = null
  /** @type {boolean} */
  @SessionState
  loading = true
  /** @type {string} */
  @SessionState
  loadError = ''
  /** @type {boolean} Prevent duplicate invite responses while SMTP/API work is in flight. */
  @SessionState
  inviteResponseBusy = false
  /** @type {boolean} Whether the inline reply form is visible */
  @SessionState
  showReply = false
  /** @type {string} Text content of the inline reply */
  @SessionState
  replyText = ''
  /** @type {'idle'|'loading'|'success'|'error'} */
  @SessionState
  unsubscribeState = 'idle'
  /** @type {string} */
  @SessionState
  unsubscribeError = ''
  /** @type {boolean} Whether a conversation print document is being prepared. */
  @SessionState
  printing = false
  /** @type {'block'|'allow'} Account-wide remote-image policy. */
  $remoteImagePolicy = 'block'
  /** @type {string[]} Exact normalized senders trusted by this user. */
  $trustedImageSenders = []
  /** @type {boolean} One-message override granted by an explicit click. */
  @SessionState
  remoteImagesVisible = false
  /** @type {string} Complete sanitized iframe document. */
  @SessionState
  protectedEmailDocument = ''
  /** @type {number} Number of network resources discovered in the message. */
  @SessionState
  remoteContentCount = 0
  /** @type {number} Number of network resources currently blocked. */
  @SessionState
  blockedRemoteContentCount = 0
  /** @type {number} Number of tracking elements removed unconditionally. */
  @SessionState
  removedTrackingCount = 0
  /** @type {Record<string, string>} Proxy-returned inert raster data URLs keyed by original URL. */
  @SessionState
  remoteImageSources = {}
  /** @type {string[]} Ordinary remote image URLs found by the sanitizer. */
  @SessionState
  remoteImageUrls = []
  /** @type {boolean} */
  @SessionState
  remoteImagesLoading = false
  /** @type {string} */
  @SessionState
  remoteImageError = ''
  /** @type {number} */
  $suspiciousLinkCount = 0
  /** @type {{ reason: string, target: string, destinationHost: string, displayedHost: string } | null} */
  @SessionState
  pendingLinkWarning = null
  /** @type {boolean} Whether recognized quoted reply content is expanded. */
  @SessionState
  showQuotedText = false
  /** @type {number} Number of top-level quoted HTML regions in this message. */
  @SessionState
  quotedTextCount = 0
  /** @type {boolean} */
  @SessionState
  remotePreferenceSaving = false
  /** @type {'html'|'text'|'source'} Reader representation selected by the user. */
  @SessionState
  renderMode = 'html'
  /** @type {string} */
  @SessionState
  translationTarget = preferredTranslationLanguage()
  /** @type {string} */
  @SessionState
  translationSource = ''
  /** @type {boolean} */
  @SessionState
  translationBusy = false
  /** @type {number} */
  @SessionState
  translationProgress = 0
  /** @type {string} */
  @SessionState
  translationError = ''
  /** @type {boolean} */
  @SessionState
  translationVisible = false
  /** @type {string} */
  @SessionState
  translatedHtmlBody = ''
  /** @type {string} */
  @SessionState
  translatedPlainText = ''
  /** @type {number} Invalidates delayed detection when the selected message changes. */
  _translationDetectionSequence = 0
  /** @type {boolean} */
  @SessionState
  sourceLoading = false
  /** @type {string} */
  @SessionState
  sourceError = ''
  /** @type {string} Exact RFC 822 source decoded as UTF-8 for inert display. */
  @SessionState
  rawSourceText = ''
  /** @type {string} Original base64 bytes retained for lossless EML download. */
  @SessionState
  rawSourceBase64 = ''
  /** @type {string} */
  @SessionState
  rawSourceFilename = ''
  /** @type {boolean} */
  @SessionState
  snoozeOpen = false
  /** @type {boolean} */
  @SessionState
  snoozeBusy = false
  /** @type {string} */
  @SessionState
  snoozeCustom = ''
  /** @type {string} */
  @SessionState
  snoozeError = ''
  /** @type {boolean} */
  @SessionState
  shareOpen = false
  /** @type {boolean} */
  @SessionState
  shareBusy = false
  /** @type {string} */
  @SessionState
  shareError = ''
  /** @type {'1'|'7'|'30'} */
  @SessionState
  shareExpiresDays = '7'
  /** @type {Array<Record<string, any>>} */
  @SessionState
  threadShares = []
  /** @type {Record<string, any> | null} Newly minted secret, held only in this view. */
  @SessionState
  createdShare = null
  /** @type {boolean} */
  @SessionState
  attachmentPreviewOpen = false
  /** @type {boolean} */
  @SessionState
  attachmentPreviewLoading = false
  /** @type {string} */
  $attachmentPreviewKind = ''
  /** @type {string} */
  @SessionState
  attachmentPreviewUrl = ''
  /** @type {string} */
  @SessionState
  attachmentPreviewText = ''
  /** @type {string} */
  @SessionState
  attachmentPreviewError = ''
  /** @type {Record<string, any> | null} */
  @SessionState
  attachmentPreviewAttachment = null
  /** @type {Record<string, string>} Safe raster data URLs keyed by normalized Content-ID. */
  @SessionState
  inlineAttachmentSources = {}
  /** @type {Map<string, Promise<Record<string, any>>>} */
  _attachmentContent = new Map()
  /** @type {ReturnType<typeof setTimeout> | null} */
  _markReadTimer = null
  /** @type {WeakSet<HTMLIFrameElement>} */
  _protectedFrames = new WeakSet()
  /** @type {Array<Record<string, any>>} Complete authorized RFC conversation. */
  @SessionState
  conversationMessages = []
  /** @type {Record<string, boolean>} Expanded reading cards by message ID. */
  @SessionState
  conversationExpanded = {}
  /** @type {Record<string, boolean>} Per-card quoted text disclosure. */
  @SessionState
  conversationQuotes = {}
  /** @type {boolean} */
  @SessionState
  conversationLoading = false
  /** @type {boolean} */
  @SessionState
  findOpen = false
  /** @type {string} */
  @SessionState
  findTerm = ''
  /** @type {number} */
  @SessionState
  findIndex = -1
  /** @type {number} */
  @SessionState
  findCount = 0
  /** @type {Array<{ index: number, messageId: string, field: string }>} */
  @SessionState
  findResults = []
  /** @type {boolean} */
  @SessionState
  participantSidebarOpen = false
  /** @type {boolean} */
  @SessionState
  participantLoading = false
  /** @type {string} */
  @SessionState
  participantError = ''
  /** @type {Record<string, any> | null} */
  @SessionState
  participantContact = null
  /** @type {'disabled'|'found'|'not-found'|'unavailable'} */
  @SessionState
  participantGithubStatus = 'disabled'
  /** @type {Record<string, any> | null} */
  @SessionState
  participantGithubProfile = null
  /** @type {Array<Record<string, any>>} */
  @SessionState
  participantThreads = []
  /** @type {boolean} */
  @SessionState
  participantThreadsExpanded = false
  /** @type {number} */
  _participantLoadSequence = 0
  /** @type {Array<Record<string, any>>} Account-scoped folder and label capabilities. */
  @SessionState
  categoryAccounts = []
  /** @type {{open:boolean,mode:'move'|'label',search:string,creating:boolean,error:string}} */
  @SessionState
  categoryPicker = { open: false, mode: 'move', search: '', creating: false, error: '' }
  /** @type {HTMLElement | null} */
  _categoryPickerReturnFocus = null

  /**
   * Initialise the email detail view by resolving the email ID and loading it.
   *
   * The email ID is resolved from (in priority order):
   * 1. Auto-bound `emailId` / `emailid` fields
   * 2. URL path `/email/<id>`
   * 3. `?email=` or `?id=` query parameters
   * 4. Auto-bound `$email` (pre-loaded object)
   *
   * @returns {Promise<void>}
   */
  async init() {
    try { this.participantSidebarOpen = localStorage.getItem('caduceus-participant-sidebar') === 'open' } catch {}
    try {
      const target = localStorage.getItem('caduceus-translation-target') || ''
      if (Object.hasOwn(TRANSLATION_LANGUAGES, target)) this.translationTarget = target
    } catch {}
    const emailId = this.resolveEmailId()
    await this.loadEmailId(emailId, this.$email)
  }

  /**
   * Reload when the selected email changes inside the split-pane inbox.
   *
   * @returns {void}
   */
  bindSelectionListener() {
    // Idempotent binding: every mount replaces the single global handler and
    // removes the previous one. Without this, each email-detail mount leaked a
    // listener bound to a now-unmounted instance, so a single selection fired
    // one `loadEmail` per accumulated listener — a burst of concurrent
    // GET/PUT requests that raced and surfaced as "Failed to fetch".
    window.removeEventListener('caduceus:email-selected', /** @type {EventListener} */ (window._caduceusEmailDetailSelect))
    window._caduceusEmailDetailSelect = (event) => {
      const emailId = /** @type {CustomEvent<{ emailId?: string }>} */ (event).detail?.emailId || ''
      if (emailId) this.loadEmailId(emailId)
    }
    window.addEventListener('caduceus:email-selected', window._caduceusEmailDetailSelect)
  }

  /** Keep an open reader synchronized when a toast undo/redo runs later. */
  bindMailActionListener() {
    window.removeEventListener('caduceus:mail-action', /** @type {EventListener} */ (window._caduceusEmailDetailAction))
    window._caduceusEmailDetailAction = (event) => {
      if (!this.$email) return
      const result = /** @type {CustomEvent<{emails?:Array<Record<string, any>>}>} */ (event).detail
      const updated = result?.emails?.find(email => email.id === this.$email?.id)
      if (updated) this.$email = this.decorateEmail({ ...this.$email, ...updated })
    }
    window.addEventListener('caduceus:mail-action', window._caduceusEmailDetailAction)
  }

  /**
   * Set the current email shell and fetch the complete message.
   *
   * @param {string} emailId
   * @param {Record<string, any> | null} [email]
   * @returns {Promise<void>}
   */
  async loadEmailId(emailId, email = null) {
    this.cancelScheduledMarkRead()
    if (!emailId && !email) {
      this.$email = null
      this.loading = false
      this.loadError = ''
      return
    }
    if (this.$email?.id === emailId && this.$email?.body && !email) return
    this.$email = email ? this.decorateEmail(email) : { id: emailId }
    this.showReply = false
    this.replyText = ''
    this.inviteResponseBusy = false
    this.unsubscribeState = 'idle'
    this.unsubscribeError = ''
    this.remoteImagesVisible = false
    this.protectedEmailDocument = ''
    this.remoteContentCount = 0
    this.blockedRemoteContentCount = 0
    this.removedTrackingCount = 0
    this.remoteImageSources = {}
    this.remoteImageUrls = []
    this.remoteImagesLoading = false
    this.remoteImageError = ''
    this.$suspiciousLinkCount = 0
    this.pendingLinkWarning = null
    this.showQuotedText = isForwardedSubject(this.$email?.subject)
    this.quotedTextCount = 0
    this.renderMode = 'html'
    this.translationSource = ''
    this.translationBusy = false
    this.translationProgress = 0
    this.translationError = ''
    this.translationVisible = false
    this.translatedHtmlBody = ''
    this.translatedPlainText = ''
    this._translationDetectionSequence += 1
    this.sourceLoading = false
    this.sourceError = ''
    this.rawSourceText = ''
    this.rawSourceBase64 = ''
    this.rawSourceFilename = ''
    this.closeAttachmentPreview()
    this.inlineAttachmentSources = {}
    this._attachmentContent.clear()
    this.conversationMessages = []
    this.conversationExpanded = {}
    this.conversationQuotes = {}
    this.participantContact = null
    this.participantGithubStatus = 'disabled'
    this.participantGithubProfile = null
    this.participantThreads = []
    this.participantError = ''
    this.participantThreadsExpanded = false
    this._participantLoadSequence += 1
    this.closeFindInThread()
    await this.loadEmail()
  }

  /**
   * Fetch the full email object from the API and mark it as read.
   *
   * If the email was previously unread, this fires a `caduceus:refresh-inbox`
   * event so the inbox list can update its unread count.
   *
   * @returns {Promise<void>}
   */
  async loadEmail() {
    if (!this.$email?.id) {
      this.loading = false
      this.loadError = ''
      return
    }
    this.loading = true; this.loadError = ''
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) { this.loading = false; return }
    try {
      const res = await apiFetch(`/inbox/${this.$email.id}`)
      if (res?.ok) {
        this.$email = this.decorateEmail(await res.json())
		void cacheOfflineMailboxMessage(offlineMailboxOwner(), this.$email)
        this.showQuotedText = isForwardedSubject(this.$email?.subject)
        this.renderMode = this.$email?.htmlBody ? 'html' : 'text'
        await this.prepareAttachmentContent()
        await this.loadConversation(this.$email.id)
        if (this.participantSidebarOpen) void this.loadParticipantContext()
        if (!this.restoreCachedTranslation()) void this.detectMessageLanguage(this.$email.id)
      }
      else throw new Error('Could not load this email.')
      if (this.$email?.htmlBody) {
        await this.loadRemoteImagePreferences()
        this.refreshProtectedEmailDocument()
        if (this.shouldLoadRemoteImages) void this.loadRemoteImageSources()
      }
      if (this.$email && !this.$email.read) this.scheduleMarkRead(this.$email.id)
    } catch (err) {
		const cached = await loadOfflineMailboxMessage(offlineMailboxOwner(), this.$email.id)
		if (cached) {
			this.$email = this.decorateEmail(cached)
			this.showQuotedText = isForwardedSubject(this.$email?.subject)
			this.renderMode = this.$email?.htmlBody ? 'html' : 'text'
			await this.prepareAttachmentContent()
			this.refreshProtectedEmailDocument()
			this.loadError = ''
		} else {
			this.loadError = err instanceof Error ? err.message : String(err) || 'Network error'
		}
    } finally {
      this.loading = false
      setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
    }
  }

  get mailPreferences() {
    return normalizeMailPreferences(window._caduceus?.mailPreferences || DEFAULT_MAIL_PREFERENCES)
  }

  cancelScheduledMarkRead() {
    if (this._markReadTimer != null) clearTimeout(this._markReadTimer)
    this._markReadTimer = null
  }

  /** Respect the account's delayed/never mark-as-read policy. */
  scheduleMarkRead(emailId) {
    this.cancelScheduledMarkRead()
    const delay = this.mailPreferences.markAsReadDelay
    if (delay < 0) return
    const mark = async () => {
      this._markReadTimer = null
      if (this.$email?.id !== emailId || this.$email?.read) return
      const apiFetch = window._caduceus?.apiFetch
      if (!apiFetch) return
      const response = await apiFetch(`/inbox/${emailId}`, {
        method: 'PUT',
        body: JSON.stringify({ read: true }),
      })
      if (!response?.ok || this.$email?.id !== emailId) return
      this.$email = { ...this.$email, read: true }
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
    if (delay === 0) void mark()
    else this._markReadTimer = setTimeout(() => { void mark() }, delay)
  }

  get categoryPickerTitle() { return this.categoryPicker.mode === 'label' ? 'Apply labels' : 'Move to folder' }
  get categoryPickerPlaceholder() { return this.categoryPicker.mode === 'label' ? 'Label as…' : 'Move to…' }
  get categoryPickerAccount() {
    return this.categoryAccounts.find(account => account.domain === this.$email?.domain) || null
  }
  get categoryPickerItems() {
    const kind = this.categoryPicker.mode === 'label' ? 'label' : 'folder'
    const items = buildCategoryPickerItems(
      this.categoryPickerAccount, kind, this.categoryPicker.search, this.$email ? [this.$email] : [],
    )
    return kind === 'folder' ? items.filter(item => item.path !== this.$email?.folder) : items
  }
  get categoryPickerCreatePath() { return normalizePickerPath(this.categoryPicker.search) }
  get categoryPickerCanCreate() {
    const path = this.categoryPickerCreatePath
    return Boolean(path && this.categoryPickerAccount?.capabilities?.create
      && !this.categoryPickerItems.some(item => item.path === path))
  }
  get categoryPickerCreateText() {
    const kind = this.categoryPicker.mode === 'label' ? 'label' : 'folder'
    return this.categoryPickerCreatePath ? `Create ${kind} “${this.categoryPickerCreatePath}”` : ''
  }

  async loadCategoryCatalog() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return false
    try {
      const response = await apiFetch('/categories')
      const result = await response?.json().catch(() => null)
      if (!response?.ok || !Array.isArray(result?.accounts)) throw new Error('Category catalog is unavailable.')
      this.categoryAccounts = result.accounts
      return true
    } catch {
      window._caduceus?.toast('Could not load folders and labels.')
      return false
    }
  }

  async openCategoryPicker(eventOrMode, requestedMode) {
    const mode = eventOrMode === 'move' || eventOrMode === 'label' ? eventOrMode : requestedMode
    if (!this.$email || (mode !== 'move' && mode !== 'label')) return
    if (!this.categoryPickerAccount && !await this.loadCategoryCatalog()) return
    const capability = mode === 'label' ? 'labels' : 'folders'
    if (!this.categoryPickerAccount?.capabilities?.[capability]) {
      window._caduceus?.toast(`This mailbox does not support ${capability}.`)
      return
    }
    this._categoryPickerReturnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    this.categoryPicker = { open: true, mode, search: '', creating: false, error: '' }
    setTimeout(() => /** @type {HTMLInputElement | null} */ (
      document.querySelector('[data-detail-category-picker] .detail-category-picker-search')
    )?.focus(), 0)
  }

  closeCategoryPicker() {
    this.categoryPicker = { open: false, mode: 'move', search: '', creating: false, error: '' }
    const returnFocus = this._categoryPickerReturnFocus
    this._categoryPickerReturnFocus = null
    setTimeout(() => returnFocus?.focus(), 0)
  }

  stopCategoryPropagation(event) { event?.stopPropagation() }
  categorySearch(event) {
    this.categoryPicker = { ...this.categoryPicker, search: String(event?.target?.value || ''), error: '' }
  }
  categoryPickerIndent(item) { return `padding-inline-start:${12 + Math.min(6, Number(item?.depth) || 0) * 18}px` }
  categoryPickerCheck(item) { return item?.selectedState === 'all' ? '✓' : item?.selectedState === 'mixed' ? '−' : '' }
  categoryPickerAriaSelected(item) { return item?.selectedState === 'all' ? 'true' : 'false' }

  categoryPickerKeydown(event) {
    if (event?.key === 'Escape') { event.preventDefault(); this.closeCategoryPicker(); return }
    const dialog = document.querySelector('[data-detail-category-picker]')
    const focusable = [...(dialog?.querySelectorAll('button:not([disabled]), input:not([disabled])') || [])]
    if (event?.key === 'Tab' && focusable.length) {
      const current = document.activeElement ? focusable.indexOf(document.activeElement) : -1
      const next = event.shiftKey
        ? current <= 0 ? focusable.length - 1 : current - 1
        : current >= focusable.length - 1 ? 0 : current + 1
      event.preventDefault()
      ;/** @type {HTMLElement} */ (focusable[next]).focus()
      return
    }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event?.key)) return
    const options = focusable.filter(element => element.classList?.contains('detail-category-picker-option'))
    if (!options.length) return
    event.preventDefault()
    const current = document.activeElement ? options.indexOf(document.activeElement) : -1
    const index = event.key === 'Home' ? 0
      : event.key === 'End' ? options.length - 1
        : event.key === 'ArrowDown' ? Math.min(options.length - 1, current + 1)
          : Math.max(0, current < 0 ? options.length - 1 : current - 1)
    ;/** @type {HTMLElement} */ (options[index]).focus()
  }

  async selectCategory(eventOrPath, requestedPath) {
    const path = typeof eventOrPath === 'string' ? eventOrPath : requestedPath
    if (path) await this.applyCategoryPath(path)
  }

  async createCategory() {
    const path = this.categoryPickerCreatePath
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || !this.categoryPickerCanCreate || !path || !this.$email) return
    this.categoryPicker = { ...this.categoryPicker, creating: true, error: '' }
    try {
      const response = await apiFetch('/categories', {
        method: 'POST',
        body: JSON.stringify({
          domain: this.$email.domain,
          kind: this.categoryPicker.mode === 'label' ? 'label' : 'folder',
          path,
        }),
      })
      const result = await response?.json().catch(() => null)
      if (!response?.ok || !result?.category?.path) throw new Error(result?.error || 'Category creation failed.')
      await this.loadCategoryCatalog()
      await this.applyCategoryPath(result.category.path)
    } catch (error) {
      this.categoryPicker = {
        ...this.categoryPicker,
        creating: false,
        error: error instanceof Error ? error.message : 'Category creation failed.',
      }
    }
  }

  async applyCategoryPath(path) {
    if (!this.$email) return
    const patch = this.categoryPicker.mode === 'label'
      ? categoryLabelPatch([this.$email], path)
      : { folder: path }
    const message = this.categoryPicker.mode === 'label' ? 'Updated labels.' : `Moved to ${path}.`
    if (await this.updateEmail(patch, message)) this.closeCategoryPicker()
  }

  /**
   * Add render-safe labels that Tachyon's bounded template expressions consume.
   * @param {Record<string, any>} email
   * @returns {Record<string, any>}
   */
  decorateEmail(email) {
    if (!email || typeof email !== 'object') return email
    const opens = Array.isArray(email.opens) ? email.opens : []
    const clicks = Array.isArray(email.clicks) ? email.clicks : []
    const trackingEvents = [
      ...opens.map(event => ({
        ...event,
        type: 'open',
        eventIcon: '◉',
        eventLabel: 'Opened',
        timestamp: event.openedAt,
        timestampLabel: this.formatTrackingTime(event.openedAt),
        recipientLabel: event.recipient || 'Recipient unavailable',
      })),
      ...clicks.map(event => ({
        ...event,
        type: 'click',
        eventIcon: '↗',
        eventLabel: 'Clicked',
        timestamp: event.clickedAt,
        timestampLabel: this.formatTrackingTime(event.clickedAt),
        recipientLabel: event.recipient || 'Recipient unavailable',
        targetLabel: event.title || event.url || 'Link',
      })),
    ].sort((a, b) => String(b.timestamp || '').localeCompare(String(a.timestamp || '')))
    const openCount = Math.max(opens.length, Math.max(0, Number(email.openCount) || 0))
    const clickCount = Math.max(clicks.length, Math.max(0, Number(email.clickCount) || 0))
    return {
      ...email,
      opens,
      clicks,
      trackingEvents,
      trackingSummary: openCount || clickCount
        ? `${openCount} ${openCount === 1 ? 'open' : 'opens'} · ${clickCount} ${clickCount === 1 ? 'click' : 'clicks'}`
        : 'No activity yet',
      githubUrl: safeGitHubTarget(email.githubUrl),
      attachments: Array.isArray(email.attachments)
        ? email.attachments.map(attachment => ({
          ...attachment,
          sizeLabel: window._caduceus?.bytesLabel?.(attachment.size) || '',
          previewKind: attachmentPreviewKind(attachment.contentType),
          thumbnailUrl: '',
        }))
        : [],
    }
  }

  /** @param {unknown} value */
  formatTrackingTime(value) {
    const timestamp = typeof value === 'string' ? Date.parse(value) : NaN
    return Number.isFinite(timestamp) ? formatPreferenceDate(timestamp) : ''
  }

  get hasTrackingInsights() {
    return (this.$email?.systemFolder || this.$email?.folder) === 'sent' && Boolean(this.$email?.openTracking || this.$email?.linkTracking)
  }

  /** Load and decorate every authorized message connected to the open RFC thread. */
  async loadConversation(emailId) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || !emailId) return
    this.conversationLoading = true
    try {
      const response = await apiFetch(`/threads?emailId=${encodeURIComponent(emailId)}`)
      const data = await response?.json()
      const messages = response?.ok && Array.isArray(data?.[0]?.emails) ? data[0].emails : []
      const direction = this.mailPreferences.conversationOrder === 'descending' ? -1 : 1
      this.conversationMessages = messages
        .map(message => this.decorateConversationMessage(message))
        .sort((left, right) => direction * (Date.parse(left.receivedAt || '') - Date.parse(right.receivedAt || '')))
      this.conversationExpanded = Object.fromEntries(messages.map(message => [message.id, message.id === emailId]))
      this.refreshConversationSearch()
    } catch {
      this.conversationMessages = this.$email ? [this.decorateConversationMessage(this.$email)] : []
      this.conversationExpanded = this.$email ? { [this.$email.id]: true } : {}
    } finally {
      this.conversationLoading = false
    }
  }

  /** @param {Record<string, any>} email */
  decorateConversationMessage(email) {
    const quote = splitQuotedText(email?.body || '')
    return {
      ...this.decorateEmail(email),
      visibleConversationSender: this.mailPreferences.detailedNames
        ? email?.headerFrom || email?.sender || ''
        : email?.sender || '',
      receivedAtLabel: email?.receivedAt ? formatPreferenceDate(email.receivedAt) : '',
      visibleConversationBody: quote.visible || (quote.hasQuoted ? '' : '(No body)'),
      quotedConversationBody: quote.quoted,
      hasConversationQuote: quote.hasQuoted,
    }
  }

  get hasConversation() {
    return this.conversationMessages.length > 1
  }

  /** @param {string} id */
  isConversationExpanded(id) {
    return Boolean(this.conversationExpanded[id])
  }

  /** @param {string} id */
  isConversationQuoteExpanded(id) {
    return Boolean(this.conversationQuotes[id])
  }

  /** Tachyon supplies the DOM event before static template arguments. */
  toggleConversationMessage(eventOrId, requestedId) {
    const id = typeof eventOrId === 'string' ? eventOrId : requestedId
    if (!id) return
    this.conversationExpanded = { ...this.conversationExpanded, [id]: !this.conversationExpanded[id] }
  }

  toggleConversationQuote(eventOrId, requestedId) {
    const event = typeof eventOrId?.stopPropagation === 'function' ? eventOrId : null
    event?.stopPropagation()
    const id = typeof eventOrId === 'string' ? eventOrId : requestedId
    if (!id) return
    this.conversationQuotes = { ...this.conversationQuotes, [id]: !this.conversationQuotes[id] }
  }

  /** @param {Record<string, any>} email */
  conversationBody(email) {
    return this.conversationQuotes[email?.id]
      ? [email.visibleConversationBody, email.quotedConversationBody].filter(Boolean).join('\n\n')
      : email.visibleConversationBody
  }

  get findProgressLabel() {
    return this.findIndex >= 0 && this.findCount > 0
      ? `${this.$findIndex + 1} of ${this.$findCount}`
      : this.findTerm ? 'No matches' : ''
  }

  refreshConversationSearch(requestedIndex = this.findIndex) {
    const result = searchConversation(this.conversationMessages, this.findTerm, requestedIndex)
    this.conversationMessages = result.messages
    this.findTerm = result.term
    this.findResults = result.results
    this.findIndex = result.index
    this.findCount = result.count
  }

  openFindInThread() {
    if (!this.hasConversation && !this.$email) return
    this.findOpen = true
    this.conversationExpanded = Object.fromEntries(this.conversationMessages.map(message => [message.id, true]))
    this.attachFindInputKeys(true)
  }

  closeFindInThread() {
    this.findOpen = false
    this.findTerm = ''
    this.findIndex = -1
    this.findCount = 0
    this.findResults = []
    if (this.conversationMessages.length) this.refreshConversationSearch()
  }

  onFindInput(event) {
    this.findTerm = String(event?.target?.value || '')
    this.refreshConversationSearch(0)
    this.focusCurrentConversationMatch()
  }

  onFindKeydown(event) {
    if (event?.key === 'Escape') {
      event.preventDefault()
      this.closeFindInThread()
    } else if (event?.key === 'Enter') {
      event.preventDefault()
      if (event.shiftKey) this.previousFindResult()
      else this.nextFindResult()
    }
  }

  /** Restore focus to the current input after a Tachyon redraw. */
  attachFindInputKeys(focus = false) {
    setTimeout(() => {
      const input = /** @type {HTMLInputElement | null} */ (document.querySelector('[data-find-thread-input]'))
      if (!input) return
      if (focus) input.focus()
    }, 0)
  }

  previousFindResult() {
    if (!this.findOpen || !this.findCount) return
    this.refreshConversationSearch(this.findIndex - 1)
    this.focusCurrentConversationMatch()
  }

  nextFindResult() {
    if (!this.findOpen || !this.findCount) return
    this.refreshConversationSearch(this.findIndex + 1)
    this.focusCurrentConversationMatch()
  }

  focusCurrentConversationMatch() {
    const result = this.findResults[this.findIndex]
    if (!result) return
    this.conversationExpanded = { ...this.conversationExpanded, [result.messageId]: true }
    setTimeout(() => {
      const match = document.querySelector(`[data-find-index="${this.$findIndex}"]`)
      match?.scrollIntoView?.({ block: 'center', behavior: 'smooth' })
      this.attachFindInputKeys()
    }, 0)
  }

  /** Resolve an email and browser event regardless of Tachyon argument order. */
  conversationActionArgs(eventOrEmail, requestedEmail) {
    const email = eventOrEmail?.id ? eventOrEmail : requestedEmail?.id ? requestedEmail : null
    const event = typeof eventOrEmail?.stopPropagation === 'function'
      ? eventOrEmail
      : typeof requestedEmail?.stopPropagation === 'function' ? requestedEmail : null
    event?.stopPropagation()
    return { email, event }
  }

  conversationReply(eventOrEmail, requestedEmail) {
    const { email } = this.conversationActionArgs(eventOrEmail, requestedEmail)
    if (!email) return
    window._caduceus?.compose({
      to: this.replyTargetFor(email),
      subject: `Re: ${email.subject}`,
      body: quotedMessageHTML(email, 'reply'),
      ...this.replyThreadMetadata(email),
    })
  }

  conversationReplyAll(eventOrEmail, requestedEmail) {
    const { email } = this.conversationActionArgs(eventOrEmail, requestedEmail)
    if (!email) return
    const to = [this.replyTargetFor(email), email.recipient].filter(Boolean).join(', ')
    window._caduceus?.compose({ to, subject: `Re: ${email.subject}`, body: quotedMessageHTML(email, 'reply'), ...this.replyThreadMetadata(email) })
  }

  conversationForward(eventOrEmail, requestedEmail) {
    const { email } = this.conversationActionArgs(eventOrEmail, requestedEmail)
    if (!email) return
    window._caduceus?.compose({
      subject: `Fwd: ${email.subject}`,
      body: quotedMessageHTML(email, 'forward'),
      sourceEmailId: email.id,
    })
  }

  openConversationMessage(eventOrEmail, requestedEmail) {
    const { email } = this.conversationActionArgs(eventOrEmail, requestedEmail)
    if (email?.id) window._caduceus?.openEmailId(email.id)
  }

  async downloadConversationOriginal(eventOrEmail, requestedEmail) {
    const { email } = this.conversationActionArgs(eventOrEmail, requestedEmail)
    const apiFetch = window._caduceus?.apiFetch
    if (!email?.id || !apiFetch) return
    try {
      const response = await apiFetch(`/inbox/${email.id}/source`)
      const data = await response?.json()
      if (!response?.ok || typeof data?.contentBase64 !== 'string') throw new Error('Original message is unavailable.')
      const bytes = Uint8Array.from(atob(data.contentBase64), character => character.charCodeAt(0))
      const url = URL.createObjectURL(new Blob([bytes], { type: 'message/rfc822' }))
      const link = document.createElement('a')
      link.href = url
      link.download = String(data.filename || 'message.eml')
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch {
      window._caduceus?.toast('Original message is unavailable.')
    }
  }

  get openedAtLabel() {
    return this.$email?.openedAt ? formatPreferenceDate(this.$email.openedAt) : ''
  }

  get receivedAtLabel() {
    return this.$email?.receivedAt ? formatPreferenceDate(this.$email.receivedAt) : ''
  }

  get senderInitials() {
    return this.$email?.sender ? window._caduceus?.initials?.(this.$email.sender) || '' : ''
  }

  /** Exact local participant used for the sidebar (recipient for sent mail). */
  get participantAddress() {
    const own = String(window._caduceus?.auth?.email || '').trim().toLowerCase()
    const sender = String(this.$email?.sender || '').trim().toLowerCase()
    const recipients = [
      this.$email?.recipient,
      ...(Array.isArray(this.$email?.to) ? this.$email.to : []),
    ].map(value => String(value || '').trim().toLowerCase())
    const address = sender && sender !== own ? sender : recipients.find(value => value && value !== own) || sender
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(address) ? address : ''
  }

  get participantDisplayName() {
    const contact = this.participantContact
    return contact?.name || [contact?.givenName, contact?.familyName].filter(Boolean).join(' ') || this.participantAddress
  }

  get participantInitials() {
    return window._caduceus?.initials?.(this.participantDisplayName || this.participantAddress) || ''
  }

  get visibleParticipantThreads() {
    return this.participantThreadsExpanded ? this.participantThreads : this.participantThreads.slice(0, 3)
  }

  get hasMoreParticipantThreads() { return this.participantThreads.length > 3 }

  get participantSidebarLabel() {
    return this.participantSidebarOpen ? 'Hide contact sidebar' : 'Show contact sidebar'
  }

  async toggleParticipantSidebar() {
    this.participantSidebarOpen = !this.participantSidebarOpen
    try { localStorage.setItem('caduceus-participant-sidebar', this.participantSidebarOpen ? 'open' : 'closed') } catch {}
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    if (this.participantSidebarOpen) void this.loadParticipantContext()
  }

  async closeParticipantSidebar() {
    if (!this.participantSidebarOpen) return
    await this.toggleParticipantSidebar()
  }

  toggleParticipantThreads() {
    this.participantThreadsExpanded = !this.participantThreadsExpanded
    void (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  /** Load exact-owner local context plus explicitly opted-in public GitHub data. */
  async loadParticipantContext() {
    const address = this.participantAddress
    const apiFetch = window._caduceus?.apiFetch
    if (!address || !apiFetch || !this.participantSidebarOpen) return
    const sequence = ++this._participantLoadSequence
    this.participantLoading = true
    this.participantError = ''
    try {
      const [contactsResponse, relatedResponse, githubResponse] = await Promise.all([
        apiFetch(`/contacts?q=${encodeURIComponent(address)}`),
        apiFetch(`/search?q=${encodeURIComponent(`from:${address}`)}`),
        apiFetch(`/github/profile?email=${encodeURIComponent(address)}`),
      ])
      const contacts = await contactsResponse?.json()
      const related = await relatedResponse?.json()
      const github = await githubResponse?.json()
      if (sequence !== this._participantLoadSequence || address !== this.participantAddress) return
      this.participantContact = contactsResponse?.ok && Array.isArray(contacts)
        ? contacts.find(contact => Array.isArray(contact?.emails) && contact.emails.some(entry => String(entry?.value || '').toLowerCase() === address)) || null
        : null
      this.participantGithubStatus = githubResponse?.ok && ['disabled', 'found', 'not-found', 'unavailable'].includes(github?.status)
        ? github.status
        : 'unavailable'
      this.participantGithubProfile = this.participantGithubStatus === 'found' && github?.profile
        ? github.profile
        : null
      const seen = new Set()
      this.participantThreads = (relatedResponse?.ok && Array.isArray(related) ? related : [])
        .filter(email => String(email?.sender || '').toLowerCase() === address)
        .sort((a, b) => String(b.receivedAt || '').localeCompare(String(a.receivedAt || '')))
        .filter(email => {
          const key = String(email.threadId || email.messageId || email.subject || email.id)
          if (!key || seen.has(key)) return false
          seen.add(key)
          return true
        })
        .slice(0, 25)
        .map(email => ({
          id: email.id,
          subject: email.subject || '(No subject)',
          snippet: String(email.snippet || email.body || '').replace(/\s+/g, ' ').trim().slice(0, 120),
          receivedAtLabel: formatPreferenceDate(email.receivedAt),
        }))
    } catch {
      if (sequence === this._participantLoadSequence) {
        this.participantGithubStatus = 'unavailable'
        this.participantGithubProfile = null
        this.participantError = 'Contact context is unavailable.'
      }
    } finally {
      if (sequence === this._participantLoadSequence) {
        this.participantLoading = false
        await (/** @type {any} */ (globalThis)).__tc_rerender?.()
      }
    }
  }

  /** @param {Event} _event @param {Record<string, any>} thread */
  openParticipantThread(_event, thread) {
    if (thread?.id) window._caduceus?.openEmailId?.(thread.id)
  }

  /** Prefer the message's Reply-To mailbox when it supplies one. */
  get replyTarget() {
    return this.replyTargetFor(this.$email)
  }

  /** @param {Record<string, any> | null} email */
  replyTargetFor(email) {
    const replyTo = email?.replyTo
    return Array.isArray(replyTo) && replyTo[0] ? replyTo[0] : email?.sender || ''
  }

  get visibleSender() {
    return this.mailPreferences.detailedNames
      ? this.$email?.headerFrom || this.$email?.sender || ''
      : this.$email?.sender || ''
  }

  get attachmentPreviewsEnabled() { return this.mailPreferences.displayAttachmentPreviews }

  get detailedHeaderRows() {
    if (!this.mailPreferences.detailedHeaders || !this.$email) return []
    const values = [
      ['From', this.$email.headerFrom || this.$email.sender],
      ['To', Array.isArray(this.$email.to) ? this.$email.to.join(', ') : this.$email.recipient],
      ['Cc', Array.isArray(this.$email.cc) ? this.$email.cc.join(', ') : this.$email.cc],
      ['Reply-To', Array.isArray(this.$email.replyTo) ? this.$email.replyTo.join(', ') : this.$email.replyTo],
      ['Message-ID', this.$email.rfcMessageId || this.$email.messageId],
    ]
    return values.filter(([, value]) => Boolean(value)).map(([label, value]) => ({ label, value: String(value) }))
  }

  /** Normalized exact sender used only for the user's image-trust preference. */
  get imageSender() {
    const sender = String(this.$email?.sender || '').trim().toLowerCase()
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(sender) ? sender : ''
  }

  /** Whether account policy permits ordinary remote content for this email. */
  get shouldLoadRemoteImages() {
    if (this.remoteImagesVisible) return true
    if (this.isSpam(this.$email || undefined)) return false
    return this.$remoteImagePolicy === 'allow' ||
      (Boolean(this.imageSender) && this.$trustedImageSenders.includes(this.imageSender))
  }

  /** Human-readable privacy report used by the reader status card. */
  get remoteContentSummary() {
    const blocked = this.blockedRemoteContentCount
    const removed = this.removedTrackingCount
    if (blocked && removed) return `${blocked} remote ${blocked === 1 ? 'image is' : 'images are'} blocked and ${removed} tracking ${removed === 1 ? 'pixel was' : 'pixels were'} removed.`
    if (blocked) return `${blocked} remote ${blocked === 1 ? 'image is' : 'images are'} blocked to protect your privacy.`
    if (removed) return `${removed} tracking ${removed === 1 ? 'pixel was' : 'pixels were'} removed.`
    return ''
  }

  /** Load typed image preferences without ever weakening the block-on-error default. */
  async loadRemoteImagePreferences() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    try {
      const response = await apiFetch('/users/me/preferences')
      if (!response?.ok) return
      const preferences = await response.json()
      this.$remoteImagePolicy = preferences?.remoteImagePolicy === 'allow' ? 'allow' : 'block'
      this.$trustedImageSenders = Array.isArray(preferences?.trustedImageSenders)
        ? preferences.trustedImageSenders.map(sender => String(sender).trim().toLowerCase())
        : []
    } catch {
      this.$remoteImagePolicy = 'block'
      this.$trustedImageSenders = []
    }
  }

  /** Rewrite the raw MIME HTML before it reaches the sandboxed iframe. */
  refreshProtectedEmailDocument() {
    const body = this.translationVisible && this.translatedHtmlBody
      ? this.translatedHtmlBody
      : this.$email?.htmlBody || ''
    const requestedMode = this.mailPreferences.emailRenderMode
    const interfaceMode = document.documentElement.getAttribute('w-theme') === 'dark'
      || document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light'
    const colorMode = /** @type {'light'|'dark'} */ (requestedMode === 'theme' ? interfaceMode : requestedMode)
    const result = buildProtectedEmailDocument(body, {
      allowRemote: this.shouldLoadRemoteImages,
      cidSources: this.inlineAttachmentSources,
      remoteSources: this.remoteImageSources,
      showQuoted: this.showQuotedText,
      colorMode,
    })
    this.protectedEmailDocument = result.documentHtml
    this.remoteContentCount = result.remoteCount
    this.blockedRemoteContentCount = result.blockedCount
    this.removedTrackingCount = result.trackingCount
    this.quotedTextCount = result.quotedCount
    this.remoteImageUrls = result.remoteUrls
    this.$suspiciousLinkCount = result.suspiciousLinkCount
  }

  get plainTextParts() {
    return splitQuotedText(this.$email?.body || '')
  }

  get visiblePlainText() {
    if (this.translationVisible && this.translatedPlainText) return this.translatedPlainText
    return this.plainTextParts.visible || (this.plainTextParts.hasQuoted ? '' : '(No body)')
  }

  get translationTargetLabel() {
    return TRANSLATION_LANGUAGES[this.translationTarget] || 'selected language'
  }

  get translationLanguageOptions() {
    return Object.entries(TRANSLATION_LANGUAGES).map(([id, label]) => ({ id, label }))
  }

  get translationSourceLabel() {
    return TRANSLATION_LANGUAGES[this.translationSource] || 'detected language'
  }

  get translationPreference() {
    return translationPreferenceFor(this.translationSource)
  }

  get translationOfferVisible() {
    if (this.translationBusy || this.translationVisible || this.translatedHtmlBody || this.translatedPlainText || this.translationError) return true
    if (!this.translationSource || this.translationSource === this.translationTarget || this.isMessageFromCurrentUser) return false
    return this.translationPreference !== 'never'
  }

  get isMessageFromCurrentUser() {
    const folder = String(this.$email?.systemFolder || this.$email?.folder || '').toLowerCase()
    if (folder === 'sent') return true
    const sender = String(this.$email?.sender || '').trim().toLowerCase()
    const current = String(window._caduceus?.auth?.email || '').trim().toLowerCase()
    return Boolean(sender && current && sender === current)
  }

  get translationStatus() {
    if (!this.translationBusy) return ''
    return this.translationProgress > 0 && this.translationProgress < 1
      ? `Downloading language model… ${Math.round(this.$translationProgress * 100)}%`
      : 'Translating on this device…'
  }

  /** Restore a bounded local translation for this message and target. */
  restoreCachedTranslation() {
    const messageId = String(this.$email?.id || '')
    if (!messageId) return false
    const cached = loadCachedMessageTranslation(messageId, this.translationTarget)
    if (!cached) return false
    this.translationSource = cached.sourceLanguage
    this.translatedHtmlBody = cached.htmlBody
    this.translatedPlainText = cached.plainText
    this.translationVisible = cached.enabled
    return true
  }

  /** Detect after a short dwell so rapidly skipped messages do no work. */
  async detectMessageLanguage(emailId) {
    const sequence = ++this._translationDetectionSequence
    await new Promise(resolve => setTimeout(resolve, 500))
    if (sequence !== this._translationDetectionSequence || this.$email?.id !== emailId || this.isMessageFromCurrentUser) return
    let text = ''
    if (this.$email?.htmlBody && typeof DOMParser === 'function') {
      text = translationDetectionTextFromHtml(this.$email.htmlBody)
    }
    if (!text) text = this.plainTextParts.visible
    text = String(text || '').replace(/\s+/g, ' ').trim().slice(0, 2_000)
    if (text.length < 20) return
    const result = await detectLanguage(text)
    if (sequence !== this._translationDetectionSequence || this.$email?.id !== emailId) return
    if (!result.language || result.confidence < 0.3 || result.language === this.translationTarget) return
    this.translationSource = result.language
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    if (this.translationPreference === 'always') await this.translateMessage()
  }

  async alwaysTranslateDetectedLanguage() {
    if (!this.translationSource) return
    setTranslationPreference(this.translationSource, 'always')
    await this.translateMessage()
  }

  async stopAutomaticallyTranslatingDetectedLanguage() {
    if (!this.translationSource) return
    setTranslationPreference(this.translationSource, 'offer')
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  async neverTranslateDetectedLanguage() {
    if (!this.translationSource) return
    setTranslationPreference(this.translationSource, 'never')
    this.showOriginalMessage()
    this.translationError = ''
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  async resetTranslationSettings() {
    resetTranslationPreferences()
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  /** @param {Event} event */
  async changeTranslationTarget(event) {
    const value = String(/** @type {HTMLSelectElement} */ (event.target)?.value || '')
    if (!Object.hasOwn(TRANSLATION_LANGUAGES, value)) return
    this.translationTarget = value
    try { localStorage.setItem('caduceus-translation-target', value) } catch {}
    this.translatedHtmlBody = ''
    this.translatedPlainText = ''
    this.translationVisible = false
    this.translationError = ''
    this.restoreCachedTranslation()
    this.refreshProtectedEmailDocument()
    await (/** @type {any} */ (globalThis)).__tc_rerender?.()
  }

  /** Translate the visible message after explicit user activation. */
  async translateMessage() {
    if (!this.$email || this.translationBusy) return
    this.translationBusy = true
    this.translationProgress = 0
    this.translationError = ''
    try {
      const progress = loaded => {
        this.translationProgress = loaded
        void (/** @type {any} */ (globalThis)).__tc_rerender?.()
      }
      if (this.$email.htmlBody) {
        const result = await translateHtml(this.$email.htmlBody, {
          sourceLanguage: this.translationSource,
          targetLanguage: this.translationTarget,
          onProgress: progress,
        })
        this.translatedHtmlBody = result.html
        this.translationSource = result.sourceLanguage
      }
      const plain = this.plainTextParts.visible
      if (plain) {
        const result = await translateText(plain, {
          sourceLanguage: this.translationSource,
          targetLanguage: this.translationTarget,
          onProgress: progress,
        })
        this.translatedPlainText = result.text
        this.translationSource ||= result.sourceLanguage
      }
      if (!this.translatedHtmlBody && !this.translatedPlainText) throw new Error('This message has no translatable text.')
      this.translationVisible = true
      saveCachedMessageTranslation({
        messageId: String(this.$email.id || ''),
        sourceLanguage: this.translationSource,
        targetLanguage: this.translationTarget,
        htmlBody: this.translatedHtmlBody,
        plainText: this.translatedPlainText,
        enabled: true,
      })
      this.refreshProtectedEmailDocument()
    } catch (error) {
      this.translationError = error instanceof Error ? error.message : 'Translation failed.'
    } finally {
      this.translationBusy = false
      this.translationProgress = 0
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  showOriginalMessage() {
    this.translationVisible = false
    if (this.$email?.id) setCachedMessageTranslationEnabled(String(this.$email.id), false)
    this.refreshProtectedEmailDocument()
  }

  showTranslatedMessage() {
    if (!this.translatedHtmlBody && !this.translatedPlainText) return
    this.translationVisible = true
    if (this.$email?.id) setCachedMessageTranslationEnabled(String(this.$email.id), true)
    this.refreshProtectedEmailDocument()
  }

  get quotedPlainText() {
    return this.plainTextParts.quoted
  }

  get hasQuotedText() {
    return this.renderMode === 'html'
      ? this.quotedTextCount > 0
      : this.renderMode === 'text' && this.plainTextParts.hasQuoted
  }

  toggleQuotedText() {
    if (!this.hasQuotedText) return
    this.showQuotedText = !this.showQuotedText
    if (this.renderMode === 'html') this.refreshProtectedEmailDocument()
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  /** Grant a one-message remote-image exception after explicit user intent. */
  async showRemoteImages() {
    this.remoteImagesVisible = true
    this.refreshProtectedEmailDocument()
    await this.loadRemoteImageSources()
  }

  /** Fetch ordinary remote images through the exact-message server proxy. */
  async loadRemoteImageSources() {
    const apiFetch = window._caduceus?.apiFetch
    const emailId = String(this.$email?.id || '')
    const urls = this.remoteImageUrls.filter(url => !this.remoteImageSources[url])
    if (!apiFetch || !emailId || !urls.length || this.remoteImagesLoading) return
    this.remoteImagesLoading = true
    this.remoteImageError = ''
    try {
      let failures = 0
      for (let index = 0; index < urls.length; index += 6) {
        const response = await apiFetch(`/inbox/${encodeURIComponent(emailId)}/images`, {
          method: 'POST',
          body: JSON.stringify({ urls: urls.slice(index, index + 6) }),
        })
        const data = await response?.json().catch(() => null)
        if (!response?.ok || !Array.isArray(data?.images)) {
          failures += Math.min(6, urls.length - index)
          continue
        }
        failures += Math.max(0, Number(data.failed) || 0)
        for (const image of data.images) {
          const source = proxyImageDataUrl(image)
          if (source) this.remoteImageSources[String(image.url)] = source
          else failures += 1
        }
      }
      if (failures) this.remoteImageError = `${failures} remote ${failures === 1 ? 'image could' : 'images could'} not be loaded safely.`
      this.refreshProtectedEmailDocument()
    } catch {
      this.remoteImageError = 'Remote images could not be loaded safely.'
    } finally {
      this.remoteImagesLoading = false
      setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
    }
  }

  /** Persist an exact-sender trust choice and immediately re-render this email. */
  async alwaysShowImagesFromSender() {
    const sender = this.imageSender
    const apiFetch = window._caduceus?.apiFetch
    if (!sender || !apiFetch || this.remotePreferenceSaving) return
    this.remotePreferenceSaving = true
    try {
      const trustedImageSenders = [...new Set([...this.$trustedImageSenders, sender])]
      const response = await apiFetch('/users/me/preferences', {
        method: 'PUT',
        body: JSON.stringify({ trustedImageSenders }),
      })
      const data = await response?.json()
      if (!response?.ok) {
        window._caduceus?.toast(data?.error || 'Could not save the image preference.')
        return
      }
      this.$trustedImageSenders = Array.isArray(data?.trustedImageSenders)
        ? data.trustedImageSenders
        : trustedImageSenders
      await this.showRemoteImages()
      window._caduceus?.toast(`Images from ${sender} will load automatically.`)
    } catch {
      window._caduceus?.toast('Could not save the image preference.')
    } finally {
      this.remotePreferenceSaving = false
      setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
    }
  }

  /** Bind warning-only link handling without a Tachyon load-event rerender loop. */
  bindProtectedFrames() {
    for (const frame of document.querySelectorAll('.email-html-frame')) {
      if (!(frame instanceof HTMLIFrameElement) || this._protectedFrames.has(frame)) continue
      this._protectedFrames.add(frame)
      frame.addEventListener('load', () => this.bindProtectedFrameDocument(frame))
      this.bindProtectedFrameDocument(frame)
    }
  }

  /** @param {HTMLIFrameElement} frame */
  bindProtectedFrameDocument(frame) {
    if (!frame.contentDocument) return
    const document = frame.contentDocument
    if (/** @type {any} */ (document).__caduceusLinkWarningsBound) return
    /** @type {any} */ (document).__caduceusLinkWarningsBound = true
    document.addEventListener('click', click => {
      // The iframe has its own DOM realm, so its nodes do not satisfy the
      // parent window's `instanceof Element` check.
      const target = /** @type {any} */ (click.target)?.closest?.('a[data-caduceus-link-warning]') || null
      if (!target) return
      click.preventDefault()
      click.stopPropagation()
      const safeTarget = safeExternalLinkTarget(target.getAttribute('data-caduceus-link-target'))
      if (!safeTarget) return
      this.pendingLinkWarning = {
        reason: target.getAttribute('data-caduceus-link-warning') || 'This link may disguise its destination.',
        target: safeTarget,
        destinationHost: target.getAttribute('data-caduceus-link-host') || new URL(safeTarget).hostname,
        displayedHost: target.getAttribute('data-caduceus-link-label-host') || '',
      }
      setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
    })
  }

  get pendingLinkReason() { return this.pendingLinkWarning?.reason || '' }
  get pendingLinkTarget() { return this.pendingLinkWarning?.target || '' }
  get pendingLinkDestinationHost() { return this.pendingLinkWarning?.destinationHost || '' }
  get pendingLinkDisplayedHost() { return this.pendingLinkWarning?.displayedHost || '' }

  cancelPendingLink() {
    this.pendingLinkWarning = null
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  openPendingLink() {
    const target = safeExternalLinkTarget(this.pendingLinkWarning?.target)
    this.pendingLinkWarning = null
    if (target) window.open(target, '_blank', 'noopener,noreferrer')
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  /** Whether the server identified a Reply-To address that differs from From. */
  get hasReplyToMismatch() {
    return Array.isArray(this.$email?.securityWarnings)
      && this.$email.securityWarnings.includes('reply_to_mismatch')
  }

  /** Whether the message advertises at least one validated unsubscribe path. */
  get canUnsubscribe() {
    return buildUnsubscribeAction(this.$email) !== null
  }

  /**
   * Switch among protected HTML, plain text, and inert original source.
   * Tachyon 26.33 passes the browser event before static template arguments.
   *
   * @param {Event | string} eventOrMode
   * @param {'html'|'text'|'source'} [requestedMode]
   * @returns {Promise<void>}
   */
  async setRenderMode(eventOrMode, requestedMode) {
    const mode = typeof eventOrMode === 'string' ? eventOrMode : requestedMode
    if (mode !== 'html' && mode !== 'text' && mode !== 'source') return
    if (mode === 'html' && !this.$email?.htmlBody) return
    if (mode === 'source' && !this.$email?.hasRawSource) return
    this.renderMode = mode
    if (mode === 'source') await this.loadRawSource()
    setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
  }

  /** Fetch the exact source once; source text is displayed only in a `<pre>`. */
  async loadRawSource() {
    if (this.rawSourceBase64 || this.sourceLoading || !this.$email?.id) return
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
    this.sourceLoading = true
    this.sourceError = ''
    try {
      const response = await apiFetch(`/inbox/${this.$email.id}/source`)
      const data = await response?.json()
      if (!response?.ok) {
        this.sourceError = data?.error || 'Could not load the original message source.'
        return
      }
      this.rawSourceBase64 = String(data.contentBase64 || '')
      this.rawSourceFilename = String(data.filename || 'message.eml')
      this.rawSourceText = new TextDecoder('utf-8', { fatal: false }).decode(
        Uint8Array.from(atob(this.rawSourceBase64), character => character.charCodeAt(0)),
      )
    } catch {
      this.sourceError = 'Could not load the original message source.'
    } finally {
      this.sourceLoading = false
    }
  }

  /** Download the exact retained bytes as an RFC 822 `.eml` file. */
  async downloadOriginal() {
    await this.loadRawSource()
    if (!this.rawSourceBase64) {
      window._caduceus?.toast(this.sourceError || 'Original message source is unavailable.')
      return
    }
    const bytes = Uint8Array.from(atob(this.rawSourceBase64), character => character.charCodeAt(0))
    const url = URL.createObjectURL(new Blob([bytes], { type: 'message/rfc822' }))
    const link = document.createElement('a')
    link.href = url
    link.download = this.rawSourceFilename || 'message.eml'
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  /**
   * Execute the message's safest advertised List-Unsubscribe method only after
   * explicit confirmation. One-click requests are opaque cross-origin POSTs;
   * preference pages open separately and mailto targets use the composer.
   *
   * @returns {Promise<void>}
   */
  async unsubscribe() {
    if (this.unsubscribeState === 'loading' || this.unsubscribeState === 'success') return
    const action = buildUnsubscribeAction(this.$email)
    if (!action || !confirm('Unsubscribe from future messages from this sender?')) return

    if (action.type === 'compose') {
      window._caduceus?.compose({ to: action.to, subject: action.subject, body: action.body })
      return
    }
    if (action.type === 'open') {
      window.open(action.url, '_blank', 'noopener,noreferrer')
      return
    }

    this.unsubscribeState = 'loading'
    this.unsubscribeError = ''
    try {
      await fetch(action.url, {
        method: 'POST',
        mode: 'no-cors',
        credentials: 'omit',
        referrerPolicy: 'no-referrer',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'List-Unsubscribe=One-Click',
      })
      this.unsubscribeState = 'success'
      window._caduceus?.toast('Unsubscribe request sent.')
    } catch (error) {
      this.unsubscribeState = 'error'
      this.unsubscribeError = error instanceof Error ? error.message : 'Network request failed'
      window._caduceus?.toast('Could not send the unsubscribe request.')
    }
  }

  /**
   * Print the complete authorized conversation containing the open message.
   * Reserve the window synchronously so browser popup protection does not
   * reject the action while the cross-folder thread is loading.
   *
   * @returns {Promise<void>}
   */
  async printConversation() {
    if (!this.$email || this.printing) return
    const popup = window.open('', '_blank', 'popup')
    if (!popup) {
      window._caduceus?.toast('Allow pop-ups to print this conversation.')
      return
    }

    this.printing = true
    try {
      const conversation = await loadPrintableConversation(
        this.$email,
        window._caduceus?.apiFetch,
      )
      const documentHtml = renderPrintDocument(conversation, {
        account: window._caduceus?.auth?.email || '',
      })
      openPrintConversation(documentHtml, window, popup)
    } catch {
      popup.close()
      window._caduceus?.toast('Could not prepare this conversation for printing.')
    } finally {
      this.printing = false
    }
  }

  /**
   * Resolve an email ID from every shape Tachyon/browser routes may pass.
   *
   * @returns {string}
   */
  resolveEmailId() {
    const value = this.emailId
      || this.emailid
      || this.$email?.id
      || (location.pathname.startsWith('/email/') ? decodeURIComponent(location.pathname.slice('/email/'.length)) : '')
      || new URLSearchParams(location.search).get('email')
      || new URLSearchParams(location.search).get('id')
      || ''
    return typeof value === 'string' ? value : ''
  }

  /**
   * Snooze the current email until a future time.
   *
   * Presents the user with preset durations (later today, tomorrow, weekend,
   * next week) or a custom date/time. On success, refreshes the inbox and
   * navigates back.
   *
   * @returns {void}
   */
  snooze() {
    if (!this.$email || this.$email.folder !== 'inbox') return
    this.snoozeError = ''
    this.shareOpen = false
    this.shareBusy = false
    this.shareError = ''
    this.threadShares = []
    this.createdShare = null
    this.snoozeOpen = true
  }

  closeSnooze() {
    if (!this.snoozeBusy) this.snoozeOpen = false
  }

  /** @param {Event} event */
  stopSnoozePropagation(event) {
    event.stopPropagation()
  }

  /** Open the exact-owner share manager for the current conversation. */
  async openShareDialog() {
    if (!this.$email?.id) return
    this.shareOpen = true
    this.shareError = ''
    this.createdShare = null
    await this.loadThreadShares()
  }

  closeShareDialog() {
    if (!this.shareBusy) {
      this.shareOpen = false
      this.createdShare = null
    }
  }

  /** @param {Event} event */
  stopSharePropagation(event) { event?.stopPropagation?.() }

  async loadThreadShares() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || !this.$email?.id) return
    this.shareBusy = true
    this.shareError = ''
    try {
      const response = await apiFetch(`/thread-shares?emailId=${encodeURIComponent(this.$email.id)}`)
      const data = await response?.json()
      if (!response?.ok || !Array.isArray(data)) throw new Error(data?.error || 'Could not load sharing links.')
      this.threadShares = data.map(share => ({ ...share, statusLabel: this.shareStatusLabel(share) }))
    } catch (error) {
      this.shareError = error instanceof Error ? error.message : 'Could not load sharing links.'
    } finally {
      this.shareBusy = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** Mint a new one-time-visible secret and revoke the thread's prior active link. */
  async createThreadShare() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || !this.$email?.id || this.shareBusy) return
    this.shareBusy = true
    this.shareError = ''
    try {
      const response = await apiFetch('/thread-shares', {
        method: 'POST',
        body: JSON.stringify({
          emailId: this.$email.id,
          expiresInDays: Number(this.shareExpiresDays),
        }),
      })
      const data = await response?.json()
      if (!response?.ok || typeof data?.url !== 'string') throw new Error(data?.error || 'Could not create sharing link.')
      this.createdShare = { ...data, absoluteUrl: new URL(data.url, location.origin).href }
      await this.loadThreadShares()
      window._caduceus?.toast('Private conversation link created.')
    } catch (error) {
      this.shareError = error instanceof Error ? error.message : 'Could not create sharing link.'
    } finally {
      this.shareBusy = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  async copyThreadShare() {
    const value = String(this.createdShare?.absoluteUrl || '')
    if (!value) return
    try {
      if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(value)
      else this.copyTextFallback(value)
      window._caduceus?.toast('Sharing link copied.')
    } catch {
      this.copyTextFallback(value)
      window._caduceus?.toast('Sharing link copied.')
    }
  }

  /** @param {string} value */
  copyTextFallback(value) {
    const input = document.createElement('textarea')
    input.value = value
    input.setAttribute('readonly', '')
    input.style.position = 'fixed'
    input.style.opacity = '0'
    document.body.appendChild(input)
    input.select()
    document.execCommand('copy')
    input.remove()
  }

  openThreadShare() {
    const url = String(this.createdShare?.absoluteUrl || '')
    if (url) window.open(url, '_blank', 'noopener,noreferrer')
  }

  /** @param {Event | Record<string, any>} eventOrShare @param {Record<string, any>} [requestedShare] */
  async revokeThreadShare(eventOrShare, requestedShare) {
    const candidate = /** @type {Record<string, any>} */ (eventOrShare)
    const share = requestedShare?.id ? requestedShare : candidate?.id ? candidate : null
    const apiFetch = window._caduceus?.apiFetch
    if (!share?.id || !apiFetch || this.shareBusy) return
    this.shareBusy = true
    this.shareError = ''
    try {
      const response = await apiFetch(`/thread-shares/${encodeURIComponent(share.id)}`, { method: 'DELETE' })
      const data = await response?.json()
      if (!response?.ok) throw new Error(data?.error || 'Could not revoke sharing link.')
      if (this.createdShare?.id === share.id) this.createdShare = null
      await this.loadThreadShares()
      window._caduceus?.toast('Sharing link revoked.')
    } catch (error) {
      this.shareError = error instanceof Error ? error.message : 'Could not revoke sharing link.'
    } finally {
      this.shareBusy = false
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  /** @param {Record<string, any>} share */
  shareStatusLabel(share) {
    if (share.revokedAt) return `Revoked ${formatPreferenceDate(share.revokedAt)}`
    if (!share.active || Date.parse(share.expiresAt) <= Date.now()) return 'Expired'
    return `Expires ${formatPreferenceDate(share.expiresAt)}`
  }

  /** @param {Event} _event @param {string} id */
  async chooseSnoozePreset(_event, id) {
    const date = resolveSnoozePreset(id)
    if (date) await this.performSnooze(date, snoozePresetLabel(id))
  }

  /** @param {Event} [event] */
  async submitCustomSnooze(event) {
    event?.preventDefault()
    const date = new Date(this.snoozeCustom)
    if (!this.snoozeCustom || !Number.isFinite(date.getTime()) || date.getTime() <= Date.now()) {
      this.snoozeError = 'Choose a future date and time.'
      return
    }
    await this.performSnooze(date, 'Custom')
  }

  /** @param {Date} date @param {string} label */
  async performSnooze(date, label) {
    if (!this.$email || date.getTime() <= Date.now()) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    this.snoozeBusy = true; this.snoozeError = ''
    const clientRequestId = crypto.randomUUID()
    const body = { emailId: this.$email.id, until: date.toISOString(), label, clientRequestId }
    try {
      const res = await apiFetch('/snooze', { method: 'POST', body: JSON.stringify(body) })
      if (res?.ok) {
        const result = await res.json()
        this.snoozeOpen = false
        const message = `Snoozed ${result.count === 1 ? 'conversation' : `${result.count} messages`} until ${formatPreferenceDate(date)}`
        const expiresAt = Date.now() + 10_000
        sessionStorage.setItem('caduceus_pending_snooze_undo', JSON.stringify({
          id: result.snoozeId, message, owner: window._caduceus?.auth.email, expiresAt,
        }))
        if (this.standalone || location.pathname.startsWith('/email/')) {
          window._caduceus?.navigate('inbox')
        } else {
          this.showSnoozeUndo(result.snoozeId, message, apiFetch)
          this.goBack()
        }
        setTimeout(() => sessionStorage.removeItem('caduceus_pending_snooze_undo'), Math.max(0, expiresAt - Date.now()))
        window.dispatchEvent(new Event('caduceus:refresh-inbox'))
      } else {
        const result = await res?.json().catch(() => null)
        this.snoozeError = result?.error || 'Snooze failed.'
      }
    } catch {
      try {
        await window._caduceus?.queueOfflineRequest?.({
          id: clientRequestId, kind: 'snooze', path: '/snooze', method: 'POST', body,
          message: `Snooze until ${formatPreferenceDate(date)}`,
          coalesceKey: `snooze:${this.$email.id}`,
          metadata: { emailId: this.$email.id },
        })
        this.snoozeOpen = false
        window._caduceus?.navigate('inbox')
      } catch { this.snoozeError = 'Network error.' }
    } finally {
      this.snoozeBusy = false
    }
  }

  /** @param {string} snoozeId @param {string} message @param {(path:string, options?:any)=>Promise<Response|null>} apiFetch */
  showSnoozeUndo(snoozeId, message, apiFetch) {
    window._caduceus?.toastAction(message, {
          label: 'Undo',
          onClick: async () => {
            sessionStorage.removeItem('caduceus_pending_snooze_undo')
            const undone = await apiFetch(`/snooze/${encodeURIComponent(snoozeId)}`, { method: 'DELETE' })
            window._caduceus?.toast(undone?.ok ? 'Snooze undone.' : 'Could not undo snooze.')
            if (undone?.ok) window.dispatchEvent(new Event('caduceus:refresh-inbox'))
          },
        })
  }

  /**
   * Permanently delete the current email.
   *
   * Prompts for confirmation before deleting. On success, refreshes the inbox
   * and navigates back.
   *
   * @async
   * @returns {Promise<void>}
   */
  async deleteEmail() {
    if (!this.$email || !confirm('Delete this message forever?')) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const res = await apiFetch(`/inbox/${this.$email.id}`, { method: 'DELETE' })
    if (res?.ok) { window._caduceus?.toast('Message deleted.'); window.dispatchEvent(new Event('caduceus:refresh-inbox')); window._caduceus?.navigate('inbox'); this.$email = null }
    else { window._caduceus?.toast('Delete failed.') }
  }

  /**
   * Pretty-print the invite's date range as `"Wed, May 18, 10:00 AM – 11:00 AM"`.
   * Done as a method (not an inline template expression) because Tachyon's
   * `{…}` interpolation can't contain `{` / `}` characters, which would
   * make inline date-formatting calls unparseable.
   *
   * @param {Record<string, any>} invite
   * @returns {string}
   */
  formatInviteWhen(invite) {
    if (!invite?.start) return ''
    const start = new Date(invite.start)
    const end = invite.end ? new Date(invite.end) : null
    if (invite.allDay) {
      // All-day values are floating calendar dates. Force UTC presentation so
      // users west of Greenwich do not see the event shifted to the prior day.
      const date = value => formatPreferenceDate(value, { weekday: 'short', month: 'short', day: 'numeric', timeZone: 'UTC' })
      if (!end) return `${date(start)} · All day`
      // RFC 5545 all-day DTEND is exclusive. Present the final occupied day.
      const inclusiveEnd = new Date(end.getTime() - 1)
      return sameUtcDay(start, inclusiveEnd)
        ? `${date(start)} · All day`
        : `${date(start)} – ${date(inclusiveEnd)} · All day`
    }
    const startLabel = formatPreferenceDate(start, { weekday: 'short', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    if (!end) return startLabel
    // If the end is on the same day, only show the time portion.
    const endLabel = sameDay(start, end)
      ? formatPreferenceDate(end, { hour: '2-digit', minute: '2-digit' })
      : formatPreferenceDate(end, { weekday: 'short', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    return `${startLabel} – ${endLabel}`
  }

  /**
   * Map an attendee status code to a short human-readable chip label.
   *
   * @param {string} status
   * @returns {string}
   */
  formatInviteStatus(status) {
    if (status === 'accepted') return 'Going'
    if (status === 'declined') return 'Not going'
    if (status === 'tentative') return 'Maybe'
    if (status === 'delegated') return 'Delegated'
    if (status === 'completed') return 'Completed'
    return 'No response'
  }

  /** Return the response represented by this specific invite email. */
  inviteDisplayStatus(invite) {
    return invite?.senderResponse?.status || invite?.myStatus || 'needsAction'
  }

  /** Present METHOD:REPLY participant state without implying it is the user's RSVP. */
  inviteSenderResponseText(invite) {
    const response = invite?.senderResponse
    if (!response) return ''
    const subject = response.name || response.email || 'The attendee'
    if (response.status === 'accepted') return `${subject} accepted this invitation.`
    if (response.status === 'declined') return `${subject} declined this invitation.`
    if (response.status === 'tentative') return `${subject} tentatively accepted this invitation.`
    if (response.status === 'delegated') return `${subject} delegated this invitation.`
    if (response.status === 'completed') return `${subject} completed this invitation.`
    return `${subject} has not responded to this invitation.`
  }

  /**
   * Return untrusted email HTML only after the content-protection module has
   * rewritten it. The iframe grants same-origin solely so Chromium can honor
   * an explicit image-load choice; it grants no script, form, popup, or
   * navigation capability, and its CSP independently enforces the policy.
   *
   * @param {string} html
   * @returns {string}
   */
  emailHtmlSrcDoc(html) {
    void html
    if (!this.protectedEmailDocument) this.refreshProtectedEmailDocument()
    setTimeout(() => this.bindProtectedFrames(), 0)
    return this.protectedEmailDocument
  }

  /**
   * Whether the current or provided email carries spam state.
   *
   * @param {Record<string, any>} [email]
   * @returns {boolean}
   */
  isSpam(email = this.$email || undefined) {
    return (email?.systemFolder || email?.folder || 'inbox') === 'spam' || email?.spamScore != null || Array.isArray(email?.spamReasons)
  }

  /**
   * RSVP to a meeting-invite linked to the current email.
   *
   * The invite arrives auto-imported when the email was delivered (see
   * `server/services/calendar-import.js`); this method tells the server
   * which response to record and to send the corresponding iCal REPLY back
   * to the organizer. The local copy of $email is patched so the widget
   * re-renders with the new myStatus immediately.
   *
   * @async
   * @param {Event | 'accepted' | 'declined' | 'tentative'} eventOrStatus
   * @param {'accepted' | 'declined' | 'tentative'} [requestedStatus] Tachyon supplies this after the DOM event.
   */
  async respondToInvite(eventOrStatus, requestedStatus) {
    const status = typeof eventOrStatus === 'string' ? eventOrStatus : requestedStatus
    if (!status) return
    const invite = this.$email?.calendarInvite
    if (!invite?.id || !invite.canRespond || this.inviteResponseBusy) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    this.inviteResponseBusy = true
    try {
      const res = await apiFetch('/calendar/invites/respond', {
        method: 'POST',
        body: JSON.stringify({ eventId: invite.id, status }),
      })
      if (!res?.ok) { window._caduceus?.toast('Could not record your response.'); return }
      // Optimistic local update: stamp myStatus so the buttons reflect the
      // new state. The next /inbox/<id> fetch will reconcile.
      this.$email = {
        ...this.$email,
        calendarInvite: { ...invite, myStatus: status },
      }
      const label = status === 'accepted' ? 'Accepted' : status === 'declined' ? 'Declined' : 'Tentative'
      window._caduceus?.toast(`${label}.`)
    } catch {
      window._caduceus?.toast('Network error.')
    } finally {
      this.inviteResponseBusy = false
      // Tachyon coalesces renders requested while an async event callback is
      // still in flight. Queue this state flush after the callback boundary so
      // the status chip and disabled controls reflect the persisted response.
      setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
    }
  }

  /**
   * Apply a partial update to the current email via the API.
   *
   * On success, the server patch is merged into the decorated message so
   * derived, read-only actions remain available, the inbox is refreshed, and
   * an optional toast message is shown.
   *
   * @async
   * @param {Record<string, any>} patch - Key/value pairs to update on the email
   * @param {string} [message] - Optional toast message shown on success
   * @returns {Promise<boolean>}
   */
  async updateEmail(patch, message) {
    if (!this.$email) return false
    const action = window._caduceus?.performMailAction
    if (action) {
      const result = await action([this.$email.id], patch, message || 'Message updated.')
      const updated = result?.emails.find(email => email.id === this.$email?.id)
      if (updated) this.$email = this.decorateEmail({ ...this.$email, ...updated })
      return Boolean(result)
    }
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return false
    const res = await apiFetch(`/inbox/${this.$email.id}`, { method: 'PUT', body: JSON.stringify(patch) })
    if (res?.ok) {
      this.$email = this.decorateEmail({ ...this.$email, ...await res.json() })
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
      if (message) window._caduceus?.toast(message)
      return true
    }
    window._caduceus?.toast('Update failed.')
    return false
  }

  toggleRead() {
    if (this.$email) void this.updateEmail(
      { read: !this.$email.read },
      this.$email.read ? 'Marked unread.' : 'Marked read.',
    )
  }

  toggleStar() {
    if (this.$email) void this.updateEmail(
      { starred: !this.$email.starred },
      this.$email.starred ? 'Star removed.' : 'Starred.',
    )
  }

  archive() { if (this.$email) void this.updateEmail({ folder: 'archive' }, 'Archived.') }
  moveToInbox() { if (this.$email) void this.updateEmail({ folder: 'inbox' }, 'Moved to inbox.') }
  moveToTrash() { if (this.$email) void this.updateEmail({ folder: 'trash' }, 'Moved to trash.') }

  // TODO(markup): wire a "Report spam"/"Not spam" toolbar action + spam-score badge.

  /**
   * Report the current email as spam.
   *
   * @async
   * @param {Event|string} [eventOrId]
   * @param {string|Event} [requestedIdOrEvent]
   * @returns {Promise<void>}
   */
  async reportSpam(eventOrId = this.$email?.id, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string'
      ? eventOrId
      : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : this.$email?.id
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    if (!id) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const res = await apiFetch(`/inbox/${id}/spam`, { method: 'PUT', body: JSON.stringify({ spam: true }) })
    if (res?.ok) {
      this.$email = this.decorateEmail({ ...this.$email, ...await res.json() })
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
      window._caduceus?.toast('Reported spam.')
    } else {
      window._caduceus?.toast('Report spam failed.')
    }
  }

  /**
   * Mark the current email as not spam.
   *
   * @async
   * @param {Event|string} [eventOrId]
   * @param {string|Event} [requestedIdOrEvent]
   * @returns {Promise<void>}
   */
  async markNotSpam(eventOrId = this.$email?.id, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string'
      ? eventOrId
      : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : this.$email?.id
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    if (!id) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    const res = await apiFetch(`/inbox/${id}/spam`, { method: 'PUT', body: JSON.stringify({ spam: false }) })
    if (res?.ok) {
      this.$email = this.decorateEmail({ ...this.$email, ...await res.json() })
      window.dispatchEvent(new Event('caduceus:refresh-inbox'))
      window._caduceus?.toast('Moved to inbox.')
    } else {
      window._caduceus?.toast('Not spam failed.')
    }
  }

  /**
   * Bind global keyboard-shortcut listeners for email actions.
   *
   * Registered shortcuts:
   * - `email:reply`, `email:reply-all`, `email:forward`
   * - `email:star`, `email:archive`, `email:trash`, `email:report-spam`
   * - `email:mark-read`, `email:mark-unread`, `email:open`
   * - `core:print-thread`, `core:find-in-thread`, find traversal, `github:open`
   *
   * @returns {void}
   */
  bindShortcuts() {
    const on = (name, fn) => window.addEventListener('caduceus:shortcut:' + name, fn)

    on('email:reply', () => this.preferredReply())
    on('email:reply-all', () => this.replyAll())
    on('email:forward', () => this.forward())
    on('email:star', () => { if (this.$email) this.updateEmail({ starred: !this.$email.starred }, this.$email.starred ? 'Star removed.' : 'Starred.') })
    on('email:archive', () => { if (this.$email) this.updateEmail({ folder: 'archive' }, 'Archived.') })
    on('email:trash', () => { if (this.$email) this.updateEmail({ folder: 'trash' }, 'Moved to trash.') })
    on('email:report-spam', () => { if (this.$email) this.reportSpam(this.$email.id) })
    on('email:mark-read', () => { if (this.$email) this.updateEmail({ read: true }, 'Marked read.') })
    on('email:mark-unread', () => { if (this.$email) this.updateEmail({ read: false }, 'Marked unread.') })
    on('email:open', () => { if (this.$email) window._caduceus?.openEmailId(this.$email.id) })
    on('core:print-thread', () => { void this.printConversation() })
    on('core:find-in-thread', () => this.openFindInThread())
    on('core:find-in-thread-next', () => this.nextFindResult())
    on('core:find-in-thread-previous', () => this.previousFindResult())
    on('github:open', () => this.openGitHub())
  }

  /** Open the exact validated GitHub notification target in a new tab. */
  openGitHub() {
    if (this.findOpen) return
    const target = safeGitHubTarget(this.$email?.githubUrl)
    if (!target) return
    const popup = window.open(target, '_blank', 'noopener,noreferrer')
    if (popup) popup.opener = null
  }

  /**
   * Navigate back from the email detail view.
   *
   * In split-pane mode (inbox / folder pages where the detail is embedded
   * next to the list), Tachyon 26.20.7 routes `this.emit('back')` to the
   * parent's `@back="mailDeselected"` listener — that handler clears
   * `selectedMail` and the `:class` binding drops `show-detail`, which on
   * tablet brings the sidebar+list view back.
   *
   * On the standalone `/email/<id>` route there's no parent component to
   * receive the event, so we navigate to /inbox.
   *
   * @returns {void}
   */
  goBack() {
    if (location.pathname.startsWith('/email/')) {
      window._caduceus?.navigate('inbox')
    } else {
      window.dispatchEvent(new CustomEvent('caduceus:mail-back'))
    }
  }

  /**
   * Whether the open email is a draft (shown when navigating to /email/:id
   * directly; list clicks open drafts straight in the composer).
   * @returns {boolean}
   */
  get isDraft() { return (this.$email?.systemFolder || this.$email?.folder || '') === 'drafts' }

  /**
   * Open the current draft in the composer for editing.
   *
   * @returns {void}
   */
  editDraft() {
    if (!this.$email) return
    window._caduceus?.compose({ draftId: this.$email.id })
  }

  /**
   * Open the compose view pre-filled as a reply to the sender.
   *
   * @returns {void}
   */
  reply() {
    if (!this.$email) return
    window._caduceus?.compose({
      to: this.replyTarget,
      subject: `Re: ${this.$email.subject}`,
      body: quotedMessageHTML(this.$email, 'reply'),
      ...this.replyThreadMetadata(),
    })
  }

  /**
   * Open the compose view pre-filled as a reply-all.
   *
   * @returns {void}
   */
  replyAll() {
    if (!this.$email) return
    const to = [this.replyTarget, this.$email.recipient].filter(Boolean).join(', ')
    window._caduceus?.compose({ to, subject: `Re: ${this.$email.subject}`, body: quotedMessageHTML(this.$email, 'reply'), ...this.replyThreadMetadata() })
  }

  preferredReply() {
    if (this.mailPreferences.defaultReplyAction === 'reply-all') this.replyAll()
    else this.reply()
  }

  /**
   * Open the compose view pre-filled as a forward of the current email.
   *
   * Includes the original sender, date, subject, and body as a quoted block.
   *
   * @returns {void}
   */
  forward() {
    if (!this.$email) return
    window._caduceus?.compose({ subject: `Fwd: ${this.$email.subject}`, body: quotedMessageHTML(this.$email, 'forward'), sourceEmailId: this.$email.id })
  }

  /**
   * Send a plain-text inline reply to the current email.
   *
   * Uses the `$replyText` field as the message body. This is the quick-reply
   * form rendered directly inside the email detail view (not the full composer).
   *
   * @async
   * @returns {Promise<void>}
   */
  async sendReply() {
    if (!this.$email || !this.replyText) return
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch) return
    try {
      const res = await apiFetch('/send', { method: 'POST', body: JSON.stringify({
        to: [this.replyTarget],
        subject: `Re: ${this.$email.subject}`,
        text: this.replyText,
        ...this.replyThreadMetadata(),
      }) })
      if (res?.ok) { window._caduceus?.toast('Reply sent.'); this.showReply = false; this.replyText = '' }
      else { window._caduceus?.toast('Send failed.') }
    } catch { window._caduceus?.toast('Network error.') }
  }

  /** Build the RFC parent/reference chain for either composer or quick reply. */
  replyThreadMetadata(email = this.$email) {
    const parent = String(email?.rfcMessageId || '')
    const sourceEmailId = String(email?.id || '')
    if (!parent) return sourceEmailId ? { sourceEmailId } : {}
    let references = Array.isArray(email?.references) ? [...email.references] : []
    if (!references.includes(parent)) {
      // Preserve the root plus the newest ancestors when an unusually deep
      // chain reaches the server's defensive 100-ID cap.
      if (references.length >= 100) references = [references[0], ...references.slice(-98)]
      references.push(parent)
    }
    return { inReplyTo: parent, references, ...(sourceEmailId ? { sourceEmailId } : {}) }
  }

  /**
   * Download an email attachment to the user's device.
   *
   * Fetches the attachment content (base64-encoded), decodes it, creates a
   * Blob, and triggers a download via a temporary anchor element.
   *
   * @async
   * @param {Event | { id: string, filename?: string }} eventOrAttachment
   * @param {{ id: string, filename?: string }} [requestedAttachment] Tachyon supplies this after the DOM event.
   * @returns {Promise<void>}
   */
  async downloadAttachment(eventOrAttachment, requestedAttachment) {
    const attachment = /** @type {{ id: string, filename?: string }} */ (requestedAttachment || eventOrAttachment)
    const apiFetch = window._caduceus?.apiFetch; if (!apiFetch || !this.$email?.id) return
    try {
      const data = await this.getAttachmentContent(attachment)
      const bytes = Uint8Array.from(atob(data.contentBase64), char => char.charCodeAt(0))
      const blob = new Blob([bytes], { type: data.contentType || 'application/octet-stream' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a'); link.href = url; link.download = data.filename || attachment.filename || 'attachment'
      document.body.appendChild(link); link.click(); link.remove(); URL.revokeObjectURL(url)
    } catch {
      window._caduceus?.toast('Attachment download failed.')
    }
  }

  /**
   * Fetch an exact owned attachment once. The authenticated API remains the
   * only way attachment bytes enter the browser; previews never expose a
   * bearer-less public URL.
   * @param {{ id: string }} attachment
   */
  async getAttachmentContent(attachment) {
    if (!this.$email?.id || !attachment?.id) throw new Error('Attachment is unavailable.')
    const emailId = this.$email.id
    const cached = this._attachmentContent.get(attachment.id)
    if (cached) return await cached
    const request = (async () => {
      const apiFetch = window._caduceus?.apiFetch
		if (apiFetch) {
			try {
				const response = await apiFetch(`/inbox/${emailId}/attachments/${attachment.id}`)
				if (response?.ok) {
					const data = await response.json()
					if (data?.id === attachment.id && typeof data?.contentBase64 === 'string') {
						void cacheOfflineAttachment(offlineMailboxOwner(), emailId, data)
						return data
					}
				}
			} catch {}
      }
		const cachedAttachment = await loadOfflineAttachment(offlineMailboxOwner(), emailId, attachment.id)
		if (cachedAttachment) return cachedAttachment
		throw new Error('Attachment could not be loaded.')
    })()
    this._attachmentContent.set(attachment.id, request)
    try {
      return await request
    } catch (error) {
      this._attachmentContent.delete(attachment.id)
      throw error
    }
  }

  /** Resolve safe raster previews and inline CID sources before rendering HTML. */
  async prepareAttachmentContent() {
    if (!this.$email?.attachments?.length) return
    const duplicateIds = new Set()
    const cidSources = {}
    const attachments = await Promise.all(this.$email.attachments.map(async attachment => {
      if (!isRasterImage(attachment.contentType)) return attachment
      try {
        const data = await this.getAttachmentContent(attachment)
        const thumbnailUrl = attachmentPreviewDataUrl(data)
        const contentId = normalizeCidReference(attachment.contentId)
        if (contentId && thumbnailUrl) {
          if (Object.hasOwn(cidSources, contentId)) {
            duplicateIds.add(contentId)
            delete cidSources[contentId]
          } else if (!duplicateIds.has(contentId)) {
            cidSources[contentId] = thumbnailUrl
          }
        }
        return { ...attachment, thumbnailUrl: this.attachmentPreviewsEnabled ? thumbnailUrl : '' }
      } catch {
        return attachment
      }
    }))
    this.$email = { ...this.$email, attachments }
    this.inlineAttachmentSources = cidSources
  }

  /**
   * Open an inert image, text, or PDF preview for an owned attachment.
   * @param {Event | Record<string, any>} eventOrAttachment
   * @param {Record<string, any>} [requestedAttachment]
   */
  async previewAttachment(eventOrAttachment, requestedAttachment) {
    if (!this.attachmentPreviewsEnabled) {
      window._caduceus?.toast('Attachment previews are disabled in Mail preferences.')
      return
    }
    const attachment = /** @type {{ id: string, [key: string]: any }} */ (requestedAttachment || eventOrAttachment)
    const kind = attachmentPreviewKind(attachment?.contentType)
    if (!kind) return
    this.attachmentPreviewOpen = true
    this.attachmentPreviewLoading = true
    this.$attachmentPreviewKind = kind
    this.attachmentPreviewUrl = ''
    this.attachmentPreviewText = ''
    this.attachmentPreviewError = ''
    this.attachmentPreviewAttachment = attachment
    try {
      const data = await this.getAttachmentContent(attachment)
      if (kind === 'text') this.attachmentPreviewText = decodeAttachmentText(data)
      else {
        this.attachmentPreviewUrl = attachmentPreviewDataUrl(data)
        if (!this.attachmentPreviewUrl) throw new Error('This attachment cannot be previewed safely.')
      }
    } catch (error) {
      this.attachmentPreviewError = error instanceof Error ? error.message : 'Attachment preview failed.'
    } finally {
      this.attachmentPreviewLoading = false
      setTimeout(() => { void (/** @type {any} */ (globalThis)).__tc_rerender?.() }, 0)
    }
  }

  closeAttachmentPreview() {
    this.attachmentPreviewOpen = false
    this.attachmentPreviewLoading = false
    this.$attachmentPreviewKind = ''
    this.attachmentPreviewUrl = ''
    this.attachmentPreviewText = ''
    this.attachmentPreviewError = ''
    this.attachmentPreviewAttachment = null
  }

  stopAttachmentPreviewPropagation(event) {
    event?.stopPropagation?.()
  }
}

/** @param {Date} a @param {Date} b */
function sameDay(a, b) {
  return a.getFullYear() === b.getFullYear()
    && a.getMonth() === b.getMonth()
    && a.getDate() === b.getDate()
}

/** @param {Date} a @param {Date} b */
function sameUtcDay(a, b) {
  return a.getUTCFullYear() === b.getUTCFullYear()
    && a.getUTCMonth() === b.getUTCMonth()
    && a.getUTCDate() === b.getUTCDate()
}

/** Convert only a bounded, server-validated raster payload into an inert source. */
function proxyImageDataUrl(image) {
  const contentType = String(image?.contentType || '').toLowerCase()
  const contentBase64 = String(image?.contentBase64 || '')
  const size = Number(image?.size)
  if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'].includes(contentType)) return ''
  if (!Number.isSafeInteger(size) || size < 1 || size > 4 * 1024 * 1024) return ''
  if (!/^[A-Za-z0-9+/]+={0,2}$/.test(contentBase64) || Math.ceil(contentBase64.length * 0.75) > 4 * 1024 * 1024 + 2) return ''
  return `data:${contentType};base64,${contentBase64}`
}

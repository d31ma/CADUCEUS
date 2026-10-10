// @ts-check

import { formatExportBytes, mboxFilename, writeMboxFromFrameStream } from '../../../shared/scripts/mbox.js'
import { emlArchiveFilename, writeEmlArchiveFromFrameStream } from '../../../shared/scripts/eml-archive.js'
import { snoozedUntilLabel } from '../../../shared/scripts/snooze-presets.js'
import {
  buildCategoryPickerItems,
  categoryLabelPatch,
  normalizePickerPath,
  singleSelectionDomain,
} from '../../../shared/scripts/category-picker.js'
import {
  applySearchSuggestion,
  getCurrentTokenAndTerm,
  searchTokenSegments,
  staticSearchSuggestions,
  wrapSearchTerm,
} from '../../../shared/scripts/thread-search.js'
import { DEFAULT_MAIL_PREFERENCES, normalizeMailPreferences } from '../../../shared/scripts/mail-preferences.js'
import { cacheOfflineMailboxSummaries, searchOfflineMailbox } from '../../../shared/scripts/offline-mailbox-index.js'

/** @param {Record<string, any> | null | undefined} email */
function logicalFolder(email) { return email?.systemFolder || email?.folder || 'inbox' }

const MAX_MEMORY_EXPORT_BYTES = 256 * 1024 * 1024
const MAX_EML_IMPORT_BYTES = 50 * 1024 * 1024
/** @type {{owner:string,draft:string,suggestions:Array<Record<string,any>>,focused:boolean,selectedIndex:number,caret:number,request:number,timer:ReturnType<typeof setTimeout>|null}} */
const THREAD_SEARCH_STATE = {
  owner: '', draft: '', suggestions: [], focused: false, selectedIndex: -1, caret: 0, request: 0, timer: null,
}

/**
 * @typedef {object} InboxViewProps
 * @property {string} [folder] - Initial folder to display (defaults to 'inbox')
 */

/**
 * Inbox / email list component.
 *
 * Displays a filterable, searchable list of emails organised by folder. Supports:
 * - Folder sidebar (inbox, archive, snoozed, trash, plus custom folders)
 * - Search with Gmail-like query syntax (from:, to:, subject:, has:attachment, etc.)
 * - Unread / starred status filters
 * - Threaded mode (fetches `/threads`)
 * - Display density toggle (comfortable / compact / default)
 * - Keyboard shortcuts for navigation, selection, and email actions
 * - Drag-and-drop (initiation)
 * - Quick actions: star, archive, trash on individual rows
 *
 * Emits a `select` event when the user clicks a row, so a parent 3-panel
 * layout can display the email detail alongside.
 *
 *
 * @prop {InboxViewProps} [props] - Optional props for the inbox view
 */
export default class {
  // Subscribe to shell-owned mailbox state before init performs its first
  // asynchronous load. Otherwise a fast account catalog response can publish
  // before the inbox listener exists, leaving its selector stale until reload.
  static __tachyonOnMount = ['bindRefresh', 'init', 'bindShortcuts', 'bindSearchInput', 'bindRowActivation']

  /**
   * @param {InboxViewProps} [props]
   */
  constructor(props = {}) {
    this.folder = String(props.folder || '')
  }

  /** @type {string} Optional folder prop, auto-bound by Tachyon. */
  folder = ''
  /** @type {Array<Record<string, any>>} Full list of emails across all folders */
  $allEmails = []
  /** @type {boolean} */
  $loading = true
  /** @type {string} */
  $error = ''
  /** @type {string} Currently selected folder name */
  $selectedFolder = /** @type {string} */ ((typeof location !== 'undefined' ? new URLSearchParams(location.search).get('folder') : null) || 'inbox')
  /** @type {string} Current search / filter query text */
  $search = ''
  /** @type {string} Status filter: 'all', 'unread', or 'starred' */
  $statusFilter = 'all'
  /** @type {string} Domain filter: 'all' or a concrete domain from the auth claims */
  $selectedDomain = 'all'
  /** @type {string} Exact connected-account ID, empty outside an account-scoped view. */
  $selectedAccount = ''
  /** @type {Array<Record<string, any>>} Redacted connected mailboxes from the shell. */
  $externalAccounts = []
  /** @type {Record<string, Array<Record<string, string>>>} Provider folders keyed by account ID. */
  $externalFolders = {}
  /** @type {Record<string, string>} Personal display labels keyed by real domain. */
  $mailboxNames = {}
  /** @type {string} ID of the currently selected email row */
  $selectedId = ''
  /** @type {string[]} Exact logical IDs selected for bulk actions. */
  $selectedIds = []
  /** @type {string} The sole row in the list's roving tab stop. */
  $focusedId = ''
  /** @type {{open:boolean,ids:string[],x:number,y:number}} */
  $contextMenu = { open: false, ids: [], x: 0, y: 0 }
  /** @type {Array<Record<string, any>>} Exact-owner per-domain category capabilities. */
  $categoryAccounts = []
  /** @type {{open:boolean,mode:'move'|'label',ids:string[],domain:string,search:string,creating:boolean,error:string}} */
  $categoryPicker = { open: false, mode: 'move', ids: [], domain: '', search: '', creating: false, error: '' }
  /** @type {HTMLElement | null} */
  _categoryPickerReturnFocus = null
  /** @type {string} Display density: 'comfortable', 'compact', or 'default' */
  $density = 'comfortable'
  /** @type {boolean} Whether threaded (conversation) mode is enabled */
  $threadedMode = false
  /** @type {Array<Record<string, any>>} Thread objects returned when threaded mode is on */
  $threads = []
  /** @type {Record<string, boolean>} Map of expanded thread subject keys */
  $expandedThreads = {}
  /** @type {number} Count of live-pushed emails not yet seen at the top of the list. */
  $newMailCount = 0
  /** @type {number} */
  _apiFetchRetries = 0
  /** @type {boolean} Whether a folder export is actively consuming source frames. */
  $exporting = false
  /** @type {number} Messages durably appended to the pending export. */
  $exportMessages = 0
  /** @type {number} Best-known total for the unfiltered current folder. */
  $exportTotal = 0
  /** @type {number} Export bytes written so far. */
  $exportBytes = 0
  /** @type {string} Safe export failure shown inline. */
  $exportError = ''
  /** @type {string} Completion or cancellation feedback shown inline. */
  $exportNotice = ''
  /** @type {'mbox' | 'eml'} Active folder export representation. */
  $exportFormat = 'mbox'
  /** @type {AbortController | null} */
  _exportController = null
  /** @type {boolean} Whether one or more EML files are being imported. */
  $importing = false
  /** @type {number} Files successfully committed during the active import. */
  $importedFiles = 0
  /** @type {number} Files selected for the active import. */
  $importTotal = 0
  /** @type {string} Name of the file currently being read or uploaded. */
  $importCurrent = ''
  /** @type {string} Safe import failure shown inline. */
  $importError = ''
  /** @type {string} Import completion or cancellation feedback. */
  $importNotice = ''
  /** @type {AbortController | null} */
  _importController = null

  /**
   * @description Capitalised display name for the currently selected folder.
   * @returns {string}
   */
  get folderTitle() {
    const name = this.$selectedFolder || 'inbox'
    return name.charAt(0).toUpperCase() + name.slice(1)
  }

  /**
   * @description Deduplicated list of folder names across all emails. Always
   * includes 'inbox', 'archive', 'snoozed', and 'trash' first, followed by any
   * custom folders sorted alphabetically.
   * @returns {string[]}
   */
  get folders() {
    const fs = new Set(this.domainEmails.map(logicalFolder))
    return ['inbox', 'archive', 'snoozed', 'trash', ...Array.from(fs).filter(f => !['inbox', 'archive', 'snoozed', 'trash'].includes(f)).sort()]
  }

  /**
   * Domains attached to the current session's auth claims.
   *
   * @returns {string[]}
   */
  get domainOptions() {
    /** @type {string[]} */
    const domains = []
    const authDomains = window._caduceus?.auth?.domains
    if (Array.isArray(authDomains)) domains.push(...authDomains)
    try {
      const sessionDomains = JSON.parse(sessionStorage.getItem('caduceus_domains') || '[]')
      if (Array.isArray(sessionDomains)) domains.push(...sessionDomains)
    } catch {}
    return [...new Set(domains.map(domain => String(domain || '').trim().toLowerCase()).filter(Boolean))].sort()
  }

  get mailboxOptions() {
    return [
      ...this.domainOptions.map(domain => ({ value: `domain:${domain}`, label: this.mailboxLabel(domain), kind: 'domain', id: domain })),
      ...this.$externalAccounts.map(account => ({
        value: `account:${account.id}`,
        label: account.label || account.mailboxAddress,
        description: account.mailboxAddress,
        kind: 'account',
        id: account.id,
      })),
    ]
  }

  get selectedMailboxValue() {
    if (this.$selectedAccount) return `account:${this.$selectedAccount}`
    if (this.$selectedDomain !== 'all') return `domain:${this.$selectedDomain}`
    return 'all'
  }

  get selectedExternalAccount() {
    return this.$externalAccounts.find(account => account.id === this.$selectedAccount) || null
  }

  /**
   * Resolve a user's personal mailbox name without changing its domain key.
   *
   * @param {string} domain
   * @returns {string}
   */
  mailboxLabel(domain) {
    const normalized = String(domain || '').toLowerCase()
    return this.$mailboxNames[normalized] || window._caduceus?.mailboxLabel?.(normalized) || normalized
  }

  /**
   * Emails narrowed by the selected domain, before folder filters.
   *
   * @returns {Array<Record<string, any>>}
   */
  get domainEmails() {
    if (this.$selectedAccount) {
      return this.$allEmails.filter(email => email.providerAccountId === this.$selectedAccount)
    }
    if (this.$selectedDomain === 'all') return this.$allEmails
    return this.$allEmails.filter(e => !e.providerAccountId && String(e.domain || '').toLowerCase() === this.$selectedDomain)
  }

  /**
   * @description Emails filtered to the currently selected folder.
   * @returns {Array<Record<string, any>>}
   */
  get emails() {
    if (this.$search.trim()) return this.domainEmails
    return this.domainEmails.filter(e => logicalFolder(e) === this.$selectedFolder)
  }

  /** Tokenized presentation model painted beneath the transparent native input. */
  get searchState() {
    return THREAD_SEARCH_STATE
  }
  get searchActiveDescendant() {
    return this.searchState.selectedIndex >= 0
      ? this.searchState.suggestions[this.searchState.selectedIndex]?.id || ''
      : ''
  }
  get searchPlaceholder() {
    if (this.selectedExternalAccount) return `Search ${this.selectedExternalAccount.label || this.selectedExternalAccount.mailboxAddress}`
    return this.$selectedDomain === 'all'
      ? 'Search all mailboxes'
      : `Search ${this.mailboxLabel(this.$selectedDomain)}`
  }

  /** Translate the current perspective into Mailspring-compatible search terms. */
  get initialSearchQuery() {
    const terms = []
    if (this.$statusFilter === 'unread') terms.push('is:unread')
    if (this.$statusFilter === 'starred') terms.push('is:starred')
    if (this.$selectedFolder && !['all', 'inbox'].includes(this.$selectedFolder)) {
      const folder = /\s/.test(this.$selectedFolder) ? wrapSearchTerm(this.$selectedFolder) : this.$selectedFolder
      terms.push(`in:${folder}`)
    }
    return terms.join(' ')
  }

  /** All currently visible logical message IDs, including collapsed threads. */
  get visibleMessageIds() {
    const ids = this.$threadedMode
      ? this.visibleThreads.flatMap(thread => (thread.emails || []).map(email => email.id))
      : this.emails.map(email => email.id)
    return [...new Set(ids.filter(Boolean))]
  }

  get selectedCount() { return this.$selectedIds.length }
  get allVisibleSelected() {
    const visible = this.visibleMessageIds
    return visible.length > 0 && visible.every(id => this.$selectedIds.includes(id))
  }

  /** Ordered IDs currently represented by keyboard-focusable rows. */
  get rovingRowIds() {
    if (!this.$threadedMode) return this.emails.map(email => email.id)
    return this.visibleThreads.flatMap(thread => [
      thread.id,
      ...(thread.expanded ? (thread.emails || []).map(email => email.id) : []),
    ])
  }

  get contextEmail() {
    const id = this.$contextMenu.ids[0]
    return this.$allEmails.find(email => email.id === id)
      || this.$threads.flatMap(thread => thread.emails || []).find(email => email.id === id)
      || null
  }

  get contextCount() { return this.$contextMenu.ids.length }

  get categoryPickerTitle() { return this.$categoryPicker.mode === 'label' ? 'Apply labels' : 'Move to folder' }
  get categoryPickerPlaceholder() { return this.$categoryPicker.mode === 'label' ? 'Label as…' : 'Move to…' }
  get categoryPickerAccount() {
    return this.$categoryAccounts.find(account => account.domain === this.$categoryPicker.domain) || null
  }
  get categoryPickerMessages() { return this._messagesForIds(this.$categoryPicker.ids) }
  get categoryPickerItems() {
    const kind = this.$categoryPicker.mode === 'label' ? 'label' : 'folder'
    const items = buildCategoryPickerItems(
      this.categoryPickerAccount, kind, this.$categoryPicker.search, this.categoryPickerMessages,
    )
    if (kind === 'label') return items
    return items.filter(item => !this.categoryPickerMessages.every(message => message.folder === item.path))
  }
  get categoryPickerCreatePath() { return normalizePickerPath(this.$categoryPicker.search) }
  get categoryPickerCanCreate() {
    const path = this.categoryPickerCreatePath
    return Boolean(path && this.categoryPickerAccount?.capabilities?.create
      && !this.categoryPickerItems.some(item => item.path === path))
  }
  get categoryPickerCreateText() {
    const kind = this.$categoryPicker.mode === 'label' ? 'label' : 'folder'
    return this.categoryPickerCreatePath ? `Create ${kind} “${this.categoryPickerCreatePath}”` : ''
  }

  /**
   * Whether the email currently carries spam state.
   *
   * @param {Record<string, any>} email
   * @returns {boolean}
   */
  isSpam(email) {
    return logicalFolder(email) === 'spam' || email?.spamScore != null || Array.isArray(email?.spamReasons)
  }

  /** @param {string | number | Date} value */
  snoozeLabel(value) {
    return snoozedUntilLabel(value)
  }

  /**
   * Threads narrowed by the selected domain.
   *
   * @returns {Array<Record<string, any>>}
   */
  get visibleThreads() {
    const threads = this.$selectedDomain === 'all' && !this.$selectedAccount
      ? this.$threads
      : this.$threads
      .map(thread => ({
        ...thread,
        emails: (thread.emails || []).filter(email => this.messageMatchesSelectedMailbox(email)),
      }))
      .filter(thread => thread.emails.length > 0)
    return threads
      .map(thread => ({
        ...thread,
        count: thread.emails.length,
        latestEmail: thread.emails[thread.emails.length - 1],
        expanded: Boolean(this.$expandedThreads[thread.id]),
      }))
  }

  /** Accessible label for MailSpring-compatible personal-level markers. */
  personalLevelLabel(level) {
    if (Number(level) === 2) return 'Sent only to you'
    if (Number(level) === 1) return 'Sent to you and others'
    return ''
  }

  /** Open a blank composer through the application shell. */
  compose() { window._caduceus?.compose?.({}) }

  /** Human-readable live progress for the folder export status region. */
  get exportProgressLabel() {
    const count = this.$exportTotal > 0
      ? `${this.$exportMessages} of ${this.$exportTotal} messages`
      : `${this.$exportMessages} ${this.$exportMessages === 1 ? 'message' : 'messages'}`
    const format = this.$exportFormat === 'eml' ? 'EML archive' : 'mbox'
    return `Exporting ${format}: ${count} · ${formatExportBytes(this.$exportBytes)}`
  }

  /** Human-readable live progress for an EML import. */
  get importProgressLabel() {
    const progress = `${this.$importedFiles} of ${this.$importTotal} imported`
    return this.$importCurrent ? `${progress} · ${this.$importCurrent}` : progress
  }

  /** Open the native multi-file picker after validating the target mailbox. */
  pickEmlImport() {
    if (this.$importing || this.$exporting) return
    if (!this.importTarget()) {
      this.$importError = 'Choose one mailbox domain before importing EML files.'
      return
    }
    /** @type {HTMLInputElement | null} */ (document.querySelector('[data-eml-import]'))?.click()
  }

  /**
   * Import selected EML files sequentially so progress, cancellation, and
   * per-file atomicity remain predictable even for large mailboxes.
   *
   * @param {Event} event
   */
  async importEmlFiles(event) {
    const input = /** @type {HTMLInputElement} */ (event.target)
    const files = [...(input.files || [])]
    input.value = ''
    if (files.length === 0 || this.$importing) return
    const target = this.importTarget()
    if (!target) {
      this.$importError = 'Choose one mailbox domain before importing EML files.'
      return
    }

    const controller = new AbortController()
    this._importController = controller
    this.$importing = true
    this.$importedFiles = 0
    this.$importTotal = files.length
    this.$importCurrent = ''
    this.$importError = ''
    this.$importNotice = ''
    const failures = []

    try {
      for (const file of files) {
        if (controller.signal.aborted) throw controller.signal.reason
        this.$importCurrent = file.name
        if (file.size === 0 || file.size > MAX_EML_IMPORT_BYTES) {
          failures.push(`${file.name}: file must be between 1 byte and 50 MiB`)
          continue
        }
        try {
          const contentBase64 = await this.readFileBase64(file, controller.signal)
          const response = await this.apiFetch('/inbox/import/eml', {
            method: 'POST',
            body: JSON.stringify({
              mailbox: target.mailbox,
              folder: this.$selectedFolder,
              contentBase64,
            }),
            signal: controller.signal,
          })
          const result = await response?.json().catch(() => ({}))
          if (!response?.ok || !result?.imported) throw new Error(result?.error || `Import failed (${response?.status || 0})`)
          this.$importedFiles += 1
        } catch (error) {
          if (controller.signal.aborted) throw error
          failures.push(`${file.name}: ${error instanceof Error ? error.message : 'import failed'}`)
        }
      }

      if (this.$importedFiles) await this.load()
      const imported = `Imported ${this.$importedFiles} ${this.$importedFiles === 1 ? 'message' : 'messages'}`
      if (failures.length) {
        this.$importError = `${imported}; ${failures.length} ${failures.length === 1 ? 'file failed' : 'files failed'}. ${failures[0]}`
      } else {
        this.$importNotice = `${imported} into ${this.folderTitle}.`
      }
    } catch (error) {
      const canceled = controller.signal.aborted || (error instanceof DOMException && error.name === 'AbortError')
      if (canceled) {
        this.$importNotice = `EML import canceled after ${this.$importedFiles} ${this.$importedFiles === 1 ? 'message' : 'messages'}.`
        if (this.$importedFiles) await this.load()
      } else {
        this.$importError = error instanceof Error ? error.message : 'EML import failed.'
      }
    } finally {
      if (this._importController === controller) this._importController = null
      this.$importing = false
      this.$importCurrent = ''
    }
  }

  /** Stop reading/uploading the current file and skip every remaining file. */
  cancelEmlImport() {
    if (!this._importController || this._importController.signal.aborted) return
    this._importController.abort(new DOMException('EML import canceled', 'AbortError'))
  }

  /** Resolve one concrete mailbox address for the selected domain. */
  importTarget() {
    if (this.$selectedAccount) return null
    const domains = this.domainOptions
    const domain = this.$selectedDomain === 'all'
      ? domains.length === 1 ? domains[0] : ''
      : this.$selectedDomain
    if (!domain) return null
    const signedIn = String(sessionStorage.getItem('caduceus_email') || '').toLowerCase()
    const [local, signedInDomain] = signedIn.split('@')
    const mailbox = signedInDomain === domain ? signedIn : `${local || 'owner'}@${domain}`
    return { domain, mailbox }
  }

  /** @param {File} file @param {AbortSignal} signal */
  readFileBase64(file, signal) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      const abort = () => reader.abort()
      signal.addEventListener('abort', abort, { once: true })
      reader.onerror = () => {
        signal.removeEventListener('abort', abort)
        reject(reader.error || new Error('Could not read EML file'))
      }
      reader.onabort = () => {
        signal.removeEventListener('abort', abort)
        reject(signal.reason || new DOMException('EML import canceled', 'AbortError'))
      }
      reader.onload = () => {
        signal.removeEventListener('abort', abort)
        const result = String(reader.result || '')
        const comma = result.indexOf(',')
        if (comma < 0) reject(new Error('Could not encode EML file'))
        else resolve(result.slice(comma + 1))
      }
      reader.readAsDataURL(file)
    })
  }

  /**
   * Stream the current folder into an mboxrd file or an archive of exact EML
   * sources.
   *
   * Chromium's File System Access API receives one record at a time and only
   * commits on `close()`. Other browsers retain chunks until the complete
   * export can be installed through a native download. Both paths discard the
   * incomplete output on cancellation or failure.
   * Tachyon passes the DOM event before declared template arguments, while
   * direct callers may pass the format as the first argument.
   *
   * @param {Event | 'mbox' | 'eml'} [eventOrFormat]
   * @param {'mbox' | 'eml'} [requestedFormat]
   */
  async exportFolder(eventOrFormat = 'mbox', requestedFormat) {
    if (this.$exporting) { this.cancelExport(); return }

    const folder = String(this.$selectedFolder || 'inbox').toLowerCase()
    const accountId = String(this.$selectedAccount || '')
    const selected = String(this.$selectedDomain || 'all').toLowerCase()
    let domains = accountId ? [] : selected === 'all' ? this.domainOptions : [selected]
    if (!accountId && domains.length === 0) {
      const own = String(sessionStorage.getItem('caduceus_email') || '').split('@')[1]?.toLowerCase()
      if (own) domains = [own]
    }
    domains = [...new Set(domains.filter(Boolean))]
    if (!accountId && domains.length === 0) {
      this.$exportError = 'No mailbox domain is available to export.'
      return
    }

    const format = requestedFormat || (typeof eventOrFormat === 'string' ? eventOrFormat : 'mbox')
    const exportFormat = format === 'eml' ? 'eml' : 'mbox'
    if (exportFormat === 'eml' && !accountId && domains.length !== 1) {
      this.$exportError = 'Choose one mailbox domain before exporting individual EML files.'
      return
    }

    const filenameDomain = accountId || (selected === 'all' && domains.length > 1 ? 'all-domains' : domains[0])
    const filename = exportFormat === 'eml'
      ? emlArchiveFilename(folder, filenameDomain)
      : mboxFilename(folder, filenameDomain)
    const controller = new AbortController()
    this._exportController = controller
    this.$exporting = true
    this.$exportMessages = 0
    this.$exportBytes = 0
    this.$exportError = ''
    this.$exportNotice = ''
    this.$exportFormat = exportFormat
    this.$exportTotal = !this.$search.trim() && this.$statusFilter === 'all' ? this.emails.length : 0

    /** @type {any} */
    let writable = null
    /** @type {Uint8Array[]} */
    const chunks = []
    let memoryBytes = 0
    try {
      const picker = /** @type {any} */ (window).showSaveFilePicker
      if (typeof picker === 'function') {
        const handle = await picker({
          suggestedName: filename,
          types: exportFormat === 'eml'
            ? [{ description: 'EML message archive', accept: { 'application/zip': ['.zip'] } }]
            : [{ description: 'Mailbox file', accept: { 'application/mbox': ['.mbox'] } }],
        })
        writable = await handle.createWritable()
      }

      let completedMessages = 0
      let completedSourceBytes = 0
      let completedOutputBytes = 0
      const exportScopes = accountId ? [{ account: accountId }] : domains.map(domain => ({ domain }))
      for (const scope of exportScopes) {
        if (controller.signal.aborted) throw controller.signal.reason
        const params = new URLSearchParams({ folder, ...scope })
        const response = await this.apiFetch(`/inbox/export/rfc822?${params}`, {
          headers: { Accept: 'application/vnd.caduceus.rfc822-frames; version=1' },
          signal: controller.signal,
        })
        if (!response?.ok || !response.body) {
          let message = `Mailbox export failed${response ? ` (${response.status})` : ''}.`
          try { message = (await response?.clone().json())?.error || message } catch {}
          throw new Error(message)
        }
        const consume = exportFormat === 'eml' ? writeEmlArchiveFromFrameStream : writeMboxFromFrameStream
        const result = await consume(
          response.body,
          chunk => {
            if (writable) return writable.write(chunk)
            if (memoryBytes + chunk.byteLength > MAX_MEMORY_EXPORT_BYTES) {
              throw new Error(
                `This browser limits in-memory mailbox exports to ${formatExportBytes(MAX_MEMORY_EXPORT_BYTES)}. Use a browser with the File System Access API.`,
              )
            }
            chunks.push(chunk)
            memoryBytes += chunk.byteLength
          },
          {
            signal: controller.signal,
            onProgress: progress => {
              this.$exportMessages = completedMessages + progress.messages
              this.$exportBytes = completedOutputBytes + progress.outputBytes
            },
          },
        )
        completedMessages += result.messages
        completedSourceBytes += result.sourceBytes
        completedOutputBytes += result.outputBytes
      }

      if (completedMessages === 0) {
        if (writable) await writable.abort()
        writable = null
        this.$exportNotice = `There are no messages in ${folder}.`
        return
      }

      if (writable) {
        await writable.close()
        writable = null
      } else {
        this.downloadExport(chunks, filename, exportFormat === 'eml' ? 'application/zip' : 'application/mbox')
      }
      const representation = exportFormat === 'eml' ? ' as individual EML files' : ''
      this.$exportNotice = `Exported ${completedMessages} ${completedMessages === 1 ? 'message' : 'messages'}${representation} (${formatExportBytes(completedSourceBytes)} source).`
    } catch (error) {
      if (writable) {
        try { await writable.abort() } catch {}
        writable = null
      }
      const canceled = controller.signal.aborted || (error instanceof DOMException && error.name === 'AbortError')
      if (canceled) {
        this.$exportNotice = 'Mailbox export canceled.'
      }
      else this.$exportError = error instanceof Error ? error.message : 'Mailbox export failed.'
    } finally {
      if (this._exportController === controller) this._exportController = null
      this.$exporting = false
    }
  }

  /** Stop the active fetch and cause its pending destination to be discarded. */
  cancelExport() {
    if (!this._exportController || this._exportController.signal.aborted) return
    this._exportController.abort(new DOMException('Mailbox export canceled', 'AbortError'))
  }

  /** @param {Uint8Array[]} chunks @param {string} filename @param {string} contentType */
  downloadExport(chunks, filename, contentType) {
    const blob = new Blob(chunks.map(chunk => Uint8Array.from(chunk).buffer), { type: contentType })
    this.downloadExportBlob(blob, filename)
  }

  /** @param {Blob} blob @param {string} filename */
  downloadExportBlob(blob, filename) {
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = filename
    anchor.click()
    setTimeout(() => URL.revokeObjectURL(url), 0)
  }

  /**
   * @description Count of unread emails in the currently displayed folder. The
   * toolbar badge reads from this and is hidden when it is zero, so the count
   * reflects the folder you're looking at rather than a global total.
   * @returns {number}
   */
  get unreadCount() { return this.emails.filter(e => !e.read).length }

  /**
   * Count emails in a specific folder.
   *
   * @param {string} name - The folder name
   * @returns {number}
   */
  folderCount(name) { return this.domainEmails.filter(e => logicalFolder(e) === name).length }

  /**
   * @description Capitalised label for the current density setting, used for UI toggle display.
   * @returns {string}
   */
  get $densityLabel() { return this.$density.charAt(0).toUpperCase() + this.$density.slice(1) }

  /**
   * Initialise the inbox: restore the saved density preference from localStorage
   * and load the email list.
   *
   * @async
   * @returns {Promise<void>}
   */
  async init() {
    const owner = sessionStorage.getItem('caduceus_email') || ''
    if (this.searchState.owner !== owner) {
      if (this.searchState.timer) clearTimeout(this.searchState.timer)
      Object.assign(this.searchState, {
        owner, draft: '', suggestions: [], focused: false,
        selectedIndex: -1, caret: 0, request: 0, timer: null,
      })
    }
    this.bindSearchInput()
    const saved = localStorage.getItem('caduceus-density')
    if (saved && ['comfortable', 'compact', 'default'].includes(saved)) {
      this.$density = saved
    }
    try { this.$mailboxNames = JSON.parse(sessionStorage.getItem('caduceus_mailbox_names') || '{}') } catch { this.$mailboxNames = {} }
    this.$externalAccounts = window._caduceus?.auth?.externalAccounts || []
    this.$externalFolders = window._caduceus?.auth?.externalFolders || {}
    // When a folder prop is provided (e.g. /folder/:name route), it is the
    // authoritative source — override any persisted $selectedFolder so the
    // URL stays in sync with the view. bindPersistentFields() runs after the
    // class field initializer and would otherwise restore a stale value.
    const propFolder = this.folder
    if (propFolder && this.$selectedFolder !== propFolder) {
      this.$selectedFolder = propFolder
    }
    // `?domain=<name>` URL parameter (set by the per-domain sidebar rows)
    // takes precedence over the persisted dropdown selection so deep links
    // and sidebar clicks always land in the intended mailbox.
    try {
      const params = new URLSearchParams(location.search)
      const domain = params.get('domain')
      const account = params.get('account')
      if (account) {
        this.$selectedAccount = account
        this.$selectedDomain = 'all'
      } else if (domain) {
        this.$selectedDomain = domain.toLowerCase()
        this.$selectedAccount = ''
      }
    } catch { /* ignore malformed URLs */ }
    this.ensureSelectedDomain()
    await Promise.all([this.load(), this.loadCategoryCatalog()])
    setTimeout(() => {
      Promise.resolve((/** @type {any} */ (globalThis)).__tc_rerender?.())
        .finally(() => this.bindSearchInput())
    }, 0)
  }

  /**
   * Bind a global `caduceus:refresh-inbox` listener that reloads the email list.
   *
   * Any previously bound handler is removed first to prevent duplicate listeners
   * when the component re-mounts.
   *
   * @returns {void}
   */
  bindRefresh() {
    window.removeEventListener('caduceus:refresh-inbox', /** @type {() => void} */ (window._caduceusInboxRefresh))
    window._caduceusInboxRefresh = async () => {
      await this.load()
      await (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
    window.addEventListener('caduceus:refresh-inbox', window._caduceusInboxRefresh)

    // Sidebar folder/mailbox rows are anchor navigations to
    // `/inbox?folder=<name>&domain=<domain>`. When the user clicks one we must
    // re-read `?folder=` and `?domain=` from the URL and reload — the inbox-view
    // stays mounted across these same-route SPA navigations, so nothing else
    // updates the active folder. Pages dispatch `tachyon:navigate` for this;
    // full-reload navigations are handled by the initial init() above.
    window.removeEventListener('tachyon:navigate', /** @type {() => void} */ (window._caduceusInboxNav))
    window._caduceusInboxNav = () => {
      try {
        const params = new URLSearchParams(location.search)
        const nextDomain = (params.get('domain') || '').toLowerCase() || 'all'
        const nextAccount = params.get('account') || ''
        let changed = false
        if (nextAccount !== this.$selectedAccount) { this.$selectedAccount = nextAccount; changed = true }
        const scopedDomain = nextAccount ? 'all' : nextDomain
        if (scopedDomain !== this.$selectedDomain) { this.$selectedDomain = scopedDomain; changed = true }
        // On the /inbox route the active folder is the `?folder=` param and
        // defaults to inbox when absent (so clicking Inbox after Snoozed resets
        // the view). On the /folder/:name route the `folder` prop is
        // authoritative, so leave the folder alone there.
        if (!this.folder) {
          const nextFolder = params.get('folder') || 'inbox'
          if (nextFolder !== this.$selectedFolder) { this.$selectedFolder = nextFolder; changed = true }
        }
        if (changed) {
          this.$selectedId = ''
          this.load()
        }
      } catch { /* ignore malformed URLs */ }
    }
    window.addEventListener('tachyon:navigate', window._caduceusInboxNav)

    window.removeEventListener('caduceus:mailbox-names', /** @type {() => void} */ (window._caduceusInboxMailboxNames))
    window._caduceusInboxMailboxNames = (event) => {
      this.$mailboxNames = /** @type {CustomEvent<Record<string, string>>} */ (event).detail || {}
    }
    window.addEventListener('caduceus:mailbox-names', window._caduceusInboxMailboxNames)

    window.removeEventListener('caduceus:external-mailboxes', /** @type {() => void} */ (window._caduceusInboxExternalMailboxes))
    window._caduceusInboxExternalMailboxes = (event) => {
      const detail = /** @type {CustomEvent<any>} */ (event).detail || {}
      this.$externalAccounts = Array.isArray(detail.accounts) ? detail.accounts : []
      this.$externalFolders = detail.folders && typeof detail.folders === 'object' ? detail.folders : {}
      if (this.$selectedAccount && !this.$externalAccounts.some(account => account.id === this.$selectedAccount)) {
        this.$selectedAccount = ''
      }
    }
    window.addEventListener('caduceus:external-mailboxes', window._caduceusInboxExternalMailboxes)

    // Live new-mail pushed by the shell's SSE stream (pages/tac.js). Splice it
    // straight into the list so the message appears without a reload; the
    // `emails` getter still filters by the active folder/domain.
    window.removeEventListener('caduceus:new-email', /** @type {() => void} */ (window._caduceusInboxNewMail))
    window._caduceusInboxNewMail = (event) => this.onNewEmail(/** @type {CustomEvent<object>} */ (event).detail)
    window.addEventListener('caduceus:new-email', window._caduceusInboxNewMail)

  }

  /**
   * Keep row activation in the native capture phase. The row contains
   * independent checkbox/action controls, so a dedicated full-row button owns
   * open/toggle behavior without nesting interactive descendants or forcing a
   * Tachyon reconciliation between selection and the shell's detail render.
   */
  bindRowActivation() {
    const host = /** @type {any} */ (window)
    const previous = host._caduceusInboxRowActivation
    if (previous) {
      document.removeEventListener('click', previous.click, true)
      document.removeEventListener('keydown', previous.keydown, true)
      document.removeEventListener('pointerdown', previous.pointerdown, true)
      document.removeEventListener('pointerup', previous.pointerup, true)
    }
    const click = event => {
      const source = event.target instanceof Element ? event.target : null
      if (!source) return
      let target = source.closest('[data-email-open], [data-thread-open]')
      if (!(target instanceof HTMLElement)) {
        // Visible row content sits above the full-row button so text, avatars,
        // and icons remain honest pointer targets. Route those clicks through
        // the same activation path, but never steal an independent control.
        if (source.closest('a, button, input, select, textarea, label, [role="button"], [role="menuitem"], w-btn, w-chip, w-switch, w-checkbox')) return
        const row = source.closest('.email-row[data-email-id], .thread-email-row[data-email-id], .thread-summary[data-thread-id]')
        if (!(row instanceof HTMLElement)) return
        target = row
      }
      event.preventDefault()
      event.stopImmediatePropagation()
      const emailId = target.dataset.emailOpen || target.dataset.emailId
      const threadId = target.dataset.threadOpen || target.dataset.threadId
      if (emailId) this.selectEmail(emailId)
      else if (threadId) this.toggleThread(threadId)
    }
    const keydown = event => {
      const source = event.target instanceof Element ? event.target : null
      const editable = source?.closest('input, textarea, select, [contenteditable="true"], [role="textbox"], [role="dialog"]')
      if (event.key === 'Backspace' && !editable) {
        const focusedRowId = source?.closest('[data-email-open]')?.getAttribute('data-email-open') || ''
        const ids = this.$selectedIds.length
          ? [...this.$selectedIds]
          : [this.$selectedId || this.$focusedId || focusedRowId].filter(Boolean)
        if (ids.length) {
          event.preventDefault()
          event.stopImmediatePropagation()
          const action = normalizeMailPreferences(window._caduceus?.mailPreferences || DEFAULT_MAIL_PREFERENCES).backspaceAction
          const folder = action === 'trash' ? 'trash' : 'archive'
          void this.applyMailboxAction(ids, { folder }, folder === 'trash' ? 'Moved to trash.' : 'Archived.')
          return
        }
      }
      const target = event.target instanceof Element ? event.target.closest('[data-email-open], [data-thread-open]') : null
      if (!(target instanceof HTMLElement)) return
      const id = target.dataset.emailOpen || target.dataset.threadOpen || ''
      if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
        event.stopImmediatePropagation()
        this._handleRovingKey(event, id)
        return
      }
      if (event.key !== 'Enter' && event.key !== ' ') return
      event.preventDefault()
      event.stopImmediatePropagation()
      if (target.dataset.emailOpen) this.selectEmail(id)
      else this.toggleThread(id)
    }
    /** @type {{id:string,x:number,y:number,pointerId:number}|null} */
    let swipe = null
    const pointerdown = event => {
      if (!['touch', 'pen'].includes(event.pointerType)) return
      const row = event.target instanceof Element
        ? event.target.closest('.email-row[data-email-id], .thread-email-row[data-email-id]')
        : null
      if (!(row instanceof HTMLElement) || !row.dataset.emailId) return
      swipe = { id: row.dataset.emailId, x: event.clientX, y: event.clientY, pointerId: event.pointerId }
    }
    const pointerup = event => {
      if (!swipe || swipe.pointerId !== event.pointerId) return
      const start = swipe
      swipe = null
      const preferences = normalizeMailPreferences(window._caduceus?.mailPreferences || DEFAULT_MAIL_PREFERENCES)
      const dx = event.clientX - start.x
      const dy = event.clientY - start.y
      if (!preferences.swipeEnabled || Math.abs(dx) < 64 || Math.abs(dy) > 48) return
      event.preventDefault()
      event.stopImmediatePropagation()
      const folder = dx > 0 ? 'archive' : 'trash'
      void this.applyMailboxAction([start.id], { folder }, folder === 'trash' ? 'Moved to trash.' : 'Archived.')
    }
    host._caduceusInboxRowActivation = { click, keydown, pointerdown, pointerup }
    document.addEventListener('click', click, true)
    document.addEventListener('keydown', keydown, true)
    document.addEventListener('pointerdown', pointerdown, true)
    document.addEventListener('pointerup', pointerup, true)
  }

  /**
   * Capture native input before Tachyon reconciles the component. This keeps
   * the live editor value independent from the last submitted search while
   * suggestions re-render around it.
   */
  bindSearchInput() {
    const host = /** @type {any} */ (document)
    const previous = host.__caduceusSearchHandlers
    if (previous) {
      previous.observer?.disconnect()
      host.removeEventListener('input', previous.input, true)
      host.removeEventListener('focusin', previous.focus)
      host.removeEventListener('blur', previous.blur, true)
      host.removeEventListener('keydown', previous.keydown, true)
      host.removeEventListener('scroll', previous.scroll, true)
    }
    const isSearchInput = event => event.target instanceof HTMLInputElement
      && event.target.matches('[data-thread-search-input]')
    const observer = new MutationObserver(() => {
      const current = this.searchInput()
      const layer = document.querySelector('[data-search-token-layer]')
      const list = /** @type {HTMLElement | null} */ (document.querySelector('#thread-search-suggestions'))
      const clear = /** @type {HTMLButtonElement | null} */ (document.querySelector('.search-clear-btn'))
      const visible = this.searchState.focused && this.searchState.suggestions.length > 0
      const expectedIds = this.searchState.suggestions.map(suggestion => suggestion.id).join('|')
      const renderedIds = list ? Array.from(list.children).map(option => option.id).join('|') : ''
      const settled = current?.value === this.searchState.draft
        && layer?.textContent === this.searchState.draft
        && clear?.hidden === !this.searchState.draft
        && list?.hidden === !visible
        && renderedIds === expectedIds
      if (!current || settled) return
      current.value = this.searchState.draft
      this.renderSearchDecorations()
      this.renderSearchSuggestions()
      this.syncSearchOverlay()
    })
    const handlers = {
      input: (event) => {
        if (isSearchInput(event)) this.onSearchInput(event)
      },
      focus: (event) => {
        if (isSearchInput(event)) this.onSearchFocus(event)
      },
      blur: (event) => {
        if (isSearchInput(event)) this.onSearchBlur()
      },
      keydown: (event) => {
        if (isSearchInput(event)) this.onSearchKeydown(event)
      },
      scroll: (event) => {
        if (isSearchInput(event)) this.syncSearchOverlay(event)
      },
      observer,
    }
    observer.observe(document.documentElement, { childList: true, subtree: true })
    host.__caduceusSearchHandlers = handlers
    host.addEventListener('input', handlers.input, true)
    host.addEventListener('focusin', handlers.focus)
    host.addEventListener('blur', handlers.blur, true)
    host.addEventListener('keydown', handlers.keydown, true)
    host.addEventListener('scroll', handlers.scroll, true)
  }

  /**
   * Splice a pushed email into the loaded list (newest-first), de-duplicating
   * against anything an in-flight `load()` already fetched.
   *
   * @param {Record<string, any>} email
   * @returns {void}
   */
  onNewEmail(email) {
    if (!email?.id || this.$threadedMode) return
    if (this.$allEmails.some(e => e.id === email.id)) {
      this.$allEmails = this.$allEmails.map(e => e.id === email.id ? { ...e, ...email } : e)
      return
    }
    this.$allEmails = [email, ...this.$allEmails]
    try { this._publishFolderCounts() } catch {}
    // Badge it only if it's actually visible in the current folder/domain view
    // and the user isn't already parked at the top of the list (where they'd
    // see it land). Bursts accumulate into a single banner count.
    const visible = this.emails.some(e => e.id === email.id)
    const list = typeof document !== 'undefined' ? document.querySelector('.inbox-list') : null
    const atTop = !list || list.scrollTop <= 8
    if (visible && !atTop) this.$newMailCount += 1
  }

  /**
   * Banner action: jump to the freshly-arrived mail at the top and clear the
   * count. (The rows are already spliced in; this just reveals them.)
   *
   * @returns {void}
   */
  showNewMail() {
    this.$newMailCount = 0
    const list = document.querySelector('.inbox-list')
    if (list) list.scrollTo({ top: 0, behavior: 'smooth' })
  }

  /**
   * Clear the banner once the user scrolls back to the top on their own.
   *
   * @param {Event} [event]
   * @returns {void}
   */
  onListScroll(event) {
    const target = /** @type {HTMLElement | undefined} */ (event?.target)
    if (this.$newMailCount > 0 && target && target.scrollTop <= 8) {
      this.$newMailCount = 0
    }
    if (this.$contextMenu.open) this.closeContextMenu()
  }

  /**
   * Load the email list from the API, respecting current filters.
   *
   * - In threaded mode, fetches from `/threads`
   * - When the search term looks like a structured query (contains field:
   *   patterns), delegates to {@link search}
   * - Otherwise fetches from `/inbox` with optional `q`, `read`, and `starred`
   *   query parameters
   *
   * @async
   * @returns {Promise<void>}
   */
  async load() {
    this.$loading = true; this.$error = ''
    const apiFetch = this.apiFetch.bind(this)
    this._apiFetchRetries = 0
    try {
    // Threaded mode: fetch from /threads
    if (this.$threadedMode) {
      const threadParams = new URLSearchParams()
      if (this.$search.trim()) threadParams.set('q', this.searchQueryWithStatus(this.$search))
      else if (this.$selectedFolder) threadParams.set('folder', this.$selectedFolder)
      this.appendMailboxScope(threadParams)
      const threadSuffix = threadParams.toString() ? `?${threadParams.toString()}` : ''
      const threadRes = await apiFetch(`/threads${threadSuffix}`)
      this.$threads = await this.readArrayResponse(threadRes)
      this._pruneBulkSelection()
      this.$loading = false
      return
    }

    // If the search term contains query syntax or the user typed a search,
    // use the dedicated /search endpoint which supports the full query syntax
    // (from:, to:, subject:, has:attachment, is:unread, before:, after:, etc.)
    if (this.$search.trim()) {
      await this.search(this.searchQueryWithStatus(this.$search))
      return
    }
    const params = new URLSearchParams()
    if (this.$search.trim()) params.set('q', this.$search.trim())
    if (this.$statusFilter === 'unread') params.set('read', 'false')
    if (this.$statusFilter === 'starred') params.set('starred', 'true')
    this.appendMailboxScope(params)
    const suffix = params.toString() ? `?${params.toString()}` : ''
    const res = await apiFetch(`/inbox${suffix}`)
    if (!res?.ok) throw new Error('Mailbox is unavailable')
    this.$allEmails = await this.readArrayResponse(res)
    void cacheOfflineMailboxSummaries(this.offlineMailboxOwner, this.$allEmails)
    this._pruneBulkSelection()
    this._publishFolderCounts()
    } catch (err) {
      this.$allEmails = await searchOfflineMailbox(this.offlineMailboxOwner, {
        folder: this.$selectedFolder, domain: this.$selectedDomain === 'all' ? '' : this.$selectedDomain,
        account: this.$selectedAccount, read: this.$statusFilter === 'unread' ? false : undefined,
        starred: this.$statusFilter === 'starred' ? true : undefined, limit: 500,
      })
      this.$error = this.$allEmails.length ? '' : ((err instanceof Error ? err.message : String(err)) || 'Failed to load emails')
    }
    this.$loading = false
  }

  /**
   * Read an API response that should contain a JSON array.
   *
   * @param {Response | null | undefined} res
   * @returns {Promise<Array<Record<string, any>>>}
   */
  async readArrayResponse(res) {
    if (!res?.ok) return []
    const data = await res.json()
    if (Array.isArray(data)) return data
    if (data?.unauthorized) return []
    throw new Error(data?.error || 'Unexpected inbox response')
  }

  /** Refresh exact-owner category capabilities and nested catalog entries. */
  async loadCategoryCatalog() {
    try {
      const response = await this.apiFetch('/categories')
      const data = response?.ok ? await response.json() : null
      this.$categoryAccounts = Array.isArray(data?.accounts) ? data.accounts : []
    } catch {
      this.$categoryAccounts = []
    }
  }

  /**
   * Publish per-folder email counts to the app shell so it can render the
   * sidebar badges. Fires a `caduceus:folder-counts` window event with a
   * `{ folderName: count }` map derived from the currently loaded emails.
   *
   * Only meaningful when the inbox is loaded without folder/status filters
   * (otherwise the counts would reflect the filtered subset, not the
   * mailbox totals). The filtered case is short-circuited so we don't
   * clobber a previously-published accurate count.
   *
   * @private
   * @returns {void}
   */
  _publishFolderCounts() {
    // Skip if the load was filtered — counts would be misleading.
    if (this.$search.trim() || this.$statusFilter !== 'all') return
    // Publish BOTH the aggregate (so Favorites/All Inboxes stays accurate
    // across all domains) and a per-domain breakdown (so per-domain sidebar
    // rows show their own counts). Iterate over $allEmails — the unfiltered
    // pool — instead of domainEmails so the aggregate reflects every domain
    // even when the user has a domain filter active in the dropdown.
    /** @type {Record<string, number>} */
    const counts = {}
    /** @type {Record<string, Record<string, number>>} */
    const byDomain = {}
    /** @type {Record<string, Record<string, number>>} */
    const byAccount = {}
    let unreadInbox = 0
    /** @type {Record<string, number>} */
    const unreadInboxByDomain = {}
    for (const e of this.$allEmails || []) {
      const folder = logicalFolder(e)
      const domain = String((e && e.domain) || '').toLowerCase() || 'unknown'
      const accountId = String(e?.providerAccountId || '')
      counts[folder] = (counts[folder] || 0) + 1
      const scopedCounts = accountId
        ? (byAccount[accountId] ||= {})
        : (byDomain[domain] ||= {})
      scopedCounts[folder] = (scopedCounts[folder] || 0) + 1
      if (folder === 'inbox' && e?.read !== true) {
        unreadInbox++
        if (!accountId) unreadInboxByDomain[domain] = (unreadInboxByDomain[domain] || 0) + 1
      }
    }
    window.dispatchEvent(new CustomEvent('caduceus:folder-counts', {
      detail: { counts, byDomain, byAccount, unreadInbox, unreadInboxByDomain },
    }))
  }

  /**
   * Keep the selected domain inside the current user's allowed domains.
   *
   * @returns {void}
   */
  ensureSelectedDomain() {
    if (this.$selectedAccount) {
      if (!/^ext_[A-Za-z0-9_-]{20,80}$/.test(this.$selectedAccount)) this.$selectedAccount = ''
      this.$selectedDomain = 'all'
      return
    }
    if (this.$selectedDomain === 'all') return
    if (!this.domainOptions.includes(this.$selectedDomain)) this.$selectedDomain = 'all'
  }

  messageMatchesSelectedMailbox(email) {
    if (this.$selectedAccount) return email?.providerAccountId === this.$selectedAccount
    if (this.$selectedDomain === 'all') return true
    return !email?.providerAccountId && String(email?.domain || '').toLowerCase() === this.$selectedDomain
  }

  appendMailboxScope(params) {
    if (this.$selectedAccount) params.set('account', this.$selectedAccount)
    else if (this.$selectedDomain !== 'all') params.set('domain', this.$selectedDomain)
    return params
  }

  /**
   * Change the domain filter without reauthenticating.
   *
   * @param {string} domain
   * @returns {void}
   */
  setDomainFilter(domain) {
    const normalized = String(domain || 'all').trim().toLowerCase()
    this.$selectedDomain = normalized === 'all' || this.domainOptions.includes(normalized) ? normalized : 'all'
    this.$selectedAccount = ''
    this.$selectedId = ''
    this.clearBulkSelection()
    this._publishFolderCounts()
  }

  /** Change the logical folder and discard selection from the previous view. */
  setFolder(folder) {
    const normalized = String(folder || 'inbox').trim().toLowerCase()
    this.$selectedFolder = normalized || 'inbox'
    this.$selectedId = ''
    this.clearBulkSelection()
  }

  /** Normalize either a DuVay value event or a native select event. */
  changeDomainFilter(event) {
    this.setDomainFilter(event?.value ?? event?.target?.value)
    if (this.$search.trim() || this.$threadedMode) void this.load()
  }

  setMailboxFilter(value) {
    const selected = String(value || 'all')
    if (selected.startsWith('account:')) {
      const accountId = selected.slice('account:'.length)
      this.$selectedAccount = this.$externalAccounts.some(account => account.id === accountId) ? accountId : ''
      this.$selectedDomain = 'all'
    } else if (selected.startsWith('domain:')) {
      this.setDomainFilter(selected.slice('domain:'.length))
    } else {
      this.$selectedAccount = ''
      this.$selectedDomain = 'all'
    }
    this.$selectedId = ''
    this.clearBulkSelection()
    this._publishFolderCounts()
  }

  changeMailboxFilter(event) {
    this.setMailboxFilter(event?.value ?? event?.target?.value)
    if (this.$search.trim() || this.$threadedMode) void this.load()
  }

  /**
   * Performs a full-text search using the /search endpoint with Gmail-like
   * query syntax support (from:, to:, subject:, has:attachment, is:unread,
   * before:, after:, etc.).
   *
   * @async
   * @param {string} q - The search query
   * @returns {Promise<void>}
   */
  async search(q) {
    const apiFetch = this.apiFetch.bind(this)
    const params = new URLSearchParams({ q, limit: '1000' })
    this.appendMailboxScope(params)
    let res
    try { res = await apiFetch(`/search?${params}`) } catch {}
    if (res?.ok) {
      this.$allEmails = await this.readArrayResponse(res)
      void cacheOfflineMailboxSummaries(this.offlineMailboxOwner, this.$allEmails)
    } else {
      this.$allEmails = await searchOfflineMailbox(this.offlineMailboxOwner, {
        query: q, domain: this.$selectedDomain === 'all' ? '' : this.$selectedDomain,
        account: this.$selectedAccount, limit: 500,
      })
      if (!this.$allEmails.length) this.$error = 'Search is unavailable offline until messages have been synchronized.'
    }
    this._pruneBulkSelection()
    this.$loading = false
  }

  get offlineMailboxOwner() {
    return String(window._caduceus?.auth?.email || sessionStorage.getItem('caduceus_email') || '').toLowerCase()
  }

  /** Keep chip filters meaningful while a structured query is active. */
  searchQueryWithStatus(query) {
    const text = String(query || '').trim()
    if (this.$statusFilter === 'unread' && !/(^|\s)is:(?:un)?read\b/i.test(text)) return `${text} is:unread`.trim()
    if (this.$statusFilter === 'starred' && !/(^|\s)is:(?:un)?starred\b/i.test(text)) return `${text} is:starred`.trim()
    return text
  }

  /**
   * Fetch through the shell helper when available, otherwise use the session
   * token directly so first-load lazy components are not blocked by mount order.
   *
   * @param {string} path
   * @param {RequestInit} [options]
   * @returns {Promise<Response | null>}
   */
  async apiFetch(path, options = {}) {
    const shellFetch = window._caduceus?.apiFetch
    if (shellFetch) return shellFetch(path, options)

    const token = sessionStorage.getItem('caduceus_token')
    const apiUrl = window.CADUCEUS_CONFIG?.apiUrl || location.origin
    const url = path.startsWith('http') ? path : `${apiUrl}${path}`
    const headers = new Headers(options.headers || {})
    if (token && !headers.has('authorization')) headers.set('authorization', `Bearer ${token}`)
    if (options.body && !headers.has('content-type')) headers.set('content-type', 'application/json')
    return fetch(url, { ...options, headers })
  }

  /** @param {Record<string, any>} suggestion */
  searchSuggestionClass(suggestion) {
    return suggestion.id === this.searchActiveDescendant ? 'search-suggestion active' : 'search-suggestion'
  }

  /** @param {Record<string, any>} suggestion */
  searchSuggestionSelected(suggestion) { return String(suggestion.id === this.searchActiveDescendant) }

  /** @param {Record<string, any>} suggestion */
  searchSuggestionTitle(suggestion) {
    if (!suggestion.token) return suggestion.term
    return `${suggestion.token}:${suggestion.term || ''}`
  }

  /** Paint tokens without asking Tachyon to replace the actively edited input. */
  renderSearchDecorations() {
    const layer = document.querySelector('[data-search-token-layer]')
    if (layer) {
      layer.replaceChildren(...searchTokenSegments(this.searchState.draft).map(segment => {
        const span = document.createElement('span')
        if (segment.token) span.className = 'search-token'
        span.textContent = segment.text
        return span
      }))
    }
    const clear = /** @type {HTMLButtonElement | null} */ (document.querySelector('.search-clear-btn'))
    if (clear) clear.hidden = !this.searchState.draft
  }

  /** Render an ARIA listbox using the stable autocomplete pattern used by Compose. */
  renderSearchSuggestions() {
    const list = /** @type {HTMLElement | null} */ (document.querySelector('#thread-search-suggestions'))
    const input = this.searchInput()
    const status = /** @type {HTMLElement | null} */ (document.querySelector('[data-search-suggestion-status]'))
    const visible = this.searchState.focused && this.searchState.suggestions.length > 0
    if (list) {
      list.replaceChildren()
      for (const suggestion of this.searchState.suggestions) {
        const option = document.createElement('button')
        option.type = 'button'
        option.id = suggestion.id
        option.className = this.searchSuggestionClass(suggestion)
        option.setAttribute('role', 'option')
        option.setAttribute('aria-selected', this.searchSuggestionSelected(suggestion))
        const expression = document.createElement('span')
        expression.className = 'search-suggestion-expression'
        expression.textContent = this.searchSuggestionTitle(suggestion)
        const description = document.createElement('span')
        description.className = 'search-suggestion-description'
        description.textContent = suggestion.description || ''
        option.append(expression, description)
        option.addEventListener('mousedown', event => { void this.chooseSearchSuggestion(event, suggestion) })
        list.append(option)
      }
      list.hidden = !visible
    }
    if (input) {
      input.setAttribute('aria-expanded', String(visible))
      if (visible && this.searchActiveDescendant) input.setAttribute('aria-activedescendant', this.searchActiveDescendant)
      else input.removeAttribute('aria-activedescendant')
    }
    if (status) status.textContent = visible ? `${this.searchState.suggestions.length} search suggestions available` : ''
  }

  /** Read the native field because it owns selection and composition state. */
  searchInput() {
    return /** @type {HTMLInputElement | null} */ (document.querySelector('[data-thread-search-input]'))
  }

  /** @param {string} value @param {number} [caret] */
  setSearchValue(value, caret = String(value || '').length) {
    this.searchState.draft = String(value || '')
    this.searchState.caret = caret
    const input = this.searchInput()
    if (input) {
      input.value = this.searchState.draft
      try { input.setSelectionRange(caret, caret) } catch {}
    }
    this.syncSearchOverlay()
    this.bindSearchInput()
    this.renderSearchDecorations()
  }

  /** Seed category/status context, then open token suggestions. */
  onSearchFocus(event) {
    const input = /** @type {HTMLInputElement | null} */ (event?.target || this.searchInput())
    const value = input?.value ?? this.searchState.draft
    const caret = input?.selectionStart ?? value.length
    this.searchState.focused = true
    const seeded = value || (this.initialSearchQuery ? `${this.initialSearchQuery} ` : '')
    this.setSearchValue(seeded, value ? caret : seeded.length)
    void this.refreshSearchSuggestions()
  }

  /** @param {Event} event */
  onSearchInput(event) {
    const input = /** @type {HTMLInputElement} */ (event.target)
    this.searchState.request += 1
    this.searchState.draft = input.value
    this.searchState.focused = document.activeElement === input
    this.searchState.caret = input.selectionStart ?? input.value.length
    this.searchState.suggestions = staticSearchSuggestions(input.value, this.searchState.caret)
      .map((suggestion, index) => ({
        ...suggestion,
        id: `thread-search-option-${this.searchState.request}-local-${index}`,
      }))
    this.searchState.selectedIndex = this.searchState.suggestions.length ? 0 : -1
    this.syncSearchOverlay(event)
    this.renderSearchDecorations()
    this.renderSearchSuggestions()
    this.queueSearchSuggestions()
  }

  /** Keep highlighted tokens aligned while a long query scrolls horizontally. */
  syncSearchOverlay(event) {
    const input = /** @type {HTMLInputElement | null} */ (event?.target || this.searchInput())
    const overlay = /** @type {HTMLElement | null} */ (document.querySelector('[data-search-token-layer]'))
    if (input && overlay) overlay.style.transform = `translateX(${-input.scrollLeft}px)`
  }

  onSearchBlur() {
    setTimeout(() => {
      this.searchState.focused = false
      this.searchState.selectedIndex = -1
      this.renderSearchSuggestions()
    }, 140)
  }

  /** @param {KeyboardEvent} event */
  onSearchKeydown(event) {
    if (event.isComposing) return
    const count = this.searchState.suggestions.length
    if (event.key === 'ArrowDown' && count) {
      event.preventDefault()
      const value = this.searchInput()?.value ?? this.searchState.draft
      this.searchState.focused = true
      this.searchState.selectedIndex = (this.searchState.selectedIndex + 1 + count) % count
      this.setSearchValue(value, this.searchState.caret)
      this.renderSearchSuggestions()
      return
    }
    if (event.key === 'ArrowUp' && count) {
      event.preventDefault()
      const value = this.searchInput()?.value ?? this.searchState.draft
      this.searchState.focused = true
      this.searchState.selectedIndex = (this.searchState.selectedIndex - 1 + count) % count
      this.setSearchValue(value, this.searchState.caret)
      this.renderSearchSuggestions()
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      const value = this.searchInput()?.value ?? this.searchState.draft
      this.searchState.suggestions = []
      this.searchState.selectedIndex = -1
      this.setSearchValue(value, this.searchState.caret)
      this.renderSearchSuggestions()
      return
    }
    if (event.key !== 'Enter') return
    event.preventDefault()
    const suggestion = this.searchState.suggestions[this.searchState.selectedIndex]
    if (suggestion) void this.chooseSearchSuggestion(suggestion)
    else void this.submitSearch()
  }

  queueSearchSuggestions() {
    if (this.searchState.timer) clearTimeout(this.searchState.timer)
    this.searchState.timer = setTimeout(() => { void this.refreshSearchSuggestions() }, 90)
  }

  /** Build local vocabulary first, then enrich it with owned contacts and mailbox data. */
  async refreshSearchSuggestions() {
    const request = ++this.searchState.request
    const input = this.searchInput()
    const query = input?.value ?? this.searchState.draft
    const caret = input?.selectionStart ?? this.searchState.caret ?? query.length
    const current = getCurrentTokenAndTerm(query, caret)
    const suggestions = staticSearchSuggestions(query, caret)
    const dynamic = []
    const term = String(current.term || '').trim()

    if (current.token === 'in') {
      const accounts = this.$selectedAccount
        ? []
        : this.$selectedDomain === 'all'
        ? this.$categoryAccounts
        : this.$categoryAccounts.filter(account => account.domain === this.$selectedDomain)
      const seen = new Set()
      for (const category of accounts.flatMap(account => account.categories || [])) {
        const value = String(category.role || category.path || '').trim()
        if (!value || seen.has(value.toLowerCase()) || (term && !value.toLowerCase().includes(term))) continue
        seen.add(value.toLowerCase())
        dynamic.push({
          token: 'in', term: /\s/.test(value) ? wrapSearchTerm(value) : value,
          description: category.path || value, submit: true, kind: 'category',
        })
      }
      for (const providerFolder of this.$externalFolders?.[this.$selectedAccount] || []) {
        const value = String(providerFolder.role || providerFolder.name || '').trim()
        if (!value || (term && !value.toLowerCase().includes(term))) continue
        dynamic.push({ token: 'in', term: /\s/.test(value) ? wrapSearchTerm(value) : value, description: providerFolder.name || value, submit: true, kind: 'category' })
      }
    }

    const wantsContacts = ['from', 'to'].includes(current.token || '')
      || (!current.token && query.trim().length >= 2)
    const wantsSubjects = current.token === 'subject'
      || (!current.token && query.trim().length >= 3)
    const lookup = term || (!current.token ? query.trim() : '')
    const requests = []
    if (wantsContacts) requests.push(this.apiFetch(`/contacts/suggestions?q=${encodeURIComponent(lookup)}`)
      .then(response => response?.ok ? response.json() : []).catch(() => []))
    else requests.push(Promise.resolve([]))
    if (wantsSubjects) {
      const params = new URLSearchParams({ q: `subject:${wrapSearchTerm(lookup)}`, limit: '12' })
      this.appendMailboxScope(params)
      requests.push(this.apiFetch(`/search?${params}`).then(response => response?.ok ? response.json() : []).catch(() => []))
    } else requests.push(Promise.resolve([]))

    const [contacts, messages] = await Promise.all(requests)
    if (request !== this.searchState.request) return
    for (const contact of contacts) {
      if (!contact?.email) continue
      dynamic.push({
        token: current.token === 'to' ? 'to' : 'from', term: wrapSearchTerm(contact.email),
        description: contact.name ? `${contact.name} · ${contact.email}` : contact.email,
        submit: true, kind: 'contact',
      })
    }
    const subjects = new Set()
    for (const message of messages) {
      const subject = String(message?.subject || '').trim()
      if (!subject || subjects.has(subject.toLowerCase())) continue
      subjects.add(subject.toLowerCase())
      dynamic.push({ token: 'subject', term: wrapSearchTerm(subject), description: subject, submit: true, kind: 'subject' })
    }

    const unique = []
    const keys = new Set()
    for (const suggestion of [...suggestions, ...dynamic]) {
      const key = `${suggestion.token || ''}\0${suggestion.term || ''}`.toLowerCase()
      if (keys.has(key)) continue
      keys.add(key)
      unique.push({ ...suggestion, id: `thread-search-option-${request}-${unique.length}` })
      if (unique.length >= 12) break
    }
    this.searchState.suggestions = unique
    this.searchState.selectedIndex = unique.length ? 0 : -1
    this.setSearchValue(query, caret)
    this.renderSearchSuggestions()
  }

  /** @param {Event|Record<string,any>} eventOrSuggestion @param {Record<string,any>} [requestedSuggestion] */
  async chooseSearchSuggestion(eventOrSuggestion, requestedSuggestion) {
    const suggestion = requestedSuggestion || (eventOrSuggestion?.target
      ? null
      : /** @type {Record<string, any>} */ (eventOrSuggestion))
    if (!suggestion || typeof suggestion.term !== 'string') return
    if (eventOrSuggestion?.preventDefault) eventOrSuggestion.preventDefault()
    const input = this.searchInput()
    const caret = input?.selectionStart ?? this.searchState.caret ?? this.searchState.draft.length
    const next = applySearchSuggestion(
      input?.value ?? this.searchState.draft,
      caret,
      /** @type {{token?:string|null,term:string,submit?:boolean}} */ (suggestion),
    )
    this.setSearchValue(next.query)
    this.searchState.suggestions = []
    this.searchState.selectedIndex = -1
    this.setSearchValue(next.query)
    this.renderSearchSuggestions()
    if (next.submit) await this.submitSearch()
    else {
      input?.focus()
      await this.refreshSearchSuggestions()
    }
  }

  async submitSearch() {
    const input = this.searchInput()
    const query = input?.value ?? this.searchState.draft
    this.searchState.draft = query
    this.$search = query
    this.searchState.focused = false
    this.searchState.suggestions = []
    this.searchState.selectedIndex = -1
    this.setSearchValue(query)
    this.renderSearchSuggestions()
    await this.load()
    this.setSearchValue(query)
    setTimeout(() => this.setSearchValue(query), 0)
  }

  /**
   * Update a specific email via the API and refresh the local list.
   *
   * @async
   * @param {Record<string, any>} email - The email object to update
   * @param {Record<string, any>} patch - Key/value pairs to update
   * @returns {Promise<Record<string, any>>} The updated email object, or the original on failure
   */
  async updateEmail(email, patch) {
    const result = await this.applyMailboxAction([email.id], patch, 'Message updated.')
    const updated = result?.emails?.find(item => item.id === email.id)
    return updated ? { ...email, ...updated } : email
  }

  /**
   * Clear the search input and status filter, then reload the inbox.
   *
   * @async
   * @returns {Promise<void>}
   */
  async clearSearch() {
    this.searchState.request += 1
    if (this.searchState.timer) clearTimeout(this.searchState.timer)
    this.searchState.suggestions = []
    this.searchState.selectedIndex = -1
    this.setSearchValue('')
    this.renderSearchSuggestions()
    this.$search = ''
    this.$statusFilter = 'all'
    await this.load()
    this.setSearchValue('')
    setTimeout(() => this.setSearchValue(''), 0)
  }

  /**
   * Cycle through the three density presets: comfortable -> compact -> default.
   *
   * Persists the selection to localStorage.
   *
   * @returns {void}
   */
  toggleDensity() {
    const cycle = { comfortable: 'compact', compact: 'default', default: 'comfortable' }
    this.$density = cycle[this.$density] || 'comfortable'
    localStorage.setItem('caduceus-density', this.$density)
  }

  /**
   * Toggle threaded (conversation) mode on or off and reload the list.
   *
   * Resets expanded thread state and selected email when switching modes.
   *
   * @async
   * @returns {Promise<void>}
   */
  async toggleThreadMode() {
    this.$threadedMode = !this.$threadedMode
    this.$expandedThreads = {}
    this.$selectedId = ''
    this.clearBulkSelection()
    await this.load()
  }

  /**
   * Set the read/star status filter and reload the list.
   *
   * @param {'all' | 'unread' | 'starred'} status - The filter to apply.
   * @returns {Promise<void>}
   */
  async setStatusFilter(status) {
    this.$statusFilter = status
    this.clearBulkSelection()
    await this.load()
  }

  /**
   * Expand or collapse a thread by its stable RFC-derived identity.
   *
   * @param {Event | string} eventOrId - Tachyon event, or thread identity
   * @param {string} [requestedId] - Static template argument when Tachyon injects the event first
   * @returns {void}
   */
  toggleThread(eventOrId, requestedId) {
    const id = typeof eventOrId === 'string' ? eventOrId : requestedId
    if (!id) return
    const expanded = { ...this.$expandedThreads }
    if (expanded[id]) {
      delete expanded[id]
    } else {
      expanded[id] = true
    }
    this.$expandedThreads = expanded
  }

  /**
   * Handle a click on an email row.
   *
   * Drafts open in the composer for editing (Gmail/Apple-Mail style); every
   * other folder publishes the `caduceus:mail-select` event so a parent split-pane can
   * show the read-only detail view.
   *
   * @param {Event | string} eventOrId - Tachyon event, or an email ID for internal callers
   * @param {string} [eventId] - The email ID supplied by a Tachyon template event
   * @returns {void}
   */
  selectEmail(eventOrId, eventId) {
    const id = typeof eventOrId === 'string' ? eventOrId : eventId
    if (!id) return
    this.$focusedId = id
    this.$selectedId = id
    const email = this.$allEmails.find(e => e.id === id)
    if ((email?.folder || this.$selectedFolder) === 'drafts') {
      window._caduceus?.compose?.({ draftId: id })
      return
    }
    window.dispatchEvent(new CustomEvent('caduceus:mail-select', { detail: { emailId: id } }))
  }

  /** Activate an email row from the keyboard. */
  activateEmail(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {KeyboardEvent} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    if (this._handleRovingKey(event, id)) return
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    this.selectEmail(id)
  }

  /** Expand or collapse a thread from the keyboard. */
  activateThread(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {KeyboardEvent} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    if (this._handleRovingKey(event, id)) return
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    this.toggleThread(id)
  }

  /** Keep exactly one rendered row in the natural tab order. */
  rowTabIndex(id) {
    const ids = this.rovingRowIds
    const active = ids.includes(this.$focusedId) ? this.$focusedId : ids[0]
    return id === active ? 0 : -1
  }

  _handleRovingKey(event, id) {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return false
    event.preventDefault()
    const ids = this.rovingRowIds
    if (ids.length === 0) return true
    const current = Math.max(0, ids.indexOf(id))
    const index = event.key === 'Home' ? 0
      : event.key === 'End' ? ids.length - 1
        : Math.min(ids.length - 1, Math.max(0, current + (event.key === 'ArrowDown' ? 1 : -1)))
    this.$focusedId = ids[index]
    setTimeout(() => /** @type {HTMLElement | null} */ (document.querySelector(`[data-roving-id="${CSS.escape(ids[index])}"]`))?.focus(), 0)
    return true
  }

  /** Open a bounded accessible action menu for one message row. */
  openEmailContextMenu(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {MouseEvent} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    if (!id) return
    event?.preventDefault()
    this._openContextMenu([id], event)
  }

  /** Open the same action surface for every message in a conversation row. */
  openThreadContextMenu(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {MouseEvent} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    const thread = this.visibleThreads.find(item => item.id === id)
    if (!thread) return
    event?.preventDefault()
    this._openContextMenu((thread.emails || []).map(email => email.id), event)
  }

  _openContextMenu(ids, event) {
    const x = Math.max(8, Math.min(Number(event?.clientX) || 8, window.innerWidth - 232))
    const y = Math.max(8, Math.min(Number(event?.clientY) || 8, window.innerHeight - 320))
    this.$contextMenu = { open: true, ids: [...new Set(ids)], x, y }
    setTimeout(() => /** @type {HTMLElement | null} */ (document.querySelector('.inbox-context-menu [role="menuitem"]'))?.focus(), 0)
  }

  closeContextMenu() { this.$contextMenu = { open: false, ids: [], x: 0, y: 0 } }
  stopContextPropagation(event) { event?.stopPropagation() }
  contextMenuKeydown(event) { if (event?.key === 'Escape') { event.preventDefault(); this.closeContextMenu() } }

  async contextArchive() { await this._contextAction({ folder: 'archive' }, 'Archived.') }
  async contextTrash() { await this._contextAction({ folder: 'trash' }, 'Moved to trash.') }
  async contextToggleRead() { await this._contextAction({ read: this.contextEmail?.read !== true }, this.contextEmail?.read ? 'Marked as unread.' : 'Marked as read.') }
  async contextToggleStar() { await this._contextAction({ starred: this.contextEmail?.starred !== true }, this.contextEmail?.starred ? 'Unstarred.' : 'Starred.') }
  contextMove() { this._openContextCategoryPicker('move') }
  contextLabels() { this._openContextCategoryPicker('label') }
  contextOpen() { const id = this.$contextMenu.ids[0]; this.closeContextMenu(); if (id) this.selectEmail(id) }

  _openContextCategoryPicker(mode) {
    const ids = [...this.$contextMenu.ids]
    this.closeContextMenu()
    this.openCategoryPicker(mode, ids)
  }

  openBulkMovePicker() { this.openCategoryPicker('move', this.$selectedIds) }
  openBulkLabelPicker() { this.openCategoryPicker('label', this.$selectedIds) }

  /** Open one account-scoped picker, matching Mailspring's account boundary. */
  openCategoryPicker(mode, ids) {
    const selectedIds = [...new Set(Array.isArray(ids) ? ids : [])]
    const messages = this._messagesForIds(selectedIds)
    const domain = singleSelectionDomain(messages)
    if (!domain) {
      window._caduceus?.toast('Choose messages from one mailbox to change folders or labels.')
      return
    }
    const account = this.$categoryAccounts.find(item => item.domain === domain)
    const capability = mode === 'label' ? 'labels' : 'folders'
    if (!account?.capabilities?.[capability]) {
      window._caduceus?.toast(`This mailbox does not support ${capability}.`)
      return
    }
    this._categoryPickerReturnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    this.$categoryPicker = { open: true, mode, ids: selectedIds, domain, search: '', creating: false, error: '' }
    setTimeout(() => /** @type {HTMLInputElement | null} */ (document.querySelector('.category-picker-search'))?.focus(), 0)
  }

  closeCategoryPicker() {
    this.$categoryPicker = { open: false, mode: 'move', ids: [], domain: '', search: '', creating: false, error: '' }
    const returnFocus = this._categoryPickerReturnFocus
    this._categoryPickerReturnFocus = null
    setTimeout(() => returnFocus?.focus(), 0)
  }

  stopCategoryPropagation(event) { event?.stopPropagation() }

  categorySearch(event) {
    const value = String(event?.target?.value ?? event?.value ?? '')
    this.$categoryPicker = { ...this.$categoryPicker, search: value, error: '' }
  }

  categoryPickerIndent(item) { return `padding-inline-start:${12 + Math.min(6, Number(item?.depth) || 0) * 18}px` }
  categoryPickerCheck(item) { return item?.selectedState === 'all' ? '✓' : item?.selectedState === 'mixed' ? '−' : '' }
  categoryPickerAriaSelected(item) { return item?.selectedState === 'all' ? 'true' : 'false' }

  categoryPickerKeydown(event) {
    if (event?.key === 'Escape') { event.preventDefault(); this.closeCategoryPicker(); return }
    const dialog = document.querySelector('.category-picker-dialog')
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
    const options = focusable.filter(element => element.classList?.contains('category-picker-option'))
    if (options.length === 0) return
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
    if (!path) return
    await this._applyCategoryPath(path)
  }

  async createCategory() {
    const path = this.categoryPickerCreatePath
    if (!this.categoryPickerCanCreate || !path) return
    this.$categoryPicker = { ...this.$categoryPicker, creating: true, error: '' }
    try {
      const response = await this.apiFetch('/categories', {
        method: 'POST',
        body: JSON.stringify({
          domain: this.$categoryPicker.domain,
          kind: this.$categoryPicker.mode === 'label' ? 'label' : 'folder',
          path,
        }),
      })
      const result = await response?.json().catch(() => null)
      if (!response?.ok || !result?.category?.path) throw new Error(result?.error || 'Category creation failed.')
      await this.loadCategoryCatalog()
      await this._applyCategoryPath(result.category.path)
    } catch (error) {
      this.$categoryPicker = {
        ...this.$categoryPicker,
        creating: false,
        error: error instanceof Error ? error.message : 'Category creation failed.',
      }
    }
  }

  async _applyCategoryPath(path) {
    const messages = this.categoryPickerMessages
    const patch = this.$categoryPicker.mode === 'label'
      ? categoryLabelPatch(messages, path)
      : { folder: path }
    const verb = this.$categoryPicker.mode === 'label' ? 'Updated labels.' : `Moved to ${path}.`
    const result = await this.applyMailboxAction(this.$categoryPicker.ids, patch, verb)
    if (!result) return
    this.clearBulkSelection()
    this.closeCategoryPicker()
  }

  _messagesForIds(ids) {
    const wanted = new Set(ids)
    const messages = [...this.$allEmails, ...this.$threads.flatMap(thread => thread.emails || [])]
    return [...new Map(messages.filter(message => wanted.has(message.id)).map(message => [message.id, message])).values()]
  }

  async _contextAction(patch, message) {
    const ids = [...this.$contextMenu.ids]
    this.closeContextMenu()
    if (ids.length) await this.applyMailboxAction(ids, patch, message)
  }

  /**
   * Initialise a drag operation for an email row.
   *
   * @param {DragEvent|string} eventOrId - Tachyon event, or an ID for internal callers
   * @param {string|DragEvent} requestedIdOrEvent - Template ID, or internal event
   * @returns {void}
   */
  dragStart(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {DragEvent} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    const ids = this.$selectedIds.includes(id) && this.$selectedIds.length > 1 ? this.$selectedIds : [id]
    event.dataTransfer?.setData('text/plain', id)
    event.dataTransfer?.setData('application/x-caduceus-mail-ids', JSON.stringify(ids))
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
  }

  /**
   * Apply an undoable server action and reconcile the loaded rows immediately.
   * @param {string[]} ids
   * @param {Record<string, any>} patch
   * @param {string} message
   * @returns {Promise<Record<string, any> | null>}
   */
  async applyMailboxAction(ids, patch, message) {
    /** @type {Record<string, any> | null} */
    let result = null
    if (window._caduceus?.performMailAction) {
      result = await window._caduceus.performMailAction(ids, patch, message)
    } else {
      try {
        const response = await this.apiFetch('/inbox/actions', {
          method: 'POST', body: JSON.stringify({ ids, patch }),
        })
        result = await response?.json().catch(() => null)
        if (!response?.ok || !result?.actionId) {
          window._caduceus?.toast(result?.error || 'Mail action failed.')
          return null
        }
        window._caduceus?.toast(message)
      } catch {
        window._caduceus?.toast('Mail action failed.')
        return null
      }
    }
    if (result) this._mergeActionEmails(result.emails || [])
    return result
  }

  /** @param {Array<Record<string, any>>} emails */
  _mergeActionEmails(emails) {
    const updates = new Map(emails.map(email => [email.id, email]))
    this.$allEmails = this.$allEmails.map(email => updates.has(email.id)
      ? { ...email, ...updates.get(email.id) }
      : email)
    this.$threads = this.$threads.map(thread => ({
      ...thread,
      emails: (thread.emails || []).map(email => updates.has(email.id)
        ? { ...email, ...updates.get(email.id) }
        : email),
    }))
    this._publishFolderCounts()
  }

  /** @param {string} id */
  isBulkSelected(id) { return this.$selectedIds.includes(id) }

  /** Toggle one row without activating the reader. */
  toggleBulkSelection(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    if (!id) return
    const selected = new Set(this.$selectedIds)
    if (selected.has(id)) selected.delete(id)
    else selected.add(id)
    this.$selectedIds = [...selected]
  }

  /** Select or clear every message represented by one conversation row. */
  toggleThreadSelection(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    const thread = this.visibleThreads.find(item => item.id === id)
    if (!thread) return
    const ids = (thread.emails || []).map(email => email.id)
    const selected = new Set(this.$selectedIds)
    const remove = ids.every(emailId => selected.has(emailId))
    for (const emailId of ids) remove ? selected.delete(emailId) : selected.add(emailId)
    this.$selectedIds = [...selected]
  }

  /** @param {Record<string, any>} thread */
  isThreadSelected(thread) {
    const ids = (thread?.emails || []).map(email => email.id)
    return ids.length > 0 && ids.every(id => this.$selectedIds.includes(id))
  }

  selectAllVisible() { this.$selectedIds = this.visibleMessageIds }
  clearBulkSelection() { this.$selectedIds = [] }
  toggleAllVisibleSelection() { this.allVisibleSelected ? this.clearBulkSelection() : this.selectAllVisible() }

  bulkArchive() { return this.bulkAction({ folder: 'archive' }, 'Archived selected messages.') }
  bulkTrash() { return this.bulkAction({ folder: 'trash' }, 'Moved selected messages to trash.') }
  bulkMarkRead() { return this.bulkAction({ read: true }, 'Marked selected messages as read.') }
  bulkMarkUnread() { return this.bulkAction({ read: false }, 'Marked selected messages as unread.') }
  bulkStar() { return this.bulkAction({ starred: true }, 'Starred selected messages.') }
  bulkUnstar() { return this.bulkAction({ starred: false }, 'Unstarred selected messages.') }

  /** @param {Record<string, string | boolean>} patch @param {string} message */
  async bulkAction(patch, message) {
    const ids = [...this.$selectedIds]
    if (ids.length === 0) return
    const result = await this.applyMailboxAction(ids, patch, message)
    if (result) this.clearBulkSelection()
  }

  _pruneBulkSelection() {
    const visible = new Set(this.visibleMessageIds)
    this.$selectedIds = this.$selectedIds.filter(id => visible.has(id))
  }

  /**
   * Toggle the starred state of an email via a quick inline action.
   *
   * @async
   * @param {Event|string} eventOrId - Tachyon event, or an ID for shortcut callers
   * @param {string|Event} [requestedIdOrEvent] - Template ID, or shortcut event
   * @returns {Promise<void>}
   */
  async quickStar(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    const row = this.$allEmails.find(m => m.id === id)
    if (!row) return
    await this.applyMailboxAction([id], { starred: !row.starred }, row.starred ? 'Unstarred.' : 'Starred.')
  }

  /**
   * Archive an email via a quick inline action.
   *
   * @async
   * @param {Event|string} eventOrId - Tachyon event, or an ID for shortcut callers
   * @param {string|Event} [requestedIdOrEvent] - Template ID, or shortcut event
   * @returns {Promise<void>}
   */
  async quickArchive(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    if (!id) return
    await this.applyMailboxAction([id], { folder: 'archive' }, 'Archived.')
  }

  /**
   * Move an email to trash via a quick inline action.
   *
   * @async
   * @param {Event|string} eventOrId - Tachyon event, or an ID for shortcut callers
   * @param {string|Event} [requestedIdOrEvent] - Template ID, or shortcut event
   * @returns {Promise<void>}
   */
  async quickTrash(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    if (!id) return
    await this.applyMailboxAction([id], { folder: 'trash' }, 'Moved to trash.')
  }

  // TODO(markup): wire a "Report spam"/"Not spam" quick-action button + spam-score badge.

  /**
   * Report an email as spam through the feedback endpoint.
   *
   * @async
   * @param {Event|string} eventOrId
   * @param {string|Event} [requestedIdOrEvent]
   * @returns {Promise<void>}
   */
  async reportSpam(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    if (!id) return
    const res = await this.apiFetch(`/inbox/${id}/spam`, { method: 'PUT', body: JSON.stringify({ spam: true }) })
    if (!res?.ok) { window._caduceus?.toast('Report spam failed.'); return }
    const updated = await res.json()
    this.$allEmails = this.$selectedFolder === 'spam'
      ? this.$allEmails.map(m => m.id === updated.id ? updated : m)
      : this.$allEmails.filter(m => m.id !== updated.id)
    window._caduceus?.toast('Reported spam.')
    window.dispatchEvent(new Event('caduceus:refresh-inbox'))
  }

  /**
   * Mark a spam email as not spam through the feedback endpoint.
   *
   * @async
   * @param {Event|string} eventOrId
   * @param {string|Event} [requestedIdOrEvent]
   * @returns {Promise<void>}
   */
  async markNotSpam(eventOrId, requestedIdOrEvent) {
    const id = typeof eventOrId === 'string' ? eventOrId : typeof requestedIdOrEvent === 'string' ? requestedIdOrEvent : ''
    const event = /** @type {Event | undefined} */ (typeof eventOrId === 'string' ? requestedIdOrEvent : eventOrId)
    event?.stopPropagation()
    if (!id) return
    const res = await this.apiFetch(`/inbox/${id}/spam`, { method: 'PUT', body: JSON.stringify({ spam: false }) })
    if (!res?.ok) { window._caduceus?.toast('Not spam failed.'); return }
    const updated = await res.json()
    this.$allEmails = this.$selectedFolder === 'spam'
      ? this.$allEmails.filter(m => m.id !== updated.id)
      : this.$allEmails.map(m => m.id === updated.id ? updated : m)
    window._caduceus?.toast('Moved to inbox.')
    window.dispatchEvent(new Event('caduceus:refresh-inbox'))
  }

  // ── Keyboard shortcuts ────────────────────────────────────────────

  /**
   * Bind global keyboard-shortcut listeners for inbox navigation and actions.
   *
   * Shortcuts:
   * - `navigation:next-email` / `navigation:prev-email` — move selection
   * - `navigation:scroll-down` / `navigation:scroll-up` — scroll the list
   * - `email:star`, `email:archive`, `email:trash`, `email:report-spam` — quick actions on selected
   * - `email:mark-read`, `email:mark-unread` — toggle read status
   * - `email:open` — open the selected email
   * - `core:select-all` / `core:deselect-all` — bulk selection helpers
   *
   * @returns {void}
   */
  bindShortcuts() {
    this._shortcuts = {
      'navigation:next-email': () => this._moveSelection(1),
      'navigation:prev-email': () => this._moveSelection(-1),
      'navigation:scroll-down': () => this._scrollList(200),
      'navigation:scroll-up': () => this._scrollList(-200),
      'email:star': () => this._shortcutAction('star'),
      'email:archive': () => this._shortcutAction('archive'),
      'email:trash': () => this._shortcutAction('trash'),
      'email:report-spam': () => this._shortcutAction('report-spam'),
      'email:mark-read': () => this._shortcutAction('mark-read'),
      'email:mark-unread': () => this._shortcutAction('mark-unread'),
      'email:open': () => { if (this.$selectedId) window._caduceus?.openEmailId(this.$selectedId) },
      'core:select-all': () => this._selectAll(),
      'core:deselect-all': () => { this.$selectedId = ''; this.clearBulkSelection() },
    }

    for (const [name, handler] of Object.entries(this._shortcuts)) {
      window.addEventListener('caduceus:shortcut:' + name, handler)
    }
  }

  /**
   * Move the selection highlight up or down in the email list.
   *
   * Automatically scrolls the newly selected row into view.
   *
   * @param {number} direction - +1 for next, -1 for previous
   * @returns {void}
   */
  _moveSelection(direction) {
    const list = this.emails
    if (list.length === 0) return
    const idx = list.findIndex(e => e.id === this.$selectedId)
    if (idx < 0) {
      this.selectEmail(list[0].id)
    } else {
      const next = Math.min(Math.max(0, idx + direction), list.length - 1)
      this.selectEmail(list[next].id)
    }
    // Scroll the selected row into view
    const row = document.querySelector('.email-row.selected')
    if (row) row.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }

  /**
   * Scroll the email list container by a given pixel amount.
   *
   * Falls back to `window.scrollBy` if the `.inbox-list` element is not found.
   *
   * @param {number} amount - Pixels to scroll (positive = down, negative = up)
   * @returns {void}
   */
  _scrollList(amount) {
    const list = document.querySelector('.inbox-list')
    if (list) list.scrollBy({ top: amount, behavior: 'smooth' })
    else window.scrollBy({ top: amount, behavior: 'smooth' })
  }

  /**
   * Perform a shortcut-triggered action on the currently selected email.
   *
   * @async
   * @param {'star'|'archive'|'trash'|'report-spam'|'mark-read'|'mark-unread'} type - The action to perform
   * @returns {Promise<void>}
   */
  async _shortcutAction(type) {
    const id = this.$selectedId
    if (!id) return
    const email = this.$allEmails.find(e => e.id === id)
    if (!email) return

    switch (type) {
      case 'star':
        await this.quickStar(id)
        break
      case 'archive':
        await this.quickArchive(id)
        this.$selectedId = ''
        break
      case 'trash':
        await this.quickTrash(id)
        this.$selectedId = ''
        break
      case 'report-spam':
        await this.reportSpam(id)
        this.$selectedId = ''
        break
      case 'mark-read':
        await this.applyMailboxAction([id], { read: true }, 'Marked as read.')
        break
      case 'mark-unread':
        await this.applyMailboxAction([id], { read: false }, 'Marked as unread.')
        break
    }
  }

  /**
   * Select every message represented by the current filtered view.
   *
   * @returns {void}
   */
  _selectAll() {
    this.selectAllVisible()
  }
}

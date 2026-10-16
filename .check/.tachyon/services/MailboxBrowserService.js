import { __tachyon_create_behaviors__ } from '../behavior-runtime.js';
/** @typedef {'automatic'|'light'|'dark'|'darkside'|'less-is-more'|'taiga'|'ubuntu'} ActiveThemeId */
       {}

/**
 * @typedef {Object} AddressReplaced
 * @property {string} target
 */
       {}

/**
 * @typedef {Object} AppBadgeChanged
 * @property {number} count
 */
       {}

/**
 * @typedef {Object} BackdropClicked
 * @property {string} dialog
 */
       {}

  /**
   * One registered binding.
   *
   * @typedef {object} Binding
   * @property {string} originalSpec
   * @property {string[]} chords        Canonicalised chord sequence
   * @property {(ev: KeyboardEvent) => (boolean | void)} callback
   */
       {}

/**
 * @typedef {Object} BrowserFeatures
 * @property {boolean} passkeys
 * @property {boolean} serviceWorker
 * @property {boolean} launchQueue
 * @property {boolean} appBadge
 * @property {boolean} notifications
 * @property {boolean} pushDisabled
 */
       {}

/**
 * @typedef {object} CachedMessageTranslation
 * @property {string} messageId
 * @property {string} sourceLanguage
 * @property {string} targetLanguage
 * @property {string} htmlBody
 * @property {string} plainText
 * @property {boolean} enabled
 */
       {}

  /**
   * Public API mirrored on `window._caduceusShortcuts`.
   *
   * @typedef {object} CaduceusShortcuts
   * @property {(spec: string, cb: (ev: KeyboardEvent) => boolean | void) => void} bind
   * @property {(spec: string) => void} unbind
   * @property {() => void} reset
   */
       {}

/**
 * @typedef {Object} CategoryPickerClosed
 * @property {string} mode
 */
       {}

/**
 * @typedef {Object} CategoryPickerDismissed
 * @property {string} key
 */
       {}

/**
 * @typedef {Object} CategoryPickerOpened
 * @property {string} mode
 */
       {}

/**
 * @typedef {Object} ClickRequested
 * @property {string} selector
 */
       {}

/**
 * @typedef {object} ComposePrefill
 * @property {string} [to] - Pre-fill the To field
 * @property {string} [cc] - Pre-fill the CC field
 * @property {string} [bcc] - Pre-fill the BCC field
 * @property {string} [replyTo] - Pre-fill the Reply-To field
 * @property {string} [from] - Pre-select an owned From address
 * @property {string} [subject] - Pre-fill the Subject field
 * @property {string} [body] - Pre-fill the message body
 * @property {string} [inReplyTo] - Canonical Message-ID being answered
 * @property {string[]} [references] - RFC reference chain for conversation threading
 * @property {string} [scheduledId] - Existing scheduled message being edited
 * @property {string} [sendAt] - Existing schedule as an ISO timestamp
 * @property {Array<any>} [attachments] - Bucket-backed attachments loaded for editing
 * @property {string} [sourceEmailId] - Authorized source conversation for Send & Archive
 */
       {}

/** @typedef {'interface'|InterfaceFontId} ComposerFontId */
       {}

/**
 * @typedef {Object} ConfirmationGiven
 * @property {string} action
 * @property {string} id
 */
       {}

/** @typedef {{ name: string, givenName: string, familyName: string, company: string, title: string, birthday: string, notes: string, nicknames: string, emails: EditableValue[], phones: EditableValue[], urls: EditableValue[], addresses: EditableAddress[], groupIds: string[] }} ContactForm */
       {}

/**
 * Escape closed the row menu.
 *
 * @typedef {Object} ContextMenuDismissed
 * @property {string} key
 */
       {}

/**
 * The row menu opened; the browser side moves the focus into it.
 *
 * @typedef {Object} ContextMenuOpened
 * @property {number} count
 */
       {}

/**
 * A right click on a row, at a viewport position already kept inside the window.
 *
 * @typedef {Object} ContextMenuRequested
 * @property {string} emailId
 * @property {string} threadId
 * @property {number} x
 * @property {number} y
 */
       {}

/** @typedef {'dark'|'darkside'} DarkThemeId */
       {}

/**
 * @typedef {Object} DocumentAttributesChanged
 * @property {string} owner
 * @property {Object<string, string>} attributes
 */
       {}

/** @typedef {{ key: string, type: string, streetAddress: string, extendedAddress: string, city: string, region: string, postalCode: string, country: string }} EditableAddress */
       {}

/** @typedef {{ key: string, value: string, type: string }} EditableValue */
       {}

/**
 * A message summary as the list routes return it.
 *
 * @typedef {Object} Email
 * @property {string} [id]
 * @property {string} [domain]
 * @property {string} [folder]
 * @property {string} [systemFolder]
 * @property {string} [sender]
 * @property {string} [subject]
 * @property {string} [snippet]
 * @property {boolean} [read]
 * @property {boolean} [starred]
 * @property {?string[]} [labels]
 * @property {string} [receivedAt]
 * @property {boolean} [hasAttachments]
 * @property {?number} [spamScore]
 * @property {?string[]} [spamReasons]
 * @property {string} [snoozedUntil]
 * @property {string} [providerAccountId]
 * @property {string} [threadId]
 * @property {*} [to]
 * @property {*} [cc]
 */
       {}

/**
 * @typedef {object} EmailDetailProps
 * @property {string} [emailId] - ID of the email to load (parent prop)
 * @property {string} [email-id] - Kebab-case property emitted by Tachyon 26.33
 * @property {string} [emailid] - Alternate casing for email ID
 * @property {Record<string, any>} [email] - Pre-loaded email object (avoids a fetch)
 * @property {() => void} [onBack] - Callback invoked when the user clicks back
 * @property {boolean} [standalone] - Whether this reader owns the full route
 */
       {}

/**
 * @typedef {Object} EscapePressed
 * @property {string} inside
 */
       {}

/**
 * @typedef {Object} ExportCancelRequested
 * @property {string} reason
 */
       {}

/**
 * How an export ended. Canceled and Failure are mutually exclusive.
 *
 * @typedef {Object} ExportFinished
 * @property {number} messages
 * @property {string} sourceBytesLabel
 * @property {boolean} canceled
 * @property {string} failure
 */
       {}

/**
 * @typedef {Object} ExportProgressed
 * @property {number} messages
 * @property {string} bytesLabel
 */
       {}

/**
 * Stream the folder into an mbox file or an archive of EML files.
 *
 * @typedef {Object} ExportRequested
 * @property {string} format
 * @property {string} folder
 * @property {string} filenameDomain
 * @property {ExportScope[]} scopes
 * @property {string} authorization
 */
       {}

/**
 * One export source: a mailbox domain or a connected account.
 *
 * @typedef {Object} ExportScope
 * @property {string} domain
 * @property {string} account
 */
       {}

/**
 * @typedef {Object} ExternalAccount
 * @property {string} id
 * @property {string} label
 * @property {string} mailboxAddress
 */
       {}

/**
 * @typedef {Object} ExternalFolder
 * @property {string} name
 * @property {string} role
 */
       {}

/**
 * @typedef {Object} FocusRequested
 * @property {string} selector
 * @property {boolean} remember
 * @property {boolean} restore
 */
       {}

/**
 * @typedef {Object} FolderDropped
 * @property {string} folder
 * @property {string} domain
 * @property {string[]} ids
 */
       {}

/**
 * @typedef {Object} FormSubmitted
 * @property {string} form
 */
       {}

/**
 * @typedef {Object} ImportCancelRequested
 * @property {string} reason
 */
       {}

/**
 * How an import ended.
 *
 * @typedef {Object} ImportFinished
 * @property {number} imported
 * @property {number} failures
 * @property {string} firstFailure
 * @property {boolean} canceled
 * @property {string} failure
 */
       {}

/**
 * Open the file picker for EML files to import into this mailbox and folder.
 *
 * @typedef {Object} ImportPickRequested
 * @property {string} mailbox
 * @property {string} folder
 * @property {string} authorization
 */
       {}

/**
 * @typedef {Object} ImportProgressed
 * @property {number} imported
 * @property {number} total
 * @property {string} current
 */
       {}

/**
 * Something changed the mailbox; the list loads again.
 *
 * @typedef {Object} InboxRefreshRequested
 * @property {string} reason
 */
       {}

/** @typedef {'plex-sans'|'system'|'humanist'|'serif'|'monospace'} InterfaceFontId */
       {}

/**
 * @typedef {'auto'|'en'|'de'|'es'|'es-419'|'fr'|'ko'|'pl'|'ja'|'zh-CN'|'zh-TW'|'pt'|'pt-BR'|'gl'|'tr'|'ru'|'cs'|'uk'|'hu'|'he'|'ar'} InterfaceLocaleChoice
 * @typedef {Exclude<InterfaceLocaleChoice, 'auto'>} ResolvedInterfaceLocale
 */
       {}

/**
 * @typedef {Object} KeyboardOverridesChanged
 * @property {Object<string, string>} overrides
 */
       {}

/** @typedef {'light'|'less-is-more'|'taiga'|'ubuntu'} LightThemeId */
       {}

/**
 * A shortcut scrolls the list by this many pixels.
 *
 * @typedef {Object} ListScrollRequested
 * @property {number} amount
 */
       {}

/**
 * @typedef {object} LoginFormProps
 * @property {string} [brandlogo] - Brand asset passed by the application shell
 */
       {}

/**
 * @typedef {Object} LoginProgressed
 * @property {string} step
 * @property {string} error
 */
       {}

/**
 * Backspace or a touch swipe on a row asks to move messages to a folder.
 *
 * @typedef {Object} MailGesture
 * @property {string} kind
 * @property {string} emailId
 * @property {string} folder
 */
       {}

/**
 * @typedef {object} MailPreferences
 * @property {string} workspaceLayout
 * @property {boolean} use24HourClock
 * @property {number} interfaceScale
 * @property {boolean} showUnreadForAllFolders
 * @property {number} markAsReadDelay
 * @property {boolean} detailedHeaders
 * @property {boolean} detailedNames
 * @property {boolean} restrictMessageWidth
 * @property {string} backspaceAction
 * @property {boolean} swipeEnabled
 * @property {string} conversationOrder
 * @property {string} emailRenderMode
 * @property {boolean} richText
 * @property {boolean} spellcheck
 * @property {string} spellcheckLanguage
 * @property {string} defaultSendAction
 * @property {string} defaultReplyAction
 * @property {number} undoSendDelay
 * @property {string} defaultFrom
 * @property {Record<string, string[]>} autoCcBySender
 * @property {Record<string, string[]>} autoBccBySender
 * @property {boolean} displayAttachmentPreviews
 * @property {boolean} unsnoozeToTop
 * @property {string[]} findInMailDisabledDomains
 */
       {}

/**
 * A row was clicked, or Enter or Space pressed on it.
 *
 * @typedef {Object} MailRowActivated
 * @property {string} emailId
 * @property {string} threadId
 * @property {boolean} narrow
 */
       {}

/**
 * The keyboard moved the list focus to a row.
 *
 * @typedef {Object} MailRowFocused
 * @property {string} id
 */
       {}

/**
 * @typedef {Object} MailSelected
 * @property {string} emailId
 */
       {}

/**
 * @typedef {Object} MailStreamHealth
 * @property {boolean} healthy
 */
       {}

/**
 * Loaded message summaries for the encrypted offline index, in slices under 64 KiB.
 *
 * @typedef {Object} MailboxSummaries
 * @property {string} owner
 * @property {Email[]} emails
 */
       {}

/** @typedef {string|Readonly<Record<string,string>>} Message */
       {}

/**
 * @typedef {Object} NavigationRequested
 * @property {string} target
 * @property {boolean} replace
 * @property {boolean} reload
 */
       {}

/**
 * The new-mail banner was clicked; the list scrolls to the top.
 *
 * @typedef {Object} NewMailShown
 * @property {number} count
 */
       {}

/**
 * @typedef {Object} NotificationActionReceived
 * @property {string} action
 * @property {string} emailId
 */
       {}

/**
 * Offline index results, in slices; Done marks the last one.
 *
 * @typedef {Object} OfflineSearchFinished
 * @property {number} request
 * @property {Email[]} emails
 * @property {boolean} done
 */
       {}

/**
 * The server could not answer; search the offline index instead.
 *
 * @typedef {Object} OfflineSearchRequested
 * @property {number} request
 * @property {string} owner
 * @property {string} query
 * @property {string} folder
 * @property {string} domain
 * @property {string} account
 * @property {string} read
 * @property {string} starred
 * @property {number} limit
 */
       {}

/**
 * @typedef {object} OfflineWriteInput
 * @property {'mail-action'|'draft'|'send'|'snooze'|'calendar'} kind
 * @property {string} owner
 * @property {string} path
 * @property {'POST'|'PUT'|'DELETE'} method
 * @property {Record<string,unknown>|null} [body]
 * @property {string} [message]
 * @property {string} [id]
 * @property {string} [coalesceKey]
 * @property {Record<string,unknown>} [metadata]
 */
       {}

/**
 * @typedef {Object} PageClicked
 * @property {boolean} insideUserMenu
 */
       {}

/**
 * @typedef {Object} PageLocation
 * @property {string} path
 * @property {string} search
 * @property {string} hash
 */
       {}

/**
 * @typedef {Object} PasskeyAnswered
 * @property {string} mode
 * @property {string} credentialId
 * @property {string} authenticatorData
 * @property {string} clientDataJSON
 * @property {string} signature
 * @property {string} attestationObject
 * @property {string} failure
 */
       {}

/**
 * @typedef {Object} PasskeyRequested
 * @property {string} mode
 * @property {string} challenge
 * @property {string} relyingPartyId
 * @property {string[]} credentialIds
 * @property {string} userId
 * @property {string} userName
 */
       {}

/**
 * @typedef {object} PrintableConversation
 * @property {string} subject
 * @property {number} count
 * @property {PrintableEmail} latestEmail
 * @property {PrintableEmail[]} emails
 */
       {}

/** @typedef {Record<string, any>} PrintableEmail */
       {}

/** @typedef {{ accountId: string, addressBookId: string, name: string, accountLabel: string, mailboxAddress: string }} RemoteAddressBook */
       {}

/**
 * @typedef {Object} SearchBlurred
 * @property {string} value
 */
       {}

/**
 * @typedef {Object} SearchKeyPressed
 * @property {string} key
 * @property {string} value
 * @property {number} caret
 */
       {}

/**
 * @typedef {Object} SearchSuggestionChosen
 * @property {number} index
 * @property {string} value
 * @property {number} caret
 */
       {}

/**
 * @typedef {Object} SearchSuggestionsWanted
 * @property {string} value
 * @property {number} caret
 */
       {}

/**
 * A shortcut moved the selected row; the list scrolls it into view.
 *
 * @typedef {Object} SelectionMoved
 * @property {string} emailId
 */
       {}

/**
 * Sign-in state that the shell publishes (retained) whenever it changes.
 *
 * @typedef {Object} SessionChanged
 * @property {string} token
 * @property {string} email
 * @property {string} role
 * @property {string[]} domains
 * @property {Record<string, string>} mailboxNames
 * @property {ExternalAccount[]} externalAccounts
 * @property {Record<string, ExternalFolder[]>} externalFolders
 */
       {}

/**
 * @typedef {Object} ShortcutGroup
 * @property {string} label
 * @property {ShortcutRow[]} rows
 */
       {}

/**
 * @typedef {Object} ShortcutGroupsListed
 * @property {ShortcutGroup[]} groups
 */
       {}

/**
 * @typedef {Object} ShortcutPressed
 * @property {string} command
 */
       {}

/**
 * @typedef {Object} ShortcutRow
 * @property {string} label
 * @property {string} displayBinding
 */
       {}

/**
 * @typedef {Object} SignOutRequested
 * @property {string} reason
 */
       {}

/** @typedef {'new-mail'|'hit-send'|'send'|'confirm'} SoundEventId */
       {}

/** @typedef {'none'|'chime'|'bell'|'pop'|'pulse'} SoundId */
       {}

/**
 * @typedef {Object} StorageWritten
 * @property {string} area
 * @property {Object<string, string>} values
 * @property {string[]} removed
 */
       {}

/**
 * @typedef {Object} StoredValues
 * @property {Object<string, string>} session
 * @property {Object<string, string>} local
 */
       {}

/**
 * @typedef {Object} TimerElapsed
 * @property {string} name
 */
       {}

/**
 * @typedef {Object} TimerRequested
 * @property {string} name
 * @property {number} delayMs
 * @property {boolean} repeat
 */
       {}

/** @typedef {'offer'|'always'|'never'} TranslationPreference */
       {}

/**
 * @typedef {Object} UpdateReady
 * @property {string} version
 */
       {}

/**
 * @typedef {Object} ViewSettled
 * @property {string} source
 */
       {}


import {
  focusedElement,
  menuKey,
  menuPosition,
  pickerKey,
  scrollList,
  searchKey,
  startDrag,
  suggestionPressed,
  watchRows,
  watchSearch,
  whenRendered,
} from '../../shared/scripts/mailbox-browser.js'

/**
 * The browser side of the mailbox list. The list itself (InboxService,
 * ThreadSearchService, CategoryPickerService) is Go and runs as WebAssembly,
 * so it never receives a DOM event. This service does what needs one —
 * preventDefault, capture-phase row activation, focus, scrolling, drag data
 * and the search box's keys — and tells the Go services through typed events.
 * The address, navigation, storage and toasts are the shell's bridges
 * (BrowserPageService, BrowserStorageService, ShellService).
 */
        
export default class MailboxBrowserService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher
  /** @type {AbortSignal} */
        
  lifecycle
  /** @type {HTMLElement | null} */
  _returnFocus = null

          
  start() {
    watchRows(this.lifecycle, {
      activate: (emailId, threadId, narrow) => this._activated(emailId, threadId, narrow),
      focus: (id) => this._focused(id),
      gesture: (kind, emailId, folder) => this._gesture(kind, emailId, folder),
    })
    watchSearch(this.lifecycle, {
      wanted: (value, caret) => this._wanted(value, caret),
      blurred: () => this._blurred(),
    })
  }

  /** @param {MouseEvent} event @param {string} emailId @param {string} threadId */
  openMenu(event, emailId, threadId) {
    /** @type {ContextMenuRequested} */
    const requested = { emailId, threadId, ...menuPosition(event) }
    this.applicationEventPublisher.publishEvent("ContextMenuRequested", requested, false)
  }

  /** @param {DragEvent} event @param {string} emailId @param {string[] | null} selected */
  dragStart(event, emailId, selected) {
    startDrag(event, emailId, selected)
  }

  /** Keep a click inside the menu or the picker from closing it. @param {Event} event */
  stop(event) {
    event.stopPropagation()
  }

  /** @param {KeyboardEvent} event */
  menuKeydown(event) {
    menuKey(event, () => this._menuDismissed())
  }

  /** @param {KeyboardEvent} event */
  pickerKeydown(event) {
    pickerKey(event, () => this._pickerDismissed())
  }

  /** @param {KeyboardEvent} event */
  searchKeydown(event) {
    searchKey(event, (key, value, caret) => this._searchKey(key, value, caret))
  }

  /** @param {MouseEvent} event @param {number} index */
  chooseSuggestion(event, index) {
    suggestionPressed(event, (value, caret) => this._chosen(index, value, caret))
  }

  /** @param {ContextMenuOpened} _opened */
          
  contextMenuOpened(_opened) {
    whenRendered('.inbox-context-menu [role="menuitem"]', (item) => item.focus())
  }

  /** @param {CategoryPickerOpened} _opened */
          
  categoryPickerOpened(_opened) {
    this._returnFocus = focusedElement()
    whenRendered('.category-picker-search', (search) => search.focus())
  }

  /** @param {CategoryPickerClosed} _closed */
          
  categoryPickerClosed(_closed) {
    const target = this._returnFocus
    this._returnFocus = null
    setTimeout(() => target?.focus(), 0)
  }

  /** @param {NewMailShown} _shown */
          
  newMailShown(_shown) {
    scrollList(0)
  }

  /** @param {SelectionMoved} moved */
          
  selectionMoved(moved) {
    whenRendered(`.email-row.selected[data-email-id="${CSS.escape(moved.emailId)}"]`, (row) => row.scrollIntoView({ block: 'nearest', behavior: 'smooth' }))
  }

  /** @param {ListScrollRequested} requested */
          
  listScrollRequested(requested) {
    scrollList(0, requested.amount)
  }

  /** @param {string} emailId @param {string} threadId @param {boolean} narrow */
  _activated(emailId, threadId, narrow) {
    /** @type {MailRowActivated} */
    const activated = { emailId, threadId, narrow }
    this.applicationEventPublisher.publishEvent("MailRowActivated", activated, false)
  }

  /** @param {string} id */
  _focused(id) {
    /** @type {MailRowFocused} */
    const focused = { id }
    this.applicationEventPublisher.publishEvent("MailRowFocused", focused, false)
  }

  /** @param {string} kind @param {string} emailId @param {string} folder */
  _gesture(kind, emailId, folder) {
    /** @type {MailGesture} */
    const gesture = { kind, emailId, folder }
    this.applicationEventPublisher.publishEvent("MailGesture", gesture, false)
  }

  /** @param {string} value @param {number} caret */
  _wanted(value, caret) {
    /** @type {SearchSuggestionsWanted} */
    const wanted = { value, caret }
    this.applicationEventPublisher.publishEvent("SearchSuggestionsWanted", wanted, false)
  }

  _blurred() {
    /** @type {SearchBlurred} */
    const blurred = { value: '' }
    this.applicationEventPublisher.publishEvent("SearchBlurred", blurred, false)
  }

  /** @param {string} key @param {string} value @param {number} caret */
  _searchKey(key, value, caret) {
    /** @type {SearchKeyPressed} */
    const pressed = { key, value, caret }
    this.applicationEventPublisher.publishEvent("SearchKeyPressed", pressed, false)
  }

  /** @param {number} index @param {string} value @param {number} caret */
  _chosen(index, value, caret) {
    /** @type {SearchSuggestionChosen} */
    const chosen = { index, value, caret }
    this.applicationEventPublisher.publishEvent("SearchSuggestionChosen", chosen, false)
  }

  _menuDismissed() {
    /** @type {ContextMenuDismissed} */
    const dismissed = { key: 'Escape' }
    this.applicationEventPublisher.publishEvent("ContextMenuDismissed", dismissed, false)
  }

  _pickerDismissed() {
    /** @type {CategoryPickerDismissed} */
    const dismissed = { key: 'Escape' }
    this.applicationEventPublisher.publishEvent("CategoryPickerDismissed", dismissed, false)
  }




}

const Behaviors = __tachyon_create_behaviors__([{type:MailboxBrowserService,"cases":[],"configuration":false,"methods":[],"name":"MailboxBrowserService","service":true}], "tac", "aaf9cfd8311124ef840964427153b0a7c078777088dba04bdb395630e1bf1f04");


export const __tachyon_event_plan__ = {"types":[{"listeners":[{"asynchronous":false,"condition":null,"events":["ContextMenuOpened"],"id":null,"method":"contextMenuOpened","models":{"ContextMenuOpened":{"fields":{"count":"!number"},"open":false}},"order":0,"replay":false},{"asynchronous":false,"condition":null,"events":["CategoryPickerOpened"],"id":null,"method":"categoryPickerOpened","models":{"CategoryPickerOpened":{"fields":{"mode":"!string"},"open":false}},"order":0,"replay":false},{"asynchronous":false,"condition":null,"events":["CategoryPickerClosed"],"id":null,"method":"categoryPickerClosed","models":{"CategoryPickerClosed":{"fields":{"mode":"!string"},"open":false}},"order":0,"replay":false},{"asynchronous":false,"condition":null,"events":["NewMailShown"],"id":null,"method":"newMailShown","models":{"NewMailShown":{"fields":{"count":"!number"},"open":false}},"order":0,"replay":false},{"asynchronous":false,"condition":null,"events":["SelectionMoved"],"id":null,"method":"selectionMoved","models":{"SelectionMoved":{"fields":{"emailId":"!string"},"open":false}},"order":0,"replay":false},{"asynchronous":false,"condition":null,"events":["ListScrollRequested"],"id":null,"method":"listScrollRequested","models":{"ListScrollRequested":{"fields":{"amount":"!number"},"open":false}},"order":0,"replay":false}],"name":"MailboxBrowserService","post_construct":"start","pre_destroy":null,"resources":[{"field":"lifecycle","name":"lifecycle"},{"field":"applicationEventPublisher","name":"applicationEventPublisher"}]}]};

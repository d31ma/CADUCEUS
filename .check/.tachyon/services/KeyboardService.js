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


/**
 * Escape hatch: the bridge to the page-wide keyboard command runtime.
 *
 * Reading keystrokes, deciding the active view from what is rendered and
 * preventing the browser default must happen synchronously in the page, so
 * the runtime stays in shared/scripts/keyboard.js (loaded by
 * shared/scripts/page-setup.js). This service hands the shell's commands and
 * the command catalog to Go and gives the runtime the user's overrides.
 */
        
export default class KeyboardService {
  static _COMMANDS = [
    'core:compose', 'core:go-inbox', 'core:go-drafts', 'core:go-sent', 'core:go-archive', 'core:go-spam',
    'core:go-trash', 'core:go-settings', 'core:toggle-screenshot-mode', 'core:refresh', 'core:search', 'core:show-shortcuts',
    'navigation:next-email', 'navigation:prev-email', 'navigation:scroll-down', 'navigation:scroll-up',
    'email:star', 'email:archive', 'email:trash', 'email:report-spam', 'email:mark-read', 'email:mark-unread',
    'email:open', 'core:select-all', 'core:deselect-all',
  ]

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  ready = false

          
  start() {
    KeyboardService._COMMANDS.forEach(command => addEventListener(`caduceus:shortcut:${command}`, () => this._pressed(command)))
    addEventListener('caduceus:keyboard-shortcuts-changed', () => this._listGroups())
    void window._caduceusKeyboard?.init?.()?.then?.(() => this._listGroups())
    this.ready = true
  }

  /** @param {KeyboardOverridesChanged} changed */
                           
  overrides(changed) {
    void window._caduceusKeyboard?.applyOverrides?.(changed.overrides)
  }

  /** @param {string} command */
  _pressed(command) {
    /** @type {ShortcutPressed} */
    const pressed = { command }
    this.applicationEventPublisher.publishEvent("ShortcutPressed", pressed, false)
  }

  /** @param {Array<{label: string, rows: Array<{label: string, displayBinding: string}>}> | undefined} groups @returns {ShortcutGroup[]} */
  _groups(groups) {
    return (groups ?? []).map(group => ({ label: group.label, rows: group.rows.map(row => ({ label: row.label, displayBinding: row.displayBinding })) }))
  }

  _listGroups() {
    /** @type {ShortcutGroupsListed} */
    const listed = { groups: this._groups(window._caduceusKeyboard?.getGroups?.()) }
    this.applicationEventPublisher.publishEvent("ShortcutGroupsListed", listed, true)
  }
}

const Behaviors = __tachyon_create_behaviors__([{type:KeyboardService,"cases":[],"configuration":false,"methods":[],"name":"KeyboardService","service":true}], "tac", "a13153c291ae66bb72110c4fac5e8143d21d82f096f3243eea814398381c9b48");


export const __tachyon_event_plan__ = {"types":[{"listeners":[{"asynchronous":false,"condition":null,"events":["KeyboardOverridesChanged"],"id":null,"method":"overrides","models":{"KeyboardOverridesChanged":{"fields":{"overrides":"!any"},"open":false}},"order":0,"replay":true}],"name":"KeyboardService","post_construct":"start","pre_destroy":null,"resources":[{"field":"applicationEventPublisher","name":"applicationEventPublisher"}]}]};

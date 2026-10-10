import {
  focusedElement,
  mailboxAddress,
  menuKey,
  menuPosition,
  pickerKey,
  scrollList,
  searchKey,
  startDrag,
  suggestionPressed,
  watchRows,
  watchSearch,
  watchShell,
  whenRendered,
} from '/shared/scripts/mailbox-browser.js'

/**
 * The browser side of the mailbox list. The list itself (InboxService,
 * ThreadSearchService, CategoryPickerService) is Go and runs as WebAssembly,
 * so it never receives a DOM event. This service does what needs one —
 * preventDefault, focus, scrolling, drag data, the address query, the shell's
 * window events — and tells the Go services through typed events.
 */
@Service
export default class MailboxBrowserService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher
  /** @type {AbortSignal} */
  @Needs
  lifecycle
  /** @type {HTMLElement | null} */
  _returnFocus = null

  @OnStart
  start() {
    /** @type {MailboxOpened} */
    const opened = mailboxAddress()
    this.applicationEventPublisher.publishEvent(opened, true)
    watchRows(this.lifecycle, {
      activate: (emailId, threadId) => this._activated(emailId, threadId),
      focus: (id) => this._focused(id),
      gesture: (kind, emailId, folder) => this._gesture(kind, emailId, folder),
    })
    watchSearch(this.lifecycle, {
      wanted: (value, caret) => this._wanted(value, caret),
      blurred: () => this._blurred(),
    })
    watchShell(this.lifecycle, {
      newMail: (email) => this._newMail(email),
      refresh: () => this._refresh(),
      closed: () => this._closed(),
      shortcut: (name) => this._shortcut(name),
    })
  }

  /** @param {MouseEvent} event @param {string} emailId @param {string} threadId */
  openMenu(event, emailId, threadId) {
    /** @type {ContextMenuRequested} */
    const requested = { emailId, threadId, ...menuPosition(event) }
    this.applicationEventPublisher.publishEvent(requested, false)
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
  @OnEvent
  contextMenuOpened(_opened) {
    whenRendered('.inbox-context-menu [role="menuitem"]', (item) => item.focus())
  }

  /** @param {CategoryPickerOpened} _opened */
  @OnEvent
  categoryPickerOpened(_opened) {
    this._returnFocus = focusedElement()
    whenRendered('.category-picker-search', (search) => search.focus())
  }

  /** @param {CategoryPickerClosed} _closed */
  @OnEvent
  categoryPickerClosed(_closed) {
    const target = this._returnFocus
    this._returnFocus = null
    setTimeout(() => target?.focus(), 0)
  }

  /** @param {NewMailShown} _shown */
  @OnEvent
  newMailShown(_shown) {
    scrollList(0)
  }

  /** @param {SelectionMoved} moved */
  @OnEvent
  selectionMoved(moved) {
    whenRendered(`.email-row.selected[data-email-id="${CSS.escape(moved.emailId)}"]`, (row) => row.scrollIntoView({ block: 'nearest', behavior: 'smooth' }))
  }

  /** @param {ListScrollRequested} requested */
  @OnEvent
  listScrollRequested(requested) {
    scrollList(0, requested.amount)
  }

  /** @param {string} emailId @param {string} threadId */
  _activated(emailId, threadId) {
    /** @type {MailRowActivated} */
    const activated = { emailId, threadId }
    this.applicationEventPublisher.publishEvent(activated, false)
  }

  /** @param {string} id */
  _focused(id) {
    /** @type {MailRowFocused} */
    const focused = { id }
    this.applicationEventPublisher.publishEvent(focused, false)
  }

  /** @param {string} kind @param {string} emailId @param {string} folder */
  _gesture(kind, emailId, folder) {
    /** @type {MailGesture} */
    const gesture = { kind, emailId, folder }
    this.applicationEventPublisher.publishEvent(gesture, false)
  }

  /** @param {string} value @param {number} caret */
  _wanted(value, caret) {
    /** @type {SearchSuggestionsWanted} */
    const wanted = { value, caret }
    this.applicationEventPublisher.publishEvent(wanted, false)
  }

  _blurred() {
    /** @type {SearchBlurred} */
    const blurred = { value: '' }
    this.applicationEventPublisher.publishEvent(blurred, false)
  }

  /** @param {string} key @param {string} value @param {number} caret */
  _searchKey(key, value, caret) {
    /** @type {SearchKeyPressed} */
    const pressed = { key, value, caret }
    this.applicationEventPublisher.publishEvent(pressed, false)
  }

  /** @param {number} index @param {string} value @param {number} caret */
  _chosen(index, value, caret) {
    /** @type {SearchSuggestionChosen} */
    const chosen = { index, value, caret }
    this.applicationEventPublisher.publishEvent(chosen, false)
  }

  _menuDismissed() {
    /** @type {ContextMenuDismissed} */
    const dismissed = { key: 'Escape' }
    this.applicationEventPublisher.publishEvent(dismissed, false)
  }

  _pickerDismissed() {
    /** @type {CategoryPickerDismissed} */
    const dismissed = { key: 'Escape' }
    this.applicationEventPublisher.publishEvent(dismissed, false)
  }

  /** @param {Record<string, unknown>} email */
  _newMail(email) {
    /** @type {NewMailArrived} */
    const arrived = { email: /** @type {Email} */ (/** @type {unknown} */ (email)) }
    this.applicationEventPublisher.publishEvent(arrived, false)
  }

  _refresh() {
    /** @type {InboxRefreshRequested} */
    const refresh = { reason: 'shell' }
    this.applicationEventPublisher.publishEvent(refresh, false)
  }

  _closed() {
    /** @type {MailClosed} */
    const closed = { emailId: '' }
    this.applicationEventPublisher.publishEvent(closed, false)
  }

  /** @param {string} name */
  _shortcut(name) {
    /** @type {MailShortcut} */
    const shortcut = { name }
    this.applicationEventPublisher.publishEvent(shortcut, false)
  }
}

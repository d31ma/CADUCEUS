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
} from '/shared/scripts/mailbox-browser.js'

/**
 * The browser side of the mailbox list. The list itself (InboxService,
 * ThreadSearchService, CategoryPickerService) is Go and runs as WebAssembly,
 * so it never receives a DOM event. This service does what needs one —
 * preventDefault, capture-phase row activation, focus, scrolling, drag data
 * and the search box's keys — and tells the Go services through typed events.
 * The address, navigation, storage and toasts are the shell's bridges
 * (BrowserPageService, BrowserStorageService, ShellService).
 */
@Service
export default class MailboxBrowserService {
  /** @type {ApplicationEventPublisher | undefined} Injected before lifecycle methods run. */
  applicationEventPublisher
  /** @type {AbortSignal | undefined} Injected before lifecycle methods run. */
  @Needs
  lifecycle
  /** @type {HTMLElement | null} */
  _returnFocus = null

  @OnStart
  start() {
    watchRows(/** @type {AbortSignal} */ (this.lifecycle), {
      activate: (emailId, threadId, narrow) => this._activated(emailId, threadId, narrow),
      focus: (id) => this._focused(id),
      gesture: (kind, emailId, folder) => this._gesture(kind, emailId, folder),
    })
    watchSearch(/** @type {AbortSignal} */ (this.lifecycle), {
      wanted: (value, caret) => this._wanted(value, caret),
      blurred: () => this._blurred(),
    })
  }

  /** @param {MouseEvent} event @param {string} emailId @param {string} threadId */
  openMenu(event, emailId, threadId) {
    /** @type {ContextMenuRequested} */
    const requested = { emailId, threadId, ...menuPosition(event) }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(requested, false)
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

  /** @param {string} emailId @param {string} threadId @param {boolean} narrow */
  _activated(emailId, threadId, narrow) {
    /** @type {MailRowActivated} */
    const activated = { emailId, threadId, narrow }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(activated, false)
  }

  /** @param {string} id */
  _focused(id) {
    /** @type {MailRowFocused} */
    const focused = { id }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(focused, false)
  }

  /** @param {string} kind @param {string} emailId @param {string} folder */
  _gesture(kind, emailId, folder) {
    /** @type {MailGesture} */
    const gesture = { kind, emailId, folder }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(gesture, false)
  }

  /** @param {string} value @param {number} caret */
  _wanted(value, caret) {
    /** @type {SearchSuggestionsWanted} */
    const wanted = { value, caret }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(wanted, false)
  }

  _blurred() {
    /** @type {SearchBlurred} */
    const blurred = { value: '' }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(blurred, false)
  }

  /** @param {string} key @param {string} value @param {number} caret */
  _searchKey(key, value, caret) {
    /** @type {SearchKeyPressed} */
    const pressed = { key, value, caret }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(pressed, false)
  }

  /** @param {number} index @param {string} value @param {number} caret */
  _chosen(index, value, caret) {
    /** @type {SearchSuggestionChosen} */
    const chosen = { index, value, caret }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(chosen, false)
  }

  _menuDismissed() {
    /** @type {ContextMenuDismissed} */
    const dismissed = { key: 'Escape' }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(dismissed, false)
  }

  _pickerDismissed() {
    /** @type {CategoryPickerDismissed} */
    const dismissed = { key: 'Escape' }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(dismissed, false)
  }




}

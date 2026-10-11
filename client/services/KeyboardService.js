/**
 * Escape hatch: the bridge to the page-wide keyboard command runtime.
 *
 * Reading keystrokes, deciding the active view from what is rendered and
 * preventing the browser default must happen synchronously in the page, so
 * the runtime stays in shared/scripts/keyboard.js (loaded by
 * shared/scripts/page-setup.js). This service hands the shell's commands and
 * the command catalog to Go and gives the runtime the user's overrides.
 */
@Service
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

  @OnStart
  start() {
    KeyboardService._COMMANDS.forEach(command => addEventListener(`caduceus:shortcut:${command}`, () => this._pressed(command)))
    addEventListener('caduceus:keyboard-shortcuts-changed', () => this._listGroups())
    void window._caduceusKeyboard?.init?.()?.then?.(() => this._listGroups())
    this.ready = true
  }

  /** @param {KeyboardOverridesChanged} changed */
  @OnEvent({"replay":true})
  overrides(changed) {
    void window._caduceusKeyboard?.applyOverrides?.(changed.overrides)
  }

  /** @param {string} command */
  _pressed(command) {
    /** @type {ShortcutPressed} */
    const pressed = { command }
    this.applicationEventPublisher.publishEvent(pressed, false)
  }

  /** @param {Array<{label: string, rows: Array<{label: string, displayBinding: string}>}> | undefined} groups @returns {ShortcutGroup[]} */
  _groups(groups) {
    return (groups ?? []).map(group => ({ label: group.label, rows: group.rows.map(row => ({ label: row.label, displayBinding: row.displayBinding })) }))
  }

  _listGroups() {
    /** @type {ShortcutGroupsListed} */
    const listed = { groups: this._groups(window._caduceusKeyboard?.getGroups?.()) }
    this.applicationEventPublisher.publishEvent(listed, true)
  }
}
